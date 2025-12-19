// Copyright (c) 2017-2025 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package component

import (
	"fmt"
	"strings"

	"daml.com/x/assistant/pkg/publish"
	"daml.com/x/assistant/pkg/publishcmd"
	"github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	c := publishcmd.PublishCmd{}

	cmd := &cobra.Command{
		Use:     "publish-component <name> <version>",
		Short:   "Publish a component to an OCI registry",
		Example: "  dpm repo publish-component foo 1.2.3-alpha -p linux/amd64=dist/foo-linux -p darwin/arm64=dist/foo-darwin",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			version, err := semver.NewVersion(args[1])
			if err != nil {
				return fmt.Errorf("invalid version argument: %w", err)
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
				Registry:       strings.TrimRight(c.Registry, "/"),
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

	cmd.Flags().StringVar(&c.Registry, "registry", "", "OCI registry to use for pushing")
	cmd.Flags().BoolVar(&c.Insecure, "insecure", false, "use http instead of https for OCI registry")
	cmd.Flags().StringVar(&c.RegistryAuth, "auth", "", "path to a config file similar to docker’s config.json to use for authenticating to the OCI registry. Defaults to docker's config.json")

	return cmd
}
