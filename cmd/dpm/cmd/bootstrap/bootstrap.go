// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"fmt"
	"path/filepath"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/builtincommand"
	"daml.com/x/assistant/pkg/sdkbundle"
	"daml.com/x/assistant/pkg/utils"
	"github.com/spf13/cobra"
)

func Cmd(config *assistantconfig.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:    string(builtincommand.Bootstrap),
		Short:  "auxiliary command for installing standalone dpm-sdk bundle",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			bundlePath := "./"
			if len(args) == 1 {
				bundlePath = args[0]
			} else {
				return fmt.Errorf("too many arguments")
			}

			cmd.Printf("bootstrapping into %q\n", config.InstalledSdkManifestsPath)

			lockFile := filepath.Join(config.InstalledSdkManifestsPath, ".lock")
			err := utils.WithInstallLock(ctx, lockFile, func() error {
				return sdkbundle.Bootstrap(ctx, config, bundlePath)
			})
			if err != nil {
				return err
			}

			cmd.Printf("Please add %q to your PATH\n", filepath.Join(config.DamlHomePath, "bin"))
			cmd.Println("successfully bootstrapped bundle")
			return nil
		},
	}

	return cmd
}
