// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package publish

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"daml.com/x/assistant/pkg/assembler"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/licenseutils"
	ociconsts "daml.com/x/assistant/pkg/oci"
	"daml.com/x/assistant/pkg/ociindex"
	"daml.com/x/assistant/pkg/ocilister"
	"daml.com/x/assistant/pkg/ocipusher"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/simpleplatform"
	"daml.com/x/assistant/pkg/utils"
	"github.com/Masterminds/semver/v3"
	"github.com/fatih/color"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/samber/lo"
	"oras.land/oras-go/v2/errdef"
)

type Config struct {
	Name                   string
	Platforms              map[simpleplatform.Platform]string
	Version                *semver.Version
	DryRun, IncludeGitInfo bool
	Annotations            map[string]string
	ExtraTags              []string

	Registry     string
	AuthFilePath string
	Insecure     bool
}

func (config *Config) RequiredAnnotations() ociconsts.DescriptorAnnotations {
	return ociconsts.DescriptorAnnotations{
		Name:    config.Name,
		Version: config.Version,
	}
}

func (config *Config) indexOpts(tag string, descriptors []v1.Descriptor) ociindex.Opts {
	return ociindex.Opts{
		Artifact:            &ociconsts.ComponentArtifact{ComponentName: config.Name},
		Tag:                 tag,
		Manifests:           descriptors,
		ExtraAnnotations:    config.Annotations,
		RequiredAnnotations: config.RequiredAnnotations(),
	}
}

type Publisher struct {
	config  *Config
	printer utils.RawPrinter
}

func New(config *Config, printer utils.RawPrinter) *Publisher {
	return &Publisher{config: config, printer: printer}
}

func (p *Publisher) Publish(ctx context.Context) (err error) {
	var pushOps []*ocipusher.PushOperation
	if p.config.Name != sdkmanifest.AssistantName {
		pushOps, err = p.prepareComponents(ctx)
		if err != nil {
			return err
		}
	} else {
		assistantPushOps, closeFn, err := p.prepareAssistant(ctx)
		defer func() { _ = closeFn() }()
		if err != nil {
			return err
		}
		pushOps = assistantPushOps
	}

	if p.config.DryRun {
		p.printer.Println("Skipping push due to --dry-run")
		return nil
	}

	if p.config.Registry == "" {
		return fmt.Errorf("--registy must be provided when not in dry-run mode")
	}

	client, err := assistantremote.New(p.config.Registry, p.config.AuthFilePath, p.config.Insecure)
	if err != nil {
		return err
	}

	// skip pushing both index and platforms' images if index already exists
	existingVersions, err := ocilister.ListComponentVersions(ctx, p.config.Name, client)
	if err != nil {
		return err
	}
	alreadyExists := lo.Contains(lo.Map(lo.Keys(existingVersions), func(v *semver.Version, _ int) string {
		return v.String()
	}), p.config.Version.String())

	if alreadyExists {
		p.printer.Println("skipped pushing because component's index already exists in remote")
	} else {
		var descriptors []v1.Descriptor
		for _, pushOp := range pushOps {
			desc, err := p.push(ctx, client, pushOp)
			if err != nil {
				return err
			}
			switch p := pushOp.Platform().(type) {
			case *simpleplatform.NonGeneric:
				desc.Platform = p.ToOras()
			case *simpleplatform.Generic:
			default:
				return fmt.Errorf("unknown platform type %t", p)
			}
			descriptors = append(descriptors, *desc)
		}
		coloredDest := color.GreenString(fmt.Sprintf("%s/%s", p.config.Name, p.config.Version.String()))
		p.printer.Println("📖 Pushing index " + coloredDest)
		tag := p.config.Version.String()

		indexDesc, err := ociindex.PushIndex(ctx, client, p.config.indexOpts(tag, descriptors))
		if err != nil {
			return err
		}
		descriptorJson, err := json.MarshalIndent(indexDesc, "", "  ")
		if err != nil {
			return err
		}
		p.printer.Printf("\n%s\n", string(descriptorJson))
		p.printer.Println("successfully published index " + coloredDest)
	}

	if p.config.ExtraTags != nil && len(p.config.ExtraTags) > 0 {
		p.printer.Println("pushing extra tags...")
		err := ociindex.Tag(ctx, client, &ociconsts.ComponentArtifact{ComponentName: p.config.Name}, p.config.Version, p.config.ExtraTags)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Publisher) PublishDar(ctx context.Context) (err error) {
	var pushOps []*ocipusher.PushOperation

	pushOps, err = p.prepareDars(ctx)
	if err != nil {
		return err
	}

	if p.config.DryRun {
		p.printer.Println("Skipping push due to --dry-run")
		return nil
	}

	if p.config.Registry == "" {
		return fmt.Errorf("--registy must be provided when not in dry-run mode")
	}

	client, err := assistantremote.New(p.config.Registry, p.config.AuthFilePath, p.config.Insecure)
	if err != nil {
		return err
	}

	// skip pushing both index and platforms' images if index already exists
	existingVersions, err := ocilister.ListDarVersions(ctx, p.config.Name, client)
	if err != nil {
		return err
	}
	alreadyExists := lo.Contains(lo.Map(existingVersions, func(v *semver.Version, _ int) string {
		return v.String()
	}), p.config.Version.String())

	if alreadyExists {
		p.printer.Println("skipped pushing because dar's index already exists in remote")
	} else {
		var descriptors []v1.Descriptor
		for _, pushOp := range pushOps {
			desc, err := p.push(ctx, client, pushOp)
			if err != nil {
				return err
			}
			switch p := pushOp.Platform().(type) {
			case *simpleplatform.NonGeneric:
				desc.Platform = p.ToOras()
			case *simpleplatform.Generic:
			default:
				return fmt.Errorf("unknown platform type %t", p)
			}
			descriptors = append(descriptors, *desc)
		}
	}
	if p.config.ExtraTags != nil && len(p.config.ExtraTags) > 0 {
		p.printer.Println("pushing extra tags...")
		err := ociindex.Tag(ctx, client, &ociconsts.DarArtifact{DarName: p.config.Name}, p.config.Version, p.config.ExtraTags)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Publisher) prepareAssistant(ctx context.Context) (pushOps []*ocipusher.PushOperation, close func() error, err error) {
	var deleteFns []func() error
	close = func() error {
		lo.ForEach(deleteFns, func(close func() error, _ int) {
			_ = close()
		})
		return nil
	}

	for platform, binPath := range p.config.Platforms {
		info, err := os.Stat(binPath)
		if err != nil {
			return nil, close, err
		}
		if info.IsDir() {
			return nil, close, fmt.Errorf("provided path %q for platform %q is not a file", binPath, platform.String())
		}

		dir, deleteFn, err := utils.MkdirTemp("", "")
		deleteFns = append(deleteFns, deleteFn)
		if err != nil {
			return nil, close, err
		}

		nonGeneric, ok := platform.(*simpleplatform.NonGeneric)
		if !ok {
			return nil, close, fmt.Errorf("platform %q is not allowed when publishing the assistant", platform.String())
		}

		destName := assembler.AssistantBinNameUnix
		if nonGeneric.OS == "windows" {
			destName = assembler.AssistantBinNameWindows
		}
		dest := filepath.Join(dir, destName)

		if err := utils.CopyFile(binPath, dest); err != nil {
			return nil, close, err
		}

		if err := os.Chmod(dest, 0755); err != nil {
			return nil, close, err
		}

		pushOp, err := p.prepare(ctx, platform, dir, true)
		if err != nil {
			return nil, close, err
		}
		pushOps = append(pushOps, pushOp)
	}

	return pushOps, close, nil
}

func (p *Publisher) prepareComponents(ctx context.Context) ([]*ocipusher.PushOperation, error) {
	var pushOps []*ocipusher.PushOperation
	for platform, dir := range p.config.Platforms {
		pushOp, err := p.prepareComponent(ctx, platform, dir)
		if err != nil {
			return nil, err
		}

		pushOps = append(pushOps, pushOp)
	}
	return pushOps, nil
}

func (p *Publisher) prepareComponent(ctx context.Context, platform simpleplatform.Platform, dir string) (*ocipusher.PushOperation, error) {
	p.printer.Printf("📦 Validating %q component manifest...\n", platform.String())
	if err := validate(ctx, dir, p.config.Name); err != nil {
		return nil, err
	}
	p.printer.Printf("Component manifest is valid ✅\n")
	p.printer.Println()

	p.printer.Printf("📦 Checking %q includes license file...\n", platform.String())
	if err := checkHasLicense(dir); err != nil {
		return nil, err
	}
	p.printer.Printf("License file included ✅\n")
	p.printer.Println()

	p.printer.Println("Content:")
	if err := p.displayContent(dir); err != nil {
		return nil, err
	}
	p.printer.Println()
	return p.prepare(ctx, platform, dir, true)
}

func (p *Publisher) prepareDars(ctx context.Context) ([]*ocipusher.PushOperation, error) {
	var pushOps []*ocipusher.PushOperation
	for platform, dir := range p.config.Platforms {
		pushOp, err := p.prepareDar(ctx, platform, dir)
		if err != nil {
			return nil, err
		}

		pushOps = append(pushOps, pushOp)
	}
	return pushOps, nil
}

func (p *Publisher) prepareDar(ctx context.Context, platform simpleplatform.Platform, dir string) (*ocipusher.PushOperation, error) {
	p.printer.Printf("📦 Checking %q includes license file...\n", platform.String())
	if err := checkHasLicense(dir); err != nil {
		return nil, err
	}
	p.printer.Printf("License file included ✅\n")
	p.printer.Println()

	return p.prepare(ctx, platform, dir, false)
}

func (p *Publisher) prepare(ctx context.Context, platform simpleplatform.Platform, dir string, isComponent bool) (*ocipusher.PushOperation, error) {
	annotations := maps.Clone(p.config.Annotations)
	if p.config.IncludeGitInfo {
		gitAnnotations, err := collectGitAnnotations()
		if err != nil {
			return nil, err
		}
		maps.Copy(annotations, gitAnnotations)
	}
	var artifact ociconsts.Artifact
	if isComponent {
		artifact = &ociconsts.ComponentArtifact{ComponentName: p.config.Name}
	} else {
		artifact = &ociconsts.DarArtifact{DarName: p.config.Name}
	}
	opts := ocipusher.Opts{
		Artifact:            artifact,
		RawTag:              p.config.Version.String(),
		Dir:                 dir,
		RequiredAnnotations: p.config.RequiredAnnotations(),
		ExtraAnnotations:    annotations,
		Platform:            platform,
	}

	pushOp, err := ocipusher.New(ctx, opts)
	if err != nil {
		if errors.Is(err, errdef.ErrSizeExceedsLimit) {
			p.printer.PrintErrln(`Failed to construct OCI manifest due to size limit.
Consider reducing the number of files at the root by moving them to subdirectories`)
		}
		return nil, err
	}

	return pushOp, nil
}

func (p *Publisher) push(ctx context.Context, client *assistantremote.Remote, pushOp *ocipusher.PushOperation) (*v1.Descriptor, error) {
	coloredDest := color.GreenString(pushOp.Destination(client.Registry))

	p.printer.Printf("Pushing %q...\n", coloredDest)
	descriptor, err := pushOp.Do(ctx, client)
	if err != nil {
		return nil, err
	}
	descriptorJson, err := json.MarshalIndent(descriptor, "", "  ")
	if err != nil {
		return nil, err
	}
	p.printer.Printf("\n%s\n", string(descriptorJson))
	p.printer.Println("successfully published " + coloredDest)
	return descriptor, nil
}

func collectGitAnnotations() (map[string]string, error) {
	r, err := git.PlainOpenWithOptions(".", &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, err
	}
	head, err := r.Head()
	if err != nil {
		return nil, err
	}

	result := map[string]string{
		"git.commit": head.Hash().String(),
	}

	tag, err := r.TagObject(head.Hash())
	if err == nil {
		result["git.tag"] = tag.Name
	} else if !errors.Is(err, plumbing.ErrObjectNotFound) {
		return nil, err
	}

	return result, nil
}

func validate(ctx context.Context, dir, name string) error {
	dummyAssembly := &sdkmanifest.SdkManifest{
		AbsolutePath: dir,
		Spec: &sdkmanifest.Spec{
			Components: map[string]*sdkmanifest.Component{
				name: {
					Name:      name,
					LocalPath: &dir,
				},
			},
		},
	}
	_, err := assembler.New(nil, nil).Assemble(ctx, dummyAssembly)
	return err
}

func (p *Publisher) displayContent(dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		var ftype string
		switch {
		case d.Type()&os.ModeSymlink == os.ModeSymlink:
			ftype = "symlink"
			if err := checkSymlinkWithinRoot(dir, path); err != nil {
				return err
			}
		case d.IsDir():
			ftype = "dir"
		default:
			ftype = "file"
		}
		p.printer.Printf("%s %s %s\n",
			color.CyanString(path),
			color.YellowString(ftype),
			color.MagentaString("%d", info.Size()),
		)

		return nil
	})
}

func checkHasLicense(dir string) error {
	des, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	_, ok := lo.Find(des, func(de os.DirEntry) bool {
		return de.Name() == licenseutils.ComponentLicenseFilename && de.Type().IsRegular()
	})
	if !ok {
		return fmt.Errorf("required %s file is missing at component root (%q)", licenseutils.ComponentLicenseFilename, dir)
	}
	return nil
}

func checkSymlinkWithinRoot(dir, symlink string) error {
	resolved, err := filepath.EvalSymlinks(symlink)
	if err != nil {
		return fmt.Errorf("failed to resolve symlink %s: %w", symlink, err)
	}

	resolvedAbs, err := filepath.Abs(resolved)
	if err != nil {
		return err
	}
	if !withinRoot(resolvedAbs, dir) {
		return fmt.Errorf("symlink points outside the root: %s -> %s", symlink, resolvedAbs)
	}

	return nil
}

func withinRoot(target, root string) bool {
	if target == root {
		return true
	}
	return strings.HasPrefix(target, root+string(os.PathSeparator))
}
