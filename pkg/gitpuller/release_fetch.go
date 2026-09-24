package gitpuller

import (
	"context"
	"fmt"
	"strings"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/gitparse"
)

// PrepareGitDependencies expands, canonicalizes, and fetches missing release assets.
func PrepareGitDependencies(ctx context.Context, config *assistantconfig.Config, yamlPath string) (*damlpackage.DamlPackage, int, error) {
	pkg, err := damlpackage.Read(yamlPath)
	if err != nil {
		return nil, 0, err
	}

	fields := []string{"dependencies", "data-dependencies"}
	rewritten := false
	for _, field := range fields {
		expanded, err := damlpackage.ExpandGitReleaseDependenciesInYaml(ctx, yamlPath, field, pkg.Deps(field))
		if err != nil {
			return nil, 0, err
		}
		rewritten = rewritten || expanded
	}
	if rewritten {
		pkg, err = damlpackage.Read(yamlPath)
		if err != nil {
			return nil, 0, err
		}
	}

	rewritten = false
	for _, field := range fields {
		canonicalized, err := damlpackage.CanonicalizeGitDependenciesInYaml(yamlPath, field, pkg.Deps(field))
		if err != nil {
			return nil, 0, err
		}
		rewritten = rewritten || canonicalized
	}
	if rewritten {
		pkg, err = damlpackage.Read(yamlPath)
		if err != nil {
			return nil, 0, err
		}
	}

	fetched, err := FetchMissingReleaseAssets(ctx, config, allParsedDeps(pkg))
	return pkg, fetched, err
}

func allParsedDeps(pkg *damlpackage.DamlPackage) []*damlpackage.ParsedDarDependency {
	if pkg == nil {
		return nil
	}
	var deps []*damlpackage.ParsedDarDependency
	for _, field := range []string{"dependencies", "data-dependencies"} {
		_, parsed := pkg.RawAndParsed(field)
		for _, dep := range parsed {
			deps = append(deps, dep)
		}
	}
	return deps
}

func ReportPreparedGitDependencies(config *assistantconfig.Config, pkg *damlpackage.DamlPackage, fetched int, extraBlankLine bool) {
	var msg string
	switch {
	case fetched > 0:
		msg = fmt.Sprintf("Fetched %d git release assets", fetched)
	default:
		if cached, total := CountCachedReleaseAssets(config, GitReleaseAssets(allParsedDeps(pkg))); total > 0 && cached == total {
			msg = fmt.Sprintf("All %d git release assets already cached", total)
		}
	}
	if msg == "" {
		return
	}
	if extraBlankLine {
		fmt.Printf("%s\n\n", msg)
		return
	}
	fmt.Println(msg)
}

// GitReleaseAssets returns git release dependencies with a non-empty asset name.
func GitReleaseAssets(deps []*damlpackage.ParsedDarDependency) []*damlpackage.ParsedDarDependency {
	var releaseDeps []*damlpackage.ParsedDarDependency
	for _, dep := range deps {
		if dep == nil || !dep.Git.Release || strings.TrimSpace(dep.Git.DarPath) == "" {
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
		if dep == nil || !dep.Git.Release || strings.TrimSpace(dep.Git.DarPath) == "" {
			continue
		}
		if DarIsCached(config, dep) {
			continue
		}
		if _, err := PullGitDar(ctx, config, dep); err != nil {
			return fetched, fmt.Errorf("git release asset %q: %w", gitparse.FormatGitYamlLine(dep.Git), err)
		}
		fetched++
	}
	return fetched, nil
}
