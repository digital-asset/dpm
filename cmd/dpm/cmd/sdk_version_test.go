package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"daml.com/x/assistant/cmd/dpm/cmd/versions"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/utils"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	globalSdkVersion = "999.999.999"

	globalSdkComponent    = "bleep" // contained in the globalSdkVersion sdk
	someSdkComponent      = "meep"  // contained in the someSdkVersion sdk
	someOtherSdkComponent = "sheep" // contained in the someOtherSdkVersion sdk
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
	"", 1, nil}

var sdkVersionTestCases = []SdkVersionTestCase{
	{
		Name:                   "only multi sdk version (multi dir)",
		MultiPackageSdkVersion: someSdkVersion,
		PackageSdkVersion:      "null",
		WorkingDir:             MultiPackageWorkingDir,
		ExpectedResolution:     expectedSdkResolution.WithSdkVersion(someSdkVersion),
	},
	{
		Name:                   "only multi sdk version (package dir)",
		MultiPackageSdkVersion: someSdkVersion,
		PackageSdkVersion:      "null",
		WorkingDir:             PackageWorkingDir,
		ExpectedResolution:     expectedSdkResolution.WithSdkVersion(someSdkVersion),
	},
	{
		Name:                   "package version differs from multi (multi dir)",
		MultiPackageSdkVersion: someSdkVersion,
		PackageSdkVersion:      someOtherSdkVersion,
		WorkingDir:             MultiPackageWorkingDir,
		ExpectedResolution: ExpectedResolution{
			globalSdkVersion,
			[]string{someOtherSdkComponent},
			2,
			someSdkVersion, 1, nil},
	},
	{
		Name:                   "package version differs from multi (package dir)",
		MultiPackageSdkVersion: someSdkVersion,
		PackageSdkVersion:      someOtherSdkVersion,
		WorkingDir:             PackageWorkingDir,
		ExpectedResolution: ExpectedResolution{
			globalSdkVersion,
			[]string{someOtherSdkComponent},
			2,
			someOtherSdkVersion, 1, nil},
	},
	{
		Name:                   "multi and package same version (multi dir)",
		MultiPackageSdkVersion: someSdkVersion,
		PackageSdkVersion:      someSdkVersion,
		WorkingDir:             MultiPackageWorkingDir,
		ExpectedResolution:     expectedSdkResolution.WithSdkVersion(someSdkVersion),
	},
	{
		Name:                   "multi and package same version (package dir)",
		MultiPackageSdkVersion: someSdkVersion,
		PackageSdkVersion:      someSdkVersion,
		WorkingDir:             PackageWorkingDir,
		ExpectedResolution:     expectedSdkResolution.WithSdkVersion(someSdkVersion),
	},
	{
		Name:                   "only package sdk version (multi dir)",
		MultiPackageSdkVersion: "null",
		PackageSdkVersion:      someSdkVersion,
		WorkingDir:             MultiPackageWorkingDir,
		ExpectedResolution:     expectedSdkResolution.WithSdkVersion(globalSdkVersion),
	},
	{
		Name:                   "only package sdk version (package dir)",
		MultiPackageSdkVersion: "null",
		PackageSdkVersion:      someSdkVersion,
		WorkingDir:             PackageWorkingDir,
		ExpectedResolution:     expectedSdkResolution.WithSdkVersion(someSdkVersion),
	},
	{
		Name:                   "no sdk version at multi or package (multi dir)",
		MultiPackageSdkVersion: "null",
		PackageSdkVersion:      "null",
		WorkingDir:             MultiPackageWorkingDir,
		ExpectedResolution: ExpectedResolution{
			globalSdkVersion,
			[]string{},
			0,
			globalSdkVersion,
			1,
			versions.ErrNoActiveSdk},
	},
	{
		Name:                   "no sdk version at multi or package (package dir)",
		MultiPackageSdkVersion: "null",
		PackageSdkVersion:      "null",
		WorkingDir:             PackageWorkingDir,
		ExpectedResolution: ExpectedResolution{
			globalSdkVersion,
			[]string{},
			0,
			"null", 1, versions.ErrNoActiveSdk},
	},
	{
		Name:                   "no sdk version at multi or package (outside project dir)",
		MultiPackageSdkVersion: "null",
		PackageSdkVersion:      "null",
		WorkingDir:             OutsideProjectDir,
		ExpectedResolution: ExpectedResolution{
			globalSdkVersion,
			[]string{},
			0,
			globalSdkVersion,
			0, nil},
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
		case OutsideProjectDir:
			dirs.WorkingDir = t.TempDir()
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
					assertNoActiveSdkVersion(t, tc.ExpectedResolution.ExpectedError)
					testResolution(t, tc.ExpectedResolution)
				})

			} else {
				t.Run("assert active sdk version", func(t *testing.T) {
					assertActiveSdkVersion(t, tc.ExpectedResolution.ExpectedSdkVersion)
					testResolution(t, tc.ExpectedResolution)
				})
			}

			// tests DPM_SDK_VERSION and DPM_RESOLUTION_FILE env vars that dpm injects when exec'ing commands
			t.Run("expected DPM_SDK_VERSION and DPM_RESOLUTION_FILE", func(t *testing.T) {
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
