package cmd

import (
	"fmt"
	"os"
	"testing"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/testutil"
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
