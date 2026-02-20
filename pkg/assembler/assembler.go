// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package assembler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/builtincommand"
	"daml.com/x/assistant/pkg/component"
	"daml.com/x/assistant/pkg/ocipuller"
	"daml.com/x/assistant/pkg/ocipuller/remotepuller"
	"daml.com/x/assistant/pkg/resolution"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/simpleplatform"
	"daml.com/x/assistant/pkg/utils"
	"github.com/samber/lo"
)

const (
	AssistantBinNameUnix    = "dpm"
	AssistantBinNameWindows = "dpm.exe"
)

func AssistantBinName(osName string) string {
	return lo.Ternary(osName == "windows", AssistantBinNameWindows, AssistantBinNameUnix)
}

type Assembler struct {
	config *assistantconfig.Config
	puller ocipuller.OciPuller

	// use this platform instead of the host machine's, this is mostly to support validating SDK bundles cross-platformly
	overridePlatform *simpleplatform.NonGeneric

	DependencyPathWarnOnly bool

	ExportsPathsWarnOnly bool
}

type AssemblyResult struct {
	ValidatedCommands map[string][]*ValidatedCommand
	// will be non-nil if the input assembly manifest included an assistant
	AssistantAbsolutePath *string

	// shallow resolution of a particular assembly
	ShallowResolution *resolution.Package
}

func New(config *assistantconfig.Config, puller ocipuller.OciPuller) *Assembler {
	return &Assembler{config, puller, nil, false, false}
}

func NewWithOverriddenPlatform(config *assistantconfig.Config, puller ocipuller.OciPuller, overridePlatform *simpleplatform.NonGeneric) *Assembler {
	return &Assembler{config, puller, overridePlatform, false, false}
}

func (a *Assembler) ReadAndAssemble(ctx context.Context, assemblyManifestPath string) (*AssemblyResult, error) {
	assemblyManifest, err := sdkmanifest.ReadSdkManifest(assemblyManifestPath)
	if err != nil {
		return nil, err
	}
	return a.Assemble(ctx, assemblyManifest)
}

// Assemble processes an sdk manifest, and crawls all the components specified in it,
// validating them and their commands.
// It automatically fetches OCI components not present locally in our dpm-home's cache,
// including the assistant itself (if included in the sdk manifest).
func (a *Assembler) Assemble(ctx context.Context, assemblyManifest *sdkmanifest.SdkManifest) (*AssemblyResult, error) {
	return a.AssembleManyWithOverlay(ctx, assemblyManifest)
}

func (a *Assembler) AssembleManyWithOverlay(ctx context.Context, assemblyManifests ...*sdkmanifest.SdkManifest) (*AssemblyResult, error) {
	components := make(map[string]*ResolvedComponent)

	var sdkVersion *sdkmanifest.SemVer
	for _, assemblyManifest := range assemblyManifests {
		manifestComponents, err := a.collectComponents(ctx, assemblyManifest)
		if err != nil {
			return nil, err
		}
		maps.Copy(components, manifestComponents)
		if assemblyManifest.Spec.Version != nil {
			sdkVersion = assemblyManifest.Spec.Version
		}
	}

	cmds := extractCommands(components)
	if err := validate(lo.Flatten(lo.Values(cmds))); err != nil {
		return nil, err
	}

	if err := a.setCommandsDependencyPaths(cmds, components); err != nil {
		return nil, err
	}

	// set commands' sdk version
	if sdkVersion != nil {
		for _, cs := range cmds {
			for _, c := range cs {
				v := sdkVersion.Value()
				c.SdkVersion = &v
			}
		}
	}

	imports, err := a.computeImports(components)
	if err != nil {
		return nil, err
	}

	result := &AssemblyResult{
		ValidatedCommands: cmds,
		ShallowResolution: &resolution.Package{
			Imports: imports,
			Components: lo.MapValues(components, func(component *ResolvedComponent, name string) string {
				return component.AbsolutePath
			}),
		},
	}

	// if the first assembly manifest (assumed to be the base, i.e. the one corresponding to the active installed SDK)
	// defines an Assistant component
	if len(assemblyManifests) > 0 && assemblyManifests[0].Spec.Assistant != nil {
		assistantBinPath, err := a.collectAssistant(ctx, assemblyManifests[0].Spec.Assistant)
		if err != nil {
			return nil, err
		}
		result.AssistantAbsolutePath = &assistantBinPath
	}

	return result, nil
}

func (a *Assembler) setCommandsDependencyPaths(cmds map[string][]*ValidatedCommand, components map[string]*ResolvedComponent) error {
	for compName, commands := range cmds {
		deps := components[compName].Spec.DependencyPaths
		if deps == nil {
			continue
		}

		resolvedDeps := map[string]string{}
		for dep, envVar := range deps {
			comp, ok := components[dep]
			if !ok {
				err := fmt.Errorf("component %q has dependency on component %q which wasn't included in the assembly", compName, dep)
				if a.DependencyPathWarnOnly {
					slog.Warn(err.Error())
					continue
				} else {
					return err
				}
			}

			if !utils.IsValidEnvVarIdentifier(envVar) {
				err := fmt.Errorf(
					"component %q has an invalid env var key (%q) for its depenency %q. "+
						"Must be a valid identifier", compName, envVar, dep,
				)
				if a.DependencyPathWarnOnly {
					slog.Warn(err.Error())
					continue
				} else {
					return err
				}
			}
			resolvedDeps[envVar] = comp.AbsolutePath
		}

		for _, cmd := range commands {
			cmd.ResolvedDependencies = maps.Clone(resolvedDeps)
		}
	}
	return nil
}

type ValidatedCommand struct {
	component.Command
	AbsolutePath         string
	ComponentName        string
	ResolvedDependencies map[string]string // <env var key> -> <some component's absolute path>
	SdkVersion           *semver.Version
}

type ResolvedComponent struct {
	*component.Component
	ComponentName string
	AbsolutePath  string
}

func extractCommands(comps map[string]*ResolvedComponent) map[string][]*ValidatedCommand {
	return lo.MapValues(comps, func(comp *ResolvedComponent, _ string) []*ValidatedCommand {
		return lo.Map(comp.Spec.AllCommands(), func(c component.Command, _ int) *ValidatedCommand {
			return &ValidatedCommand{
				Command:       c,
				AbsolutePath:  utils.ResolvePath(comp.AbsolutePath, c.GetPath()),
				ComponentName: comp.ComponentName,
			}
		})
	})
}

func validate(commands []*ValidatedCommand) error {
	var errs []error

	groupedByName := lo.GroupByMap(commands, func(cmd *ValidatedCommand) (string, string) {
		return cmd.GetName(), cmd.ComponentName
	})

	for cmd, comps := range groupedByName {
		if len(comps) > 1 {
			errs = append(errs, fmt.Errorf("command named %q is defined in multiple components %v", cmd, comps))
		}
	}

	builtin := lo.SliceToMap(builtincommand.BuiltinCommands, func(b builtincommand.BuiltinCommand) (string, struct{}) {
		return string(b), struct{}{}
	})
	for _, cmd := range commands {
		_, ok := builtin[cmd.GetName()]
		if ok {
			errs = append(errs, fmt.Errorf("command named %q (from component %q) conflicts with the assistant's built-in commands", cmd.GetName(), cmd.ComponentName))
		}
	}

	aliases := lo.FlatMap(commands, func(c *ValidatedCommand, _ int) []lo.Entry[string, string] {
		return lo.Map(c.GetAliases(), func(alias string, _ int) lo.Entry[string, string] {
			return lo.Entry[string, string]{
				Key:   alias,
				Value: c.ComponentName,
			}
		})
	})
	groupedByAlias := lo.GroupByMap(aliases, func(p lo.Entry[string, string]) (string, string) {
		return p.Key, p.Value
	})
	for alias, comps := range groupedByAlias {
		if len(comps) > 1 {
			errs = append(errs, fmt.Errorf("command alias %q is used by multiple components %v", alias, comps))
		}
	}

	uniqueByPath := lo.UniqBy(commands, func(cmd *ValidatedCommand) string { return cmd.AbsolutePath })
	for _, c := range uniqueByPath {
		errMsg := fmt.Sprintf("component %q command validation failed for command %q", c.ComponentName, c.GetName())
		f, err := os.Stat(c.AbsolutePath)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", errMsg, err))
			continue
		}
		if f.IsDir() {
			errs = append(errs, fmt.Errorf("%s: %q is a directory", errMsg, c.AbsolutePath))
		}
	}

	return errors.Join(errs...)
}

func (a *Assembler) collectAssistant(ctx context.Context, assistant *sdkmanifest.Component) (string, error) {
	if assistant.LocalPath != nil {
		return "", fmt.Errorf("assistant can only be OCI and not a local-path")
	}
	p, err := a.handleOCI(ctx, assistant)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		return "", err
	}
	msg := "collected assistant binary is invalid"
	filenames := lo.Map(entries, func(de os.DirEntry, _ int) string {
		return de.Name()
	})

	// TODO this can be improved by instead using the platform metadata of the pulled OCI image
	bin, ok := lo.Find(entries, func(de os.DirEntry) bool {
		return lo.Contains([]string{AssistantBinNameUnix, AssistantBinNameWindows}, de.Name())
	})
	if !ok {
		return "", fmt.Errorf("%s: could not determine the dpm binary file among %v", msg, filenames)
	}

	absPath := filepath.Join(p, bin.Name())

	info, err := bin.Info()
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("collected assistant binary %q is invalid: expect a file and not a directory", absPath)
	}
	return absPath, nil
}

// component name -> *ResolvedComponent
func (a *Assembler) collectComponents(ctx context.Context, assemblyManifest *sdkmanifest.SdkManifest) (result map[string]*ResolvedComponent, err error) {
	result = make(map[string]*ResolvedComponent)
	for _, comp := range assemblyManifest.Spec.Components {
		resolved, err := a.collectComponent(ctx, assemblyManifest.AbsolutePath, comp)
		if err != nil {
			return nil, err
		}
		result[comp.Name] = resolved
	}

	return result, nil
}

func (a *Assembler) collectComponent(ctx context.Context, basePath string, comp *sdkmanifest.Component) (*ResolvedComponent, error) {
	var p string
	var err error
	if comp.LocalPath != nil {
		p = a.handleLocalDir(filepath.Dir(basePath), *comp.LocalPath)
	} else {
		p, err = a.handleOCI(ctx, comp)
		if err != nil {
			return nil, err
		}
	}

	parsedComp, err := component.ReadComponent(filepath.Join(p, "component.yaml"))
	if err != nil {
		return nil, err
	}

	absPath, err := filepath.Abs(p)
	if err != nil {
		return nil, err
	}

	return &ResolvedComponent{
		Component:     parsedComp,
		ComponentName: comp.Name,
		AbsolutePath:  absPath,
	}, nil
}

func (a *Assembler) handleLocalDir(basePath, componentPath string) string {
	return utils.ResolvePath(basePath, componentPath)
}

func (a *Assembler) handleOCI(ctx context.Context, comp *sdkmanifest.Component) (string, error) {
	destPath := a.ociComponentPath(comp)
	tag := ComputeTagOrDigest(comp)

	// check if component is already in the cache
	ok, err := utils.DirExists(destPath)
	if err != nil {
		return "", err
	}
	if !ok {
		if _, isRemote := a.puller.(*remotepuller.RemoteOciPuller); isRemote && !a.config.AutoInstall {
			// TODO there's no dedicated command for installing remote overridden components like there is for installing SDKs.
			// As auto-installation of SDKs is disabled by default, so is pulling overridden remote components, as both SDKs and remote overrides
			// are using the same AutoInstall config variable...
			// so currently the only way the assistant to have the assistant pull remote overrides is to have the user enable auto-install
			return "", fmt.Errorf("sdk component is missing and won't be downloaded because auto-install is disabled")
		}
		platform := simpleplatform.CurrentPlatform()
		if a.overridePlatform != nil {
			platform = a.overridePlatform
		}
		fmt.Printf("pulling sdk component %s %s...\n", comp.Name, tag)
		if err := a.puller.PullComponent(ctx, comp.Name, tag, destPath, platform); err != nil {
			return "", err
		}
	}

	return destPath, nil
}

func ComputeTagOrDigest(comp *sdkmanifest.Component) string {
	// TODO fully flesh this out
	return comp.Version.Value().String()
}

func (a *Assembler) ociComponentPath(comp *sdkmanifest.Component) string {
	return filepath.Join(a.config.CachePath, "components", comp.Name, comp.Version.Value().String())
}

// computeImports merges all components' component.Exports, taking into account their conflict strategy,
// and spits out resulting Imports
func (a *Assembler) computeImports(components map[string]*ResolvedComponent) (resolution.Imports, error) {
	mergedExports, err := a.mergeComponentsExports(lo.Values(components))
	if err != nil {
		return nil, err
	}

	return mergedExports.AsImports(), nil
}

// var string -> component names set
type exportsConflicts map[string]map[string]struct{}

func (conflicts exportsConflicts) append(key, componentName string) {
	if _, exists := conflicts[key]; !exists {
		conflicts[key] = make(map[string]struct{})
	}
	conflicts[key][componentName] = struct{}{}
}

func (conflicts exportsConflicts) asError() error {
	if len(conflicts) == 0 {
		return nil
	}
	var errs []error

	for k, componentNamesSet := range conflicts {
		componentNames := strings.Join(lo.Keys(componentNamesSet), ", ")
		if len(componentNamesSet) > 1 {
			errs = append(errs, fmt.Errorf("multiple components ([%s]) export the var '%s', but at least one of them defined its conflict-strategy as '%s'", componentNames, k, component.ExportConflictStrategyFail))
		}
	}

	return errors.Join(errs...)
}

func (a *Assembler) mergeComponentsExports(components []*ResolvedComponent) (component.Exports, error) {
	conflicts := make(exportsConflicts)
	var pathErrs []error

	exports := make(component.Exports)
	for _, c := range components {
		compExports := c.Spec.Exports
		if compExports == nil {
			continue
		}

		for k, newExport := range compExports {
			// make sure the component name is set
			newExport.ComponentName = c.ComponentName

			if _, alreadyExists := exports[k]; !alreadyExists {
				exports[k] = &component.Export{
					ComponentName:    newExport.ComponentName, // use the first encountered component's name
					Var:              k,
					Paths:            []string{},
					ConflictStrategy: newExport.ConflictStrategy,
				}
			}
			e := exports[k]

			// check for conflicts
			if e.ComponentName != newExport.ComponentName && (e.ConflictStrategy != component.ExportConflictStrategyExtend || newExport.ConflictStrategy != component.ExportConflictStrategyExtend) {
				conflicts.append(k, e.ComponentName)
				conflicts.append(k, newExport.ComponentName)
				continue
			}

			absolutePaths := lo.Map(newExport.Paths, func(p string, _ int) string {
				return utils.ResolvePath(c.AbsolutePath, p)
			})

			// validate paths
			for _, p := range absolutePaths {
				_, err := os.Stat(p)
				if os.IsNotExist(err) {
					pathErr := fmt.Errorf("component's %q export %q defines a path that doesn't exist: %q", c.ComponentName, k, p)
					pathErrs = append(pathErrs, pathErr)
				} else if err != nil {
					slog.Warn(err.Error())
				}
			}

			e.Paths = append(e.Paths, absolutePaths...)
		}
	}

	err := errors.Join(pathErrs...)
	if err != nil {
		if !a.ExportsPathsWarnOnly {
			return nil, err
		}

		for _, e := range pathErrs {
			slog.Warn(e.Error())
		}
	}

	if err := errors.Join(conflicts.asError()); err != nil {
		return nil, err
	}

	return exports, nil
}
