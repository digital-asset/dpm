package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/multipackage"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/testutil"
	"daml.com/x/assistant/pkg/utils"
	"github.com/Masterminds/semver/v3"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
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
	{
		Name:                         "1",
		WorkingDir:                   MultiPackageWorkingDir,
		MultiPackageComponents:       map[string]string{"foo": "0.0.1"},
		PackageComponents:            nil,
		ExpectedHelpCommands:         []string{"foo"},
		ExpectedResolutionComponents: map[string]string{"foo": "0.0.1"},
	},
	{
		Name:                         "2",
		WorkingDir:                   PackageWorkingDir,
		MultiPackageComponents:       map[string]string{"foo": "0.0.1"},
		PackageComponents:            nil,
		ExpectedHelpCommands:         []string{"foo"},
		ExpectedResolutionComponents: map[string]string{"foo": "0.0.1"},
	},
	{
		Name:                         "3",
		WorkingDir:                   MultiPackageWorkingDir,
		MultiPackageComponents:       map[string]string{"foo": "0.0.1"},
		PackageComponents:            map[string]string{"foo": "0.0.2"},
		ExpectedHelpCommands:         []string{"foo"},
		ExpectedResolutionComponents: map[string]string{"foo": "0.0.1"},
	},
	{
		Name:                         "4",
		WorkingDir:                   PackageWorkingDir,
		MultiPackageComponents:       map[string]string{"foo": "0.0.1"},
		PackageComponents:            map[string]string{"foo": "0.0.2"},
		ExpectedHelpCommands:         []string{"foo"},
		ExpectedResolutionComponents: map[string]string{"foo": "0.0.2"},
	},
	{
		Name:                         "5",
		WorkingDir:                   MultiPackageWorkingDir,
		MultiPackageComponents:       nil,
		PackageComponents:            map[string]string{"foo": "0.0.2"},
		ExpectedHelpCommands:         []string{},
		ExpectedResolutionComponents: map[string]string{"foo": "0.0.2"},
	},
	{
		Name:                         "6",
		WorkingDir:                   PackageWorkingDir,
		MultiPackageComponents:       nil,
		PackageComponents:            map[string]string{"foo": "0.0.2"},
		ExpectedHelpCommands:         []string{"foo"},
		ExpectedResolutionComponents: map[string]string{"foo": "0.0.2"},
	},
	{
		Name:                         "7",
		WorkingDir:                   MultiPackageWorkingDir,
		MultiPackageComponents:       map[string]string{"foo": "0.0.1", "meep": "0.0.1"},
		PackageComponents:            nil,
		ExpectedHelpCommands:         []string{"foo", "meep"},
		ExpectedResolutionComponents: map[string]string{"foo": "0.0.1", "meep": "0.0.1"},
	},
	{
		Name:                         "8",
		WorkingDir:                   PackageWorkingDir,
		MultiPackageComponents:       map[string]string{"foo": "0.0.1", "meep": "0.0.1"},
		PackageComponents:            nil,
		ExpectedHelpCommands:         []string{"foo", "meep"},
		ExpectedResolutionComponents: map[string]string{"foo": "0.0.1", "meep": "0.0.1"},
	},
	{
		Name:                         "9",
		WorkingDir:                   MultiPackageWorkingDir,
		MultiPackageComponents:       map[string]string{"foo": "0.0.1"},
		PackageComponents:            map[string]string{"meep": "0.0.2"},
		ExpectedHelpCommands:         []string{"foo"},
		ExpectedResolutionComponents: map[string]string{"foo": "0.0.1", "meep": "0.0.2"},
	},
	{
		Name:                         "10",
		WorkingDir:                   PackageWorkingDir,
		MultiPackageComponents:       map[string]string{"foo": "0.0.1"},
		PackageComponents:            map[string]string{"meep": "0.0.2"},
		ExpectedHelpCommands:         []string{"foo", "meep"},
		ExpectedResolutionComponents: map[string]string{"foo": "0.0.1", "meep": "0.0.2"},
	},
	{
		//no multi-pkg structure - in pkg dir
		Name:                         "11",
		WorkingDir:                   PackageWorkingDir,
		MultiPackageComponents:       nil,
		PackageComponents:            map[string]string{"foo": "0.0.2"},
		ExpectedHelpCommands:         []string{"foo"},
		ExpectedResolutionComponents: map[string]string{"foo": "0.0.2"},
	},
	{
		//no multi-pkg structure - in pkg dir
		Name:                         "12",
		WorkingDir:                   PackageWorkingDir,
		MultiPackageComponents:       nil,
		PackageComponents:            nil,
		ExpectedHelpCommands:         []string{}, //blank DS or nil
		ExpectedResolutionComponents: map[string]string{},
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
			t.Run(tc.Name+" dpm --help", func(t *testing.T) {

				output := runHelpCommand(t)

				// assert number of cmds in output as expected
				_, trimmedOutput, _ := strings.Cut(output, "Dpm-SDK Commands\n")
				trimmedOutput, _, _ = strings.Cut(trimmedOutput, "\nAdditional Commands")
				count := strings.Count(trimmedOutput, "\n")
				assert.Equal(t, len(tc.ExpectedHelpCommands), count)

				// assert values in output as expected
				for _, command := range tc.ExpectedHelpCommands {
					assert.Contains(t, trimmedOutput, command)
				}
			})
			t.Run(tc.Name+" dpm resolve", func(t *testing.T) {
				deepResolution := runResolveCommand(t)

				assert.Len(t, lo.Values(deepResolution.Packages)[0].ComponentsV2, len(tc.ExpectedResolutionComponents))

				for component, version := range tc.ExpectedResolutionComponents {
					assert.Contains(t, lo.Values(deepResolution.Packages)[0].ComponentsV2, component)
					resolvedVersion := lo.Values(deepResolution.Packages)[0].ComponentsV2[component]["version"]
					assert.Equal(t, version, resolvedVersion)
				}

			})
		})
	}

}
