package packagelock

import (
	"fmt"
	"os"
	"path/filepath"

	"daml.com/x/assistant/pkg/schema"
	"github.com/goccy/go-yaml"
)

const (
	PackageLockKind       = "PackageLock"
	PackageLockVersion    = "v1"
	PackageLockAPIVersion = schema.APIGroup + "/" + PackageLockVersion
)

var ErrInvalidPackageLock = fmt.Errorf("invalid package lock")

type PackageLock struct {
	schema.ManifestMeta `yaml:",inline"`
	Dars                []*Dar `yaml:"dars"`
}

type Dar struct {
	Name   string `yaml:"name"`
	Digest string `yaml:"digest"`
}

func ReadPackageLock(filePath string) (*PackageLock, error) {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}
	bytes, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	return ReadPackageLockContents(bytes)
}

func ReadPackageLockContents(contents []byte) (*PackageLock, error) {
	var c PackageLock
	if err := yaml.Unmarshal(contents, &c); err != nil {
		return nil, err
	}

	s := schema.ManifestMeta{
		APIVersion: PackageLockAPIVersion,
		Kind:       PackageLockKind,
	}
	if err := s.ValidateSchema(c.ManifestMeta); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidPackageLock, err.Error())
	}

	return &c, nil
}
