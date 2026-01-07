// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package uninstall

import (
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/builtincommand"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"
	"os"
)

func Cmd(config *assistantconfig.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s <version>", string(builtincommand.UnInstall)),
		Short: "uninstall a dpm-sdk version",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("expected a single argument <dpm-sdk version>")
			}
			cmd.SilenceUsage = true

			v, err := semver.NewVersion(args[0])
			if err != nil {
				return fmt.Errorf("invalid sdk version. %w", err)
			}

			sdk, err := assistantconfig.GetInstalledSdkVersion(config, v)
			if err != nil {
				return err
			}

			if err := os.Remove(sdk.ManifestPath); err != nil {
				return err
			}

			// TODO remove this version's components that aren't part of other sdk versions

			cmd.Println("successfully uninstalled sdk version " + v.String())
			return nil
		},
	}

	return cmd
}
