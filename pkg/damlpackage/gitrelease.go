package damlpackage

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"daml.com/x/assistant/pkg/githubrelease"
	"daml.com/x/assistant/pkg/gitparse"
	"daml.com/x/assistant/pkg/yamledit"
	"github.com/goccy/go-yaml"
)

// ExpandGitReleaseDependenciesInYaml expands release entries without assets in a daml.yaml field.
func ExpandGitReleaseDependenciesInYaml(ctx context.Context, yamlPath, fieldName string, rawDeps []*RawDependency) (bool, error) {
	if len(rawDeps) == 0 {
		return false, nil
	}

	expansionInputs, err := resolveGitAliasInputs(yamlPath, fieldName, rawDeps, func(dep *ParsedDarDependency) (string, bool) {
		if dep == nil || !dep.Git.Release || strings.TrimSpace(dep.Git.DarPath) != "" {
			return "", false
		}
		return gitparse.FormatGitReleaseBaseLine(dep.Git), true
	})
	if err != nil {
		return false, err
	}
	expanded, sourceIndices, err := expandReleaseGitDependenciesRaw(ctx, expansionInputs)
	if err != nil {
		return false, err
	}
	return rewriteYamlDependencyField(yamlPath, fieldName, rawDeps, expanded, sourceIndices)
}

// CanonicalizeGitDependenciesInYaml rewrites git dependency entries in a daml.yaml field
// to canonical git: one-liners.
func CanonicalizeGitDependenciesInYaml(yamlPath, fieldName string, rawDeps []*RawDependency) (bool, error) {
	if len(rawDeps) == 0 {
		return false, nil
	}

	aliasResolved, err := resolveGitAliasInputs(yamlPath, fieldName, rawDeps, func(dep *ParsedDarDependency) (string, bool) {
		if dep == nil || dep.Scheme() != "git" {
			return "", false
		}
		return gitparse.FormatGitYamlLine(dep.Git), true
	})
	if err != nil {
		return false, err
	}
	canonicalized, changed, err := CanonicalizeRawGitDependencies(aliasResolved)
	if err != nil {
		return false, err
	}
	if !changed && rawDependenciesEqual(rawDeps, aliasResolved) {
		return false, nil
	}
	return rewriteYamlDependencyField(yamlPath, fieldName, rawDeps, canonicalized, nil)
}

// ExpandReleaseGitDependenciesRaw expands release entries with no asset, preserving other entry shapes.
func ExpandReleaseGitDependenciesRaw(ctx context.Context, rawDeps []*RawDependency) ([]*RawDependency, error) {
	expanded, _, err := expandReleaseGitDependenciesRaw(ctx, rawDeps)
	return expanded, err
}

func expandReleaseGitDependenciesRaw(ctx context.Context, rawDeps []*RawDependency) ([]*RawDependency, []int, error) {
	existingAssets := map[string]struct{}{}
	for _, rawDep := range rawDeps {
		line, err := rawDep.Value()
		if err != nil {
			return nil, nil, err
		}
		if !gitparse.IsGitDependencyLine(line) {
			continue
		}
		dep, err := gitparse.ParseGitDependency(line)
		if err != nil {
			return nil, nil, err
		}
		if !dep.Git.Release || strings.TrimSpace(dep.Git.DarPath) == "" {
			continue
		}
		key, err := gitparse.GitLockKeyForDep(dep.Git)
		if err != nil {
			return nil, nil, err
		}
		existingAssets[key] = struct{}{}
	}

	var out []*RawDependency
	var sourceIndices []int
	for sourceIndex, rawDep := range rawDeps {
		line, err := rawDep.Value()
		if err != nil {
			return nil, nil, err
		}
		if !gitparse.IsGitDependencyLine(line) {
			out = append(out, rawDep)
			sourceIndices = append(sourceIndices, sourceIndex)
			continue
		}
		dep, err := gitparse.ParseGitDependency(line)
		if err != nil {
			return nil, nil, err
		}
		if !dep.Git.Release {
			out = append(out, rawDep)
			sourceIndices = append(sourceIndices, sourceIndex)
			continue
		}
		if err := githubrelease.ValidateReleaseHost(dep.Git.CloneURL); err != nil {
			return nil, nil, err
		}
		if strings.TrimSpace(dep.Git.DarPath) != "" {
			out = append(out, rawDep)
			sourceIndices = append(sourceIndices, sourceIndex)
			continue
		}
		_, _ = fmt.Fprintf(os.Stderr, "Resolving git release: listing .dar assets for %s release %q\n",
			dep.Git.CloneURL.String(), dep.Git.Ref)
		assets, err := githubrelease.ListDarAssets(ctx, dep.Git.CloneURL, dep.Git.Ref)
		if err != nil {
			return nil, nil, fmt.Errorf("git release %q: %w", dep.Git.Ref, err)
		}
		for _, asset := range assets {
			assetDep := *dep
			assetDep.Git.DarPath = asset
			key, err := gitparse.GitLockKeyForDep(assetDep.Git)
			if err != nil {
				return nil, nil, err
			}
			if _, exists := existingAssets[key]; exists {
				continue
			}
			existingAssets[key] = struct{}{}

			expandedLine := gitparse.FormatGitYamlLine(assetDep.Git)
			expanded, err := rawDependencyWithValue(rawDep, expandedLine)
			if err != nil {
				return nil, nil, err
			}
			out = append(out, expanded)
			sourceIndices = append(sourceIndices, sourceIndex)
		}
	}
	return out, sourceIndices, nil
}

func resolveGitAliasInputs(yamlPath, fieldName string, rawDeps []*RawDependency, rewrite func(*ParsedDarDependency) (string, bool)) ([]*RawDependency, error) {
	hasAlias := false
	for _, rawDep := range rawDeps {
		line, err := rawDep.Value()
		if err != nil {
			return nil, err
		}
		if _, ok := ArtifactLocationAlias(line); ok {
			hasAlias = true
			break
		}
	}
	if !hasAlias {
		return rawDeps, nil
	}

	pkg, err := Read(yamlPath)
	if err != nil {
		return nil, err
	}
	_, parsedDeps := pkg.RawAndParsed(fieldName)

	out := append([]*RawDependency(nil), rawDeps...)
	for i, rawDep := range rawDeps {
		line, err := rawDep.Value()
		if err != nil {
			return nil, err
		}
		if _, ok := ArtifactLocationAlias(line); !ok {
			continue
		}
		value, ok := rewrite(parsedDeps[line])
		if !ok {
			continue
		}
		out[i], err = rawDependencyWithValue(rawDep, value)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func rewriteYamlDependencyField(yamlPath, fieldName string, original, rewritten []*RawDependency, sourceIndices []int) (bool, error) {
	if rawDependenciesEqual(original, rewritten) {
		return false, nil
	}
	if sourceIndices == nil {
		sourceIndices = make([]int, len(rewritten))
		for i := range sourceIndices {
			sourceIndices[i] = i
		}
	}

	b, err := os.ReadFile(yamlPath)
	if err != nil {
		return false, err
	}

	out, err := yamledit.ReplaceListAnyPreservingComments(b, fieldName, rewritten, sourceIndices)
	if err != nil {
		return false, err
	}

	if err := os.WriteFile(yamlPath, []byte(out), 0644); err != nil {
		return false, err
	}

	return true, nil
}

// CanonicalizeRawGitDependencies rewrites git entries to canonical one-liners.
// {value, main-package-id} wrappers are kept.
func CanonicalizeRawGitDependencies(rawDeps []*RawDependency) ([]*RawDependency, bool, error) {
	out := make([]*RawDependency, 0, len(rawDeps))
	changed := false
	for _, rawDep := range rawDeps {
		canonical, itemChanged, err := canonicalizeRawGitDependency(rawDep)
		if err != nil {
			return nil, false, err
		}
		if itemChanged {
			changed = true
		}
		out = append(out, canonical)
	}
	return out, changed, nil
}

func canonicalizeRawGitDependency(raw *RawDependency) (*RawDependency, bool, error) {
	switch {
	case raw.WithPackageId != nil:
		canonical, itemChanged, err := canonicalizeGitLineValue(raw.WithPackageId.Value)
		if err != nil || !itemChanged {
			return raw, itemChanged, err
		}
		return &RawDependency{WithPackageId: &withPackageId{
			Value:         canonical,
			MainPackageId: raw.WithPackageId.MainPackageId,
		}}, true, nil
	case raw.ValueOnly != nil:
		canonical, itemChanged, err := canonicalizeGitLineValue(*raw.ValueOnly)
		if err != nil || !itemChanged {
			return raw, itemChanged, err
		}
		return rawDependencyFromValue(canonical), true, nil
	default:
		return raw, false, nil
	}
}

func canonicalizeGitLineValue(line string) (string, bool, error) {
	if !gitparse.IsGitDependencyLine(line) {
		return line, false, nil
	}
	canonical, err := gitparse.CoerceGitDependencyInput(line, gitparse.GitInputOptions{RequireGitPrefix: true})
	if err != nil {
		return "", false, err
	}
	return canonical, canonical != line, nil
}

func rawDependencyFromValue(value string) *RawDependency {
	v := value
	return &RawDependency{ValueOnly: &v}
}

func rawDependencyWithValue(raw *RawDependency, value string) (*RawDependency, error) {
	if raw != nil && raw.WithPackageId != nil {
		return &RawDependency{WithPackageId: &withPackageId{
			Value:         value,
			MainPackageId: raw.WithPackageId.MainPackageId,
		}}, nil
	}
	return rawDependencyFromValue(value), nil
}

// MarshalDependencyWithValue writes value, keeping {value, main-package-id} wrappers.
func MarshalDependencyWithValue(raw *RawDependency, value string) (string, error) {
	updated, err := rawDependencyWithValue(raw, value)
	if err != nil {
		return "", err
	}
	out, err := yaml.Marshal(updated)
	return string(out), err
}

func rawDependenciesEqual(a, b []*RawDependency) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ya, err := yaml.Marshal(a[i])
		if err != nil {
			return false
		}
		yb, err := yaml.Marshal(b[i])
		if err != nil {
			return false
		}
		if !bytes.Equal(ya, yb) {
			return false
		}
	}
	return true
}
