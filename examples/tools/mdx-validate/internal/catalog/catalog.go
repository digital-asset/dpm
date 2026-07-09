// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

// Package components is the hand-curated catalog of Mintlify MDX components
// that this docs site uses, plus the prop specs that downstream tools need to
// emit or validate component usage.
//
// Source of truth: https://www.mintlify.com/docs/components/index
//
// Both rst-to-mdx (when emitting MDX from RST) and mdx-validate (when checking
// existing MDX) consult this catalog. Add entries when you encounter a new
// Mintlify component in docs-main/.
package catalog

import "sort"

// PropSpec describes a single component prop.
type PropSpec struct {
	Name       string
	Required   bool
	EnumValues []string // non-nil for enum-like props with a fixed value set
}

// Component is the spec for one Mintlify MDX component.
type Component struct {
	Name           string
	Description    string
	Props          []PropSpec
	AllowsChildren bool
	DocsURL        string
}

// RequiredProps returns the names of props that must be present.
func (c Component) RequiredProps() []string {
	var out []string
	for _, p := range c.Props {
		if p.Required {
			out = append(out, p.Name)
		}
	}
	return out
}

// PropByName returns the spec for a named prop. Second return is false if the
// component does not declare that prop.
func (c Component) PropByName(name string) (PropSpec, bool) {
	for _, p := range c.Props {
		if p.Name == name {
			return p, true
		}
	}
	return PropSpec{}, false
}

// Lookup returns the component spec for the given JSX tag name.
// The boolean is false for unknown components.
func Lookup(name string) (Component, bool) {
	c, ok := catalog[name]
	return c, ok
}

// All returns all known component names in alphabetical order. Useful
// for surfacing a deterministic list in validator error messages.
func All() []string {
	out := make([]string, 0, len(catalog))
	for name := range catalog {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// catalog is the in-memory registry. Keep entries alphabetical by component
// name to make diffs readable.
var catalog = map[string]Component{
	"Accordion": {
		Name:           "Accordion",
		Description:    "Collapsible disclosure with a title and hidden body content.",
		AllowsChildren: true,
		DocsURL:        "https://www.mintlify.com/docs/components/accordions",
		Props: []PropSpec{
			{Name: "title", Required: true},
			{Name: "description"},
			{Name: "defaultOpen"},
			{Name: "icon"},
			{Name: "iconType"},
		},
	},
	"AccordionGroup": {
		Name:           "AccordionGroup",
		Description:    "Container that groups related Accordion components.",
		AllowsChildren: true,
		DocsURL:        "https://www.mintlify.com/docs/components/accordions",
	},
	"Card": {
		Name:           "Card",
		Description:    "Linked or static content card with title, optional icon, and child body.",
		AllowsChildren: true,
		DocsURL:        "https://www.mintlify.com/docs/components/cards",
		Props: []PropSpec{
			{Name: "title", Required: true},
			{Name: "icon"},
			{Name: "iconType"},
			{Name: "color"},
			{Name: "href"},
			{Name: "horizontal"},
			{Name: "arrow"},
			{Name: "cta"},
			{Name: "img"},
		},
	},
	"CardGroup": {
		Name:           "CardGroup",
		Description:    "Grid container that lays out Card components in N columns.",
		AllowsChildren: true,
		DocsURL:        "https://www.mintlify.com/docs/components/card-group",
		Props: []PropSpec{
			{Name: "cols"},
		},
	},
	"Check": {
		Name:           "Check",
		Description:    "Green checkmark callout for confirmations.",
		AllowsChildren: true,
		DocsURL:        "https://www.mintlify.com/docs/components/callouts",
	},
	"CodeGroup": {
		Name:           "CodeGroup",
		Description:    "Tabbed grouping of consecutive fenced code blocks.",
		AllowsChildren: true,
		DocsURL:        "https://www.mintlify.com/docs/components/code-group",
	},
	"Columns": {
		Name:           "Columns",
		Description:    "Multi-column layout container.",
		AllowsChildren: true,
		DocsURL:        "https://www.mintlify.com/docs/components/columns",
		Props: []PropSpec{
			{Name: "cols"},
		},
	},
	"Expandable": {
		Name:           "Expandable",
		Description:    "Collapsible block, typically used inside ResponseField for nested schemas.",
		AllowsChildren: true,
		DocsURL:        "https://www.mintlify.com/docs/components/expandable",
		Props: []PropSpec{
			{Name: "title"},
			{Name: "defaultOpen"},
		},
	},
	"Frame": {
		Name:           "Frame",
		Description:    "Bordered container for images or media, with optional caption.",
		AllowsChildren: true,
		DocsURL:        "https://www.mintlify.com/docs/components/frames",
		Props: []PropSpec{
			{Name: "caption"},
		},
	},
	"Icon": {
		Name:        "Icon",
		Description: "Inline icon by name (FontAwesome, Lucide, etc.).",
		DocsURL:     "https://www.mintlify.com/docs/components/icons",
		Props: []PropSpec{
			{Name: "icon", Required: true},
			{Name: "color"},
			{Name: "size"},
			{Name: "iconType", EnumValues: []string{"regular", "solid", "light", "thin", "sharp-solid", "duotone", "brands"}},
		},
	},
	"Info": {
		Name:           "Info",
		Description:    "Blue informational callout.",
		AllowsChildren: true,
		DocsURL:        "https://www.mintlify.com/docs/components/callouts",
	},
	"Note": {
		Name:           "Note",
		Description:    "Neutral callout for sidebar context.",
		AllowsChildren: true,
		DocsURL:        "https://www.mintlify.com/docs/components/callouts",
	},
	"ParamField": {
		Name:           "ParamField",
		Description:    "API request parameter declaration. Used in API reference pages.",
		AllowsChildren: true,
		DocsURL:        "https://www.mintlify.com/docs/api-playground/params",
		Props: []PropSpec{
			{Name: "path"},
			{Name: "query"},
			{Name: "body"},
			{Name: "header"},
			{Name: "type"},
			{Name: "required"},
			{Name: "default"},
			{Name: "placeholder"},
		},
	},
	"RequestExample": {
		Name:           "RequestExample",
		Description:    "Container for example API request snippets.",
		AllowsChildren: true,
		DocsURL:        "https://www.mintlify.com/docs/api-playground/migrating",
	},
	"ResponseExample": {
		Name:           "ResponseExample",
		Description:    "Container for example API response snippets.",
		AllowsChildren: true,
		DocsURL:        "https://www.mintlify.com/docs/api-playground/migrating",
	},
	"ResponseField": {
		Name:           "ResponseField",
		Description:    "API response field declaration.",
		AllowsChildren: true,
		DocsURL:        "https://www.mintlify.com/docs/api-playground/params",
		Props: []PropSpec{
			{Name: "name", Required: true},
			{Name: "type"},
			{Name: "required"},
			{Name: "default"},
		},
	},
	"Step": {
		Name:           "Step",
		Description:    "Single step in a Steps sequence.",
		AllowsChildren: true,
		DocsURL:        "https://www.mintlify.com/docs/components/steps",
		Props: []PropSpec{
			{Name: "title", Required: true},
			{Name: "icon"},
			{Name: "iconType"},
			{Name: "stepNumber"},
		},
	},
	"Steps": {
		Name:           "Steps",
		Description:    "Numbered procedural sequence wrapping Step components.",
		AllowsChildren: true,
		DocsURL:        "https://www.mintlify.com/docs/components/steps",
	},
	"Tab": {
		Name:           "Tab",
		Description:    "Single tab inside a Tabs container.",
		AllowsChildren: true,
		DocsURL:        "https://www.mintlify.com/docs/components/tabs",
		Props: []PropSpec{
			{Name: "title", Required: true},
		},
	},
	"Tabs": {
		Name:           "Tabs",
		Description:    "Tabbed container for grouped Tab components.",
		AllowsChildren: true,
		DocsURL:        "https://www.mintlify.com/docs/components/tabs",
	},
	"Tip": {
		Name:           "Tip",
		Description:    "Green callout for helpful suggestions.",
		AllowsChildren: true,
		DocsURL:        "https://www.mintlify.com/docs/components/callouts",
	},
	"Tooltip": {
		Name:        "Tooltip",
		Description: "Inline hover tooltip.",
		DocsURL:     "https://www.mintlify.com/docs/components/tooltips",
		Props: []PropSpec{
			{Name: "tip", Required: true},
		},
	},
	"Update": {
		Name:           "Update",
		Description:    "Changelog/update entry block.",
		AllowsChildren: true,
		DocsURL:        "https://www.mintlify.com/docs/components/update",
		Props: []PropSpec{
			{Name: "label", Required: true},
			{Name: "description", Required: true},
			{Name: "tags"},
		},
	},
	"Warning": {
		Name:           "Warning",
		Description:    "Yellow/red callout for cautions.",
		AllowsChildren: true,
		DocsURL:        "https://www.mintlify.com/docs/components/callouts",
	},
}
