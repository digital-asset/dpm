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
}

var sdkVersionTestCases = []SdkVersionTestCase{
	{
		Name:                   "1 multi:some pkg:some wd:multi",
		MultiPackageSdkVersion: someSdkVersion,
		PackageSdkVersion:      someSdkVersion,
		WorkingDir:             MultiPackageWorkingDir,
	},
	{
		Name:                   "2 multi:some pkg:other wd:multi",
		MultiPackageSdkVersion: someSdkVersion,
		PackageSdkVersion:      someOtherSdkVersion,
		WorkingDir:             MultiPackageWorkingDir,
	},
	{
		Name:                   "3 multi:some pkg:null wd:multi",
		MultiPackageSdkVersion: someSdkVersion,
		PackageSdkVersion:      "null",
		WorkingDir:             MultiPackageWorkingDir,
	},
	{
		Name:                   "4 multi:other pkg:some wd:multi",
		MultiPackageSdkVersion: someOtherSdkVersion,
		PackageSdkVersion:      someSdkVersion,
		WorkingDir:             MultiPackageWorkingDir,
	},
	{
		Name:                   "5 multi:other pkg:other wd:multi",
		MultiPackageSdkVersion: someOtherSdkVersion,
		PackageSdkVersion:      someOtherSdkVersion,
		WorkingDir:             MultiPackageWorkingDir,
	},
	{
		Name:                   "6 multi:other pkg:null wd:multi",
		MultiPackageSdkVersion: someOtherSdkVersion,
		PackageSdkVersion:      "null",
		WorkingDir:             MultiPackageWorkingDir,
	},
	{
		Name:                   "7 multi:null pkg:some wd:multi",
		MultiPackageSdkVersion: "null",
		PackageSdkVersion:      someSdkVersion,
		WorkingDir:             MultiPackageWorkingDir,
	},
	{
		Name:                   "8 multi:null pkg:other wd:multi",
		MultiPackageSdkVersion: "null",
		PackageSdkVersion:      someOtherSdkVersion,
		WorkingDir:             MultiPackageWorkingDir,
	},
	{
		Name:                   "9 multi:null pkg:null wd:multi",
		MultiPackageSdkVersion: "null",
		PackageSdkVersion:      "null",
		WorkingDir:             MultiPackageWorkingDir,
	},
	{
		Name:                   "10 multi:some pkg:some wd:pkg",
		MultiPackageSdkVersion: someSdkVersion,
		PackageSdkVersion:      someSdkVersion,
		WorkingDir:             PackageWorkingDir,
	},
	{
		Name:                   "11 multi:some pkg:other wd:pkg",
		MultiPackageSdkVersion: someSdkVersion,
		PackageSdkVersion:      someOtherSdkVersion,
		WorkingDir:             PackageWorkingDir,
	},
	{
		Name:                   "12 multi:some pkg:null wd:pkg",
		MultiPackageSdkVersion: someSdkVersion,
		PackageSdkVersion:      "null",
		WorkingDir:             PackageWorkingDir,
	},
	{
		Name:                   "13 multi:other pkg:some wd:pkg",
		MultiPackageSdkVersion: someOtherSdkVersion,
		PackageSdkVersion:      someSdkVersion,
		WorkingDir:             PackageWorkingDir,
	},
	{
		Name:                   "14 multi:other pkg:other wd:pkg",
		MultiPackageSdkVersion: someOtherSdkVersion,
		PackageSdkVersion:      someOtherSdkVersion,
		WorkingDir:             PackageWorkingDir,
	},
	{
		Name:                   "15 multi:other pkg:null wd:pkg",
		MultiPackageSdkVersion: someOtherSdkVersion,
		PackageSdkVersion:      "null",
		WorkingDir:             PackageWorkingDir,
	},
	{
		Name:                   "16 multi:null pkg:some wd:pkg",
		MultiPackageSdkVersion: "null",
		PackageSdkVersion:      someSdkVersion,
		WorkingDir:             PackageWorkingDir,
	},
	{
		Name:                   "17 multi:null pkg:other wd:pkg",
		MultiPackageSdkVersion: "null",
		PackageSdkVersion:      someOtherSdkVersion,
		WorkingDir:             PackageWorkingDir,
	},
	{
		Name:                   "18 multi:null pkg:null wd:pkg",
		MultiPackageSdkVersion: "null",
		PackageSdkVersion:      "null",
		WorkingDir:             PackageWorkingDir,
	},
}

func (suite *MainSuite) TestActiveSdkVersionExhaustive() {
	t := suite.T()

	tmpDamlHome := t.TempDir()
	t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDamlHome)

	globalSdkVersion := "999.999.999"
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
			switch tc.WorkingDir {
			case PackageWorkingDir:
				// this takes highest precedence
				if tc.PackageSdkVersion != "null" {
					assertActiveSdkVersion(t, tc.PackageSdkVersion)
				} else if tc.MultiPackageSdkVersion != "null" {
					assertActiveSdkVersion(t, tc.MultiPackageSdkVersion)
				} else {
					assertNoActiveSdkVersion(t)
				}
			case MultiPackageWorkingDir:
				if tc.MultiPackageSdkVersion != "null" {
					assertActiveSdkVersion(t, tc.MultiPackageSdkVersion)
				} else {
					assertActiveSdkVersion(t, globalSdkVersion)
				}
			default:
				assertActiveSdkVersion(t, globalSdkVersion)
			}
		})
	}
}
