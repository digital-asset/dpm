// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package simpleplatform

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/goccy/go-yaml"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

const GenericPlatformStr = "generic"

type Platform interface {
	yaml.BytesUnmarshaler
	yaml.BytesMarshaler
	IsGeneric() bool
	String() string
	ImageTag(rawTag string) string
	Equal(platform Platform) bool
}

func ParsePlatform(platformStr string) (Platform, error) {
	if platformStr == GenericPlatformStr {
		return &Generic{}, nil
	}

	parts := strings.Split(platformStr, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("failed to parse platform %q: expected format os/arch]", platformStr)
	}

	return &NonGeneric{
		OS:           parts[0],
		Architecture: parts[1],
	}, nil
}

func FromOras(platform *v1.Platform) Platform {
	if platform == nil {
		return &Generic{}
	}
	return &NonGeneric{OS: platform.OS, Architecture: platform.Architecture}
}

type NonGeneric struct {
	// Architecture field specifies the CPU architecture, for example
	// `amd64` or `ppc64le`.
	Architecture string

	// OS specifies the operating system, for example `linux` or `windows`.
	OS string
}

func (p *NonGeneric) Equal(platform Platform) bool {
	nonGeneric, ok := platform.(*NonGeneric)
	return ok && nonGeneric.OS == p.OS && nonGeneric.Architecture == nonGeneric.Architecture
}

func (p *NonGeneric) IsGeneric() bool {
	return false
}

func (p *NonGeneric) String() string {
	return fmt.Sprintf("%s/%s", p.OS, p.Architecture)
}

func (p *NonGeneric) MarshalYAML() ([]byte, error) {
	return []byte(p.String()), nil
}

func (p *NonGeneric) UnmarshalYAML(bytes []byte) error {
	var unmarshalled string
	if err := yaml.Unmarshal(bytes, &unmarshalled); err != nil {
		return fmt.Errorf("failed to unmarshal NonGeneric platform: %w", err)
	}
	parsed, err := ParsePlatform(unmarshalled)
	if err != nil {
		return err
	}

	nonGeneric, ok := (parsed).(*NonGeneric)
	if !ok {
		return fmt.Errorf("expected a platform specific string in the form '<OS>/<arch>'")
	}
	*p = *nonGeneric
	return nil
}

func (p *NonGeneric) ImageTag(rawTag string) string {
	return fmt.Sprintf("%s.%s_%s", rawTag, p.OS, p.Architecture)
}

func (p *NonGeneric) ToOras() *v1.Platform {
	return &v1.Platform{OS: p.OS, Architecture: p.Architecture}
}

type Generic struct{}

func (p *Generic) Equal(platform Platform) bool {
	return platform.IsGeneric()
}

func (p *Generic) UnmarshalYAML(bytes []byte) error {
	var unmarshalled string
	if err := yaml.Unmarshal(bytes, &unmarshalled); err != nil {
		return fmt.Errorf("failed to unmarshal Generic platform: %w", err)
	}
	parsed, err := ParsePlatform(unmarshalled)
	if err != nil {
		return err
	}

	generic, ok := (parsed).(*Generic)
	if !ok {
		return fmt.Errorf("a generic platform must be the string %q", GenericPlatformStr)
	}
	*p = *generic
	return nil
}

func (p *Generic) MarshalYAML() ([]byte, error) {
	return []byte(p.String()), nil
}

func (p *Generic) IsGeneric() bool {
	return true
}

func (p *Generic) String() string {
	return GenericPlatformStr
}

func (p *Generic) ImageTag(rawTag string) string {
	return fmt.Sprintf("%s.%s", rawTag, p.String())
}

var _ Platform = (*NonGeneric)(nil)
var _ Platform = (*Generic)(nil)

func CurrentPlatform() *NonGeneric {
	return &NonGeneric{OS: runtime.GOOS, Architecture: runtime.GOARCH}
}
