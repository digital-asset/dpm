// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package resolver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"daml.com/x/assistant/cmd/dpm/cmd/resolve/resolutionerrors"
	"daml.com/x/assistant/pkg/assembler"
	"daml.com/x/assistant/pkg/assembler/assemblyplan"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/darmanifest"
	"daml.com/x/assistant/pkg/multipackage"
	"daml.com/x/assistant/pkg/packagelock"
	"daml.com/x/assistant/pkg/resolution"
	"daml.com/x/assistant/pkg/schema"
	"daml.com/x/assistant/pkg/utils"
	"github.com/samber/lo"
)

type DeepResolver struct {
	assembler *assembler.Assembler
	config    *assistantconfig.Config
}

func New(config *assistantconfig.Config, a *assembler.Assembler) *DeepResolver {
	return &DeepResolver{
		assembler: a,
		config:    config,
	}
}

// RunDeepResolution resolves the active components needed by each and every package in a (single or multi-package) project
func (d *DeepResolver) RunDeepResolution(ctx context.Context) (*resolution.Resolution, error) {
	pkgs, err := d.resolvePackages(ctx)
	if err != nil {
		return nil, err
	}

	defaultSdk := d.resolveDefaultSdk(ctx)

	return &resolution.Resolution{
		ManifestMeta: schema.ManifestMeta{
			APIVersion: resolution.ApiVersion,
			Kind:       resolution.Kind,
		},
		Packages:   pkgs,
		DefaultSDK: defaultSdk,
	}, nil
}

func (d *DeepResolver) resolvePackages(ctx context.Context) (resolution.Packages, error) {
	// multi-package
	multiPackagePath, isMultiPackage, err := assistantconfig.GetMultiPackageAbsolutePath()
	if err != nil {
		return nil, fmt.Errorf("failed to determine whether a multi-package is in scope: %w", err)
	}
	if isMultiPackage {
		multiPackage, err := multipackage.Read(multiPackagePath)
		if err != nil {
			return nil, err
		}
		return d.resolve(ctx, multiPackage.AbsolutePackages()...)
	}

	// single package
	damlPackagePath, isDamlPackage, err := assistantconfig.GetDamlPackageAbsolutePath()
	if err != nil {
		return nil, err
	}
	if isDamlPackage {
		return d.resolve(ctx, filepath.Dir(damlPackagePath))
	}

	// no packages to resolve at all
	return make(resolution.Packages), nil
}

func (d *DeepResolver) resolve(ctx context.Context, packageAbsolutePaths ...string) (resolution.Packages, error) {
	pkgs := make(resolution.Packages)

	for _, p := range packageAbsolutePaths {
		// if the path is a symlink, resolve it first
		resolvedPath, err := filepath.EvalSymlinks(p)
		if err != nil {
			pkgs[p] = &resolution.Package{Errors: []*resolutionerrors.ResolutionError{
				resolutionerrors.NewDamlYamlNotFoundError(err),
			}}
			continue
		}

		if result, err := d.resolvePackageAndDars(ctx, resolvedPath); err != nil {
			pkgs[resolvedPath] = &resolution.Package{
				Errors: resolutionerrors.Standardize(err),
			}
		} else {
			pkgs[resolvedPath] = result
		}
	}

	return pkgs, nil
}

func (d *DeepResolver) resolvePackageAndDars(ctx context.Context, absPath string) (*resolution.Package, error) {
	result, err := d.resolvePackage(ctx, absPath)
	if err != nil {
		return nil, err
	}

	if !assistantconfig.DpmLockfileEnabled() {
		return result.ShallowResolution, nil

	}
	lock, err := packagelock.ReadPackageLock(filepath.Join(absPath, assistantconfig.DpmLockFileName))
	if errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	paths := lo.Map(lock.Dars, func(d *packagelock.Dar, _ int) string {
		return d.Path
	})
	if len(paths) > 0 {
		result.ShallowResolution.Imports[resolution.DarImportsFields] = paths
	}

	return result.ShallowResolution, nil
}

func (d *DeepResolver) resolvePackage(ctx context.Context, absPath string) (*assembler.AssemblyResult, error) {
	assemblyPlan, err := assemblyplan.NewShallow(ctx, d.config, d.assembler, filepath.Join(absPath, assistantconfig.DamlPackageFilename))
	if err != nil {
		return nil, err
	}
	result, err := assemblyPlan.Assemble(ctx)
	if err != nil {
		return nil, err
	}

	if assistantconfig.DpmDarsEnabled() {
		resolvedDeps, resolvedDataDeps, err := d.resolvePackageDars(absPath)
		if err != nil {
			return nil, err
		}
		result.ShallowResolution.ResolvedDependencies = resolvedDeps
		result.ShallowResolution.ResolvedDataDependencies = resolvedDataDeps
	}

	return result, nil
}

func (d *DeepResolver) resolvePackageDars(absPath string) (deps []string, dataDeps []string, err error) {
	p, err := damlpackage.Read(filepath.Join(absPath, assistantconfig.DamlPackageFilename))
	if err != nil {
		return nil, nil, err
	}

	var errs []error

	for _, dar := range p.ParsedDarDependencies.Dependencies {
		r, err := d.resolveDar(dar)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		deps = append(deps, r...)
	}

	for _, dar := range p.ParsedDarDependencies.DataDependencies {
		r, err := d.resolveDar(dar)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		dataDeps = append(dataDeps, r...)
	}

	if err := errors.Join(errs...); err != nil {
		return nil, nil, err
	}

	return
}

func (d *DeepResolver) resolveDar(dar *damlpackage.ParsedDarDependency) ([]string, error) {
	scheme := dar.FullUrl.Scheme

	if scheme == "builtin" {
		return []string{strings.TrimPrefix(dar.FullUrl.String(), scheme+"://")}, nil
	}
	if scheme == "file" {
		f := strings.TrimPrefix(dar.FullUrl.String(), scheme+"://")
		// verify the file exists
		if _, err := os.Stat(f); err != nil {
			return nil, fmt.Errorf("dar file doesn't exist: %w", err)
		}

		// evaluate symlink (if it is one)
		evaluatedSymlink, err := filepath.EvalSymlinks(f)
		if err != nil {
			return nil, err
		}

		return []string{evaluatedSymlink}, nil
	}
	if scheme == "oci" {
		_, ref, err := dar.GetOciRemote()
		if err != nil {
			return nil, err
		}

		darDir := d.config.CachePathForDar(ref)
		ok, err := utils.DirExists(darDir)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, resolutionerrors.NewDarNotInstalled(fmt.Errorf("dar %q is not installed", dar.FullUrl))
		}

		darManifestPath := filepath.Join(darDir, assistantconfig.DarManifestName)
		darManifest, err := darmanifest.ReadDarManifest(darManifestPath)
		if err != nil {
			return nil, err
		}

		var dars []string
		for _, p := range darManifest.Spec.Paths {
			dars = append(dars, utils.ResolvePath(darDir, p))
		}
		return dars, nil
	}

	return nil, fmt.Errorf("unsupported schema %s", scheme)
}

func (d *DeepResolver) resolveDefaultSdk(ctx context.Context) resolution.DefaultSDK {
	// <sdk-version> -> resolution.Package
	defaultSdk := make(resolution.DefaultSDK)

	installedSdk, err := assistantconfig.GetInstalledSdkFromEnvOrDefault(d.config)
	if err != nil {
		// in case where we can't determine what the sdk-version is,
		// because there aren't any installed, and the user didn't set DPM_SDK_VERSION
		defaultSdk[assistantconfig.GetSdkVersionOverrideWithFallback("unknown–sdk-version")] = &resolution.Package{
			Errors: resolutionerrors.Standardize(err),
		}
		return defaultSdk
	}

	result, err := d.assembler.ReadAndAssemble(ctx, installedSdk.ManifestPath)
	if err != nil {
		defaultSdk[installedSdk.Version.String()] = &resolution.Package{
			Errors: resolutionerrors.Standardize(err),
		}
	} else {
		v := installedSdk.Version.String()
		defaultSdk[v] = result.ShallowResolution
		defaultSdk[v].SdkVersion = v
	}
	return defaultSdk
}
