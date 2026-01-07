// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tags

import (
	"fmt"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	ociconsts "daml.com/x/assistant/pkg/oci"
	"daml.com/x/assistant/pkg/ocilister"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

func Cmd(config *assistantconfig.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "tags <repo or component name>",
		Short: "list published tags of a component",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := assistantremote.NewFromConfig(config)
			if err != nil {
				return err
			}

			repoName := ociconsts.ComponentRepoPrefix + args[0]
			tags, found, err := ocilister.ListTags(cmd.Context(), client, repoName)
			if err != nil {
				return err
			}

			if !found {
				return fmt.Errorf("repo %q doesn't exist in the OCI registry", repoName)
			}

			if len(tags) == 0 {
				cmd.Printf("No tags found under %q\n", repoName)
				return nil
			}

			lo.ForEach(tags, func(t string, _ int) {
				cmd.Println(t)
			})
			return nil
		},
	}
}
