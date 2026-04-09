package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/utils"
	"github.com/stretchr/testify/require"
)

type WorkingDir int

const (
	MultiPackageWorkingDir = iota
	PackageWorkingDir
)

type TestCaseDirs struct {
	WorkingDir, DamlPackageDir, MultiPackageDir string
}

type SdkVersionTestCase struct {
	Name                                      string
	MultiPackageSdkVersion, PackageSdkVersion string
	WorkingDir                                WorkingDir
	ExpectedVersion                           string
}

const globalSdkVersion = "999.999.999"

var sdkVersionTestCases = []SdkVersionTestCase{
	{
		Name:                   "1 multi:some pkg: some, wd:multi",
		MultiPackageSdkVersion: someSdkVersion,
		PackageSdkVersion:      someSdkVersion,
		WorkingDir:             MultiPackageWorkingDir,
		ExpectedVersion:        someSdkVersion,
	},
	{
		Name:                   "2 multi:some pkg:other wd:multi",
		MultiPackageSdkVersion: someSdkVersion,
		PackageSdkVersion:      someOtherSdkVersion,
		WorkingDir:             MultiPackageWorkingDir,
		ExpectedVersion:        someSdkVersion,
	},
	{
		Name:                   "3 multi:some pkg:null wd:multi",
		MultiPackageSdkVersion: someSdkVersion,
		PackageSdkVersion:      "null",
		WorkingDir:             MultiPackageWorkingDir,
		ExpectedVersion:        someSdkVersion,
	},
	{
		Name:                   "7 multi:null pkg:some wd:multi",
		MultiPackageSdkVersion: "null",
		PackageSdkVersion:      someSdkVersion,
		WorkingDir:             MultiPackageWorkingDir,
		ExpectedVersion:        globalSdkVersion,
	},
	{
		Name:                   "9 multi:null pkg:null wd:multi",
		MultiPackageSdkVersion: "null",
		PackageSdkVersion:      "null",
		WorkingDir:             MultiPackageWorkingDir,
		ExpectedVersion:        globalSdkVersion,
	},
	{
		Name:                   "10 multi:some pkg:some wd:pkg",
		MultiPackageSdkVersion: someSdkVersion,
		PackageSdkVersion:      someSdkVersion,
		WorkingDir:             PackageWorkingDir,
		ExpectedVersion:        someSdkVersion,
	},
	{
		Name:                   "11 multi:some pkg:other wd:pkg",
		MultiPackageSdkVersion: someSdkVersion,
		PackageSdkVersion:      someOtherSdkVersion,
		WorkingDir:             PackageWorkingDir,
		ExpectedVersion:        someOtherSdkVersion,
	},
	{
		Name:                   "12 multi:some pkg:null wd:pkg",
		MultiPackageSdkVersion: someSdkVersion,
		PackageSdkVersion:      "null",
		WorkingDir:             PackageWorkingDir,
		ExpectedVersion:        someSdkVersion,
	},
	{
		Name:                   "13 multi:other pkg:some wd:pkg",
		MultiPackageSdkVersion: someOtherSdkVersion,
		PackageSdkVersion:      someSdkVersion,
		WorkingDir:             PackageWorkingDir,
		ExpectedVersion:        someSdkVersion,
	},
	{
		Name:                   "16 multi:null pkg:some wd:pkg",
		MultiPackageSdkVersion: "null",
		PackageSdkVersion:      someSdkVersion,
		WorkingDir:             PackageWorkingDir,
		ExpectedVersion:        someSdkVersion,
	},
	{
		Name:                   "18 multi:null pkg:null wd:pkg",
		MultiPackageSdkVersion: "null",
		PackageSdkVersion:      "null",
		WorkingDir:             PackageWorkingDir,
		ExpectedVersion:        "null",
	},
}

func (suite *MainSuite) TestActiveSdkVersionExhaustive() {
	t := suite.T()
	testActiveSdkVersionExhaustive(t, func(t *testing.T, testCase SdkVersionTestCase, dirs TestCaseDirs) {})
}

func testActiveSdkVersionExhaustive(t *testing.T, hook func(t *testing.T, testCase SdkVersionTestCase, dirs TestCaseDirs)) {
	tmpDamlHome := t.TempDir()
	t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDamlHome)

	sdkVersions := []string{someSdkVersion, someOtherSdkVersion, globalSdkVersion}
	installSdk(t, sdkVersions)

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

			if tc.ExpectedVersion == "null" {

				t.Run("assert no active sdk version", func(t *testing.T) {
					assertNoActiveSdkVersion(t)
				})

			} else {
				t.Run("assert active sdk version", func(t *testing.T) {
					assertActiveSdkVersion(t, tc.ExpectedVersion)
					testResolution(t, 1, globalSdkVersion)
				})
			}
		})
	}
}
