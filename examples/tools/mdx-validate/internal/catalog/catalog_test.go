// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"sort"
	"strings"
	"testing"
)

func TestLookupKnownComponents(t *testing.T) {
	for _, name := range []string{
		"Accordion", "Card", "CardGroup", "CodeGroup", "Frame",
		"Icon", "Note", "Step", "Steps", "Tab", "Tabs", "Tip", "Warning",
	} {
		c, ok := Lookup(name)
		if !ok {
			t.Errorf("expected component %q to be in catalog", name)
			continue
		}
		if c.Name != name {
			t.Errorf("Lookup(%q).Name = %q, want %q", name, c.Name, name)
		}
		if c.DocsURL == "" {
			t.Errorf("component %q is missing DocsURL", name)
		}
	}
}

func TestLookupUnknownReturnsFalse(t *testing.T) {
	if _, ok := Lookup("DefinitelyNotAMintlifyComponent"); ok {
		t.Error("Lookup of unknown component returned ok=true")
	}
}

func TestRequiredPropsCoverKnownCases(t *testing.T) {
	cases := map[string][]string{
		"Card":      {"title"},
		"Tab":       {"title"},
		"Step":      {"title"},
		"Accordion": {"title"},
		"Icon":      {"icon"},
		"Tooltip":   {"tip"},
		"Update":    {"label", "description"},
	}
	for name, want := range cases {
		c, ok := Lookup(name)
		if !ok {
			t.Fatalf("missing component %q", name)
		}
		got := c.RequiredProps()
		sort.Strings(got)
		sort.Strings(want)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s required props: got %v, want %v", name, got, want)
		}
	}
}

func TestCalloutsHaveNoRequiredProps(t *testing.T) {
	for _, name := range []string{"Note", "Tip", "Warning", "Info", "Check"} {
		c, ok := Lookup(name)
		if !ok {
			t.Fatalf("missing callout %q", name)
		}
		if req := c.RequiredProps(); len(req) != 0 {
			t.Errorf("%s should have no required props, got %v", name, req)
		}
		if !c.AllowsChildren {
			t.Errorf("%s should allow children", name)
		}
	}
}

func TestPropByNameFindsDeclaredProp(t *testing.T) {
	card, _ := Lookup("Card")
	p, ok := card.PropByName("href")
	if !ok {
		t.Fatal("Card.href should exist in catalog")
	}
	if p.Required {
		t.Error("Card.href should not be required")
	}
	if _, ok := card.PropByName("nonsense"); ok {
		t.Error("Card.PropByName(nonsense) should return ok=false")
	}
}

func TestAllReturnsAtLeastBaseline(t *testing.T) {
	all := All()
	if len(all) < 20 {
		t.Errorf("expected at least 20 components in catalog, got %d", len(all))
	}
}
