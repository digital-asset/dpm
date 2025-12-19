// Copyright (c) 2017-2025 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sdkmanifest

import (
	"fmt"
	"os"
	"path/filepath"

	"daml.com/x/assistant/pkg/schema"
	"github.com/Masterminds/semver/v3"
	"github.com/goccy/go-yaml"
)

var ErrInvalidAssemblyManifest = fmt.Errorf("invalid assembly manifest")
var MissingAssemblyField = fmt.Errorf("%w: a required field is missing", ErrInvalidAssemblyManifest)

const (
	SdkManifestKind       = "SdkManifest"
	SdkManifestVersion    = "v1"
	SdkManifestAPIVersion = schema.APIGroup + "/" + SdkManifestVersion
)

type SdkManifest struct {
	AbsolutePath string `yaml:"-"`

	schema.ManifestMeta `yaml:",inline"`
	Spec                *Spec `yaml:"spec"`
}

type SemVer semver.Version

func AssemblySemVer(v *semver.Version) *SemVer {
	a := SemVer(*v)
	return &a
}

func (v *SemVer) Value() semver.Version {
	return (semver.Version)(*v)
}

func (v *SemVer) UnmarshalYAML(data []byte) error {
	var versionStr string
	if err := yaml.Unmarshal(data, &versionStr); err != nil {
		return fmt.Errorf("failed to unmarshal 'version': %w", err)
	}
	parsedVersion, err := semver.NewVersion(versionStr)
	if err != nil {
		return fmt.Errorf("invalid semantic version: %w", err)
	}
	*v = SemVer(*parsedVersion)
	return nil
}

func (v *SemVer) MarshalYAML() ([]byte, error) {
	return []byte(v.Value().String()), nil
}

var _ yaml.BytesUnmarshaler = (*SemVer)(nil)
var _ yaml.BytesMarshaler = (*SemVer)(nil)

func ReadSdkManifest(filePath string) (*SdkManifest, error) {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}
	bytes, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	return ReadSdkManifestContents(bytes, filePath)
}

func ReadSdkManifestContents(contents []byte, absPath string) (*SdkManifest, error) {
	var c SdkManifest
	if err := yaml.Unmarshal(contents, &c); err != nil {
		return nil, err
	}

	s := schema.ManifestMeta{
		APIVersion: SdkManifestAPIVersion,
		Kind:       SdkManifestKind,
	}
	if err := s.ValidateSchema(c.ManifestMeta); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidAssemblyManifest, err.Error())
	}

	if c.Spec == nil {
		return nil, fmt.Errorf("%w: 'spec'", MissingAssemblyField)
	}

	c.AbsolutePath = absPath
	return &c, nil
}
