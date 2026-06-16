// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package install

import (
	"fmt"

	installPackage "daml.com/x/assistant/cmd/dpm/cmd/install/package"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/builtincommand"
	ociconsts "daml.com/x/assistant/pkg/oci"
	"daml.com/x/assistant/pkg/ociindex"
	"daml.com/x/assistant/pkg/sdkinstall"
	"github.com/spf13/cobra"
)

func Cmd(config *assistantconfig.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s [version or tag]", string(builtincommand.Install)),
		Short: "install project's dependencies or specific dpm-sdk version",
		Long: `
When called with no arguments, this behaves an an alias for 'dpm install package'.
When an sdk-version argument is passed, it installs that sdk-version.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if len(args) == 0 {
				return installPackage.InstallPackage(config, cmd)
			}

			if len(args) > 1 {
				return fmt.Errorf("expected at most a single optional [dpm-sdk version or tag] argument")
			}

			cmd.SilenceUsage = true

			client, err := assistantremote.NewFromConfig(config)
			if err != nil {
				return err
			}

			cmd.Println("resolving sdk version...")
			repoName, err := config.SdkManifestsRepo()
			if err != nil {
				return err
			}

			artifact := &ociconsts.SdkManifestArtifact{
				SdkManifestsRepo: repoName,
			}
			sdkVersion, err := ociindex.ResolveTag(ctx, client, artifact, args[0])
			if err != nil {
				return err
			}
			cmd.Printf("resolved to %s\n", sdkVersion.String())

			modifiedConfig := config
			modifiedConfig.AutoInstall = true
			if _, err := sdkinstall.InstallSdkVersion(ctx, config, sdkVersion); err != nil {
				return err
			}

			cmd.Println("Successfully installed SDK " + sdkVersion.String())
			return nil
		},
	}

	cmd.AddCommand(installPackage.Cmd(config))
	return cmd
}
