// Copyright (c) 2017-2025 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sdkmanifest

import (
	"fmt"
	"strings"

	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/sdkbundle"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	var registry, registryAuth string
	var insecure bool
	var publishConfigPath string
	var blobCache string
	var extraTags []string

	cmd := &cobra.Command{
		Use:     "publish-sdk-manifest",
		Short:   "publish an sdk's manifest",
		Long:    "Creates, validates and then publishes an sdk manifest to OCI registry",
		Example: "  dpm repo publish-sdk-manifest --registry=gar.io/foo-org -f publish.yaml",
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

			return sdkbundle.Publish(cmd.Context(), cmd, client, publishConfigPath, blobCache, extraTags)
		},
	}

	cmd.Flags().StringVar(&registry, "registry", "", "OCI registry to use for pulling/pushing")
	cmd.MarkFlagRequired("registry")
	cmd.Flags().BoolVar(&insecure, "insecure", false, "use http instead of https for OCI registry")
	cmd.Flags().StringVar(&registryAuth, "auth", "", "path to a config file similar to docker’s config.json to use for authenticating to the OCI registry. Defaults to docker's config.json")
	cmd.Flags().StringVarP(&publishConfigPath, "config-file", "f", "", `REQUIRED config file path"`)
	cmd.MarkFlagRequired("config-file")
	cmd.Flags().StringVar(&blobCache, "oci-cache", "", "use an oci-cache to speed up pulls")

	cmd.Flags().StringSliceVarP(&extraTags, "extra-tags", "t", []string{}, "publish extra tags besides the semver")

	return cmd
}
