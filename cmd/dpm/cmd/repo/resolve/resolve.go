// Copyright (c) 2017-2025 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package resolve

import (
	"context"
	"fmt"
	"strings"

	"github.com/samber/lo"

	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/oci"
	"daml.com/x/assistant/pkg/ociindex"
	"daml.com/x/assistant/pkg/sdkbundle"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"github.com/Masterminds/semver/v3"
	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

type resolveCmd struct {
	Registry, RegistryAuth string
	Insecure               bool
	PublishConfigPath      string
}

func Cmd() *cobra.Command {
	c := resolveCmd{}

	cmd := &cobra.Command{
		Use:   "resolve-tags <component>:<tag>...<component>:<tag>",
		Short: "resolve the tag of one or more components to corresponding (semantic) versions",
		Long: `
Resolve the tag (e.g. 'latest') of one or more components to corresponding (semantic) versions.

Components can be passed directly as cli arguments. 
Alternatively, you can pass the config file used with "dpm repo create-tarball"

...
components:
  foo:
    image-tag: latest
  bar:
    version: 1.2.3-whatever
  baz:
    image-tag: some-tag
assistant:
  image-tag: latest

The output will be the same content, but with components that have "image-tag" replaced with resolved versions.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c.Registry = strings.TrimRight(c.Registry, "/")

			client, err := assistantremote.New(c.Registry, c.RegistryAuth, c.Insecure)
			if err != nil {
				return err
			}

			if c.PublishConfigPath == "" {
				if len(args) == 0 {
					return fmt.Errorf("one or more components must be provided as args")
				}
				badArgs := lo.Filter(args, func(arg string, _ int) bool {
					return strings.Count(arg, ":") != 1
				})
				if len(badArgs) > 0 {
					return fmt.Errorf("one or more provided components are invalid. Each must be of the form '<component>:<tag>'")
				}
				parsed := lo.SliceToMap(args, func(arg string) (string, string) {
					s := strings.Split(arg, ":")
					return s[0], s[1]
				})
				resolved, err := resolveAll(ctx, client, parsed)
				if err != nil {
					return err
				}
				for _, a := range args {
					cmd.Println(resolved[strings.Split(a, ":")[0]].String())
				}
				return nil
			}

			publishConfig, err := resolveFromPublishConfig(ctx, client, c.PublishConfigPath)
			if err != nil {
				return err
			}
			output, err := yaml.Marshal(publishConfig)
			if err != nil {
				return err
			}
			cmd.Print(string(output))
			return nil
		},
	}

	cmd.Flags().StringVar(&c.PublishConfigPath, "from-publish-config", "", `resolve component tags in publish.yaml file`)

	cmd.Flags().StringVar(&c.Registry, "registry", "", "OCI registry to use for pushing")
	cmd.MarkFlagRequired("registry")
	cmd.Flags().BoolVar(&c.Insecure, "insecure", false, "use http instead of https for OCI registry")
	cmd.Flags().StringVar(&c.RegistryAuth, "auth", "", "path to a config file similar to docker’s config.json to use for authenticating to the OCI registry. Defaults to docker's config.json")

	return cmd
}

func resolveFromPublishConfig(ctx context.Context, client *assistantremote.Remote, publishConfigPath string) (*sdkbundle.PublishConfig, error) {
	publishConfig, err := sdkbundle.ReadPublishConfig(publishConfigPath)
	if err != nil {
		return nil, err
	}

	components := make(map[string]string)
	for _, comp := range publishConfig.PlatformlessComponents() {
		if t := comp.ImageTag; t != nil {
			components[comp.Name] = *t
		}
	}
	if t := publishConfig.Assistant.ImageTag; t != nil {
		components[publishConfig.Assistant.Name] = *t
	}

	resolved, err := resolveAll(ctx, client, components)
	if err != nil {
		return nil, err
	}

	for name, ver := range resolved {
		if name == sdkmanifest.AssistantName {
			publishConfig.Assistant.Component = &sdkmanifest.Component{
				Name:    name,
				Version: sdkmanifest.AssemblySemVer(ver),
			}
			continue
		}

		publishConfig.Components[name].Component = &sdkmanifest.Component{
			Name:    name,
			Version: sdkmanifest.AssemblySemVer(ver),
		}
	}

	return publishConfig, nil
}

func resolveAll(ctx context.Context, client *assistantremote.Remote, componentTags map[string]string) (map[string]*semver.Version, error) {
	resolved := map[string]*semver.Version{}
	for comp, tag := range componentTags {
		version, err := ociindex.ResolveTag(ctx, client, &oci.ComponentArtifact{ComponentName: comp}, tag)
		if err != nil {
			return nil, err
		}
		resolved[comp] = version
	}
	return resolved, nil
}
