// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package assistantremote

import (
	"path/filepath"
	"testing"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/utils"
	"github.com/stretchr/testify/require"
)

// An auth config path that cannot be read must fail here, not later at the
// registry. oras returns an empty config and no error for a missing file, so
// without an explicit check the client is silently unauthenticated and the
// first failure arrives as a 401 or 403 from the registry, which points at
// permissions rather than at the path.
func TestNewFailsOnUnreadableAuthConfig(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-config.json")

	_, err := New("registry.example.com", missing, false)

	require.ErrorContains(t, err, "failed to read registry auth config")
	require.ErrorContains(t, err, missing)
}

// Each Remote must own its auth client. auth.DefaultClient is a package-level
// pointer, so a Remote that assigned to its fields would overwrite the
// credential of every Remote already built, and hand its own credential to
// every Remote built afterwards for a different registry.
func TestNewDoesNotShareTheDefaultAuthClient(t *testing.T) {
	withMagicEnv(t, func() {
		dir, deleteFn, err := utils.MkdirTemp("", "")
		require.NoError(t, err)
		t.Cleanup(func() { _ = deleteFn() })

		config, err := assistantconfig.GetWithCustomDamlHome(dir)
		require.NoError(t, err)

		withAuth, err := NewFromConfig(config)
		require.NoError(t, err)

		// A second Remote for an unrelated registry, with no auth config.
		t.Setenv("DOCKER_CONFIG", t.TempDir())
		withoutAuth, err := New("other.example.com", "", false)
		require.NoError(t, err)

		require.NotSame(t, withAuth.client, withoutAuth.client,
			"each Remote must have its own auth client")
		require.NotNil(t, withAuth.client.Credential,
			"the first Remote must keep the credential it was built with")
	})
}
