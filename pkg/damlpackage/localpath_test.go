// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package damlpackage

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalDarPathsPreserveFilenameCharacters(t *testing.T) {
	for _, name := range []string{"a b.dar", "a%20b.dar", "100%.dar", "a#b.dar", "a?b.dar", "café.dar"} {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "workspace with spaces")
			p := &DamlPackage{AbsolutePath: filepath.Join(root, "daml.yaml")}
			path := filepath.Join(root, name)
			parsed, err := p.parseLocations([]*RawDependency{{ValueOnly: &path}}, nil)
			require.NoError(t, err)
			dep := parsed[path]
			require.NotNil(t, dep)
			assert.Equal(t, "file", dep.FullUrl.Scheme)
			assert.Equal(t, filepath.ToSlash(path), dep.FullUrl.Path)
			assert.Empty(t, dep.FullUrl.Fragment)
			assert.Empty(t, dep.FullUrl.RawQuery)
		})
	}
}
