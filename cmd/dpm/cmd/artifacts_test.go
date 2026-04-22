package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (suite *RepoSuite) TestPublishDar() {
	t := suite.T()

	testutil.StartRegistry(t)

	tmpDamlHome, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDamlHome)

	publishDar(t)
	testutil.StartRegistry(t)
	destinationRegistry := os.Getenv(assistantconfig.OciRegistryEnvVar)
	tmpDamlHome, err = os.MkdirTemp("", "")
	require.NoError(t, err)
	t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDamlHome)

	cmd := createStdTestRootCmd(t)
	args := []string{
		"artifacts", "publish", "dar", "--name", "meep", "--version", "1.2.3",
		"-f", testutil.TestdataPath(t, "test-dar"),
		"--registry", destinationRegistry,
	}
	if os.Getenv(assistantconfig.AllowInsecureRegistryEnvVar) == "true" {
		args = append(args, "--insecure")
	}
	cmd.SetArgs(args)
	require.NoError(t, cmd.Execute())
}

func (suite *RepoSuite) TestPublishThirdPartyComponents() {
	t := suite.T()
	_, _ = testutil.StartRegistry(t)
	uri := fmt.Sprintf("%s/x/y/z", os.Getenv(assistantconfig.OciRegistryEnvVar))

	args := []string{"artifacts", "publish", "component", "--name", "meep", "--version", "1.2.3",
		"-p", "windows/amd64=" + testutil.TestdataPath(t, "meepy-component", "windows"),
		"-p", "linux/amd64=" + testutil.TestdataPath(t, "meepy-component", "unix"),
		"-p", "darwin/amd64=" + testutil.TestdataPath(t, "meepy-component", "unix"),
		"-p", "darwin/arm64=" + testutil.TestdataPath(t, "meepy-component", "unix"),
		"--registry", "oci://" + uri,
	}

	if os.Getenv(assistantconfig.AllowInsecureRegistryEnvVar) == "true" {
		args = append(args, "--insecure")
	}

	require.NoError(t, createStdTestRootCmd(t, args...).Execute())
}

func (suite *RepoSuite) TestListTags() {
	t := suite.T()
	_, reg := testutil.StartRegistry(t)

	args := testutil.PushComponentUri(reg, "meep", "bar/foo", "1.2.3", testutil.TestdataPath(t, "meepy-component", testutil.OS))
	require.NoError(t, createStdTestRootCmd(t, args...).Execute())

	args = testutil.PushComponentUri(reg, "meep", "foo/bar", "1.2.4", testutil.TestdataPath(t, "meepy-component", testutil.OS))
	require.NoError(t, createStdTestRootCmd(t, args...).Execute())

	t.Run("test tags for arbitrary repo", func(t *testing.T) {
		res := listComponentTags(t, "meep", "oci://"+strings.TrimPrefix(reg.URL, "http://")+"/foo/bar")
		expected := []string{"1.2.4", "1.2.4.generic"}
		assert.Equal(t, expected, res)
	})

	t.Run("test tags for arbitrary repo", func(t *testing.T) {
		res := listComponentTags(t, "meep", "oci://"+strings.TrimPrefix(reg.URL, "http://")+"/bar/foo")
		expected := []string{"1.2.3", "1.2.3.generic"}
		assert.Equal(t, expected, res)
	})

}

func publishDar(t *testing.T) {
	t.Run("publish dar", func(t *testing.T) {
		cmd := createStdTestRootCmd(t)

		args := []string{"artifacts", "publish", "dar", "--name", "meep", "--version", "1.2.3",
			"-f", testutil.TestdataPath(t, "test-dar"), "--dry-run",
		}

		cmd.SetArgs(appendRegistryArgsFromEnv(args))
		require.NoError(t, cmd.Execute())
	})

}

func listComponentTags(t *testing.T, component, registry string) []string {
	t.Setenv(assistantconfig.OciRegistryEnvVar, registry)

	cmd, r, w := createTestRootCmd(t)
	cmd.SetArgs([]string{
		"artifacts", "list", "component", "--name", component, "--registry", registry,
	})
	require.NoError(t, cmd.Execute())
	assert.NoError(t, w.Close())

	output, err := io.ReadAll(r)
	assert.NoError(t, err)
	return strings.Split(strings.TrimSpace(string(output)), "\n")
}
