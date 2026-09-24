package gitparse

import (
	"fmt"
	"net/url"
)

// GitSource is the parsed git identity of a daml.yaml dependency.
type GitSource struct {
	Ref      string
	DarPath  string
	CloneURL *url.URL
	Release  bool
}

// Dependency is a parsed git: line (canonical URL plus source fields).
type Dependency struct {
	FullUrl *url.URL
	Git     GitSource
}

// WithGitRef returns a copy pinned to ref, updating the canonical git:// URL.
func (d *Dependency) WithGitRef(ref string) *Dependency {
	if d == nil {
		return nil
	}
	copy := *d
	copy.FullUrl, copy.Git = WithGitRef(d.FullUrl, d.Git, ref)
	return &copy
}

// WithGitRef returns the canonical URL and source with Ref set to ref.
func WithGitRef(fullURL *url.URL, git GitSource, ref string) (*url.URL, GitSource) {
	git.Ref = ref
	if fullURL != nil && fullURL.Scheme == "git" && !git.Release {
		repoPath := gitRepoPathFromURI(fullURL)
		pinned, err := url.Parse(fmt.Sprintf("git://%s/%s@%s?path=%s",
			fullURL.Host,
			repoPath,
			url.PathEscape(ref),
			url.QueryEscape(git.DarPath),
		))
		if err == nil {
			return pinned, git
		}
	}
	return fullURL, git
}
