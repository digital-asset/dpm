package damlpackage

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
)

var RawDependenciesSchemaErr = fmt.Errorf("dar dependencies fields must be of type string, structured git object, or '{ value: <string>, main-package-id: <string> }' object")

// GitStructuredFields is the structured git dependency form in daml.yaml.
type GitStructuredFields struct {
	URL     string `yaml:"url"`
	Ref     string `yaml:"ref"`
	Path    string `yaml:"path"`
	Release string `yaml:"release"`
	Asset   string `yaml:"asset"`
}

// RawDependency is the 'string | {...}' sum-type for
// dependencies / data-dependencies YAML fields
type RawDependency struct {
	ValueOnly     *string
	WithPackageId *withPackageId
	GitStructured *GitStructuredFields
}

type withPackageId struct {
	Value         string `yaml:"value"`
	MainPackageId string `yaml:"main-package-id"`
}

type gitStructuredEntry struct {
	Git GitStructuredFields `yaml:"git"`
}

func (r *RawDependency) Value() (string, error) {
	switch {
	case r.GitStructured != nil:
		return FormatGitStructuredLine(r.GitStructured)
	case r.WithPackageId != nil:
		return r.WithPackageId.Value, nil
	case r.ValueOnly != nil:
		return *r.ValueOnly, nil
	default:
		return "", RawDependenciesSchemaErr
	}
}

func (r *RawDependency) GetMainPackageId() *string {
	if r.WithPackageId == nil {
		return nil
	}
	return &r.WithPackageId.MainPackageId
}

func (r *RawDependency) UnmarshalYAML(b []byte) error {
	var s string
	if err := yaml.Unmarshal(b, &s); err == nil {
		r.ValueOnly = &s
		return nil
	}

	var gitEntry gitStructuredEntry
	if err := yaml.Unmarshal(b, &gitEntry); err == nil && gitEntry.Git.URL != "" {
		r.GitStructured = &gitEntry.Git
		return nil
	}

	var obj withPackageId
	if err := yaml.Unmarshal(b, &obj); err == nil {
		if strings.TrimSpace(obj.Value) == "" {
			return RawDependenciesSchemaErr
		}
		r.WithPackageId = &obj
		return nil
	}

	return RawDependenciesSchemaErr
}

func (r *RawDependency) MarshalYAML() (any, error) {
	switch {
	case r.GitStructured != nil:
		return gitStructuredEntry{Git: *r.GitStructured}, nil
	case r.WithPackageId != nil:
		return *r.WithPackageId, nil
	case r.ValueOnly != nil:
		return r.ValueOnly, nil
	default:
		return nil, nil
	}
}
