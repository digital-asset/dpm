// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package dar

import (
	"fmt"
	"strings"

	"daml.com/x/assistant/pkg/publishcmd"
	"daml.com/x/assistant/pkg/publishdar"
	"github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	c := publishcmd.PublishDarCmd{}
	cmd := &cobra.Command{
		Use:     "publish-dar <name> <version>",
		Short:   "Publish a dar to an OCI registry",
		Example: "  dpm repo publish-dar foo 1.2.3-alpha -f path/to/foo.dar",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			version, err := semver.NewVersion(args[1])
			if err != nil {
				return fmt.Errorf("invalid version argument: %w", err)
			}
			cmd.SilenceUsage = true
			publishDarConfig := &publishdar.DarConfig{
				File:           c.File,
				Name:           name,
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
