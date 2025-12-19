// Copyright (c) 2017-2025 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sdkbundle

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"daml.com/x/assistant/pkg/assembler"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	ociconsts "daml.com/x/assistant/pkg/oci"
	"daml.com/x/assistant/pkg/ociindex"
	"daml.com/x/assistant/pkg/ocipuller/localpuller"
	"daml.com/x/assistant/pkg/ocipusher/sdkmanifestpusher"
	"daml.com/x/assistant/pkg/schema"
	"daml.com/x/assistant/pkg/sdkinstall"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/simpleplatform"
	"daml.com/x/assistant/pkg/utils"
	"daml.com/x/assistant/pkg/utils/fileinfo"
	"github.com/goccy/go-yaml"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/samber/lo"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
)

// Bootstrap installs an SDK from a bundle
func Bootstrap(ctx context.Context, config *assistantconfig.Config, bundlePath string) error {
	return bootstrap(ctx, config, bundlePath, nil)
}

func bootstrap(ctx context.Context, config *assistantconfig.Config, bundlePath string, overridePlatform *simpleplatform.NonGeneric) error {
	assemblyPath := filepath.Join(bundlePath, "sdk-manifest.yaml")
	manifest, err := sdkmanifest.ReadSdkManifest(assemblyPath)
	if err != nil {
		return err
	}

	if manifest.Spec.Assistant == nil {
		return fmt.Errorf("assembly missing the assistant")
	}

	puller := localpuller.New(config, filepath.Join(bundlePath, "oci-registry"))

	a := assembler.New(config, puller)
	if overridePlatform != nil {
		a = assembler.NewWithOverriddenPlatform(config, puller, overridePlatform)
	}
	assemblyResult, err := a.Assemble(ctx, manifest)
	if err != nil {
		return err
	}

	sdkVersion := manifest.Spec.Version.Value()
	if _, err := sdkinstall.LinkAssistantIfNewerSdk(config, *assemblyResult.AssistantAbsolutePath, &sdkVersion); err != nil {
		return err
	}

	cachedAssemblyPath := filepath.Join(config.InstalledSdkManifestsPath, manifest.Spec.Edition.String(), manifest.Spec.Version.Value().String()+".yaml")
	if err := utils.CopyFile(assemblyPath, cachedAssemblyPath); err != nil {
		return err
	}

	// dpm-config.yaml
	dpmConfigPath := filepath.Join(config.DamlHomePath, assistantconfig.DpmConfigFileName)
	f, err := os.OpenFile(dpmConfigPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: will skip writing %q because it already exists from a previous dpm installation\n", dpmConfigPath)
			return nil
		}
		return err
	}
	defer func() { _ = f.Close() }()

	bytes, err := os.ReadFile(filepath.Join(bundlePath, assistantconfig.DpmConfigFileName))
	if err != nil {
		return err
	}
	_, err = f.Write(bytes)
	return err
}

func Create(ctx context.Context, client *assistantremote.Remote, publishConfig *PublishConfig, bundlePath, blobCache string) error {
	bundlePath, err := filepath.Abs(bundlePath)
	if err != nil {
		return err
	}
	if err := utils.EnsureDirs(bundlePath); err != nil {
		return err
	}

	manifests := publishConfig.AssemblyManifests(schema.ManifestMeta{
		APIVersion: sdkmanifest.SdkManifestAPIVersion,
		Kind:       sdkmanifest.SdkManifestKind,
	})

	for p, manifest := range manifests {
		if blobCache == "" {
			tmp, deleteFn, err := utils.MkdirTemp("", "")
			if err != nil {
				return err
			}
			defer func() { _ = deleteFn() }()
			blobCache = tmp
		}
		if err := utils.EnsureDirs(blobCache); err != nil {
			return err
		}

		output, err := createFromManifest(ctx, client, manifest, bundlePath, &p, blobCache)
		if err != nil {
			return err
		}
		fmt.Printf("Validating bundle for %q.\n", p.String())

		if err := validate(ctx, output, blobCache, &p, *publishConfig.Edition); err != nil {
			return fmt.Errorf("error validating bundle %q: %w", p.String(), err)
		}
	}
	return nil
}

func Publish(ctx context.Context, printer utils.RawPrinter, client *assistantremote.Remote, publishConfigPath, blobCache string, extraTags []string) error {
	tmpBundlePath, deleteFn, err := utils.MkdirTemp("", "")
	if err != nil {
		return err
	}
	defer func() { _ = deleteFn() }()

	publishConfig, err := ReadPublishConfig(publishConfigPath)
	if err != nil {
		return err
	}
	edition := *publishConfig.Edition
	sdkVersion := publishConfig.Version.Value()

	err = Create(ctx, client, publishConfig, tmpBundlePath, blobCache)
	if err != nil {
		return err
	}

	fmt.Println("Publishing assembly to registry...")
	manifests := lo.SliceToMap(publishConfig.Platforms, func(platform *simpleplatform.NonGeneric) (simpleplatform.NonGeneric, string) {
		return *platform, filepath.Join(tmpBundlePath, platformDir(platform.String()), "sdk-manifest.yaml")
	})
	assemblyPusher := sdkmanifestpusher.New(printer, &sdkmanifestpusher.PushArgs{
		Edition:     edition,
		Version:     &sdkVersion,
		Annotations: map[string]string{}, // TODO
		ExtraTags:   extraTags,
	})
	_, err = assemblyPusher.PushSdkManifest(ctx, client, manifests)
	if err != nil {
		return err
	}
	return nil
}

func validate(ctx context.Context, bundlePath, blobCache string, platform *simpleplatform.NonGeneric, edition sdkmanifest.Edition) error {
	tmp, deleteFn, err := utils.MkdirTemp("", "")
	if err != nil {
		return err
	}
	defer func() { _ = deleteFn() }()
	config, err := assistantconfig.GetWithCustomDamlHome(tmp)
	if err != nil {
		return err
	}
	config.Edition = assistantconfig.NewLazyEdition(edition)
	config.OciLayoutCache = blobCache
	if err := config.EnsureDirs(); err != nil {
		return err
	}
	return bootstrap(ctx, config, bundlePath, platform)
}

func platformDir(platform string) string {
	return strings.ReplaceAll(platform, "/", "-")
}

func createFromManifest(ctx context.Context, client *assistantremote.Remote, manifest *sdkmanifest.SdkManifest, bundlePath string, platform *simpleplatform.NonGeneric, blobCache string) (string, error) {
	platformStr := platform.String()
	platformBundlePath := filepath.Join(bundlePath, platformDir(platformStr))
	localRegistryPath := filepath.Join(platformBundlePath, "oci-registry")

	comps := lo.Values(manifest.Spec.Components)
	comps = append(comps, manifest.Spec.Assistant)
	for _, comp := range comps {
		repoName := ociconsts.ComponentRepoPrefix + comp.Name
		tag := assembler.ComputeTagOrDigest(comp)
		fmt.Printf("pulling component %s/%s:%s...\n", client.Registry, repoName, tag)
		desc, err := clone(ctx, client, localRegistryPath, repoName, tag, platform, blobCache)
		if err != nil {
			return "", fmt.Errorf("failed to pull component '%s:%s'. %w", repoName, tag, err)
		}

		// put a symlink to the assistant binary at known location in the bundle
		if comp == manifest.Spec.Assistant {
			imageManifestPath := filepath.Join(localRegistryPath, repoName, "blobs", "sha256", desc.Digest.Hex())
			if err := linkAssistant(platformBundlePath, imageManifestPath); err != nil {
				return "", err
			}
		}
	}

	fmt.Printf("Pulled all components for %q.\n", platformStr)

	fmt.Printf("Writing assembly manifest for platform %q\n", platform.String())
	bytes, err := yaml.Marshal(manifest)
	if err != nil {
		return "", err
	}
	err = os.WriteFile(filepath.Join(platformBundlePath, "sdk-manifest.yaml"), bytes, 0444)
	if err != nil {
		return "", err
	}
	fmt.Printf("\n%s\n", string(bytes))

	dpmConfigBytes, err := yaml.Marshal(assistantconfig.Config{
		Registry: client.Registry,
	})
	if err != nil {
		return "", err
	}
	err = os.WriteFile(filepath.Join(platformBundlePath, assistantconfig.DpmConfigFileName), dpmConfigBytes, os.FileMode(0644))
	if err != nil {
		return "", err
	}

	fmt.Printf("Bundle for %s created at %q.\n", platform.String(), platformBundlePath)
	return platformBundlePath, nil
}

func linkAssistant(dir, imageManifestPath string) error {
	bytes, err := os.ReadFile(imageManifestPath)
	if err != nil {
		return err
	}
	manifest := v1.Manifest{}
	if err := json.Unmarshal(bytes, &manifest); err != nil {
		return err
	}

	dpmBinLayer, ok := lo.Find(manifest.Layers, func(l v1.Descriptor) bool {
		filename, ok := l.Annotations[fileinfo.FileNameAnnotation]
		if !ok {
			slog.Warn("layer missing annotation", "annotation", fileinfo.FileNameAnnotation)
			return false
		}
		return filename == assembler.AssistantBinNameWindows || filename == assembler.AssistantBinNameUnix
	})

	if !ok {
		return fmt.Errorf("could not determine assistant binary's layer")
	}
	binBlobPath := filepath.Join(filepath.Dir(imageManifestPath), dpmBinLayer.Digest.Hex())

	// TODO figure out why running linked blob fails on windows, instead of this.
	// (seems windows isn't happy with the blob filename not having a .exe)
	if runtime.GOOS == "windows" {
		return utils.CopyFile(binBlobPath, filepath.Join(dir, "bin", dpmBinLayer.Annotations[fileinfo.FileNameAnnotation]))
	}

	// re-use the assistant linking functionality from `dpm install <version>`
	dummyConfig := &assistantconfig.Config{DamlHomePath: dir}
	p, err := sdkinstall.LinkAssistant(dummyConfig, binBlobPath)
	if err != nil {
		return err
	}

	renamed := filepath.Join(filepath.Dir(p), dpmBinLayer.Annotations[fileinfo.FileNameAnnotation])
	if err := os.Rename(p, renamed); err != nil {
		return err
	}

	return os.Chmod(binBlobPath, os.FileMode(0755))
}

// note: the oci-layout's index.json resulting from `oras push --oci-layout` includes a
// "annotations":{"org.opencontainers.image.ref.name":"<tag>"}
// which the ORAS_CACHE's oci-layout seems to not have.
//
// TODO this function can be DRY-ed a bit using pieces from other parts of the codebase
func clone(ctx context.Context, client *assistantremote.Remote, registryPath, repoName, tag string, platform *simpleplatform.NonGeneric, blobCache string) (*v1.Descriptor, error) {
	index, indexBytes, err := ociindex.FetchIndex(ctx, client, repoName, tag)
	if err != nil {
		return nil, err
	}

	descriptor, err := ociindex.FindTargetPlatform(index.Manifests, platform)
	if err != nil {
		return nil, err
	}

	layout, err := oci.New(filepath.Join(registryPath, repoName))
	if err != nil {
		return nil, err
	}

	repo, err := client.CachedRepo(repoName, blobCache)
	if err != nil {
		return nil, err
	}

	opts := oras.DefaultCopyOptions
	if descriptor.Platform != nil {
		opts.WithTargetPlatform(descriptor.Platform)
	}
	_, err = oras.Copy(ctx, repo, tag, layout, tag, opts)
	if err != nil {
		return nil, err
	}

	// push the index too
	_, err = oras.TagBytes(ctx, layout, v1.MediaTypeImageIndex, indexBytes, tag)
	if err != nil {
		return nil, err
	}
	return descriptor, nil
}
