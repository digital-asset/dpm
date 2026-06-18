// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package damlpackage

import (
	"fmt"
	"maps"
	"os"
	"strings"

	"daml.com/x/assistant/pkg/componentlist"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"github.com/goccy/go-yaml"
)

type ParsedDarDependencies struct {
	Dependencies     map[string]*ParsedDarDependency
	DataDependencies map[string]*ParsedDarDependency
}

type DamlPackage struct {
	SdkVersion string `yaml:"sdk-version"`

	ComponentsList componentlist.ComponentList       `yaml:"components,omitempty"`
	Components     map[string]*sdkmanifest.Component `yaml:"-"`

	// deprecated in favor of Components
	DeprecatedOverrideComponents map[string]*sdkmanifest.Component `yaml:"override-components,omitempty"`

	Dependencies          []*RawDependency       `yaml:"dependencies,omitempty"`
	DataDependencies      []*RawDependency       `yaml:"data-dependencies,omitempty"`
	ArtifactLocations     ArtifactLocations      `yaml:"artifact-locations,omitempty"`
	ParsedDarDependencies *ParsedDarDependencies `yaml:"-"`

	// absolute path to daml.yaml
	AbsolutePath string `yaml:"-"`
}

func Read(absoluteFilePath string) (*DamlPackage, error) {
	bytes, err := os.ReadFile(absoluteFilePath)
	if err != nil {
		return nil, err
	}
	return ReadFromContents(bytes, absoluteFilePath)
}

func ReadFromContents(contents []byte, absoluteFilePath string) (*DamlPackage, error) {
	expanded, err := expandEnv(contents)
	if err != nil {
		return nil, err
	}

	var obj DamlPackage
	if err := yaml.UnmarshalWithOptions(expanded, &obj); err != nil {
		return nil, err
	}

	obj.AbsolutePath = absoluteFilePath

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

	// populate the in-memory Alias field for artifact-locations
	// TODO this should really happen during unmarshalling of that field
	for alias, artifactLoc := range obj.ArtifactLocations {
		if !strings.HasPrefix(alias, "@") {
			return nil, fmt.Errorf("artifact-location alias %q is invalid. Must begin with '@'", alias)
		}
		artifactLoc.Alias = alias
	}

	obj.ParsedDarDependencies = &ParsedDarDependencies{}
	if len(obj.Dependencies) > 0 {
		obj.ParsedDarDependencies.Dependencies, err = obj.parseLocations(obj.Dependencies, obj.ArtifactLocations)
		if err != nil {
			return nil, fmt.Errorf("failed to parse provided dependencies: %w", err)
		}
	}
	if len(obj.DataDependencies) > 0 {
		obj.ParsedDarDependencies.DataDependencies, err = obj.parseLocations(obj.DataDependencies, obj.ArtifactLocations)
		if err != nil {
			return nil, fmt.Errorf("failed to parse provided data-dependencies: %w", err)
		}
	}

	return &obj, nil
}

func expandEnv(contents []byte) ([]byte, error) {
	var undefinedVars []string

	out := os.Expand(string(contents), func(key string) string {
		val, ok := os.LookupEnv(key)
		if !ok {
			undefinedVars = append(undefinedVars, key)
			return ""
		}
		return val
	})

	if len(undefinedVars) > 0 {
		return []byte{}, fmt.Errorf("environment variables used in daml.yaml are not set: %v", undefinedVars)
	}
	return []byte(out), nil
}
