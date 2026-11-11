// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package resolver

import (
	"net/url"
	"testing"

	"daml.com/x/assistant/pkg/damlpackage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveDarReturnsFilesystemPaths(t *testing.T) {
	for _, path := range []string{"/workspace with spaces/a.dar", "/workspace/a%20b.dar", "/workspace/a#b?.dar", "/workspace/café.dar"} {
		t.Run(path, func(t *testing.T) {
			dep := &damlpackage.ParsedDarDependency{FullUrl: &url.URL{Scheme: "file", Path: path}}
			got, err := (&DeepResolver{}).resolveDar(dep)
			require.NoError(t, err)
			assert.Equal(t, []string{path}, got)
		})
	}
	for _, tc := range []struct{ uri, want string }{
		{"builtin://daml-script", "daml-script"},
		{"file://C:/Users/Alice%20Smith/test.dar", "C:/Users/Alice Smith/test.dar"},
		{"file:////server/share/a%20b.dar", "//server/share/a b.dar"},
	} {
		t.Run(tc.uri, func(t *testing.T) {
			u, err := url.Parse(tc.uri)
			require.NoError(t, err)
			got, err := (&DeepResolver{}).resolveDar(&damlpackage.ParsedDarDependency{FullUrl: u})
			require.NoError(t, err)
			assert.Equal(t, []string{tc.want}, got)
		})
	}
}
