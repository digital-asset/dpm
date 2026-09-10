package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"strings"
	"testing"
)

type GitHubReleaseAsset struct {
	Name string
	Body []byte
}

func GitHubReleaseServer(t *testing.T, listenerHost bool, assets ...GitHubReleaseAsset) string {
	t.Helper()

	listed := make([]map[string]string, 0, len(assets))
	bodies := make(map[string][]byte, len(assets))
	for _, asset := range assets {
		listed = append(listed, map[string]string{"name": asset.Name})
		if asset.Body != nil {
			bodies[asset.Name] = asset.Body
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/releases/tags/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"assets": listed})
		case strings.Contains(r.URL.Path, "/releases/download/"):
			name := path.Base(r.URL.Path)
			if unescaped, err := url.PathUnescape(name); err == nil {
				name = unescaped
			}
			body, ok := bodies[name]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(body)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	host := "github.com"
	if listenerHost {
		host = srv.Listener.Addr().String()
	}
	t.Setenv("DPM_TEST_GITHUB_API_BASE", srv.URL)
	t.Setenv("DPM_TEST_GITHUB_RELEASE_HOST", host)
	return host
}
