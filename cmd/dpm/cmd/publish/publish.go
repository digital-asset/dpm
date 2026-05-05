// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package publish

import (
	"fmt"
	"strings"

	publishcomponent "daml.com/x/assistant/cmd/dpm/cmd/publish/component"
	publishdar "daml.com/x/assistant/cmd/dpm/cmd/publish/dar"
	"daml.com/x/assistant/pkg/builtincommand"
	ociconsts "daml.com/x/assistant/pkg/oci"
	"daml.com/x/assistant/pkg/publish"
	"daml.com/x/assistant/pkg/publishcmd"
	"github.com/Masterminds/semver/v3"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"oras.land/oras-go/v2/registry"
)

func Cmd() *cobra.Command {
	c := publishcmd.PublishCmd{}
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s <registry>", string(builtincommand.Publish)),
		Short: "Command for publishing an artifact to an OCI registry",
		Long:  "Command/subcommands for publishing artifacts to an OCI registry",
		RunE: func(cmd *cobra.Command, args []string) error {
			oci := args[0]

			if strings.HasPrefix(oci, "oci://") {
				oci = strings.TrimPrefix(oci, "oci://")
			} else {
				return fmt.Errorf("invalid oci registry argument, must be formatted as oci uri ie. oci://whatever.dev/bar/test/foo:1.2.3-alpha")
			}

			ref, err := registry.ParseReference(oci)
			if err != nil {
				return fmt.Errorf("invalid registry formatting: %s", oci)
			}

			version, err := semver.StrictNewVersion(ref.Reference)
			name, _ := lo.Last(strings.Split(ref.Repository, "/"))

			destination := &publish.Destination{
				Registry: ref.Registry,
				Artifact: &ociconsts.GenericArtifact{
					ArtifactName: ref.Repository,
				},
			}

			platforms, err := c.ParsePlatforms()
			if err != nil {
				return err
			}

			cmd.SilenceUsage = true
			publishConfig := &publish.Config{
				Platforms:      platforms,
				Name:           name,
				Version:        version,
				DryRun:         c.DryRun,
				IncludeGitInfo: c.IncludeGitInfo,
				Annotations:    c.Annotations,
				Destination:    destination,
				AuthFilePath:   c.RegistryAuth,
				Insecure:       c.Insecure,
				ExtraTags:      c.ExtraTags,
			}
			return publish.New(publishConfig, cmd).Publish(cmd.Context())
		},
	}

	cmd.Flags().BoolVarP(&c.DryRun, "dry-run", "d", false, "don't actually push to the registry")
	cmd.Flags().BoolVarP(&c.IncludeGitInfo, "include-git-info", "g", false, "include git info as annotations on the published manifest")
	cmd.Flags().StringToStringVarP(&c.Annotations, "annotations", "a", map[string]string{}, "annotations to include in the published OCI artifact")

	cmd.Flags().StringToStringVarP(&c.Platforms, publishcmd.PlatformFlagName, "p", map[string]string{}, `REQUIRED <os>/<arch>=<path-to-component> or generic=<path-to-component>`)
	cmd.MarkFlagRequired(publishcmd.PlatformFlagName)

	cmd.Flags().StringSliceVarP(&c.ExtraTags, "extra-tags", "t", []string{}, "publish extra tags besides the semver")

	cmd.Flags().BoolVar(&c.Insecure, "insecure", false, "use http instead of https for OCI registry")
	cmd.Flags().StringVar(&c.RegistryAuth, "auth", "", "path to a config file similar to docker’s config.json to use for authenticating to the OCI registry. Defaults to docker's config.json")

	cmd.AddCommand(publishcomponent.Cmd())
	cmd.AddCommand(publishdar.Cmd())

	return cmd
}
