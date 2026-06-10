// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"context"
	ociconsts "daml.com/x/assistant/pkg/oci"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"strings"

	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/multipackage"
	"daml.com/x/assistant/pkg/ocilister"
	"daml.com/x/assistant/pkg/ocipuller/remotepuller"
	"daml.com/x/assistant/pkg/simpleplatform"
	"daml.com/x/assistant/pkg/utils"
	"github.com/samber/lo"
	"oras.land/oras-go/v2/registry"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/sdkinstall"
	"github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"
)

func Cmd(config *assistantconfig.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "package",
		Short:  "install the SDK and all opt-in components (if any) for a package",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cmd.SilenceUsage = true

			modifiedConfig := config
			modifiedConfig.AutoInstall = true
			multiPackagePath, hasMultiPackage, err := assistantconfig.GetMultiPackageAbsolutePath()
			shas := make(map[string]string)
			if err != nil {
				return err
			}
			if hasMultiPackage {
				multiDamlPackage, err := multipackage.Read(multiPackagePath)
				if err != nil {
					return err
				}

				if multiDamlPackage.SdkVersion != "" {
					sdkVersion, err := semver.NewVersion(multiDamlPackage.SdkVersion)
					if err != nil {
						return err
					}
					if err := installSdk(ctx, cmd, config, sdkVersion); err != nil {
						return err
					}
				}

				multiPkgSHAs, err := resolveOCIs(ctx, cmd, config, multiPackagePath, true)
				if err != nil {
					return err
				}
				maps.Copy(shas, *multiPkgSHAs)

				pkgs := multiDamlPackage.AbsolutePackages()

				for _, p := range pkgs {
					cmd.Printf("Processing package %q...\n", p)
					damlPackagePath := filepath.Join(p, assistantconfig.DamlPackageFilename)
					if err := processDamlPackage(ctx, cmd, modifiedConfig, damlPackagePath); err != nil {
						return err
					}
					subPkgSHAs, err := resolveOCIs(ctx, cmd, config, damlPackagePath, false)
					if err != nil {
						return nil
					}
					maps.Copy(shas, *subPkgSHAs)
				}

			} else {
				damlPackagePath, isDamlPackage, err := assistantconfig.GetDamlPackageAbsolutePath()
				if err != nil {
					return err
				}
				if !isDamlPackage {
					return fmt.Errorf("not in a package directory or subdirectory")
				}
				if err := processDamlPackage(ctx, cmd, modifiedConfig, damlPackagePath); err != nil {
					return err
				}
				pkgSHAs, err := resolveOCIs(ctx, cmd, config, damlPackagePath, false)
				if err != nil {
					return nil
				}
				maps.Copy(shas, *pkgSHAs)
			}
			return nil
		},
	}

	return cmd
}

func processDamlPackage(ctx context.Context, cmd *cobra.Command, config *assistantconfig.Config, damlPath string) error {
	damlPackage, err := damlpackage.Read(damlPath)
	if err != nil {
		return err
	}
	if damlPackage.SdkVersion != "" {
		sdkVersion, err := semver.NewVersion(damlPackage.SdkVersion)
		if err != nil {
			return err
		}
		if err := installSdk(ctx, cmd, config, sdkVersion); err != nil {
			return err
		}
	}

	if err := installDars(ctx, config, lo.Values(damlPackage.ParsedDarDependencies.Dependencies)); err != nil {
		return err
	}
	if err := installDars(ctx, config, lo.Values(damlPackage.ParsedDarDependencies.DataDependencies)); err != nil {
		return err
	}

	return nil
}

func resolveOCIs(ctx context.Context, cmd *cobra.Command, config *assistantconfig.Config, damlPath string, multi bool) (*map[string]string, error) {
	// puller, err := remotepuller.NewFromRemoteConfig(config)
	ocis := make(map[string]string)
	var components map[string]*sdkmanifest.Component

	if multi {
		multiPackage, err := multipackage.Read(damlPath)
		if err != nil {
			return nil, err
		}
		components = multiPackage.Components
	} else {
		damlPackage, err := damlpackage.Read(damlPath)
		if err != nil {
			return nil, err
		}
		components = damlPackage.Components
	}
	if len(components) == 0 {
		cmd.Printf("No packages to install for %s\n", damlPath)
	}

	for _, comp := range components {
		if comp.LocalPath != nil {
			continue
		} else if comp.Uri != nil {
			fullURI := comp.Uri

			ref, err := registry.ParseReference(strings.TrimPrefix(*fullURI, "oci://"))
			if err != nil {
				return nil, err
			}

			version, err := semver.StrictNewVersion(ref.Reference)
			if err != nil {
				return nil, err
			}

			destPath := ociComponentPath(fmt.Sprintf("%s/%s", ref.Registry, ref.Repository), version.String(), config)
			ok, err := utils.DirExists(destPath)
			if err != nil {
				return nil, err
			}
			if !ok {
				fmt.Printf("pulling sdk component %s ...\n", *fullURI)
				customRemote, err := assistantremote.New(ref.Registry, config.RegistryAuthPath, config.Insecure)
				if err != nil {
					return nil, err
				}

				platform := simpleplatform.CurrentPlatform()
				customPuller := remotepuller.New(config.OciLayoutCache, customRemote)
				desc, err := customPuller.PullComponentSHAByFullPath(ctx, ref.Repository, version.String(), destPath, platform)
				if err != nil {
					return nil, err
				}
				ocis[fmt.Sprintf("%s/%s", ref.Registry, ref.Repository)] = desc.Digest.String()
			}
		} else {
			// This code path handles the imageVersion case, pull component and save repo + digest for use in nudging to pinning
			destPath := ociComponentPath(comp.Name, comp.Version.Value().String(), config)

			// TODO - Copied ComputeTagOrDigest function from assembler, need to flesh out
			tag := comp.Version.Value().String()

			puller, err := remotepuller.NewFromRemoteConfig(config)
			// check if component is already in the cache
			ok, err := utils.DirExists(destPath)
			if err != nil {
				return nil, err
			}
			if !ok {
				platform := simpleplatform.CurrentPlatform()
				fmt.Printf("pulling sdk component %s %s...\n", comp.Name, tag)
				desc, err := puller.PullComponentSHA(ctx, comp.Name, tag, destPath, platform)
				if err != nil {
					return nil, err
				}
				ocis[fmt.Sprintf("%s/%s", config.Registry, ociconsts.ComponentRepoPrefix+comp.Name)] = desc.Digest.String()
			}
		}
	}
	return &ocis, nil

}

func ociComponentPath(componentUri string, tag string, config *assistantconfig.Config) string {
	return filepath.Join(config.CachePath, "components", utils.UrlToFilePath(componentUri), tag)
}

func installDars(ctx context.Context, config *assistantconfig.Config, dars []*damlpackage.ParsedDarDependency) error {
	for _, d := range dars {
		if err := installDar(ctx, config, d); err != nil {
			return err
		}
	}
	return nil
}

func installDar(ctx context.Context, config *assistantconfig.Config, dar *damlpackage.ParsedDarDependency) error {
	if dar.FullUrl.Scheme != "oci" {
		return nil
	}
	fmt.Printf("installing dar %q...\n", dar.FullUrl.String())

	client, ref, err := dar.GetOciRemote()
	if err != nil {
		return err
	}

	if ocilister.IsFloaty(ref.Reference) {
		return fmt.Errorf("tag not allowed in %q: only strict semver OCI tags are supported currently", dar.FullUrl.String())
	}

	puller := remotepuller.New(config.OciLayoutCache, client)
	darDir := config.CachePathForDar(ref)

	ok, err := utils.DirExists(darDir)
	if err != nil {
		return err
	}
	if ok {
		fmt.Println("Dar already installed.")
		return nil
	}

	return puller.PullDarByFullPath(ctx, ref.Repository, ref.Reference, darDir)
}

func installSdk(ctx context.Context, cmd *cobra.Command, config *assistantconfig.Config, sdkVersion *semver.Version) error {
	_, err := assistantconfig.GetInstalledSdkVersion(config, sdkVersion)
	if err == nil {
		cmd.Printf("SDK version %s is already installed\n", sdkVersion.String())
	} else if !errors.Is(err, assistantconfig.ErrTargetSdkNotInstalled) {
		return err
	}

	if _, err := sdkinstall.InstallSdkVersion(ctx, config, sdkVersion); err != nil {
		return err
	}
	cmd.Println("Successfully installed SDK " + sdkVersion.String())
	return nil
}
