// Copyright (c) 2017-2025 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package resolution

import (
	"daml.com/x/assistant/cmd/dpm/cmd/resolve/resolutionerrors"
	"daml.com/x/assistant/pkg/schema"
)

const (
	ApiVersion = "v1"
	Kind       = "Resolution"
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
	Errors     []*resolutionerrors.ResolutionError `yaml:"errors,omitempty"`
	Components map[string]string                   `yaml:"components,omitempty"`
	Imports    Imports                             `yaml:"imports,omitempty"`
}

// Imports is export Var -> paths list mapping
type Imports map[string][]string
