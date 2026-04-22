// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package componenttags

import (
	"fmt"
	"strings"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/listcmd"
	"daml.com/x/assistant/pkg/ocilister"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"oras.land/oras-go/v2/registry"
)

func Cmd(config *assistantconfig.Config) *cobra.Command {
	c := listcmd.ListCmd{}
	cmd := &cobra.Command{
		Use:     "component",
		Short:   "list published tags of a component",
		Long:    "Will list all tags associated with a component at an arbitrary OCI registry",
		Example: "dpm artifacts list component --name foo --registry 'oci://whatever.dev/bar/test'",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.HasPrefix(c.Registry, "oci://") {
				c.Registry = strings.TrimPrefix(c.Registry, "oci://")
			} else {
				return fmt.Errorf("invalid registry argument, must be formatted as oci uri ie. oci://whatever.dev")
			}

			ref, err := registry.ParseReference(c.Registry)
			if err != nil {
				return err
			}

			customRemote, err := assistantremote.New(ref.Registry, config.RegistryAuthPath, config.Insecure)
			repoName := c.Name

			// registry flag should be full path to component not including the component and the forward slash
			tags, found, err := ocilister.ListTags(cmd.Context(), customRemote, ref.Repository+"/"+repoName)
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
	cmd.Flags().StringVarP(&c.Name, "name", "n", "", "name of component to search for")
	cmd.MarkFlagRequired("name")

	cmd.Flags().StringVar(&c.Registry, "registry", "", "OCI registry to search in")
	cmd.MarkFlagRequired("registry")
	return cmd
}
