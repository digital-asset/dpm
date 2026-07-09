// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package navindex

import (
	"os"
	"path/filepath"
	"testing"
)

const fixtureJSON = `{
  "$schema": "https://mintlify.com/docs.json",
  "name": "Test",
  "navigation": {
    "dropdowns": [
      {
        "dropdown": "App Development",
        "versions": [
          {
            "version": "MainNet",
            "groups": [
              {
                "group": "Get Started",
                "pages": [
                  "appdev/get-started/intro",
                  "appdev/get-started/quickstart"
                ]
              },
              {
                "group": "Tutorials",
                "pages": [
                  "appdev/tutorials/canton-getting-started",
                  "appdev/tutorials/daml-getting-started"
                ]
              }
            ]
          },
          {
            "version": "TestNet",
            "groups": [
              {
                "group": "Get Started",
                "pages": [
                  "appdev/get-started/intro"
                ]
              }
            ]
          }
        ]
      },
      {
        "dropdown": "Overview",
        "versions": [
          {
            "version": "MainNet",
            "groups": [
              {
                "group": "Architecture",
                "pages": [
                  "overview/learn/architecture"
                ]
              }
            ]
          }
        ]
      }
    ]
  },
  "favicon": "/favicon.ico",
  "logo": {"light": "/logo.svg"}
}`

func writeFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "docs.json")
	if err := os.WriteFile(p, []byte(fixtureJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestBuild_FlatPageSet(t *testing.T) {
	idx, err := Build(writeFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"appdev/get-started/intro",
		"appdev/get-started/quickstart",
		"appdev/tutorials/canton-getting-started",
		"appdev/tutorials/daml-getting-started",
		"overview/learn/architecture",
	}
	if idx.Size() != len(want) {
		t.Errorf("want %d pages, got %d (%v)", len(want), idx.Size(), idx.Pages())
	}
	for _, w := range want {
		if !idx.HasPage(w) {
			t.Errorf("missing %q", w)
		}
	}
}

func TestBuild_SkipsAssetsAndExternalURLs(t *testing.T) {
	idx, err := Build(writeFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range idx.Pages() {
		if filepath.Ext(p) != "" {
			t.Errorf("asset path slipped through: %q", p)
		}
	}
}

func TestFindByBasename(t *testing.T) {
	idx, _ := Build(writeFixture(t))
	hits := idx.FindByBasename("intro")
	if len(hits) != 1 || hits[0] != "appdev/get-started/intro" {
		t.Errorf("FindByBasename(intro) = %v", hits)
	}
}

func TestBestMatch_ExactWins(t *testing.T) {
	idx, _ := Build(writeFixture(t))
	if got := idx.BestMatch("appdev/get-started/intro"); got != "appdev/get-started/intro" {
		t.Errorf("exact match should win, got %q", got)
	}
}

func TestBestMatch_PrefersOverlappingPath(t *testing.T) {
	idx, _ := Build(writeFixture(t))
	// "Canton Getting Started" RST living under canton/3.5/participant/tutorials/
	// should resolve to appdev/tutorials/canton-getting-started, not the
	// daml one — heuristic: shared `tutorials` segment.
	if got := idx.BestMatch("canton/3.5/participant/tutorials/canton-getting-started"); got != "appdev/tutorials/canton-getting-started" {
		t.Errorf("expected canton-getting-started match, got %q", got)
	}
}

func TestBestMatch_StemNormalization(t *testing.T) {
	idx, _ := Build(writeFixture(t))
	// RST file with underscores should still match the kebab-case page.
	if got := idx.BestMatch("foo/canton_getting_started.rst"); got != "appdev/tutorials/canton-getting-started" {
		t.Errorf("expected stem normalization to find canton-getting-started, got %q", got)
	}
}

func TestBuild_MissingFile(t *testing.T) {
	if _, err := Build("/no/such/path.json"); err == nil {
		t.Error("expected error for missing file")
	}
}
