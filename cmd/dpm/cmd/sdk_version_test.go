package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/utils"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type SdkVersionTestCase struct {
	Name                                      string
	MultiPackageSdkVersion, PackageSdkVersion string
	WorkingDir                                WorkingDir
	ExpectedResolution                        ExpectedResolution
}

var expectedSdkResolution = ExpectedResolution{
	globalSdkVersion,
	[]string{someSdkComponent},
	2,
	""}

var sdkVersionTestCases = []SdkVersionTestCase{
	{
		Name:                   "1 multi:some pkg: some, wd:multi",
		MultiPackageSdkVersion: someSdkVersion,
		PackageSdkVersion:      someSdkVersion,
		WorkingDir:             MultiPackageWorkingDir,
		ExpectedResolution:     expectedSdkResolution.WithSdkVersion(someSdkVersion),
	},
	{
		Name:                   "2 multi:some pkg:other wd:multi",
		MultiPackageSdkVersion: someSdkVersion,
		PackageSdkVersion:      someOtherSdkVersion,
		WorkingDir:             MultiPackageWorkingDir,
		ExpectedResolution: ExpectedResolution{
			globalSdkVersion,
			[]string{someOtherSdkComponent},
			2,
			someSdkVersion},
	},
	{
		Name:                   "3 multi:some pkg:null wd:multi",
		MultiPackageSdkVersion: someSdkVersion,
		PackageSdkVersion:      "null",
		WorkingDir:             MultiPackageWorkingDir,
		ExpectedResolution:     expectedSdkResolution.WithSdkVersion(someSdkVersion),
	},
	{
		Name:                   "7 multi:null pkg:some wd:multi",
		MultiPackageSdkVersion: "null",
		PackageSdkVersion:      someSdkVersion,
		WorkingDir:             MultiPackageWorkingDir,
		ExpectedResolution:     expectedSdkResolution.WithSdkVersion(globalSdkVersion),
	},
	{
		Name:                   "9 multi:null pkg:null wd:multi",
		MultiPackageSdkVersion: "null",
		PackageSdkVersion:      "null",
		WorkingDir:             MultiPackageWorkingDir,
		ExpectedResolution: ExpectedResolution{
			globalSdkVersion,
			[]string{},
			0,
			globalSdkVersion},
	},
	{
		Name:                   "18 multi:null pkg:null wd:pkg",
		MultiPackageSdkVersion: "null",
		PackageSdkVersion:      "null",
		WorkingDir:             PackageWorkingDir,
		ExpectedResolution: ExpectedResolution{
			globalSdkVersion,
			[]string{},
			0,
			"null"},
	},
	{
		Name:                   "10 multi:some pkg:some wd:pkg",
		MultiPackageSdkVersion: someSdkVersion,
		PackageSdkVersion:      someSdkVersion,
		WorkingDir:             PackageWorkingDir,
		ExpectedResolution:     expectedSdkResolution.WithSdkVersion(someSdkVersion),
	},
	{
		Name:                   "11 multi:some pkg:other wd:pkg",
		MultiPackageSdkVersion: someSdkVersion,
		PackageSdkVersion:      someOtherSdkVersion,
		WorkingDir:             PackageWorkingDir,
		ExpectedResolution: ExpectedResolution{
			globalSdkVersion,
			[]string{someOtherSdkComponent},
			2,
			someOtherSdkVersion},
	},
	{
		Name:                   "12 multi:some pkg:null wd:pkg",
		MultiPackageSdkVersion: someSdkVersion,
		PackageSdkVersion:      "null",
		WorkingDir:             PackageWorkingDir,
		ExpectedResolution:     expectedSdkResolution.WithSdkVersion(someSdkVersion),
	},
	{
		Name:                   "13 multi:other pkg:some wd:pkg",
		MultiPackageSdkVersion: someOtherSdkVersion,
		PackageSdkVersion:      someSdkVersion,
		WorkingDir:             PackageWorkingDir,
		ExpectedResolution:     expectedSdkResolution.WithSdkVersion(someSdkVersion),
	},
	{
		Name:                   "16 multi:null pkg:some wd:pkg",
		MultiPackageSdkVersion: "null",
		PackageSdkVersion:      someSdkVersion,
		WorkingDir:             PackageWorkingDir,
		ExpectedResolution:     expectedSdkResolution.WithSdkVersion(someSdkVersion),
	},
}

func (suite *MainSuite) TestActiveSdkVersionExhaustive() {
	t := suite.T()
	testActiveSdkVersionExhaustive(t, func(t *testing.T, testCase SdkVersionTestCase, dirs TestCaseDirs) {})
}

func testActiveSdkVersionExhaustive(t *testing.T, hook func(t *testing.T, testCase SdkVersionTestCase, dirs TestCaseDirs)) {
	tmpDamlHome := t.TempDir()
	t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDamlHome)

	installSdkForComponent(t, globalSdkVersion, globalSdkComponent, "999.999.999")
	installSdkForComponent(t, someSdkVersion, someSdkComponent, "1.2.3")
	installSdkForComponent(t, someOtherSdkVersion, someOtherSdkComponent, "4.5.6")

	setupTestCase := func(tc SdkVersionTestCase) (dirs TestCaseDirs) {
		tmpDir := t.TempDir()
		dirs.MultiPackageDir = filepath.Join(tmpDir, "multi-package")
		dirs.DamlPackageDir = filepath.Join(dirs.MultiPackageDir, "daml-package")
		require.NoError(t, utils.EnsureDirs(dirs.MultiPackageDir, dirs.DamlPackageDir))

		// create multi-package.yaml
		multiPackageContents := fmt.Sprintf(`
sdk-version: %s
packages:
 - ./daml-package`, tc.MultiPackageSdkVersion)
		require.NoError(t,
			os.WriteFile(filepath.Join(dirs.MultiPackageDir, "multi-package.yaml"), []byte(multiPackageContents), 0666),
		)
		// create daml.yaml
		require.NoError(t,
			os.WriteFile(filepath.Join(dirs.DamlPackageDir, "daml.yaml"), []byte(`sdk-version: `+tc.PackageSdkVersion), 0666),
		)

		// chdir
		switch tc.WorkingDir {
		case PackageWorkingDir:
			dirs.WorkingDir = dirs.DamlPackageDir
		case MultiPackageWorkingDir:
			dirs.WorkingDir = dirs.MultiPackageDir
		default:
		}
		t.Chdir(dirs.WorkingDir)

		return
	}

	for _, tc := range sdkVersionTestCases {
		t.Run(tc.Name, func(t *testing.T) {
			dirs := setupTestCase(tc)

			hook(t, tc, dirs)

			if tc.ExpectedResolution.ExpectedSdkVersion == "null" {
				t.Run("assert no active sdk version", func(t *testing.T) {
					assertNoActiveSdkVersion(t)
					testResolution(t, tc.ExpectedResolution)
				})

			} else {
				t.Run("assert active sdk version", func(t *testing.T) {
					assertActiveSdkVersion(t, tc.ExpectedResolution.ExpectedSdkVersion)
					testResolution(t, tc.ExpectedResolution)
				})
			}

			// tests DPM_SDK_VERSION and DPM_RESOLUTION_FILE env vars that dpm injects when exec'ing commands
			t.Run("dynamically injected env vars", func(t *testing.T) {
				wasCalled := atomic.Bool{}
				assertEnv := func(cmd *exec.Cmd) {
					wasCalled.Store(true)

					expected := tc.ExpectedResolution.ExpectedSdkVersion
					if expected == "null" {
						expected = ""
					}

					assert.Contains(t, cmd.Env, fmt.Sprintf("%s=%s", assistantconfig.DpmSdkVersionEnvVar, expected))
					kv, _ := lo.Find(cmd.Env, func(kv string) bool {
						return strings.Contains(kv, assistantconfig.ResolutionFilePathEnvVar)
					})
					_, val, _ := strings.Cut(kv, "=")
					contents, err := os.ReadFile(val)
					require.NoError(t, err)
					require.Contains(t, string(contents), "kind: Resolution")
					assert.True(t, filepath.IsAbs(val))
				}

				commands := lo.Filter(createStdTestRootCmd(t, "--help").Commands(), func(c *cobra.Command, _ int) bool {
					return c.GroupID == sdkGroupId
				})
				for _, c := range commands {
					_ = createStdTestRootCmdWithPreRunHook(t, assertEnv, c.Use).Execute()
				}

				if tc.ExpectedResolution.ExpectedSdkVersion == "null" {
					// TODO since in this testcase there's no sdk, no SDK commands will be available
					// to run (i.e. in the --help) so that we can obtain the injected env var(s).
					// To test this, we'd need to add an override-component, so that we have at least 1 command
				} else {
					assert.True(t, wasCalled.Load())
				}
			})

		})
	}
}
