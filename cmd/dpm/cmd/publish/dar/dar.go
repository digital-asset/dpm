// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package publishdar

import (
	"fmt"
	"strings"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/publishcmd"
	"daml.com/x/assistant/pkg/publishdar"
	"github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	c := publishcmd.PublishDarCmd{}
	cmd := &cobra.Command{
		Use:     "dar",
		Short:   "Publish a dar to an OCI registry",
		Example: "dpm artifacts publish dar --name foo --version 1.2.3-alpha -f path/to/foo.dar",
		Hidden:  !assistantconfig.DpmLockfileEnabled(), // Use single feature flag to represent features in current release
		RunE: func(cmd *cobra.Command, args []string) error {
			version, err := semver.StrictNewVersion(c.Version)
			if err != nil {
				return fmt.Errorf("invalid version argument: %w", err)
			}

			if strings.HasPrefix(c.Registry, "oci://") {
				c.Registry = strings.TrimPrefix(c.Registry, "oci://")
			} else {
				return fmt.Errorf("invalid registry argument, must be formatted as oci uri ie. oci://whatever.dev")
			}

			cmd.SilenceUsage = true
			publishDarConfig := &publishdar.DarConfig{
				File:           c.File,
				Name:           c.Name,
				Version:        version,
				DryRun:         c.DryRun,
				IncludeGitInfo: c.IncludeGitInfo,
				Annotations:    c.Annotations,
				Registry:       strings.TrimRight(c.Registry, "/"),
				AuthFilePath:   c.RegistryAuth,
				Insecure:       c.Insecure,
				ExtraTags:      c.ExtraTags,
			}
			return publishdar.New(publishDarConfig, cmd).PublishDar(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&c.Name, publishcmd.DarNameFlagName, "n", "", "name of dar to be pushed")
	cmd.MarkFlagRequired(publishcmd.DarNameFlagName)
	cmd.Flags().StringVarP(&c.Version, publishcmd.VersionFlagName, "v", "", "version of dar to be pushed")
	cmd.MarkFlagRequired(publishcmd.VersionFlagName)

	cmd.Flags().BoolVarP(&c.DryRun, "dry-run", "d", false, "don't actually push to the registry")
	cmd.Flags().BoolVarP(&c.IncludeGitInfo, "include-git-info", "g", false, "include git info as annotations on the published manifest")
	cmd.Flags().StringToStringVarP(&c.Annotations, "annotations", "a", map[string]string{}, "annotations to include in the published OCI artifact")

	cmd.Flags().StringVarP(&c.File, "file", "f", "", `REQUIRED path to the dar file to publish`)
	cmd.MarkFlagRequired(publishcmd.FileFlagName)

	cmd.Flags().StringSliceVarP(&c.ExtraTags, "extra-tags", "t", []string{}, "publish extra tags besides the semver")

	cmd.Flags().StringVar(&c.Registry, "registry", "", "OCI registry to use for pushing")
	cmd.Flags().BoolVar(&c.Insecure, "insecure", false, "use http instead of https for OCI registry")
	cmd.Flags().StringVar(&c.RegistryAuth, "auth", "", "path to a config file similar to docker’s config.json to use for authenticating to the OCI registry. Defaults to docker's config.json")

	return cmd
}
