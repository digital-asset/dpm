package damlpackage

import (
	"fmt"

	"github.com/goccy/go-yaml"
)

var RawDependenciesSchemaErr = fmt.Errorf("dar dependencies fields must be of type string or '{ value: <string>, main-package-id: <string> }' object")

// RawDependency is the 'string | {...}' sum-type for
// dependencies / data-dependencies YAML fields
type RawDependency struct {
	ValueOnly     *string
	WithPackageId *withPackageId
}

type withPackageId struct {
	Value         string `yaml:"value"`
	MainPackageId string `yaml:"main-package-id"`
}

func (r *RawDependency) Value() (string, error) {
	switch {
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

	var obj withPackageId
	if err := yaml.Unmarshal(b, &obj); err == nil {
		r.WithPackageId = &obj
		return nil
	}

	return RawDependenciesSchemaErr
}

func (r *RawDependency) MarshalYAML() (any, error) {
	switch {
	case r.WithPackageId != nil:
		return *r.WithPackageId, nil
	case r.ValueOnly != nil:
		return r.ValueOnly, nil
	default:
		return nil, nil
	}
}
