// Copyright (c) 2017-2025 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"context"
	"daml.com/x/assistant/pkg/assembler"
	"daml.com/x/assistant/pkg/assembler/assemblyplan"
	"daml.com/x/assistant/pkg/ocipuller/remotepuller"
	"daml.com/x/assistant/pkg/utils"
	"errors"
	"fmt"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/sdkinstall"
	"github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"
)

func Cmd(config *assistantconfig.Config) *cobra.Command {
	return &cobra.Command{
		Use:    "package",
		Short:  "install the SDK and all overrides (if any) for a package",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cmd.SilenceUsage = true

			modifiedConfig := config
			modifiedConfig.AutoInstall = true

			damlPackagePath, isDamlPackage, err := assistantconfig.GetDamlPackageAbsolutePath()
			if err != nil {
				return err
			}
			if !isDamlPackage {
				return fmt.Errorf("not in a package directory or subdirectory")
			}

			damlPackage, err := damlpackage.Read(damlPackagePath)
			if err != nil {
				return err
			}

			if damlPackage.SdkVersion != "" {
				sdkVersion, err := semver.NewVersion(damlPackage.SdkVersion)
				if err != nil {
					return err
				}

				if err := installSdk(ctx, cmd, modifiedConfig, sdkVersion); err != nil {
					return err
				}
			}

			return installOverrides(ctx, cmd, modifiedConfig)
		},
	}
}

func installOverrides(ctx context.Context, cmd *cobra.Command, config *assistantconfig.Config) error {
	puller, err := remotepuller.NewFromRemoteConfig(config)
	if err != nil {
		return err
	}
	a := assembler.New(config, puller)
	assemblyPlan, err := assemblyplan.New(ctx, config, a)
	if err != nil {
		return err
	}
	if !assemblyPlan.HasOverrides() {
		cmd.Println("No overrides to install")
		return nil
	}
	cmd.Println("Installing overrides...")
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
	cmd.Println("Successfully installed SDK" + sdkVersion.String())
	return nil
}
