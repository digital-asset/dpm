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

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/sdkinstall"
	"github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"
)

func Cmd(config *assistantconfig.Config) *cobra.Command {
	var skipSDK bool
	cmd := &cobra.Command{
		Use:    "package",
		Short:  "install the SDK and all overrides (if any) for a package",
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
					if !skipSDK {
						if err := installSdk(ctx, cmd, config, sdkVersion); err != nil {
							return err
						}
					} else {
						cmd.Println("Skipping installation of multi-package sdk version ", sdkVersion)
					}
				}

				installOverrides(ctx, cmd, config, multiPackagePath, false)
				pkgs := multiDamlPackage.AbsolutePackages()

				for _, p := range pkgs {
					cmd.Printf("Processing package %q...\n", p)
					damlPackagePath := filepath.Join(p, assistantconfig.DamlPackageFilename)
					processDamlPackage(ctx, cmd, modifiedConfig, damlPackagePath, skipSDK)
					installOverrides(ctx, cmd, config, damlPackagePath, true)
				}

			} else {
				damlPackagePath, isDamlPackage, err := assistantconfig.GetDamlPackageAbsolutePath()
				if err != nil {
					return err
				}
				if !isDamlPackage {
					return fmt.Errorf("not in a package directory or subdirectory")
				}
				processDamlPackage(ctx, cmd, modifiedConfig, damlPackagePath, skipSDK)
				return installOverrides(ctx, cmd, config, damlPackagePath, false)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&skipSDK, "skip-sdk", false, "run install packages with overwrites only")
	return cmd
}
func processDamlPackage(ctx context.Context, cmd *cobra.Command, config *assistantconfig.Config, damlPath string, skip bool) error {
	damlPackage, err := damlpackage.Read(damlPath)
	if err != nil {
		return err
	}
	if damlPackage.SdkVersion != "" {
		sdkVersion, err := semver.NewVersion(damlPackage.SdkVersion)
		if err != nil {
			return err
		}
		if !skip {
			if err := installSdk(ctx, cmd, config, sdkVersion); err != nil {
				return err
			}
		} else {
			cmd.Println("Skipping installation of SDK with version:", sdkVersion)
		}
	}
	return nil
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
		cmd.Println("No overrides to install")
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
	cmd.Println("Successfully installed overrides")
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
