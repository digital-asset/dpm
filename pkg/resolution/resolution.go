// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package resolution

import (
	"maps"

	"daml.com/x/assistant/cmd/dpm/cmd/resolve/resolutionerrors"
	"daml.com/x/assistant/pkg/schema"
)

const (
	ApiVersion = "v1"
	Kind       = "Resolution"

	DarImportsFields = "resolved-data-dependencies"
)

type Resolution struct {
	schema.ManifestMeta `yaml:",inline"`
	Packages            Packages   `yaml:"packages"`
	DefaultSDK          DefaultSDK `yaml:"default-sdk"`
}

// Packages is a <package path> -> Package mapping
type Packages map[string]*Package

// DefaultSDK is a single-entry <currently active sdk version> -> Package mapping
type DefaultSDK map[string]*Package

type Package struct {
	Errors       []*resolutionerrors.ResolutionError `yaml:"errors,omitempty"`
	Components   map[string]string                   `yaml:"components,omitempty"`
	ComponentsV2 map[string]map[string]string        `yaml:"componentsV2,omitempty"`
	Imports      Imports                             `yaml:"imports,omitempty"`
	SdkVersion   string                              `yaml:"sdk-version"`
}

// Imports is export Var -> paths list mapping
type Imports map[string][]string

const (
	ResolvedDependenciesField     = "resolved-dependencies"
	ResolvedDataDependenciesField = "resolved-data-dependencies"
)

func (pkg *Package) GetResolvedDependencies() []string {
	result, ok := pkg.Imports[ResolvedDependenciesField]
	if !ok {
		return []string{}
	}
	return result
}

func (pkg *Package) GetResolvedDataDependencies() []string {
	result, ok := pkg.Imports[ResolvedDataDependenciesField]
	if !ok {
		return []string{}
	}
	return result
}

func (pkg *Package) GetNonDarDependenciesImports() Imports {
	result := make(Imports)
	maps.Copy(result, pkg.Imports)
	delete(result, ResolvedDependenciesField)
	delete(result, ResolvedDataDependenciesField)
	return result
}
