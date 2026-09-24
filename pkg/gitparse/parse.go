package gitparse

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

const (
	errGitPathRequired = "git dependency %q: ?path= is required (e.g. git:github.com/org/repo.git#main?path=loyalty.dar)"
	pathQueryPrefix    = "?path="
)

// ParseGitDependency parses a daml.yaml git dependency (repo file or GitHub release asset).
func ParseGitDependency(raw string) (*Dependency, error) {
	remainder := strings.TrimPrefix(raw, "git:")
	if remainder == raw {
		return nil, fmt.Errorf("dependency %q is not a git dependency", raw)
	}
	if err := validateGitInlineDependency(remainder); err != nil {
		return nil, err
	}

	basePart := remainder
	if hashIdx := strings.Index(remainder, "#"); hashIdx >= 0 {
		basePart = remainder[:hashIdx]
	}
	u, err := url.Parse(basePart)
	if err == nil && u.Query().Get("release") != "" {
		return parseGitReleaseDependency(remainder, raw)
	}
	return parseGitRepoDependency(remainder, raw)
}

func parseGitReleaseDependency(remainder, raw string) (*Dependency, error) {
	u, err := url.Parse(remainder)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse git release dependency url %q: %w", raw, err)
	}

	releaseTag := u.Query().Get("release")
	if releaseTag == "" {
		return nil, fmt.Errorf("git release dependency %q: release query parameter is required", raw)
	}
	asset := u.Query().Get("asset")
	if asset != "" {
		if err := validateGitReleaseAsset(asset); err != nil {
			return nil, fmt.Errorf("git release dependency %q: %w", raw, err)
		}
	}

	cloneURL, host, repoPath, err := cloneURLFromHTTPS(u, raw)
	if err != nil {
		return nil, err
	}

	canonicalStr := fmt.Sprintf("git://%s/%s@%s", host, repoPath, url.PathEscape(releaseTag))
	if asset != "" {
		canonicalStr += "?asset=" + url.QueryEscape(asset)
	}
	canonical, err := url.Parse(canonicalStr)
	if err != nil {
		return nil, fmt.Errorf("couldn't build canonical git release url for dependency %q: %w", raw, err)
	}

	return &Dependency{
		FullUrl: canonical,
		Git: GitSource{
			Ref:      releaseTag,
			DarPath:  asset,
			CloneURL: cloneURL,
			Release:  true,
		},
	}, nil
}

func parseGitRepoDependency(remainder, raw string) (*Dependency, error) {
	basePart, afterHash, ok := strings.Cut(remainder, "#")
	if !ok {
		return nil, fmt.Errorf("git dependency %q: ref is required after # (e.g. #main, #v1.0.0, or a commit SHA)", raw)
	}

	pathIdx := strings.Index(afterHash, pathQueryPrefix)
	if pathIdx < 0 {
		return nil, fmt.Errorf(errGitPathRequired, raw)
	}

	gitRef := afterHash[:pathIdx]
	if gitRef == "" {
		return nil, fmt.Errorf("git dependency %q: ref is required after # (e.g. #main, #v1.0.0, or a commit SHA)", raw)
	}

	darPath, err := url.QueryUnescape(afterHash[pathIdx+len(pathQueryPrefix):])
	if err != nil {
		return nil, fmt.Errorf("git dependency %q: invalid ?path= value: %w", raw, err)
	}
	if darPath == "" {
		return nil, fmt.Errorf(errGitPathRequired, raw)
	}
	if err := validateGitRepoDarPath(darPath); err != nil {
		return nil, fmt.Errorf("git dependency %q: %w", raw, err)
	}

	u, err := url.Parse(basePart)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse git dependency url %q: %w", raw, err)
	}

	cloneURL, host, repoPath, err := cloneURLFromHTTPS(u, raw)
	if err != nil {
		return nil, err
	}

	canonical, err := url.Parse(fmt.Sprintf("git://%s/%s@%s?path=%s",
		host,
		repoPath,
		url.PathEscape(gitRef),
		url.QueryEscape(darPath),
	))
	if err != nil {
		return nil, fmt.Errorf("couldn't build canonical git url for dependency %q: %w", raw, err)
	}

	return &Dependency{
		FullUrl: canonical,
		Git: GitSource{
			Ref:      gitRef,
			DarPath:  darPath,
			CloneURL: cloneURL,
		},
	}, nil
}

func cloneURLFromHTTPS(u *url.URL, raw string) (*url.URL, string, string, error) {
	u, err := normaliseGitCloneURL(u, raw)
	if err != nil {
		return nil, "", "", err
	}
	cloneURL := &url.URL{
		Scheme: u.Scheme,
		Host:   u.Host,
		Path:   u.Path,
	}
	if u.Scheme == "file" && cloneURL.Path == "" && u.Host != "" {
		cloneURL.Path = "/" + u.Host
		cloneURL.Host = ""
	}

	repoPath := strings.TrimPrefix(u.Path, "/")
	repoPath = strings.TrimSuffix(repoPath, ".git")
	if u.Scheme == "file" {
		repoPath = strings.TrimPrefix(u.Path, "/")
	}

	host := u.Host
	if u.Scheme == "file" && host == "" {
		host = "local"
	}
	if host == "" || repoPath == "" {
		return nil, "", "", fmt.Errorf("git dependency %q: clone URL must include a host and repository path", raw)
	}

	return cloneURL, host, repoPath, nil
}

func normaliseGitCloneURL(u *url.URL, raw string) (*url.URL, error) {
	if u.Scheme == "" && u.Host == "" {
		normalised, err := url.Parse("https://" + u.String())
		if err != nil {
			return nil, fmt.Errorf("couldn't parse git dependency url %q: %w", raw, err)
		}
		u = normalised
	}
	if u.Scheme != "https" {
		if u.Scheme == "http" && os.Getenv("DPM_TEST_GITHUB_RELEASE_HOST") != "" && u.Host == os.Getenv("DPM_TEST_GITHUB_RELEASE_HOST") {
			return u, nil
		}
		if u.Scheme != "file" || os.Getenv("DPM_TEST_ALLOW_FILE_GIT") != "true" {
			return nil, fmt.Errorf("git dependency %q: only https:// clone URLs are supported", raw)
		}
	}
	return u, nil
}

func gitRepoPathFromURI(u *url.URL) string {
	p := strings.TrimPrefix(u.Path, "/")
	if at := strings.LastIndex(p, "@"); at >= 0 {
		p = p[:at]
	}
	return p
}

// GitRefFromURI returns the ref segment from a canonical or pinned git:// URI path.
func GitRefFromURI(u *url.URL) string {
	p := strings.TrimPrefix(u.Path, "/")
	at := strings.LastIndex(p, "@")
	if at < 0 {
		return ""
	}
	ref, err := url.PathUnescape(p[at+1:])
	if err != nil {
		return p[at+1:]
	}
	return ref
}

// PinnedGitURI returns a lockfile URI with the resolved ref (commit SHA for repo deps, release tag for release deps).
func PinnedGitURI(fullURL *url.URL, git GitSource, resolvedRef string) (*url.URL, error) {
	if fullURL == nil || fullURL.Scheme != "git" {
		return nil, fmt.Errorf("not a git dependency")
	}
	if git.Release {
		return url.Parse(fullURL.String())
	}
	repoPath := gitRepoPathFromURI(fullURL)
	return url.Parse(fmt.Sprintf("git://%s/%s@%s?path=%s",
		fullURL.Host,
		repoPath,
		url.PathEscape(resolvedRef),
		url.QueryEscape(git.DarPath),
	))
}

// GitRefIsMutable reports whether a ref may move (branch or tag name vs commit SHA).
func GitRefIsMutable(ref string) bool {
	return len(ref) != 40 || !isHex(ref)
}

// GitPinMismatchError reports a pinned ref that no longer matches the resolved commit.
func GitPinMismatchError(git GitSource, resolved string) error {
	return fmt.Errorf(
		"git dependency %q is pinned to commit %q but resolved %q; run 'dpm update'",
		FormatGitYamlLine(git),
		git.Ref,
		resolved,
	)
}

// GitMissingPinError reports a mutable git ref that must be pinned.
func GitMissingPinError(git GitSource) error {
	return fmt.Errorf(
		"git dependency %q has no commit pin; run 'dpm install package' or 'dpm update'",
		FormatGitYamlLine(git),
	)
}

func isHex(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// GitLockKey returns a stable map key for lockfile diffing (repo identity without ref).
func GitLockKey(u *url.URL) string {
	if u == nil {
		return ""
	}
	if asset := u.Query().Get("asset"); asset != "" {
		return fmt.Sprintf("git://%s/%s?release=%s&asset=%s",
			u.Host, gitRepoPathFromURI(u), GitRefFromURI(u), asset)
	}
	return fmt.Sprintf("git://%s/%s?path=%s", u.Host, gitRepoPathFromURI(u), u.Query().Get("path"))
}

func cloneURLIdentity(cloneURL *url.URL) string {
	if cloneURL == nil {
		return "unknown/unknown"
	}
	host := cloneURL.Host
	if cloneURL.Scheme == "file" {
		host = "local"
	}
	repoPath := strings.TrimPrefix(cloneURL.Path, "/")
	if cloneURL.Scheme == "file" && cloneURL.Host != "" {
		repoPath = cloneURL.Host + "/" + repoPath
	}
	repoPath = strings.TrimSuffix(repoPath, ".git")
	if host == "" {
		host = "unknown"
	}
	if repoPath == "" {
		repoPath = "unknown"
	}
	return host + "/" + repoPath
}

// GitLockKeyForDep returns a stable key for matching git deps declared in daml.yaml.
func GitLockKeyForDep(git GitSource) (string, error) {
	if git.CloneURL == nil {
		return "", fmt.Errorf("git dependency missing clone URL")
	}
	identity := cloneURLIdentity(git.CloneURL)
	if git.Release {
		return fmt.Sprintf("git-release:%s:%s:%s", identity, git.Ref, git.DarPath), nil
	}
	return fmt.Sprintf("git:%s:%s", identity, git.DarPath), nil
}
