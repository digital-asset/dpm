// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sdkmanifest

import (
	"fmt"

	ociconsts "daml.com/x/assistant/pkg/oci"
	"github.com/goccy/go-yaml"
)

type Edition int

const (
	OpenSource Edition = iota
	Enterprise
	Private
)

func ParseEdition(s string) (Edition, error) {
	switch s {
	case "open-source":
		return OpenSource, nil
	case "enterprise":
		return Enterprise, nil
	case "private":
		return Private, nil
	default:
		return 0, fmt.Errorf("%w: invalid edition. must be one of 'open-source', 'enterprise', 'private'", ErrInvalidAssemblyManifest)
	}
}

func (e Edition) String() string {
	switch e {
	case OpenSource:
		return "open-source"
	case Enterprise:
		return "enterprise"
	case Private:
		return "private"
	default:
		return "Unknown"
	}
}

func (e Edition) SdkManifestsRepo() (string, error) {
	switch e {
	case OpenSource:
		return ociconsts.SdkManifestsOpenSourceRepo, nil
	case Enterprise:
		return ociconsts.SdkManifestsEnterpriseRepo, nil
	case Private:
		return ociconsts.SdkManifestsPrivateRepo, nil
	default:
		return "", fmt.Errorf("unknown edition %v", e)
	}
}

func (e *Edition) UnmarshalYAML(data []byte) error {
	var unmarshalled string
	if err := yaml.Unmarshal(data, &unmarshalled); err != nil {
		return fmt.Errorf("failed to unmarshal edition: %w", err)
	}
	s, err := ParseEdition(unmarshalled)
	if err != nil {
		return err
	}

	*e = s
	return nil
}

func (e *Edition) MarshalYAML() ([]byte, error) {
	s := e.String()
	if s == "Unknown" {
		return nil, fmt.Errorf("%w: invalid edition enum value", ErrInvalidAssemblyManifest)
	}
	return []byte(s), nil
}

var _ yaml.BytesUnmarshaler = (*Edition)(nil)
var _ yaml.BytesMarshaler = (*Edition)(nil)
