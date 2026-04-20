// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package multipackage

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"daml.com/x/assistant/pkg/utils"
	"github.com/samber/lo"
	"oras.land/oras-go/v2/registry"

	"daml.com/x/assistant/pkg/sdkmanifest"
	"github.com/goccy/go-yaml"
)

type MultiPackage struct {
	SdkVersion   string   `yaml:"sdk-version"`
	AbsolutePath string   `yaml:"-"`
	Packages     []string `yaml:"packages"`

	// yaml:components expected to be a list of strings
	ComponentsList ComponentList `yaml:"components"` // parse as string, generate map from the list
	// deprecated in favor of Components
	DeprecatedOverrideComponents map[string]*sdkmanifest.Component `yaml:"override-components"`
	Components                   map[string]*sdkmanifest.Component
}

type ComponentList []string

func (compList ComponentList) GenerateAsMap() (map[string]*sdkmanifest.Component, error) {
	var compMap map[string]*sdkmanifest.Component
	var errs []error

	for _, c := range compList {
		if strings.HasPrefix(c, "oci://") { // oci://whatever.dev/foo/bar/comp:1.2.3
			u, err := registry.ParseReference(strings.TrimPrefix(c, "oci://"))
			if err != nil {
				errs = append(errs, fmt.Errorf("couldn't parse component url %q: %w", c, err))
				continue
			}
			registryList := strings.Split(u.Registry, "/")
			compMap[u.Reference] = &sdkmanifest.Component{Name: registryList[len(registryList)-1], Uri: u.String()}
		} else if strings.HasPrefix(c, "http://") || strings.HasPrefix(c, "https://") {
			// TODO
			errs = append(errs, fmt.Errorf("couldn't parse component %q: http dependencies not yet supported", c))
			continue
		} else if strings.HasPrefix(c, ".") {
			// TODO
			errs = append(errs, fmt.Errorf("couldn't parse dependency %q: file paths not yet supported", c))
			continue
		} else if strings.HasPrefix(c, "@") {
			errs = append(errs, fmt.Errorf("couldn't parse component %q: aliases not yet supported", c))
			continue
			// parsed := regex.FindStringSubmatch(d)
			// if len(parsed) < 2 {
			// 	errs = append(errs, fmt.Errorf("error parsing dependency %q: Dependencies beginning with @ must be of the form '@<artifact-location>/<suffix>'", d))
			// 	continue
			// }
			// location, ok := p.ArtifactLocations[parsed[1]]
			// if !ok {
			// 	errs = append(errs, fmt.Errorf("dependency %q has no corresponding artifact location", d))
			// 	continue
			// }

			// if location.Url == "" {
			// 	errs = append(errs, fmt.Errorf("invalid artifact location %q. Must have a non-empty url", location.Url))
			// 	continue
			// }

			// rawUrl := strings.Replace(d, parsed[1], location.Url, 1)
			// u, err := url.Parse(rawUrl)
			// if err != nil {
			// 	errs = append(errs, fmt.Errorf("couldn't parse full url %q for dependency %q: ", rawUrl, d))
			// 	continue
			// }
			// resolved[d] = &ResolvedDependency{
			// 	Location: location,
			// 	FullUrl:  u,
			// }
		} else if strings.Contains(c, ":") {
			// if defaultLocation == nil {
			// 	errs = append(errs, fmt.Errorf("failed to resolve dependency's artifact location for %q: no default artifact location is specified", d))
			// 	continue
			// }
			registryList := strings.Split(u.Registry, "/")
			compMap[registryList[0]] = &sdkmanifest.Component{Name: registryList[0], Version: registryList[len(registryList)-1]}
		} else {
			// builtin libs (like "daml-script")
			errs = append(errs, fmt.Errorf("couldn't parse full yaml for component %q: ", c))
			continue
		}
	}
	return compMap, nil
}

// func (v *ComponentList) UnmarshalYAML(data []byte) error {

// }

// func (v *ComponentList) MarshalYAML() ([]byte, error) {
// 	return nil, fmt.Errorf("not implemented")
// }

// var _ yaml.BytesUnmarshaler = (*ComponentList)(nil)
// var _ yaml.BytesMarshaler = (*ComponentList)(nil)

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

	if obj.Components != nil {
		for name, comp := range obj.Components {
			comp.Name = name // fxn parseComponentsinList()
		}
	}
	// obj.Components := c.asdas.MakeMapFromList()

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
