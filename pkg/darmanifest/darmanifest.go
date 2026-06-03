package darmanifest

import (
	"errors"
	"fmt"
	"os"

	"daml.com/x/assistant/pkg/schema"
	"github.com/goccy/go-yaml"
)

const (
	DarKind          = "Dar"
	DarSchemaVersion = "v1"
	DarAPIVersion    = DarSchemaVersion
)

var ErrInvalidDarManifest = fmt.Errorf("invalid dar manifest")

type DarManifest struct {
	schema.ManifestMeta `yaml:",inline"`
	Spec                *Spec `yaml:"spec"`
}

type Spec struct {
	Paths []string `yaml:"paths"`
}

func ReadDarManifest(filePath string) (*DarManifest, error) {
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return ReadDarManifestContents(bytes)
}

func ReadDarManifestContents(contents []byte) (*DarManifest, error) {
	var d DarManifest
	if err := yaml.UnmarshalWithOptions(contents, &d, yaml.Strict()); err != nil {
		return nil, errors.Join(ErrInvalidDarManifest, err)
	}

	s := schema.ManifestMeta{
		APIVersion: DarAPIVersion,
		Kind:       DarKind,
	}

	err := s.ValidateSchema(d.ManifestMeta)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidDarManifest, err.Error())
	}

	if d.Spec == nil || d.Spec.Paths == nil || len(d.Spec.Paths) == 0 {
		return nil, fmt.Errorf("%w: missing required .spec.paths field", ErrInvalidDarManifest)
	}

	return &d, nil
}
