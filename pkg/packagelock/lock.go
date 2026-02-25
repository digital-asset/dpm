package packagelock

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/ocilister"
	"daml.com/x/assistant/pkg/schema"
	"daml.com/x/assistant/pkg/utils/stringset"
	"github.com/goccy/go-yaml"
	"oras.land/oras-go/v2/registry"
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
	URI    string `yaml:"uri"`
	Digest string `yaml:"digest,omitempty"`

	Dependency *damlpackage.ResolvedDependency `yaml:"-"`
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

// toDiffableMap separates the tag from the rest of the URI which is helpful for
// diffing existing lockfile against the expected one
// example: { "oci://example.com/foo/bar" -> [latest, 3.4], ... }
func (l *PackageLock) toDiffableMap() (map[string]stringset.StringSet, error) {
	m := map[string]stringset.StringSet{}
	for _, d := range l.Dars {
		ref, err := registry.ParseReference(strings.TrimPrefix(d.URI, "oci://"))
		if err != nil {
			return nil, err
		}
		k := fmt.Sprintf("%s/%s", ref.Registry, ref.Repository)

		if _, ok := m[k]; !ok {
			m[k] = make(stringset.StringSet)
		}
		m[k].Add(ref.Reference)
	}
	return m, nil
}

// isInSync checks whether this (existing) lockfile matches an expected lockfile.
// it takes into account the fact that tags in the expected lockfile might be floaty
func (l *PackageLock) isInSync(expected *PackageLock) (bool, error) {
	expectedMap, err := expected.toDiffableMap()
	if err != nil {
		return false, err
	}
	existingMap, err := l.toDiffableMap()
	if err != nil {
		return false, err
	}

	if len(existingMap) != len(expectedMap) {
		return false, nil
	}

	for k, xs := range expectedMap {
		ys, ok := existingMap[k]
		if !ok {
			return false, nil
		}

		if len(xs) != len(ys) {
			return false, nil
		}

		for x := range xs {
			if ocilister.IsFloaty(x) {
				continue
			}
			if !ys.Contains(x) {
				return false, nil
			}
		}
	}

	return true, nil
}
