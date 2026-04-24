package componentlist

import (
	"errors"
	"fmt"
	"strings"

	"daml.com/x/assistant/pkg/sdkmanifest"
	"github.com/Masterminds/semver/v3"
	"github.com/goccy/go-yaml"
	"oras.land/oras-go/v2/registry"
)

var SchemaError = fmt.Errorf(`component must be one of "<name>:<version>", "oci://<reference>" or {name: "<name>", path: "<path to component directory>"}`)

type ComponentList []*ComponentEntry

type ComponentEntry struct {
	StringBased *string
	FileBased   *FileBased
}

type FileBased struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

func (e *ComponentEntry) UnmarshalYAML(b []byte) error {
	var s string
	if err := yaml.Unmarshal(b, &s); err == nil {
		e.StringBased = &s
		return nil
	}

	var fc FileBased
	if err := yaml.Unmarshal(b, &fc); err == nil {
		e.FileBased = &fc
		return nil
	}

	return SchemaError
}

func (compList ComponentList) ToMap() (map[string]*sdkmanifest.Component, error) {
	compMap := make(map[string]*sdkmanifest.Component)
	var errs []error

	for _, entry := range compList {
		name, comp, err := entry.toComponent()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		compMap[name] = comp
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("%w: %w", SchemaError, errors.Join(errs...))
	}
	return compMap, nil
}

func (e *ComponentEntry) toComponent() (string, *sdkmanifest.Component, error) {
	if e.FileBased != nil {
		return fromFileBasedComponent(e.FileBased)
	}
	if e.StringBased != nil {
		return fromStringBasedComponent(*e.StringBased)
	}
	return "", nil, fmt.Errorf("invalid component entry")
}

func fromFileBasedComponent(c *FileBased) (string, *sdkmanifest.Component, error) {
	if c.Name == "" {
		return "", nil, fmt.Errorf("file-based component is missing name")
	}
	if c.Path == "" {
		return "", nil, fmt.Errorf("file-based component %q is missing path", c.Name)
	}

	return c.Name, &sdkmanifest.Component{
		Name:      c.Name,
		LocalPath: &c.Path,
	}, nil
}

func fromStringBasedComponent(c string) (string, *sdkmanifest.Component, error) {
	if strings.HasPrefix(c, "oci://") { // oci://whatever.dev/foo/bar/comp:1.2.3
		u, err := registry.ParseReference(strings.TrimPrefix(c, "oci://"))
		if err != nil {
			return "", nil, fmt.Errorf("couldn't parse component url %q: %w", c, err)
		}
		name := fmt.Sprintf("%s/%s", u.Registry, u.Repository)

		return name, &sdkmanifest.Component{Name: name, Uri: &c}, nil
	} else if strings.Contains(c, ":") && !strings.Contains(c, "/") {
		// e.g. "damlc:1.2.3"

		parts := strings.Split(c, ":")
		name, version := parts[0], parts[1]

		semVer, err := semver.StrictNewVersion(version)
		if err != nil {
			return "", nil, fmt.Errorf("couldn't parse component %q: %w", c, err)
		}

		return name, &sdkmanifest.Component{Name: name, Version: sdkmanifest.AssemblySemVer(semVer)}, nil
	}

	return "", nil, fmt.Errorf("couldn't parse component %q", c)
}
