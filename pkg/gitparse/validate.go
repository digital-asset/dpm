package gitparse

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"daml.com/x/assistant/pkg/utils"
)

type gitDependencyModes struct {
	release string
	ref     string
	path    string
	asset   string
}

func trimmedGitStructuredFields(fields *GitStructuredFields) (*GitStructuredFields, error) {
	if fields == nil {
		return nil, fmt.Errorf("missing git fields")
	}
	trimmed := &GitStructuredFields{
		URL:     strings.TrimSpace(fields.URL),
		Ref:     strings.TrimSpace(fields.Ref),
		Path:    strings.TrimSpace(fields.Path),
		Release: strings.TrimSpace(fields.Release),
		Asset:   strings.TrimSpace(fields.Asset),
	}
	if err := validateGitStructuredFields(trimmed); err != nil {
		return nil, err
	}
	if trimmed.URL == "" {
		return nil, fmt.Errorf("git dependency: url is required")
	}
	return trimmed, nil
}

func gitDependencyModesFromStructured(fields *GitStructuredFields) gitDependencyModes {
	if fields == nil {
		return gitDependencyModes{}
	}
	return gitDependencyModes{
		release: strings.TrimSpace(fields.Release),
		ref:     strings.TrimSpace(fields.Ref),
		path:    strings.TrimSpace(fields.Path),
		asset:   strings.TrimSpace(fields.Asset),
	}
}

func gitDependencyModesFromInline(remainder string) (gitDependencyModes, error) {
	modes := gitDependencyModes{}

	basePart, fragment, _ := strings.Cut(remainder, "#")

	if u, err := parseGitCloneBase(basePart); err == nil {
		modes.release = strings.TrimSpace(u.Query().Get("release"))
		modes.asset = strings.TrimSpace(u.Query().Get("asset"))
	}

	if fragment == "" {
		return modes, nil
	}

	pathIdx := strings.Index(fragment, pathQueryPrefix)
	if pathIdx < 0 {
		modes.ref = strings.TrimSpace(fragment)
		return modes, nil
	}

	modes.ref = strings.TrimSpace(fragment[:pathIdx])
	darPath, err := url.QueryUnescape(fragment[pathIdx+len(pathQueryPrefix):])
	if err != nil {
		return modes, fmt.Errorf("invalid ?path= value: %w", err)
	}
	modes.path = strings.TrimSpace(darPath)
	return modes, nil
}

func validateGitDependencyModes(modes gitDependencyModes) error {
	isRelease := modes.release != ""
	isRepo := modes.ref != "" || modes.path != ""

	if isRelease && isRepo {
		return fmt.Errorf("git dependency: release cannot be combined with ref or path")
	}
	if modes.asset != "" && !isRelease {
		return fmt.Errorf("git dependency: asset requires release")
	}
	if modes.path != "" && modes.ref == "" && !isRelease {
		return fmt.Errorf("git dependency: path requires ref")
	}
	return nil
}

// IsGitDependencyLine reports whether a dependency entry is a git: one-liner.
func IsGitDependencyLine(s string) bool {
	return strings.HasPrefix(s, "git:")
}

func validateRepoRelativeDarPath(path string) error {
	if filepath.IsAbs(path) {
		return fmt.Errorf("repo-relative path %q must not be absolute", path)
	}
	cleaned := filepath.ToSlash(filepath.Clean(path))
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
		return fmt.Errorf("repo-relative path %q must stay inside the repository", path)
	}
	if cleaned == "." {
		return fmt.Errorf("repo-relative path must name a .dar file")
	}
	return nil
}

func validateGitRepoDarPath(path string) error {
	if err := validateRepoRelativeDarPath(path); err != nil {
		return err
	}
	if !utils.IsDarPath(path) {
		return fmt.Errorf("repo-relative path %q must end with .dar", path)
	}
	return nil
}

func validateGitStructuredFields(fields *GitStructuredFields) error {
	if fields == nil {
		return fmt.Errorf("missing git fields")
	}
	return validateGitDependencyModes(gitDependencyModesFromStructured(fields))
}

// validateGitRefName applies git check-ref-format to a branch, tag, or release name.
// '#' is also rejected: a one-liner treats it as the start of the ref.
func validateGitRefName(ref string) error {
	for _, r := range ref {
		if r < 0x20 || r == 0x7f || strings.ContainsRune(" ~^:?*[\\#", r) {
			return fmt.Errorf("%q contains %q", ref, string(r))
		}
	}
	if ref == "@" ||
		strings.HasPrefix(ref, "/") || strings.HasSuffix(ref, "/") || strings.Contains(ref, "//") ||
		strings.Contains(ref, "..") || strings.HasSuffix(ref, ".") || strings.Contains(ref, "@{") {
		return fmt.Errorf("%q is not a valid git ref", ref)
	}
	for _, part := range strings.Split(ref, "/") {
		if strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
			return fmt.Errorf("%q is not a valid git ref", ref)
		}
	}
	return nil
}

func validateGitInlineDependency(remainder string) error {
	modes, err := gitDependencyModesFromInline(remainder)
	if err != nil {
		return fmt.Errorf("git dependency: %w", err)
	}
	return validateGitDependencyModes(modes)
}

func validateGitReleaseAsset(asset string) error {
	if !utils.IsDarPath(asset) {
		return fmt.Errorf("git asset %q must end with .dar", asset)
	}
	return validateReleaseAssetName(asset)
}

func validateReleaseAssetName(asset string) error {
	if strings.ContainsAny(asset, `/\`) {
		return fmt.Errorf("release asset %q must be a filename, not a path", asset)
	}
	if asset == "." || asset == ".." {
		return fmt.Errorf("release asset %q is invalid", asset)
	}
	return nil
}

// JoinRepoRelativeDarPath joins repoRoot and relPath after validateRepoRelativeDarPath.
func JoinRepoRelativeDarPath(repoRoot, relPath string) (string, error) {
	if err := validateRepoRelativeDarPath(relPath); err != nil {
		return "", err
	}
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", err
	}
	cleaned := filepath.FromSlash(filepath.ToSlash(filepath.Clean(relPath)))
	return filepath.Join(absRoot, cleaned), nil
}
