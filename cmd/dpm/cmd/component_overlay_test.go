package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/multipackage"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/testutil"
	"daml.com/x/assistant/pkg/utils"
	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"
)

type ComponentOverlayTestCase struct {
	Name                         string
	WorkingDir                   WorkingDir
	MultiPackageComponents       map[string]string
	PackageComponents            map[string]string
	ExpectedHelpCommands         []string
	ExpectedResolutionComponents map[string]string
}

var componentOverlayTestCases = []ComponentOverlayTestCase{
	// TODO fill out add the remaining test cases
	{
		Name:                         "1",
		WorkingDir:                   MultiPackageWorkingDir,
		MultiPackageComponents:       map[string]string{"foo": "0.0.1"},
		PackageComponents:            nil,
		ExpectedHelpCommands:         []string{"foo"},
		ExpectedResolutionComponents: map[string]string{"foo": "0.0.1"},
	},
}

func (suite *MainSuite) TestComponentOverlay() {
	t := suite.T()

	tmpDamlHome := t.TempDir()
	t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDamlHome)

	t.Setenv("PATH", testutil.TestdataPath(t, "fake-java", testutil.OS)+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, reg := testutil.StartRegistry(t)

	pushGenericComponentWithCommand(t, reg, "foo", "0.0.1", "foo")
	pushGenericComponentWithCommand(t, reg, "foo", "0.0.2", "foo")
	pushGenericComponentWithCommand(t, reg, "meep", "0.0.1", "meep")
	pushGenericComponentWithCommand(t, reg, "meep", "0.0.2", "meep")

	installComponent(t, "foo", "0.0.1")
	installComponent(t, "foo", "0.0.2")
	installComponent(t, "meep", "0.0.1")
	installComponent(t, "meep", "0.0.2")

	setupTestCase := func(tc ComponentOverlayTestCase) (dirs TestCaseDirs) {
		tmpDir := t.TempDir()
		dirs.MultiPackageDir = filepath.Join(tmpDir, "multi-package")
		dirs.DamlPackageDir = filepath.Join(dirs.MultiPackageDir, "daml-package")
		require.NoError(t, utils.EnsureDirs(dirs.MultiPackageDir, dirs.DamlPackageDir))

		// create multi-package.yaml
		multiPackage := multipackage.MultiPackage{
			Packages: []string{"./daml-package"},
		}

		for compName, version := range tc.MultiPackageComponents {
			// TODO DeprecatedOverrideComponents is being used here because
			// the Components field is being ignored (`yaml:"-"`) in the YAML marshaling
			sv, err := semver.StrictNewVersion(version)
			require.NoError(t, err)

			multiPackage.DeprecatedOverrideComponents = map[string]*sdkmanifest.Component{
				compName: {
					Name:    compName,
					Version: sdkmanifest.AssemblySemVer(sv),
				},
			}
		}
		require.NoError(t,
			os.WriteFile(filepath.Join(dirs.MultiPackageDir, "multi-package.yaml"), testutil.MustMarshal(t, multiPackage), 0666),
		)

		// create daml.yaml
		damlPackage := damlpackage.DamlPackage{}
		for compName, version := range tc.PackageComponents {
			sv, err := semver.StrictNewVersion(version)
			require.NoError(t, err)
			damlPackage.DeprecatedOverrideComponents = map[string]*sdkmanifest.Component{
				compName: {
					Name:    compName,
					Version: sdkmanifest.AssemblySemVer(sv),
				},
			}
		}
		require.NoError(t,
			os.WriteFile(filepath.Join(dirs.DamlPackageDir, "daml.yaml"), testutil.MustMarshal(t, damlPackage), 0666),
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

		require.NoError(t, createStdTestRootCmd(t, "install", "package", "--skip-sdk").Execute())

		return
	}

	for _, tc := range componentOverlayTestCases {
		t.Run(tc.Name, func(t *testing.T) {
			setupTestCase(tc)
			// TODO test the --help command and the resolution
		})
	}

}
