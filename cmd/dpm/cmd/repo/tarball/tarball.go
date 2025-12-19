// Copyright (c) 2017-2025 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tarball

import (
	"fmt"
	"strings"

	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/sdkbundle"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	var outputPath string
	var registry, registryAuth string
	var insecure bool
	var publishConfigPath string
	var blobCache string

	cmd := &cobra.Command{
		Use:     "create-tarball",
		Short:   "create an sdk tarball(s) for one or more platforms",
		Long:    "Pulls down components (including the assistant) from the OCI registry, then dumps out and validates an sdk bundle (for each specified platform) ",
		Example: "  dpm repo create-tarball --registry=gar.io/foo-org -f publish.yaml",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			registry = strings.TrimRight(registry, "/")
			if registry == "" {
				return fmt.Errorf("registry can't be the empty string")
			}
			client, err := assistantremote.New(registry, registryAuth, insecure)
			if err != nil {
				return err
			}

			publishConfig, err := sdkbundle.ReadPublishConfig(publishConfigPath)
			if err != nil {
				return err
			}

			return sdkbundle.Create(cmd.Context(), client, publishConfig, outputPath, blobCache)
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", ".", "output path of the bundle")
	cmd.Flags().StringVar(&registry, "registry", "", "OCI registry to use for pulling/pushing")
	cmd.MarkFlagRequired("registry")
	cmd.Flags().BoolVar(&insecure, "insecure", false, "use http instead of https for OCI registry")
	cmd.Flags().StringVar(&registryAuth, "auth", "", "path to a config file similar to docker’s config.json to use for authenticating to the OCI registry. Defaults to docker's config.json")
	cmd.Flags().StringVarP(&publishConfigPath, "config-file", "f", "", `REQUIRED config file path"`)
	cmd.MarkFlagRequired("config-file")
	cmd.Flags().StringVar(&blobCache, "oci-cache", "", "use an oci-cache to speed up pulls")

	return cmd
}
