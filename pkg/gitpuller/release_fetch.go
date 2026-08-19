package gitpuller

import (
	"context"
	"fmt"
	"strings"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/damlpackage"
	"github.com/samber/lo"
)

// PrepareGitDependencies expands, canonicalizes, and fetches missing release assets.
func PrepareGitDependencies(ctx context.Context, config *assistantconfig.Config, yamlPath string) (*damlpackage.DamlPackage, int, error) {
	pkg, err := damlpackage.Read(yamlPath)
	if err != nil {
		return nil, 0, err
	}

	expanded, err := damlpackage.ExpandGitReleaseDependenciesInYaml(ctx, yamlPath, "dependencies", pkg.Dependencies)
	if err != nil {
		return nil, 0, err
	}
	expandedData, err := damlpackage.ExpandGitReleaseDependenciesInYaml(ctx, yamlPath, "data-dependencies", pkg.DataDependencies)
	if err != nil {
		return nil, 0, err
	}
	if expanded || expandedData {
		pkg, err = damlpackage.Read(yamlPath)
		if err != nil {
			return nil, 0, err
		}
	}

	canonicalized, err := damlpackage.CanonicalizeGitDependenciesInYaml(yamlPath, "dependencies", pkg.Dependencies)
	if err != nil {
		return nil, 0, err
	}
	canonicalizedData, err := damlpackage.CanonicalizeGitDependenciesInYaml(yamlPath, "data-dependencies", pkg.DataDependencies)
	if err != nil {
		return nil, 0, err
	}
	if canonicalized || canonicalizedData {
		pkg, err = damlpackage.Read(yamlPath)
		if err != nil {
			return nil, 0, err
		}
	}

	deps := append(
		lo.Values(pkg.ParsedDarDependencies.Dependencies),
		lo.Values(pkg.ParsedDarDependencies.DataDependencies)...,
	)
	fetched, err := FetchMissingReleaseAssets(ctx, config, deps)
	return pkg, fetched, err
}

// GitReleaseAssets returns git release dependencies with a non-empty asset name.
func GitReleaseAssets(deps []*damlpackage.ParsedDarDependency) []*damlpackage.ParsedDarDependency {
	var releaseDeps []*damlpackage.ParsedDarDependency
	for _, dep := range deps {
		if dep == nil || !dep.GitRelease || strings.TrimSpace(dep.DarPath) == "" {
			continue
		}
		releaseDeps = append(releaseDeps, dep)
	}
	return releaseDeps
}

// CountCachedReleaseAssets returns how many release assets are cached and the total count.
func CountCachedReleaseAssets(config *assistantconfig.Config, deps []*damlpackage.ParsedDarDependency) (cached, total int) {
	releaseDeps := GitReleaseAssets(deps)
	total = len(releaseDeps)
	for _, dep := range releaseDeps {
		if DarIsCached(config, dep) {
			cached++
		}
	}
	return cached, total
}

// FetchMissingReleaseAssets downloads uncached git release assets from deps.
func FetchMissingReleaseAssets(ctx context.Context, config *assistantconfig.Config, deps []*damlpackage.ParsedDarDependency) (int, error) {
	fetched := 0
	for _, dep := range deps {
		if dep == nil || !dep.GitRelease || strings.TrimSpace(dep.DarPath) == "" {
			continue
		}
		if DarIsCached(config, dep) {
			continue
		}
		if _, err := PullGitDar(ctx, config, dep); err != nil {
			return fetched, fmt.Errorf("git release asset %q: %w", damlpackage.FormatGitYamlLine(dep), err)
		}
		fetched++
	}
	return fetched, nil
}
