// Copyright (c) 2017-2025 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sdkbundle

import (
	"fmt"
	"os"

	"daml.com/x/assistant/pkg/schema"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/simpleplatform"
	"github.com/goccy/go-yaml"
	"github.com/samber/lo"
)

type PublishConfig struct {
	Edition *sdkmanifest.Edition `yaml:"edition"`
	Version *sdkmanifest.SemVer  `yaml:"version"`

	// this is just the Assistant's platforms, and it determines the platform that the SDK will be published for
	Platforms []*simpleplatform.NonGeneric `yaml:"-"`

	Components map[string]*PlatformComponent `yaml:"components"`
	Assistant  *PlatformComponent            `yaml:"assistant"`
}

func (c *PublishConfig) PlatformlessComponents() map[string]*sdkmanifest.Component {
	return lo.MapValues(c.Components, func(c *PlatformComponent, _ string) *sdkmanifest.Component {
		return c.Component
	})
}

// AssemblyManifests returns the correct assembly manifest for each SDK platform.
// note that the assembly manifest from one platform to another can be different because some components might only be available
// on some platforms but not others!
func (c *PublishConfig) AssemblyManifests(manifestSchema schema.ManifestMeta) map[simpleplatform.NonGeneric]*sdkmanifest.SdkManifest {
	result := make(map[simpleplatform.NonGeneric]*sdkmanifest.SdkManifest)

	for _, p := range c.Platforms {
		comps := lo.FilterMap(lo.Values(c.Components), func(comp *PlatformComponent, index int) (*sdkmanifest.Component, bool) {
			matchingNonGeneric := lo.ContainsBy(comp.Platforms, func(compPlatform *simpleplatform.NonGeneric) bool {
				return p.Equal(compPlatform)
			})
			if comp.Platforms == nil || len(comp.Platforms) == 0 || matchingNonGeneric {
				return comp.Component, true
			}
			return nil, false
		})

		result[*p] = &sdkmanifest.SdkManifest{
			ManifestMeta: manifestSchema,
			Spec: &sdkmanifest.Spec{
				Edition: c.Edition,
				Version: c.Version,
				Components: lo.SliceToMap(comps, func(c *sdkmanifest.Component) (string, *sdkmanifest.Component) {
					return c.Name, c
				}),
				Assistant: c.Assistant.Component,
			},
		}
	}

	return result
}

type PlatformComponent struct {
	*sdkmanifest.Component `yaml:",inline"`

	// null or empty list value is assumed to mean generic platform!
	Platforms []*simpleplatform.NonGeneric `yaml:"platforms"`
}

// this is a hack around goccy/go-yaml bug when unmarshalling embedded structs with fields that have custom unmarshalers
func (pc *PlatformComponent) UnmarshalYAML(bytes []byte) error {
	c := &sdkmanifest.Component{}
	if err := yaml.Unmarshal(bytes, c); err != nil {
		return err
	}

	type temp struct {
		Platforms []*simpleplatform.NonGeneric `yaml:"platforms"`
	}
	p := &temp{}
	if err := yaml.Unmarshal(bytes, p); err != nil {
		return err
	}

	*pc = PlatformComponent{
		Component: c,
		Platforms: p.Platforms,
	}
	return nil
}

func (c *PublishConfig) UnmarshalYAML(bytes []byte) error {
	type Alias PublishConfig
	alias := &Alias{}
	if err := yaml.UnmarshalWithOptions(bytes, alias, yaml.Strict()); err != nil {
		return err
	}
	if alias.Components == nil || len(alias.Components) == 0 {
		return fmt.Errorf("must provide one or more components")
	}
	var componentsPlatforms []*simpleplatform.NonGeneric
	for name, c := range alias.Components {
		c.Name = name
		if err := verifyRemote(c.Component); err != nil {
			return err
		}

		if c.Platforms != nil {
			if len(toSet(c.Platforms)) != len(c.Platforms) {
				return fmt.Errorf("component %q platforms contains duplicates", name)
			}
			componentsPlatforms = append(componentsPlatforms, c.Platforms...)
		}

	}

	if _, ok := alias.Components[sdkmanifest.AssistantName]; ok {
		return fmt.Errorf("the assistant can only be included under `.%s' and not under `.components.%s`",
			sdkmanifest.AssistantName, sdkmanifest.AssistantName)
	}

	if alias.Assistant == nil {
		return fmt.Errorf("must specify the assistant to bundle in")
	}
	alias.Assistant.Name = sdkmanifest.AssistantName
	if err := verifyRemote(alias.Assistant.Component); err != nil {
		return err
	}

	if alias.Edition == nil {
		return fmt.Errorf("edition field is required")
	}

	if alias.Version == nil {
		return fmt.Errorf("version field is required")
	}

	// validate platforms
	if alias.Assistant.Platforms == nil || len(alias.Assistant.Platforms) == 0 {
		return fmt.Errorf("the 'assistant' field is missing a required, non-empty 'platforms' list")
	}
	assistantPlatformsSet := toSet(alias.Assistant.Platforms)
	if len(assistantPlatformsSet) != len(alias.Assistant.Platforms) {
		return fmt.Errorf("assistant's platforms contains duplicates")
	}

	for name, c := range alias.Components {
		if c.Platforms == nil {
			continue
		}

		for _, p := range c.Platforms {
			if _, ok := assistantPlatformsSet[p.String()]; !ok {
				return fmt.Errorf("component %q has a platform (%s) that the assistant lacks. The assistant's platforms must be a superset of components' platforms", name, p.String())
			}
		}
	}

	alias.Platforms = alias.Assistant.Platforms

	*c = (PublishConfig)(*alias)
	return nil
}

func toSet(platforms []*simpleplatform.NonGeneric) map[string]*simpleplatform.NonGeneric {
	return lo.SliceToMap(platforms, func(p *simpleplatform.NonGeneric) (string, *simpleplatform.NonGeneric) {
		return p.String(), p
	})
}

var _ yaml.BytesUnmarshaler = (*PublishConfig)(nil)

func ReadPublishConfig(filePath string) (*PublishConfig, error) {
	contents, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	c := &PublishConfig{}
	if err := yaml.Unmarshal(contents, c); err != nil {
		return nil, err
	}
	return c, nil
}

func verifyRemote(component *sdkmanifest.Component) error {
	if component.LocalPath != nil {
		return fmt.Errorf("components (including the assistant) in publish config can't be local-paths")
	}
	return nil
}
