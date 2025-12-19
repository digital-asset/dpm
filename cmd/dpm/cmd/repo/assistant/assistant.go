// Copyright (c) 2017-2025 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package assistant

import (
	"fmt"
	"strings"

	"daml.com/x/assistant/pkg/publish"
	"daml.com/x/assistant/pkg/publishcmd"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	c := publishcmd.PublishCmd{}

	cmd := &cobra.Command{
		Use:     "publish-dpm <version>",
		Short:   "Publish the assistant to an OCI registry",
		Example: "  dpm repo publish-dpm 1.2.3-alpha -p linux/arm64=dist/dpm -p windows/amd64=dist/dpm.exe",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			version, err := semver.NewVersion(args[0])
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
				Name:           sdkmanifest.AssistantName,
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

	cmd.Flags().StringToStringVarP(&c.Platforms, publishcmd.PlatformFlagName, "p", map[string]string{}, `REQUIRED <os>/<arch>=<path-to-assistant's-binary>`)
	cmd.MarkFlagRequired(publishcmd.PlatformFlagName)

	cmd.Flags().StringSliceVarP(&c.ExtraTags, "extra-tags", "t", []string{}, "publish extra tags besides the semver")

	cmd.Flags().StringVar(&c.Registry, "registry", "", "OCI registry to use for pushing")
	cmd.Flags().BoolVar(&c.Insecure, "insecure", false, "use http instead of https for OCI registry")
	cmd.Flags().StringVar(&c.RegistryAuth, "auth", "", "path to a config file similar to docker’s config.json to use for authenticating to the OCI registry. Defaults to docker's config.json")

	return cmd
}
