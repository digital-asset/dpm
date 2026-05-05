// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package damlpackage

import (
	"fmt"
	"maps"
	"os"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/componentlist"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/utils"
	"github.com/goccy/go-yaml"
)

type DamlPackage struct {
	SdkVersion string `yaml:"sdk-version"`

	ComponentsList componentlist.ComponentList       `yaml:"components,omitempty"`
	Components     map[string]*sdkmanifest.Component `yaml:"-"`

	// deprecated in favor of Components
	DeprecatedOverrideComponents map[string]*sdkmanifest.Component `yaml:"override-components,omitempty"`

	Dependencies         []string                       `yaml:"dependencies,omitempty"`
	ArtifactLocations    ArtifactLocations              `yaml:"artifact-locations,omitempty"`
	ResolvedDependencies map[string]*ResolvedDependency `yaml:"-"`
}

func Read(filePath string) (*DamlPackage, error) {
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return ReadFromContents(bytes)
}

func ReadFromContents(contents []byte) (*DamlPackage, error) {
	expanded, err := utils.ExpandEnv(contents)
	if err != nil {
		return nil, err
	}

	var obj DamlPackage
	if err := yaml.UnmarshalWithOptions(expanded, &obj); err != nil {
		return nil, err
	}

	if obj.ComponentsList != nil {
		obj.Components, err = obj.ComponentsList.ToMap()
		if err != nil {
			return nil, err
		}
	}

	if obj.DeprecatedOverrideComponents != nil {
		for name, comp := range obj.DeprecatedOverrideComponents {
			comp.Name = name
		}

		if obj.Components != nil {
			return nil, fmt.Errorf("fields 'components' and 'override-components' cannot be both simultaneously specified. Prefer 'components' as 'override-components' is deprecated")
		}

		obj.Components = make(map[string]*sdkmanifest.Component)
		maps.Copy(obj.Components, obj.DeprecatedOverrideComponents)

		// zero it out to make sure we really aren't relying on it past this point
		obj.DeprecatedOverrideComponents = nil
	}

	if assistantconfig.DpmLockfileEnabled() {
		_, defaultLocation, err := obj.ArtifactLocations.GetDefaultLocation()
		if err != nil {
			return nil, fmt.Errorf("invalid artifact locations: %w", err)
		}

		if len(obj.Dependencies) > 0 {
			obj.ResolvedDependencies, err = obj.computeResolvedDependencies(defaultLocation)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve provided dependencies: %w", err)
			}
		}
	}

	return &obj, nil
}
