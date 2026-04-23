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
	RegistryAuth string
}

func Cmd(config *assistantconfig.Config) *cobra.Command {
	c := ListCmd{}
	cmd := &cobra.Command{
		Use:     "tags",
		Short:   "list published tags of an artifact",
		Long:    "List all tags associated with an artifact (dar/component) at an arbitrary OCI registry",
		Example: "dpm tags oci://whatever.dev/bar/test",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			artifact := args[0]
			if strings.HasPrefix(artifact, "oci://") {
				artifact = strings.TrimPrefix(artifact, "oci://")
			} else {
				return fmt.Errorf("invalid registry argument, must be formatted as oci uri ie. oci://whatever.dev/test")
			}

			ref, err := registry.ParseReference(artifact)
			if err != nil {
				return err
			}
			var auth string
			if c.RegistryAuth != "" {
				auth = c.RegistryAuth
			} else {
				auth = config.RegistryAuthPath
			}
			customRemote, err := assistantremote.New(ref.Registry, auth, config.Insecure)
			if err != nil {
				return fmt.Errorf("invalid registry argument, must be formatted as oci uri ie. oci://whatever.dev/test")
			}

			// registry flag should be full path to component not including the component and the forward slash
			tags, found, err := ocilister.ListTags(cmd.Context(), customRemote, ref.Repository)
			if err != nil {
				return err
			}

			if !found {
				return fmt.Errorf("repo %q doesn't exist in the OCI registry", ref.Repository)
			}

			if len(tags) == 0 {
				cmd.Printf("No tags found under %q\n", ref.Repository)
				return nil
			}

			lo.ForEach(tags, func(t string, _ int) {
				cmd.Println(t)
			})
			return nil
		},
	}

	cmd.Flags().StringVar(&c.RegistryAuth, "registry-auth", "", "path to a config file similar to docker’s config.json to use for authenticating to the OCI registry. Defaults to docker's config.json")

	return cmd
}
