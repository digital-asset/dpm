// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package publishcomponent

import (
	"fmt"
	"strings"

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

	// TODO stop publishing a tag for every platform
	cmd := &cobra.Command{
		Use:     "component registry",
		Short:   "Publish a component to an OCI registry",
		Long:    "Will publish the component (OCI index) to <registry>/<name>:<version>",
		Example: "dpm artifacts publish component 'oci://whatever.dev/bar/test/foo:1.2.3-alpha' -p linux/amd64=dist/foo-linux -p darwin/arm64=dist/foo-darwin ",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {

			oci := args[0]

			if !strings.HasPrefix(oci, "oci://") {
				return fmt.Errorf("invalid oci registry argument, must be formatted as oci uri ie. oci://whatever.dev/bar/test/foo:1.2.3-alpha")
			}

			ref, err := registry.ParseReference(oci)
			if err != nil {
				return fmt.Errorf("invalid registry formatting: %s", oci)
			}

			version, err := semver.StrictNewVersion(ref.Reference)
			name, _ := lo.Last(strings.Split(ref.Repository, "/"))

			if err != nil {
				return fmt.Errorf("invalid version argument: %w", err)
			}

			platforms, err := c.ParsePlatforms()
			if err != nil {
				return err
			}

			if err != nil {
				return err
			}
			destination := &publish.Destination{
				Registry: ref.Registry,
				Artifact: &ociconsts.ComponentArtifact{
					ComponentRepo: ref.Repository,
				},
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
	//cmd.Flags().StringVarP(&c.Name, publishcmd.ComponentNameFlagName, "n", "", "name of component to be pushed")
	//cmd.MarkFlagRequired(publishcmd.ComponentNameFlagName)
	//cmd.Flags().StringVarP(&c.Version, publishcmd.VersionFlagName, "v", "", "version of component to be pushed")
	//cmd.MarkFlagRequired(publishcmd.VersionFlagName)

	cmd.Flags().BoolVarP(&c.DryRun, "dry-run", "d", false, "don't actually push to the registry")
	cmd.Flags().BoolVarP(&c.IncludeGitInfo, "include-git-info", "g", false, "include git info as annotations on the published manifest")
	cmd.Flags().StringToStringVarP(&c.Annotations, "annotations", "a", map[string]string{}, "annotations to include in the published OCI artifact")

	cmd.Flags().StringToStringVarP(&c.Platforms, publishcmd.PlatformFlagName, "p", map[string]string{}, `REQUIRED <os>/<arch>=<path-to-component> or generic=<path-to-component>`)
	cmd.MarkFlagRequired(publishcmd.PlatformFlagName)

	cmd.Flags().StringSliceVarP(&c.ExtraTags, "extra-tags", "t", []string{}, "publish extra tags besides the semver")

	//cmd.Flags().StringVar(&c.Registry, "registry", "", "OCI registry to use for pushing")

	cmd.Flags().BoolVar(&c.Insecure, "insecure", false, "use http instead of https for OCI registry")
	cmd.Flags().StringVar(&c.RegistryAuth, "auth", "", "path to a config file similar to docker’s config.json to use for authenticating to the OCI registry. Defaults to docker's config.json")

	return cmd
}
