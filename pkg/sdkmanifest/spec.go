// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sdkmanifest

import (
	"fmt"

	"github.com/goccy/go-yaml"
)

const AssistantName = "dpm" // this is also the OCI repo name

type Spec struct {
	Components map[string]*Component `yaml:"components"`
	Assistant  *Component            `yaml:"assistant"`

	Version *SemVer  `yaml:"version"`
	Edition *Edition `yaml:"edition"`
}

func (s *Spec) UnmarshalYAML(bytes []byte) error {
	type Alias Spec
	alias := Alias{}
	if err := yaml.Unmarshal(bytes, &alias); err != nil {
		return fmt.Errorf("failed to unmarshal Spec: %w", err)
	}

	if alias.Version == nil {
		return fmt.Errorf("%w: 'version'", MissingAssemblyField)
	}
	if alias.Edition == nil {
		return fmt.Errorf("%w: 'edition'", MissingAssemblyField)
	}

	if alias.Components == nil {
		return fmt.Errorf("%w: spec 'components'", MissingAssemblyField)
	}

	if len(alias.Components) == 0 {
		return fmt.Errorf("%w: must have at least one component", ErrInvalidAssemblyManifest)
	}

	for k, v := range alias.Components {
		v.Name = k
	}

	if _, ok := alias.Components[AssistantName]; ok {
		return fmt.Errorf("%w: the assistant can only be included under `.spec.%s' and not under `.spec.compponents.%s`",
			ErrInvalidAssemblyManifest, AssistantName, AssistantName)
	}

	if alias.Assistant != nil {
		alias.Assistant.Name = AssistantName
		if alias.Assistant.LocalPath != nil {
			return fmt.Errorf("%w: assistant can only be an OCI and not a local-path", ErrInvalidAssemblyManifest)
		}
	}

	*s = Spec(alias)
	return nil
}

type Component struct {
	Name      string  `yaml:"-"`
	Version   *SemVer `yaml:"version,omitempty"`
	ImageTag  *string `yaml:"image-tag,omitempty"`
	LocalPath *string `yaml:"local-path,omitempty"`
}

func (c *Component) UnmarshalYAML(bytes []byte) error {
	type Alias Component
	alias := Alias{}
	if err := yaml.Unmarshal(bytes, &alias); err != nil {
		return fmt.Errorf("failed to unmarshal Component: %w", err)
	}

	if alias.Version == nil && alias.LocalPath == nil && alias.ImageTag == nil {
		return fmt.Errorf("%w: a component must include `local-path`, `image-tag` or `version` field", ErrInvalidAssemblyManifest)
	}
	if alias.LocalPath != nil {
		if alias.ImageTag != nil || alias.Version != nil {
			return fmt.Errorf("%w: a component can't simultaneously be local ('local-path') and remote ('version', 'image-tag')", ErrInvalidAssemblyManifest)
		}
	}
	*c = Component(alias)
	return nil
}

var _ yaml.BytesUnmarshaler = (*Spec)(nil)
var _ yaml.BytesUnmarshaler = (*Component)(nil)
