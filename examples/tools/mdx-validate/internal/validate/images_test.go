// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile is a tiny helper to create a file (and parents) under dir.
func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestImageValidator(t *testing.T) {
	// Build an ephemeral docs tree:
	//   <root>/docs.json
	//   <root>/images/present.png
	//   <root>/sub/page.mdx
	//   <root>/sub/local.png
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "docs.json"))
	writeFile(t, filepath.Join(root, "images", "present.png"))
	writeFile(t, filepath.Join(root, "sub", "local.png"))
	page := filepath.Join(root, "sub", "page.mdx")

	codes := func(content string) []string {
		fs := ImageValidator{}.Validate(page, []byte(content), nil)
		out := make([]string, 0, len(fs))
		for _, f := range fs {
			out = append(out, f.Code)
		}
		return out
	}

	cases := []struct {
		name    string
		content string
		wantErr bool
	}{
		{"absolute present", "![a](/images/present.png)", false},
		{"absolute missing", "![a](/images/missing.png)", true},
		{"relative present", "![a](local.png)", false},
		{"relative missing", "![a](nope.png)", true},
		{"parent-relative resolves", "![a](../images/present.png)", false},
		{"img src present", `<img src="/images/present.png" />`, false},
		{"img src missing", `<img src="/images/missing.png" />`, true},
		{"single-quoted src missing", "<img src='/images/missing.png' />", true},
		{"external url skipped", "![a](https://example.com/x.png)", false},
		{"protocol-relative skipped", "![a](//cdn.example.com/x.png)", false},
		{"youtube iframe skipped", `<iframe src="https://www.youtube.com/embed/abc"></iframe>`, false},
		{"expression src skipped", `<img src={logo} />`, false},
		{"non-image src skipped", `<source src="/media/clip.mp4" />`, false},
		{"query/fragment stripped, present", "![a](/images/present.png?v=2#x)", false},
		{"case mismatch is reported", "![a](/images/Present.png)", true},
		{"title after url, present", `![a](/images/present.png "caption")`, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := codes(tc.content)
			hasErr := false
			for _, c := range got {
				if c == "image-not-found" {
					hasErr = true
				}
			}
			if hasErr != tc.wantErr {
				t.Errorf("content %q: got codes %v, wantErr=%v", tc.content, got, tc.wantErr)
			}
		})
	}
}
