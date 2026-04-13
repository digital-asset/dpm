// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package assemblyplan

import (
	"context"
	"errors"
	"fmt"
	"os"

	"daml.com/x/assistant/cmd/dpm/cmd/resolve/resolutionerrors"
	"daml.com/x/assistant/pkg/assembler"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/multipackage"
	"daml.com/x/assistant/pkg/sdkinstall"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/versions"
	"github.com/Masterminds/semver/v3"
)

// AssemblyPlan decides what the final commands that'll be added to the assistant root command are,
// or rather where they'll be sourced from.
// It takes into account the existence of daml.yaml, multi-package.yaml, override-component or lack
// thereof (and also, importantly, the CWD) in making that decision.
type AssemblyPlan struct {
	// the base will contain all the components from the effective SDK version,
	// or be completely empty in the blank sdk case
	Base       sdkmanifest.SdkManifest
	SdkVersion *semver.Version // the effective sdk version (for the current working directory)

	// this will contain any component-overrides (from multi-package.yaml) to overlay on top of the base
	MultiPackage *sdkmanifest.SdkManifest

	// this will contain any component-overrides (from daml.yaml) to overlay on top of (Base + MultiPackage).
	// i.e. this will effectively have the highest precedence
	DamlPackage *sdkmanifest.SdkManifest

	assembler *assembler.Assembler
	config    *assistantconfig.Config
}

func New(ctx context.Context, config *assistantconfig.Config, a *assembler.Assembler) (plan *AssemblyPlan, err error) {
	if forceAssemblyPath, ok := os.LookupEnv("DPM_ASSEMBLY"); ok {
		b, err := sdkmanifest.ReadSdkManifest(forceAssemblyPath)
		if err != nil {
			return nil, err
		}
		return &AssemblyPlan{
			config:    config,
			assembler: a,
			Base:      *b,
		}, nil
	}

	damlPackagePath, _, err := assistantconfig.GetDamlPackageAbsolutePath()
	if err != nil {
		return nil, err
	}

	return NewShallow(ctx, config, a, damlPackagePath)
}

func NewShallow(ctx context.Context, config *assistantconfig.Config, a *assembler.Assembler, damlPackagePath string) (*AssemblyPlan, error) {
	sdkVersion, err := versions.GetActiveVersion(config, damlPackagePath)
	if err != nil {
		return nil, err
	}

	plan := &AssemblyPlan{
		config:     config,
		assembler:  a,
		SdkVersion: sdkVersion,
	}

	var installedSdk *assistantconfig.InstalledSdkVersion

	if damlPackagePath != "" {
		damlPackage, err := damlpackage.Read(damlPackagePath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, resolutionerrors.NewDamlYamlNotFoundError(err)
			}
			return nil, resolutionerrors.NewMalformedDamlYamlError(err)
		}

		if sdkVersion == nil {
			plan.Base = sdkmanifest.SdkManifest{
				AbsolutePath: "",
				Spec: &sdkmanifest.Spec{
					Components: map[string]*sdkmanifest.Component{},
				},
			}
		} else {
			installedSdk, err = getOrAutoInstallPackageSdk(ctx, config, sdkVersion.String(), damlPackagePath)
			if err != nil {
				return nil, err
			}

			base, err := sdkmanifest.ReadSdkManifest(installedSdk.ManifestPath)
			if err != nil {
				return nil, err
			}
			plan.Base = *base
		}

		if damlPackage.OverrideComponents != nil {
			plan.DamlPackage = &sdkmanifest.SdkManifest{
				AbsolutePath: damlPackagePath,
				Spec: &sdkmanifest.Spec{
					Components: damlPackage.OverrideComponents,
				},
			}
		}
	} else {
		installedSdk, err = assistantconfig.GetInstalledSdkVersion(config, sdkVersion)
		if err != nil {
			return nil, err
		}

		base, err := sdkmanifest.ReadSdkManifest(installedSdk.ManifestPath)
		if err != nil {
			return nil, err
		}
		plan.Base = *base
	}

	if err := configureMultiPackage(plan); err != nil {
		return nil, err
	}
	return plan, nil
}

func getOrAutoInstallPackageSdk(ctx context.Context, config *assistantconfig.Config, unparsedVersion string, sourceManifest string) (*assistantconfig.InstalledSdkVersion, error) {
	version, err := semver.NewVersion(unparsedVersion)
	if err != nil {
		return nil, err
	}

	installedSdk, err := assistantconfig.GetInstalledSdkVersion(config, version)
	if err == nil {
		return installedSdk, nil
	} else if !errors.Is(err, assistantconfig.ErrTargetSdkNotInstalled) {
		return nil, err
	}

	if !config.AutoInstall {
		return nil, resolutionerrors.NewSdkNotInstalledError(fmt.Errorf("%w. You can install the needed sdk (for %q) via 'dpm install %s'", err, sourceManifest, unparsedVersion))
	}

	installedSdk, err = sdkinstall.InstallSdkVersion(ctx, config, version)
	if err != nil {
		return nil, fmt.Errorf("error while auto-installing sdk version %q: %w", version.String(), err)
	}
	return installedSdk, nil
}

// configureMultiPackage mutates the AssemblyPlan to account for multi-package.yaml (if any)
func configureMultiPackage(plan *AssemblyPlan) error {
	multiPackagePath, hasMultiPackage, err := assistantconfig.GetMultiPackageAbsolutePath()
	if err != nil {
		return err
	}
	if !hasMultiPackage {
		return nil
	}
	multiPackage, err := multipackage.Read(multiPackagePath)
	if err != nil {
		return err
	}

	plan.MultiPackage = &sdkmanifest.SdkManifest{
		AbsolutePath: multiPackagePath,
		Spec: &sdkmanifest.Spec{
			Components: multiPackage.OverrideComponents,
		},
	}
	return nil
}

func (plan *AssemblyPlan) getOverrides() (assemblies []*sdkmanifest.SdkManifest) {
	if plan.MultiPackage != nil {
		assemblies = append(assemblies, plan.MultiPackage)
	}

	if plan.DamlPackage != nil {
		assemblies = append(assemblies, plan.DamlPackage)
	}
	return
}

// HasOverrides whether there are component-overrides defined in either multi-package.yaml or daml.yaml
func (plan *AssemblyPlan) HasOverrides() bool {
	return len(plan.getOverrides()) != 0
}

func (plan *AssemblyPlan) Assemble(ctx context.Context) (*assembler.AssemblyResult, error) {
	assemblies := []*sdkmanifest.SdkManifest{
		&plan.Base,
	}
	assemblies = append(assemblies, plan.getOverrides()...)

	result, err := plan.assembler.AssembleManyWithOverlay(ctx, assemblies...)
	if err != nil {
		return nil, err
	}

	for _, cs := range result.ValidatedCommands {
		for _, c := range cs {
			c.DpmSdkVersionEnvVar = assistantconfig.BlankSdkVersion
			if plan.SdkVersion != nil {
				c.DpmSdkVersionEnvVar = plan.SdkVersion.String()
			}
		}
	}

	return result, nil
}
