// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tags

import (
	"fmt"
	"strings"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/ocilister"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"oras.land/oras-go/v2/registry"
)

type ListCmd struct {
	Name string
}

func Cmd(config *assistantconfig.Config) *cobra.Command {
	c := ListCmd{}
	cmd := &cobra.Command{
		Use:     "tags",
		Short:   "list published tags of an artifact",
		Long:    "Will list all tags associated with an artifact (dar/component) at an arbitrary OCI registry",
		Example: "dpm artifacts list --name 'oci://whatever.dev/bar/test:0.0.0'",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.HasPrefix(c.Name, "oci://") {
				c.Name = strings.TrimPrefix(c.Name, "oci://")
			} else {
				return fmt.Errorf("invalid registry argument, must be formatted as oci uri ie. oci://whatever.dev/test")
			}

			ref, err := registry.ParseReference(c.Name)
			if err != nil {
				return err
			}

			customRemote, err := assistantremote.New(ref.Registry, config.RegistryAuthPath, config.Insecure)
			repoName, _ := lo.Last(strings.Split(ref.Repository, "/"))
			if err != nil {
				return fmt.Errorf("invalid registry argument, must be formatted as oci uri ie. oci://whatever.dev/test")
			}

			// registry flag should be full path to component not including the component and the forward slash
			tags, found, err := ocilister.ListTags(cmd.Context(), customRemote, ref.Repository)
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
	cmd.Flags().StringVarP(&c.Name, "name", "n", "", "full uri of artifact to search for")
	cmd.MarkFlagRequired("name")

	return cmd
}
