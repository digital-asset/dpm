// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package multipackage

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"daml.com/x/assistant/pkg/componentlist"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/utils"
	"github.com/goccy/go-yaml"
	"github.com/samber/lo"
)

type MultiPackage struct {
	SdkVersion   string   `yaml:"sdk-version"`
	AbsolutePath string   `yaml:"-"`
	Packages     []string `yaml:"packages"`

	ComponentsList componentlist.ComponentList       `yaml:"components"`
	Components     map[string]*sdkmanifest.Component `yaml:"-"`

	// deprecated in favor of Components
	DeprecatedOverrideComponents map[string]*sdkmanifest.Component `yaml:"override-components"`
}

func (m *MultiPackage) AbsolutePackages() []string {
	return lo.Map(m.Packages, func(p string, index int) string {
		return utils.ResolvePath(filepath.Dir(m.AbsolutePath), p)
	})
}

// IncludesDamlPackage returns true if this multi-package references the given daml package
// (given as absolute path to its daml.yaml)
func (m *MultiPackage) IncludesDamlPackage(damlPackageAbsPath string) (ok bool, err error) {
	d := filepath.Dir(m.AbsolutePath)
	for _, p := range m.Packages {
		properPath := p
		if !filepath.IsAbs(p) {
			properPath, err = filepath.Abs(filepath.Join(d, p))
			if err != nil {
				return false, err
			}
		}
		if properPath == filepath.Dir(damlPackageAbsPath) {
			return true, nil
		}
	}
	return
}

func Read(filePath string) (*MultiPackage, error) {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}

	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return ReadFromContents(bytes, abs)
}

func ReadFromContents(contents []byte, absPath string) (*MultiPackage, error) {
	var obj MultiPackage
	var err error

	if err = yaml.UnmarshalWithOptions(contents, &obj, yaml.Strict()); err != nil {
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

	obj.AbsolutePath = absPath
	return &obj, nil
}
