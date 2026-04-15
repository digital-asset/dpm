// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package assistantremote

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type FakeRegistry struct {
	t *testing.T
}

const expectedSuccessBody = "okdokey"

// Tests authenticating to a registry via a docker credsStore
// https://docs.docker.com/reference/cli/docker/login/#credential-stores
func TestGetRemote(t *testing.T) {
	withMagicEnv(t, func() {
		dir, deleteFn, err := utils.MkdirTemp("", "")
		require.NoError(t, err)
		t.Cleanup(func() { _ = deleteFn() })

		config, err := assistantconfig.GetWithCustomDamlHome(dir)
		require.NoError(t, err)

		r, err := NewFromConfig(config)
		require.NoError(t, err)

		req, err := http.NewRequest("GET", r.Registry, nil)
		require.NoError(t, err)

		resp, err := r.client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, expectedSuccessBody, string(body))
	})
}

func (f *FakeRegistry) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	assert.Equal(f.t, assistantconfig.GetAssistantUserAgent(), r.UserAgent(), "wrong user-agent")

	username, password, ok := r.BasicAuth()
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if username == "meep" && password == "meep!" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedSuccessBody))
	} else {
		http.Error(w, "wrong username/password", http.StatusUnauthorized)
	}
}

func withMagicEnv(t *testing.T, fn func()) {
	registryServer := httptest.NewServer(&FakeRegistry{t})
	defer registryServer.Close()

	credHelperDir, err := filepath.Abs(filepath.Join("testdata"))
	require.NoError(t, err)

	goos := "windows"
	if runtime.GOOS != "windows" {
		goos = "unix"
	}
	binPath := filepath.Join(credHelperDir, goos)

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", binPath+string(os.PathListSeparator)+oldPath)
	t.Setenv(assistantconfig.OciRegistryEnvVar, registryServer.URL)
	t.Setenv(assistantconfig.RegistryAuthConfigPathEnvVar, filepath.Join(credHelperDir, "fake-docker-config.json"))

	fn()
}
