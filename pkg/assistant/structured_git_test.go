package assistant

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"daml.com/x/assistant/pkg/assembler"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSDKCommandStructuredGitCheck(t *testing.T) {
	const structured = `dependencies:
  - git:
      url: github.com/org/repo
      ref: main
      path: pkg/foo.dar
`
	const oneLiner = `dependencies:
  - git:github.com/org/repo#main?path=pkg/foo.dar
`

	cases := []struct {
		name         string
		yaml         string
		sdkVersion   string
		runs         bool
		errText      string
		multiPackage bool
	}{
		{
			name:       "effective 3.6 runs even when the file has no sdk-version",
			yaml:       structured,
			sdkVersion: "3.6.0",
			runs:       true,
		},
		{
			name:       "version below 3.6 is rejected even when the file says 3.6",
			yaml:       "sdk-version: 3.6.0\n" + structured,
			sdkVersion: "3.5.2",
			errText:    "sdk-version is 3.5.2",
		},
		{
			name:       "empty sdk version is rejected",
			yaml:       structured,
			sdkVersion: "",
			errText:    "sdk-version is not set",
		},
		{
			name: "components skip the check",
			yaml: `components:
  - damlc:3.6.0
` + structured,
			sdkVersion: "",
			runs:       true,
		},
		{
			name:         "multi-package components skip the check",
			yaml:         structured,
			sdkVersion:   "",
			runs:         true,
			multiPackage: true,
		},
		{
			name:       "one-liner runs on 3.5",
			yaml:       "sdk-version: 3.5.2\n" + oneLiner,
			sdkVersion: "3.5.2",
			runs:       true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			yamlPath := filepath.Join(dir, "daml.yaml")
			require.NoError(t, os.WriteFile(yamlPath, []byte(tc.yaml), 0o644))
			t.Setenv(assistantconfig.DamlPackageEnvVar, dir)
			if tc.multiPackage {
				multiDir := t.TempDir()
				require.NoError(t, os.WriteFile(filepath.Join(multiDir, assistantconfig.DamlMultiPackageFilename), []byte("components:\n  - damlc:3.6.0\n"), 0o644))
				t.Setenv(assistantconfig.DamlMultiPackageEnvVar, multiDir)
			}

			ran := false
			da := &DamlAssistant{
				ExitFn: func(int) {},
				CmdPreRunHook: func(*exec.Cmd) {
					ran = true
				},
			}
			cmds, err := da.toCobraCommands(context.Background(), &assistantconfig.Config{
				DamlHomePath: t.TempDir(),
			}, []*assembler.ValidatedCommand{{
				Command: &component.NativeCommand{
					Name:     "build",
					ExecArgs: []string{"-test.run=^$", "-test.count=1"},
				},
				AbsolutePath:        os.Args[0],
				DpmSdkVersionEnvVar: tc.sdkVersion,
			}}, "")
			require.NoError(t, err)
			require.Len(t, cmds, 1)

			err = cmds[0].RunE(cmds[0], nil)
			if tc.runs {
				require.NoError(t, err)
				assert.True(t, ran, "sdk command should run")
				return
			}
			require.Error(t, err)
			assert.False(t, ran, "sdk command should be rejected before it runs")
			assert.Contains(t, err.Error(), tc.errText)
			assert.Contains(t, err.Error(), yamlPath)
			assert.Contains(t, err.Error(), "SDK 3.6 and later")
			assert.Contains(t, err.Error(), "git: one-liner")
		})
	}
}
