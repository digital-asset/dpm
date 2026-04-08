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

	tmpDamlHome := t.TempDir()
	t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDamlHome)

	sdkVersions := []string{someSdkVersion, someOtherSdkVersion, globalSdkVersion}
	installSdk(t, sdkVersions)

	setupTestCase := func(tc SdkVersionTestCase) (multiPackageDir string, damlPackageDir string) {
		tmpDir := t.TempDir()
		multiPackageDir = filepath.Join(tmpDir, "multi-package")
		damlPackageDir = filepath.Join(multiPackageDir, "daml-package")
		require.NoError(t, utils.EnsureDirs(multiPackageDir, damlPackageDir))

		// create multi-package.yaml
		multiPackageContents := fmt.Sprintf(`
sdk-version: %s
packages:
 - ./daml-package`, tc.MultiPackageSdkVersion)
		require.NoError(t,
			os.WriteFile(filepath.Join(tmpDir, "multi-package.yaml"), []byte(multiPackageContents), 0666),
		)
		// create daml.yaml
		require.NoError(t,
			os.WriteFile(filepath.Join(damlPackageDir, "daml.yaml"), []byte(`sdk-version: `+tc.PackageSdkVersion), 0666),
		)

		// chdir
		switch tc.WorkingDir {
		case PackageWorkingDir:
			t.Chdir(damlPackageDir)
		case MultiPackageWorkingDir:
			t.Chdir(tmpDir)
		default:
		}

		return
	}

	for _, tc := range sdkVersionTestCases {
		t.Run(tc.Name, func(t *testing.T) {
			setupTestCase(tc)

			// workdir is package
			if tc.ExpectedVersion == "null" {
				assertNoActiveSdkVersion(t)
			} else {
				assertActiveSdkVersion(t, tc.ExpectedVersion)
			}
		})
	}
}
