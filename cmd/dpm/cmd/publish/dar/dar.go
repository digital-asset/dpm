// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package publishdar

import (
	"fmt"
	"strings"

	"daml.com/x/assistant/pkg/assistantconfig"
	ociconsts "daml.com/x/assistant/pkg/oci"
	"daml.com/x/assistant/pkg/publish"
	"daml.com/x/assistant/pkg/publishcmd"
	"daml.com/x/assistant/pkg/publishdar"
	"github.com/Masterminds/semver/v3"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"oras.land/oras-go/v2/registry"
)

func Cmd() *cobra.Command {
	c := publishcmd.PublishDarCmd{}
	cmd := &cobra.Command{
		Use:     "dar <registry>",
		Short:   "Publish a dar to an OCI registry",
		Example: "dpm publish dar 'oci://whatever.dev/bar/test/foo:1.2.3-alpha' -f path/to/foo.dar",
		Hidden:  !assistantconfig.DpmLockfileEnabled(), // Use single feature flag to represent features in current release
		Args:    cobra.ExactArgs(1),
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
				Artifact: &ociconsts.DarArtifact{
					DarRepo: ref.Repository,
				},
			}

			cmd.SilenceUsage = true
			publishDarConfig := &publishdar.DarConfig{
				File:           c.File,
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
			return publishdar.New(publishDarConfig, cmd).PublishDar(cmd.Context())
		},
	}

	cmd.Flags().BoolVarP(&c.DryRun, "dry-run", "d", false, "don't actually push to the registry")
	cmd.Flags().BoolVarP(&c.IncludeGitInfo, "include-git-info", "g", false, "include git info as annotations on the published manifest")
	cmd.Flags().StringToStringVarP(&c.Annotations, "annotations", "a", map[string]string{}, "annotations to include in the published OCI artifact")

	cmd.Flags().StringVarP(&c.File, "file", "f", "", `REQUIRED path to the dar file to publish`)
	cmd.MarkFlagRequired(publishcmd.FileFlagName)

	cmd.Flags().StringSliceVarP(&c.ExtraTags, "extra-tags", "t", []string{}, "publish extra tags besides the semver")

	cmd.Flags().BoolVar(&c.Insecure, "insecure", false, "use http instead of https for OCI registry")
	cmd.Flags().StringVar(&c.RegistryAuth, "auth", "", "path to a config file similar to docker’s config.json to use for authenticating to the OCI registry. Defaults to docker's config.json")

	return cmd
}
