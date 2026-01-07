// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package multipackage

import (
	"daml.com/x/assistant/pkg/utils"
	"github.com/samber/lo"
	"os"
	"path/filepath"

	"daml.com/x/assistant/pkg/sdkmanifest"
	"github.com/goccy/go-yaml"
)

type MultiPackage struct {
	AbsolutePath       string                            `yaml:"-"`
	Packages           []string                          `yaml:"packages"`
	OverrideComponents map[string]*sdkmanifest.Component `yaml:"override-components"`
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
	if err := yaml.UnmarshalWithOptions(contents, &obj, yaml.Strict()); err != nil {
		return nil, err
	}
	if obj.OverrideComponents != nil {
		for name, comp := range obj.OverrideComponents {
			comp.Name = name
		}
	}
	obj.AbsolutePath = absPath
	return &obj, nil
}
