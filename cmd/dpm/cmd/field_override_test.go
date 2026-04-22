package cmd

import (
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/component"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/multipackage"
	"daml.com/x/assistant/pkg/schema"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/testutil"
	"daml.com/x/assistant/pkg/utils"
	"github.com/Masterminds/semver/v3"
	"github.com/goccy/go-yaml"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
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

type FieldOverrideTestCase struct {
	Name                                      string
	MultiPackageSdkVersion, PackageSdkVersion string
	WorkingDir                                WorkingDir
	ExpectedResolution                        ExpectedResolution
	MultiPackageAdditionalComponent           string
	PackageAdditionalComponent                string
}

const (
	globalSdkVersion = "999.999.999"

	globalSdkComponent    = "bleep" // contained in the globalSdkVersion sdk
	someSdkComponent      = "meep"  // contained in the someSdkVersion sdk
	someOtherSdkComponent = "sheep" // contained in the someOtherSdkVersion sdk

	AdditionalMultiPackageComponent = "multi-package-comp"
	AdditionalPackageComponent      = "daml-package-comp"
)

var expectedResolution = ExpectedResolution{
	globalSdkVersion,
	[]string{someSdkComponent},
	2,
	""}

var vanillaSdkVersionTestCases = []FieldOverrideTestCase{
	{
		Name:                   "1 multi:some pkg: some wd:multi",
		MultiPackageSdkVersion: someSdkVersion,
		PackageSdkVersion:      someSdkVersion,
		WorkingDir:             MultiPackageWorkingDir,
		ExpectedResolution:     expectedResolution.WithSdkVersion(someSdkVersion),
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
		ExpectedResolution:     expectedResolution.WithSdkVersion(someSdkVersion),
	},
	{
		Name:                   "7 multi:null pkg:some wd:multi",
		MultiPackageSdkVersion: "null",
		PackageSdkVersion:      someSdkVersion,
		WorkingDir:             MultiPackageWorkingDir,
		ExpectedResolution:     expectedResolution.WithSdkVersion(globalSdkVersion),
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
		ExpectedResolution:     expectedResolution.WithSdkVersion(someSdkVersion),
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
		ExpectedResolution:     expectedResolution.WithSdkVersion(someSdkVersion),
	},
	{
		Name:                   "13 multi:other pkg:some wd:pkg",
		MultiPackageSdkVersion: someOtherSdkVersion,
		PackageSdkVersion:      someSdkVersion,
		WorkingDir:             PackageWorkingDir,
		ExpectedResolution:     expectedResolution.WithSdkVersion(someSdkVersion),
	},
	{
		Name:                   "16 multi:null pkg:some wd:pkg",
		MultiPackageSdkVersion: "null",
		PackageSdkVersion:      someSdkVersion,
		WorkingDir:             PackageWorkingDir,
		ExpectedResolution:     expectedResolution.WithSdkVersion(someSdkVersion),
	},
}

func makeTestCases(additionalPackageComponent, additionalMultiPackageComponent string) (result []FieldOverrideTestCase) {
	for _, tc := range vanillaSdkVersionTestCases {
		name := tc.Name
		var extraComps []string
		if additionalPackageComponent != "" {
			extraComps = append(extraComps, additionalPackageComponent)
			name += " extra:pkg"
		}
		if additionalMultiPackageComponent != "" {
			extraComps = append(extraComps, additionalMultiPackageComponent)
			name += " extra:multi"
		}
		testCase := FieldOverrideTestCase{
			Name:                            name,
			MultiPackageSdkVersion:          tc.MultiPackageSdkVersion,
			PackageSdkVersion:               tc.PackageSdkVersion,
			WorkingDir:                      tc.WorkingDir,
			ExpectedResolution:              tc.ExpectedResolution.WithExtraComponents(extraComps...),
			MultiPackageAdditionalComponent: additionalMultiPackageComponent,
			PackageAdditionalComponent:      additionalPackageComponent,
		}

		result = append(result, testCase)
	}
	return
}

var fieldOverrideTestCases = lo.Flatten([][]FieldOverrideTestCase{
	makeTestCases("", ""),
	makeTestCases("", AdditionalMultiPackageComponent),
	makeTestCases(AdditionalPackageComponent, ""),
	makeTestCases(AdditionalPackageComponent, AdditionalMultiPackageComponent),
})

func pushGenericComponentWithCommand(t *testing.T, reg *httptest.Server, componentName, componentVersion, command string) {
	comp := component.Component{
		ManifestMeta: schema.ManifestMeta{
			Kind:       component.ComponentKind,
			APIVersion: component.ComponentAPIVersion,
		},
		Spec: &component.Spec{
			JarCommands: []component.JarCommand{
				{
					Name: command,
					Path: "./dummy",
					Desc: &command,
				},
			},
			Exports: component.Exports{
				"MEEP_EXTERNAL_DAR": &component.Export{
					Paths:            []string{"./component.yaml", "./"},
					ConflictStrategy: "extend",
				},

				"SHEEP_EXTERNAL_DAR": &component.Export{
					Paths:            []string{"./component.yaml", "./"},
					ConflictStrategy: "extend",
				},
			},
		},
	}

	compBytes, err := yaml.Marshal(comp)
	require.NoError(t, err)

	ctx := testutil.Context(t)

	compDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(compDir, "component.yaml"), compBytes, 0666))
	require.NoError(t, os.WriteFile(filepath.Join(compDir, "dummy"), []byte{}, 0666))
	testutil.PushComponent(t, ctx, reg, componentName, componentVersion, compDir)
}

func (suite *MainSuite) TestFieldOverrideExhaustive() {
	t := suite.T()
	testFieldOverrideExhaustive(t, func(t *testing.T, testCase FieldOverrideTestCase, dirs TestCaseDirs) {})
}

func installComponent(t *testing.T, componentName, componentVersion string) {
	contents := fmt.Sprintf(`
components:
  - %s:%s
`, componentName, componentVersion)

	t.Run("install component "+componentName, func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "daml.yaml"), []byte(contents), 0666))
		t.Chdir(tmpDir)
		require.NoError(t, createStdTestRootCmd(t, "install", "package").Execute())
	})

}

func testFieldOverrideExhaustive(t *testing.T, hook func(t *testing.T, testCase FieldOverrideTestCase, dirs TestCaseDirs)) {
	tmpDamlHome := t.TempDir()
	t.Setenv(assistantconfig.DpmHomeEnvVar, tmpDamlHome)

	installSdkForComponent(t, globalSdkVersion, globalSdkComponent, "999.999.999")
	installSdkForComponent(t, someSdkVersion, someSdkComponent, "1.2.3")
	installSdkForComponent(t, someOtherSdkVersion, someOtherSdkComponent, "4.5.6")

	t.Setenv("PATH", testutil.TestdataPath(t, "fake-java", testutil.OS)+string(os.PathListSeparator)+os.Getenv("PATH"))
	v, _ := semver.StrictNewVersion("1.2.3")

	_, reg := testutil.StartRegistry(t)
	pushGenericComponentWithCommand(t, reg, AdditionalMultiPackageComponent, v.String(), AdditionalMultiPackageComponent)
	pushGenericComponentWithCommand(t, reg, AdditionalPackageComponent, v.String(), AdditionalPackageComponent)
	installComponent(t, AdditionalPackageComponent, v.String())
	installComponent(t, AdditionalMultiPackageComponent, v.String())

	setupTestCase := func(tc FieldOverrideTestCase) (dirs TestCaseDirs) {
		tmpDir := t.TempDir()
		dirs.MultiPackageDir = filepath.Join(tmpDir, "multi-package")
		dirs.DamlPackageDir = filepath.Join(dirs.MultiPackageDir, "daml-package")
		require.NoError(t, utils.EnsureDirs(dirs.MultiPackageDir, dirs.DamlPackageDir))

		// create multi-package.yaml
		multiPackage := multipackage.MultiPackage{
			SdkVersion: asSdkVersion(tc.MultiPackageSdkVersion),
			Packages:   []string{"./daml-package"},
		}

		if tc.MultiPackageAdditionalComponent != "" {
			// TODO DeprecatedOverrideComponents is being used here because
			// the Components field is being ignored (`yaml:"-"`) in the YAML marshaling
			multiPackage.DeprecatedOverrideComponents = map[string]*sdkmanifest.Component{
				AdditionalMultiPackageComponent: {
					Name:    AdditionalMultiPackageComponent,
					Version: sdkmanifest.AssemblySemVer(v),
				},
			}
		}
		require.NoError(t,
			os.WriteFile(filepath.Join(dirs.MultiPackageDir, "multi-package.yaml"), testutil.MustMarshal(t, multiPackage), 0666),
		)

		// create daml.yaml
		damlPackage := damlpackage.DamlPackage{
			SdkVersion: asSdkVersion(tc.PackageSdkVersion),
		}
		if tc.PackageAdditionalComponent != "" {
			damlPackage.DeprecatedOverrideComponents = map[string]*sdkmanifest.Component{
				AdditionalPackageComponent: {
					Name:    AdditionalPackageComponent,
					Version: sdkmanifest.AssemblySemVer(v),
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

	for _, tc := range fieldOverrideTestCases {
		t.Run(tc.Name, func(t *testing.T) {
			dirs := setupTestCase(tc)

			hook(t, tc, dirs)

			if tc.ExpectedResolution.ExpectedSdkVersion == "null" {
				t.Run("assert no active sdk version", func(t *testing.T) {
					assertNoActiveSdkVersion(t)
				})
				t.Run("test resolution", func(t *testing.T) {
					testResolution(t, tc.ExpectedResolution)
				})

			} else {
				t.Run("assert active sdk version", func(t *testing.T) {
					assertActiveSdkVersion(t, tc.ExpectedResolution.ExpectedSdkVersion)
				})
				t.Run("test resolution", func(t *testing.T) {
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

func asSdkVersion(s string) string {
	if s == "null" {
		return ""
	}
	return s
}
