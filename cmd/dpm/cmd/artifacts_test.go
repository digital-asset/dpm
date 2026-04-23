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
	destinationRegistry := os.Getenv(assistantconfig.OciRegistryEnvVar)
	tmpDamlHome, err = os.MkdirTemp("", "")
	require.NoError(t, err)
	t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDamlHome)

	cmd := createStdTestRootCmd(t)
	args := []string{
		"publish", "dar", "--name", "meep", "--version", "1.2.3",
		"-f", testutil.TestdataPath(t, "test-dar"),
		"--registry", "oci://" + destinationRegistry,
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

	args := []string{"publish", "component", "--name", "meep", "--version", "1.2.3",
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

func (suite *RepoSuite) TestComponentTags() {
	t := suite.T()
	_, reg := testutil.StartRegistry(t)

	args := testutil.PushComponentUri(reg, "meep", "bar/foo", "1.2.3", testutil.TestdataPath(t, "meepy-component", testutil.OS))
	require.NoError(t, createStdTestRootCmd(t, args...).Execute())

	args = testutil.PushComponentUri(reg, "meep", "foo/bar", "1.2.4", testutil.TestdataPath(t, "meepy-component", testutil.OS))
	require.NoError(t, createStdTestRootCmd(t, args...).Execute())

	t.Run("test tags for arbitrary repo", func(t *testing.T) {
		res := listArtifactTags(t, "oci://"+strings.TrimPrefix(reg.URL, "http://")+"/foo/bar/meep")
		expected := []string{"1.2.4", "1.2.4.generic"}
		assert.Equal(t, expected, res)
	})

	t.Run("test tags for arbitrary repo", func(t *testing.T) {
		res := listArtifactTags(t, "oci://"+strings.TrimPrefix(reg.URL, "http://")+"/bar/foo/meep")
		expected := []string{"1.2.3", "1.2.3.generic"}
		assert.Equal(t, expected, res)
	})
}
func (suite *RepoSuite) TestDarTags() {
	t := suite.T()
	t.Setenv(assistantconfig.DpmLockfileEnabledEnvVar, "true")
	_, reg := testutil.StartRegistry(t)

	args := testutil.PushDarUri(reg, "meep", "foo/bar", "1.2.3", testutil.TestdataPath(t, "test-dar"))
	require.NoError(t, createStdTestRootCmd(t, args...).Execute())

	args = testutil.PushDarUri(reg, "meep", "bar/foo", "1.2.4", testutil.TestdataPath(t, "test-dar"))
	require.NoError(t, createStdTestRootCmd(t, args...).Execute())

	t.Run("test tags for arbitrary repo", func(t *testing.T) {
		res := listArtifactTags(t, "oci://"+strings.TrimPrefix(reg.URL, "http://")+"/foo/bar/meep")
		expected := []string{"1.2.3"}
		assert.Equal(t, expected, res)
	})

	t.Run("test tags for arbitrary repo", func(t *testing.T) {
		res := listArtifactTags(t, "oci://"+strings.TrimPrefix(reg.URL, "http://")+"/bar/foo/meep")
		expected := []string{"1.2.4"}
		assert.Equal(t, expected, res)
	})
}

func listArtifactTags(t *testing.T, pathToArtifact string) []string {

	cmd, r, w := createTestRootCmd(t)
	cmd.SetArgs([]string{
		"tags", pathToArtifact,
	})
	require.NoError(t, cmd.Execute())
	assert.NoError(t, w.Close())

	output, err := io.ReadAll(r)
	assert.NoError(t, err)
	return strings.Split(strings.TrimSpace(string(output)), "\n")
}
