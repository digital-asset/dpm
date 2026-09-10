package githubrelease

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"daml.com/x/assistant/pkg/utils"
)

const defaultAPIBase = "https://api.github.com"

var httpClient = &http.Client{Timeout: 5 * time.Minute}

func apiBase() string {
	if base := os.Getenv("DPM_TEST_GITHUB_API_BASE"); base != "" {
		return strings.TrimSuffix(base, "/")
	}
	return defaultAPIBase
}

func githubHostAllowed(host string) bool {
	if host == "github.com" {
		return true
	}
	testHost := os.Getenv("DPM_TEST_GITHUB_RELEASE_HOST")
	return testHost != "" && host == testHost
}

// ValidateReleaseHost reports whether cloneURL's host can serve release assets.
func ValidateReleaseHost(cloneURL *url.URL) error {
	_, _, err := ParseGitHubRepo(cloneURL)
	return err
}

// unsupportedReleaseHostError returns an error for hosts without GitHub releases API support.
func unsupportedReleaseHostError(host string) error {
	if host == "" {
		host = "this host"
	}
	return fmt.Errorf(
		"%s does not expose the GitHub releases API, so ?release= dependencies are only supported for github.com; "+
			"to depend on a .dar committed in a %s repository, use the git:%s/<owner>/<repo>#<ref>?path=<file>.dar form instead",
		host, host, host,
	)
}

type releaseResponse struct {
	Assets []struct {
		Name string `json:"name"`
	} `json:"assets"`
}

// ParseGitHubRepo extracts owner and repo from a github.com/owner/repo.git clone URL.
func ParseGitHubRepo(cloneURL *url.URL) (owner, repo string, err error) {
	if cloneURL == nil {
		return "", "", fmt.Errorf("missing clone URL")
	}
	cloneURL = normaliseSchemelessGitHubURL(cloneURL)
	if !githubHostAllowed(cloneURL.Host) {
		return "", "", unsupportedReleaseHostError(cloneURL.Host)
	}
	if cloneURL.Scheme != "" && cloneURL.Scheme != "https" && !(cloneURL.Scheme == "http" && os.Getenv("DPM_TEST_GITHUB_RELEASE_HOST") != "") {
		return "", "", fmt.Errorf("git release dependencies require an https://github.com/... clone URL")
	}
	parts := strings.Split(strings.Trim(path.Clean("/"+strings.TrimPrefix(cloneURL.Path, "/")), "/"), "/")
	if len(parts) != 2 || parts[0] == "" {
		return "", "", fmt.Errorf("couldn't parse github owner/repo from %q", cloneURL.String())
	}
	repoName := strings.TrimSuffix(parts[1], ".git")
	if repoName == "" {
		return "", "", fmt.Errorf("couldn't parse github owner/repo from %q", cloneURL.String())
	}
	return parts[0], repoName, nil
}

// normaliseSchemelessGitHubURL fills Host/Path for github.com/owner/repo-style URLs.
func normaliseSchemelessGitHubURL(cloneURL *url.URL) *url.URL {
	if cloneURL.Scheme != "" || cloneURL.Host != "" {
		return cloneURL
	}
	parts := strings.SplitN(strings.TrimPrefix(cloneURL.Path, "/"), "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		return cloneURL
	}
	normalised := *cloneURL
	normalised.Host = parts[0]
	if len(parts) == 2 {
		normalised.Path = "/" + parts[1]
	} else {
		normalised.Path = ""
	}
	return &normalised
}

// ListDarAssets returns .dar asset filenames for a GitHub release tag.
func ListDarAssets(ctx context.Context, cloneURL *url.URL, tag string) ([]string, error) {
	owner, repo, err := ParseGitHubRepo(cloneURL)
	if err != nil {
		return nil, err
	}

	apiURL := fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", apiBase(), owner, repo, url.PathEscape(tag))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release %q for %s/%s: %w", tag, owner, repo, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("release %q not found for %s/%s", tag, owner, repo)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf(
			"failed to fetch release %q for %s/%s: HTTP %d: %s",
			tag, owner, repo, resp.StatusCode, strings.TrimSpace(string(body)),
		)
	}

	var rel releaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("failed to decode release metadata for %s/%s: %w", owner, repo, err)
	}

	var dars []string
	for _, a := range rel.Assets {
		if strings.HasSuffix(strings.ToLower(a.Name), ".dar") {
			dars = append(dars, a.Name)
		}
	}
	if len(dars) == 0 {
		return nil, fmt.Errorf("release %q for %s/%s has no .dar assets", tag, owner, repo)
	}
	return dars, nil
}

// DownloadAsset downloads a release asset into destDir and returns the local file path.
func DownloadAsset(ctx context.Context, cloneURL *url.URL, tag, asset, destDir string) (string, error) {
	owner, repo, err := ParseGitHubRepo(cloneURL)
	if err != nil {
		return "", err
	}
	if !strings.HasSuffix(strings.ToLower(asset), ".dar") {
		return "", fmt.Errorf("asset %q must end with .dar", asset)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	destPath := filepath.Join(destDir, filepath.Base(asset))

	downloadURL := releaseDownloadURL(cloneURL, tag, asset)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download %s/%s release asset %q: %w", owner, repo, asset, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf(
			"asset %q not found in %s/%s release %q: check that the release tag and the asset name both exist",
			asset, owner, repo, tag,
		)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf(
			"failed to download %s/%s release asset %q: HTTP %d: %s",
			owner, repo, asset, resp.StatusCode, strings.TrimSpace(string(body)),
		)
	}

	if err := utils.AtomicWriteFile(destPath, resp.Body); err != nil {
		return "", err
	}
	return destPath, nil
}

func releaseDownloadURL(cloneURL *url.URL, tag, asset string) string {
	cloneURL = normaliseSchemelessGitHubURL(cloneURL)
	base := fmt.Sprintf("%s%s", cloneURL.Host, strings.TrimSuffix(cloneURL.Path, ".git"))
	if cloneURL.Scheme != "" {
		base = cloneURL.Scheme + "://" + base
	}
	return strings.TrimSuffix(base, "/") + "/releases/download/" + url.PathEscape(tag) + "/" + url.PathEscape(asset)
}
