// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"daml.com/x/assistant/pkg/assembler"
	"daml.com/x/assistant/pkg/assembler/assemblyplan"
	"daml.com/x/assistant/pkg/multipackage"
	"daml.com/x/assistant/pkg/ocipuller/remotepuller"
	"daml.com/x/assistant/pkg/utils"
	"github.com/samber/lo"

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

				if err := installOverrides(ctx, cmd, config, multiPackagePath, false); err != nil {
					return err
				}
				pkgs := multiDamlPackage.AbsolutePackages()

				for _, p := range pkgs {
					cmd.Printf("Processing package %q...\n", p)
					damlPackagePath := filepath.Join(p, assistantconfig.DamlPackageFilename)
					if err := processDamlPackage(ctx, cmd, modifiedConfig, damlPackagePath); err != nil {
						return err
					}
					if err := installOverrides(ctx, cmd, config, damlPackagePath, true); err != nil {
						return err
					}
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
				return installOverrides(ctx, cmd, config, damlPackagePath, false)
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

	if assistantconfig.DpmDarsEnabled() {
		if err := installDars(ctx, config, lo.Values(damlPackage.ParsedDarDependencies.Dependencies)); err != nil {
			return err
		}
		if err := installDars(ctx, config, lo.Values(damlPackage.ParsedDarDependencies.DataDependencies)); err != nil {
			return err
		}
	}

	return nil
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

func installOverrides(ctx context.Context, cmd *cobra.Command, config *assistantconfig.Config, absPath string, sub bool) error {
	puller, err := remotepuller.NewFromRemoteConfig(config)
	if err != nil {
		return err
	}
	a := assembler.New(config, puller)
	assemblyPlan, err := assemblyplan.NewShallow(ctx, config, a, absPath)
	if err != nil {
		return err
	}
	if sub {
		assemblyPlan.MultiPackage = nil
	}
	if !assemblyPlan.HasOverrides() {
		cmd.Println("No opt-in components to install")
		return nil
	}
	cmd.Println("Installing components...")
	err = utils.WithInstallLock(ctx, config.InstallLocalFilePath, func() error {
		_, err := assemblyPlan.Assemble(ctx)
		return err
	})
	if err != nil {
		return err
	}
	cmd.Println("Successfully installed opt-in components")
	return nil
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
