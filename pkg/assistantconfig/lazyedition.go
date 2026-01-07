// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package assistantconfig

import (
	"fmt"

	"daml.com/x/assistant/pkg/sdkmanifest"

	"github.com/goccy/go-yaml"
)

var ErrMissingEdition = fmt.Errorf("edition must be specified, either via %s or %s environment variable", DpmConfigFileName, EditionEnvVar)

type LazyEdition struct {
	value *sdkmanifest.Edition
}

func (l *LazyEdition) MarshalYAML() ([]byte, error) {
	if l.value == nil {
		return nil, fmt.Errorf("edition not set")
	}
	return l.value.MarshalYAML()
}

func NewLazyEdition(value sdkmanifest.Edition) *LazyEdition {
	return &LazyEdition{&value}
}

var _ yaml.BytesUnmarshaler = (*LazyEdition)(nil)
var _ yaml.BytesMarshaler = (*LazyEdition)(nil)

func (l *LazyEdition) UnmarshalYAML(data []byte) error {
	var tmp sdkmanifest.Edition
	if err := yaml.Unmarshal(data, &tmp); err != nil {
		return err
	}
	l.value = &tmp
	return nil
}

func (l *LazyEdition) Get() (sdkmanifest.Edition, error) {
	if l.value == nil {
		return 0, ErrMissingEdition
	}
	return *l.value, nil
}
