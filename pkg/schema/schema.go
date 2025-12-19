// Copyright (c) 2017-2025 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"fmt"
)

const (
	APIGroup = "digitalasset.com"
)

type ManifestMeta struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
}

func (m ManifestMeta) ValidateSchema(target ManifestMeta) error {
	if target.Kind == "" {
		return fmt.Errorf("missing required field 'kind'")
	} else if target.Kind != m.Kind {
		return fmt.Errorf("unsupported kind %q. expected %q", target.Kind, m.Kind)
	}

	if target.APIVersion == "" {
		return fmt.Errorf("missing required field 'apiVersion'")
	}
	if target.APIVersion != m.APIVersion {
		return fmt.Errorf("unsupported apiVersion %q. expected %q", target.APIVersion, m.APIVersion)
	}

	return nil
}
