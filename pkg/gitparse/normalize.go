package gitparse

import (
	"fmt"
	"net/url"
	"strings"

	"daml.com/x/assistant/pkg/githubrelease"
)

// GitInputOptions controls which git dependency input shapes are accepted.
type GitInputOptions struct {
	// RequireGitPrefix requires the git: prefix (daml.yaml one-liners).
	RequireGitPrefix bool
}

// CoerceGitDependencyInput normalizes accepted git dependency inputs to the
// canonical daml.yaml one-liner: git:host/owner/repo#ref?path=...
func CoerceGitDependencyInput(raw string, opts GitInputOptions) (string, error) {
	line, err := acceptGitDependencyInput(raw, opts)
	if err != nil {
		return "", err
	}
	dep, err := ParseGitDependency(line)
	if err != nil {
		return "", err
	}
	if dep.Git.Release {
		if err := githubrelease.ValidateReleaseHost(dep.Git.CloneURL); err != nil {
			return "", err
		}
	}
	return FormatGitYamlLine(dep.Git), nil
}

func acceptGitDependencyInput(raw string, opts GitInputOptions) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty git dependency")
	}
	if !hasGitDependencyShape(raw, opts) {
		if opts.RequireGitPrefix {
			return "", fmt.Errorf("dependency %q is not a git dependency", raw)
		}
		return "", fmt.Errorf("dependency %q is not a recognized git dependency URL", raw)
	}

	remainder := strings.TrimPrefix(raw, "git:")
	if blob, ok, err := tryParseWebBlobURL(remainder); err != nil {
		return "", err
	} else if ok {
		return fmt.Sprintf("git:%s/%s#%s?path=%s",
			blob.host, blob.repoPath, blob.ref, escapeGitDarPathQuery(blob.filePath)), nil
	}
	return "git:" + remainder, nil
}

func hasGitDependencyShape(raw string, opts GitInputOptions) bool {
	if strings.HasPrefix(raw, "git:") {
		return true
	}
	if opts.RequireGitPrefix {
		return false
	}
	if u, err := url.Parse(raw); err == nil && u.Scheme == "https" && u.Host != "" {
		return isWellKnownGitHost(u.Host) || hasGitDependencySyntax(raw)
	}
	return hasHostFirstShape(raw) && hasGitDependencySyntax(raw)
}

func hasGitDependencySyntax(raw string) bool {
	base := raw
	if hashIdx := strings.Index(raw, "#"); hashIdx >= 0 {
		if strings.Contains(raw[hashIdx:], pathQueryPrefix) {
			return true
		}
		base = raw[:hashIdx]
	}
	if u, err := url.Parse(base); err == nil && u.Query().Get("release") != "" {
		return true
	}
	_, ok, _ := tryParseWebBlobURL(raw)
	return ok
}

// NormalizeDarDependencyURI normalizes uri when it is a git dependency input.
func NormalizeDarDependencyURI(uri string) (string, bool, error) {
	normalized, err := CoerceGitDependencyInput(uri, GitInputOptions{RequireGitPrefix: false})
	if err != nil {
		if IsGitDependencyLine(uri) {
			return "", false, err
		}
		return uri, false, nil
	}
	return normalized, true, nil
}

type webBlobRef struct {
	host     string
	repoPath string
	ref      string
	filePath string
}

func tryParseWebBlobURL(raw string) (*webBlobRef, bool, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, false, err
	}
	if u.Scheme == "https" && u.Host != "" {
		return blobRefFromPath(u.Host, strings.Trim(u.Path, "/"), raw)
	}
	if u.Scheme == "" && u.Host == "" {
		host, rest := splitHostFirstBase(strings.Trim(u.Path, "/"))
		if !looksLikeHost(host) {
			return nil, false, nil
		}
		return blobRefFromPath(host, rest, raw)
	}
	return nil, false, nil
}

func blobRefFromPath(host, trimmedPath, raw string) (*webBlobRef, bool, error) {
	repoPath, rest, ok := splitAtBlobMarker(strings.Split(trimmedPath, "/"))
	if !ok {
		return nil, false, nil
	}
	ref, filePath, ok := splitContentRef(rest)
	if !ok {
		return nil, false, nil
	}
	if err := validateGitRepoDarPath(filePath); err != nil {
		return nil, true, fmt.Errorf("git blob url %q: %w", raw, err)
	}
	return &webBlobRef{
		host:     host,
		repoPath: repoPath,
		ref:      ref,
		filePath: filePath,
	}, true, nil
}

func splitAtBlobMarker(parts []string) (repoPath string, rest []string, ok bool) {
	for i, part := range parts {
		if part != "blob" && part != "raw" {
			continue
		}
		repoParts := parts[:i]
		if i > 0 && parts[i-1] == "-" {
			repoParts = parts[:i-1]
		}
		if len(repoParts) < 2 || len(parts) <= i+1 {
			continue
		}
		return strings.Join(repoParts, "/"), parts[i+1:], true
	}
	return "", nil, false
}

func splitContentRef(parts []string) (ref, filePath string, ok bool) {
	if len(parts) < 2 {
		return "", "", false
	}
	if len(parts) >= 4 && parts[0] == "refs" && (parts[1] == "heads" || parts[1] == "tags") {
		return parts[2], strings.Join(parts[3:], "/"), true
	}
	return parts[0], strings.Join(parts[1:], "/"), true
}

func basePartBeforeFragment(raw string) string {
	basePart, _, _ := strings.Cut(raw, "#")
	basePart, _, _ = strings.Cut(basePart, "?")
	return basePart
}

var wellKnownGitHosts = map[string]bool{
	"github.com":    true,
	"gitlab.com":    true,
	"bitbucket.org": true,
	"codeberg.org":  true,
}

func isWellKnownGitHost(host string) bool {
	return wellKnownGitHosts[host]
}

func looksLikeHost(segment string) bool {
	if segment == "" || strings.ContainsAny(segment, " \t/@") {
		return false
	}
	name := segment
	if colon := strings.LastIndex(segment, ":"); colon >= 0 {
		name = segment[:colon]
	}
	return name == "localhost" || strings.Contains(name, ".")
}

func hasHostFirstShape(remainder string) bool {
	basePart := basePartBeforeFragment(remainder)
	if strings.Contains(basePart, "://") {
		return false
	}
	host, repoPath := splitHostFirstBase(basePart)
	return host != "" && repoPath != ""
}

func splitHostFirstBase(basePart string) (host, repoPath string) {
	host, repoPath, ok := strings.Cut(basePart, "/")
	if !ok {
		return "", ""
	}
	repoPath = strings.TrimSuffix(repoPath, ".git")
	return host, repoPath
}
