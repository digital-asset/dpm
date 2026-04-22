// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package run

import (
	"fmt"
	"os"

	"daml.com/x/assistant/pkg/assistant"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/schema"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"
)

func Cmd(config *assistantconfig.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "run <component name> <component version> [args]",
		Short: "pull down and run a remote component",
		Args:  cobra.MinimumNArgs(2),
		Long: fmt.Sprintf(`pull down and run a remote component.
The registry to pull from is the one configured via $%s $%s/%s`, assistantconfig.DpmHomeEnvVar, assistantconfig.OciRegistryEnvVar, assistantconfig.DpmConfigFileName),
		RunE: func(cmd *cobra.Command, args []string) error {

			name := args[0]
			rawVersison := args[1]
			version, err := semver.NewVersion(rawVersison)
			if err != nil {
				return err
			}

			da := assistant.DamlAssistant{
				Stderr: os.Stderr,
				Stdout: os.Stdout,
				Stdin:  os.Stdin,
				ExitFn: os.Exit,
			}

			dummyAssembly := &sdkmanifest.SdkManifest{
				AbsolutePath: "",
				ManifestMeta: schema.ManifestMeta{},
				Spec: &sdkmanifest.Spec{
					Version: nil,
					Edition: nil,
					Components: map[string]*sdkmanifest.Component{
						name: {
							Name:    name,
							Version: sdkmanifest.AssemblySemVer(version),
						},
					},
				},
			}
			componentCommands, err := da.ComputeSdkCommandsFromAssemblyManifest(cmd.Context(), config, dummyAssembly)
			if err != nil {
				return err
			}

			root := cobra.Command{}
			root.AddCommand(componentCommands...)
			root.SetArgs(args[2:])
			return root.ExecuteContext(cmd.Context())
		},
	}
}
