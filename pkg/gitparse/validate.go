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

func gitDependencyModesFromInline(remainder string) (gitDependencyModes, error) {
	modes := gitDependencyModes{}

	basePart, fragment, _ := strings.Cut(remainder, "#")

	if u, err := url.Parse(basePart); err == nil {
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
