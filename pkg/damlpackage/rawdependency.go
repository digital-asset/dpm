package damlpackage

import (
	"fmt"
	"strings"

	"daml.com/x/assistant/pkg/gitparse"
	"github.com/goccy/go-yaml"
)

var RawDependenciesSchemaErr = fmt.Errorf("dar dependencies fields must be of type string, structured git object, or '{ value: <string>, main-package-id: <string> }' object")

// RawDependency is the 'string | {...}' sum-type for
// dependencies / data-dependencies YAML fields
type RawDependency struct {
	ValueOnly     *string
	WithPackageId *withPackageId
	GitStructured *gitparse.GitStructuredFields
	// GitMainPackageId is the main-package-id sibling of a structured git entry.
	// It is set only when that key is present.
	GitMainPackageId *string
}

type withPackageId struct {
	Value         string `yaml:"value"`
	MainPackageId string `yaml:"main-package-id"`
}

// gitStructuredEntry is a structured git dependency.
// main-package-id is the only key allowed beside git.
type gitStructuredEntry struct {
	Git           *gitparse.GitStructuredFields `yaml:"git"`
	MainPackageId *string                       `yaml:"main-package-id,omitempty"`
}

func (r *RawDependency) Value() (string, error) {
	switch {
	case r.GitStructured != nil:
		return gitparse.FormatGitStructuredLine(r.GitStructured)
	case r.WithPackageId != nil:
		return r.WithPackageId.Value, nil
	case r.ValueOnly != nil:
		return *r.ValueOnly, nil
	default:
		return "", RawDependenciesSchemaErr
	}
}

func (r *RawDependency) GetMainPackageId() *string {
	if r == nil {
		return nil
	}
	if r.WithPackageId != nil {
		return &r.WithPackageId.MainPackageId
	}
	return r.GitMainPackageId
}

func (r *RawDependency) UnmarshalYAML(b []byte) error {
	var s string
	if err := yaml.Unmarshal(b, &s); err == nil {
		r.ValueOnly = &s
		return nil
	}

	var asMap map[string]any
	if err := yaml.Unmarshal(b, &asMap); err == nil {
		if _, ok := asMap["git"]; ok {
			var gitEntry gitStructuredEntry
			// Strict applies to the git mapping and to its sibling keys, so a
			// misspelt field such as assets, or a sibling other than
			// main-package-id, is rejected instead of ignored.
			if err := yaml.UnmarshalWithOptions(b, &gitEntry, yaml.Strict()); err != nil {
				return fmt.Errorf("git dependency: %w", err)
			}
			if gitEntry.Git == nil {
				gitEntry.Git = &gitparse.GitStructuredFields{}
			}
			r.GitStructured = gitEntry.Git
			if gitEntry.MainPackageId != nil {
				id := *gitEntry.MainPackageId
				r.GitMainPackageId = &id
			}
			return nil
		}
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
		return gitStructuredEntry{Git: r.GitStructured, MainPackageId: r.GitMainPackageId}, nil
	case r.WithPackageId != nil:
		return *r.WithPackageId, nil
	case r.ValueOnly != nil:
		return r.ValueOnly, nil
	default:
		return nil, nil
	}
}
