package gitparse

import (
	"fmt"
	"net/url"
	"strings"
)

// GitStructuredFields is the structured git dependency form in daml.yaml.
// damlc accepts this mapping on SDK 3.6 and later, not on earlier SDKs.
//
//	git:
//	  url: github.com/org/repo
//	  ref: main
//	  path: pkg/foo.dar
//
// Release assets use release and asset instead of ref and path.
type GitStructuredFields struct {
	URL     string `yaml:"url,omitempty"`
	Ref     string `yaml:"ref,omitempty"`
	Path    string `yaml:"path,omitempty"`
	Release string `yaml:"release,omitempty"`
	Asset   string `yaml:"asset,omitempty"`
}

// ParseGitStructured validates structured YAML fields and builds a dependency from them.
// Each field is checked on its own. The fields are not joined into a git: line.
func ParseGitStructured(fields *GitStructuredFields) (*Dependency, error) {
	trimmed, err := trimmedGitStructuredFields(fields)
	if err != nil {
		return nil, err
	}
	cloneURL, host, repoPath, err := parseStructuredCloneURL(trimmed.URL)
	if err != nil {
		return nil, err
	}

	if trimmed.Release != "" {
		if err := validateGitRefName(trimmed.Release); err != nil {
			return nil, fmt.Errorf("git dependency: release %w", err)
		}
		if trimmed.Asset != "" {
			if err := validateGitReleaseAsset(trimmed.Asset); err != nil {
				return nil, fmt.Errorf("git dependency: %w", err)
			}
		}
		return newGitDependency(GitSource{
			Ref:      trimmed.Release,
			DarPath:  trimmed.Asset,
			CloneURL: cloneURL,
			Release:  true,
		}, host, repoPath)
	}

	if trimmed.Ref == "" {
		return nil, fmt.Errorf("git dependency: ref or release is required")
	}
	if trimmed.Path == "" {
		return nil, fmt.Errorf("git dependency: path is required for repo-file dependencies")
	}
	if err := validateGitRefName(trimmed.Ref); err != nil {
		return nil, fmt.Errorf("git dependency: ref %w", err)
	}
	if err := validateGitRepoDarPath(trimmed.Path); err != nil {
		return nil, fmt.Errorf("git dependency: %w", err)
	}
	return newGitDependency(GitSource{
		Ref:      trimmed.Ref,
		DarPath:  trimmed.Path,
		CloneURL: cloneURL,
	}, host, repoPath)
}

func parseStructuredCloneURL(raw string) (*url.URL, string, string, error) {
	raw = strings.TrimPrefix(strings.TrimSpace(raw), "git:")
	if strings.ContainsAny(raw, "#?") {
		return nil, "", "", fmt.Errorf("git dependency: url must not contain '#' or '?'")
	}
	u, err := parseGitCloneBase(raw)
	if err != nil {
		return nil, "", "", fmt.Errorf("couldn't parse git dependency url %q: %w", raw, err)
	}
	if u.User != nil {
		return nil, "", "", fmt.Errorf("git dependency: url must not contain credentials")
	}
	return cloneURLFromHTTPS(u, raw)
}

// StructuredFieldsFromGit builds the structured YAML fields for a parsed git source.
func StructuredFieldsFromGit(git GitSource) *GitStructuredFields {
	fields := &GitStructuredFields{
		URL: gitDependencyCloneURLString(git.CloneURL),
	}
	if git.Release {
		fields.Release = git.Ref
		fields.Asset = git.DarPath
		return fields
	}
	fields.Ref = git.Ref
	fields.Path = git.DarPath
	return fields
}

// StructuredFieldsFromLine parses a git: line into structured YAML fields.
func StructuredFieldsFromLine(line string) (*GitStructuredFields, error) {
	dep, err := ParseGitDependency(line)
	if err != nil {
		return nil, err
	}
	return StructuredFieldsFromGit(dep.Git), nil
}
