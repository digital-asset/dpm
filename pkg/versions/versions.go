// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package versions

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/samber/lo"
)

type Version struct {
	Version   *semver.Version `json:"version,omitempty"`
	Installed bool            `json:"installed,omitempty"`
	Remote    bool            `json:"remote,omitempty"`
	Active    bool            `json:"active,omitempty"`
	Tags      []string        `json:"tags,omitempty"`
}

type Versions []*Version

type versionsMap map[string]*Version

func New(active *semver.Version, installed []*semver.Version, remote map[*semver.Version][]string) Versions {
	m := versionsMap{}

	if active != nil {
		m.add(&Version{Version: active, Active: true})
	}

	for _, v := range installed {
		m.add(&Version{Version: v, Installed: true})
	}

	for v, tags := range remote {
		m.add(&Version{Version: v, Remote: true, Tags: tags})
	}

	r := Versions(lo.Values(m))
	r.Sort()
	return r
}

func (v versionsMap) add(e *Version) {
	key := e.Version.String()
	_, ok := v[key]

	if !ok {
		v[key] = e
		return
	}

	v[key].Installed = v[key].Installed || e.Installed
	v[key].Remote = v[key].Remote || e.Remote
	v[key].Active = v[key].Active || e.Active
	v[key].Tags = append(v[key].Tags, e.Tags...)
}

func (v Versions) Copy() Versions {
	r := make(Versions, len(v))
	lo.ForEach(v, func(e *Version, i int) {
		r[i] = &Version{
			Version: semver.New(
				e.Version.Major(),
				e.Version.Minor(),
				e.Version.Patch(),
				e.Version.Prerelease(),
				e.Version.Metadata(),
			),
			Installed: e.Installed,
			Remote:    e.Remote,
			Active:    e.Active,
			Tags:      e.Tags,
		}
	})
	return r
}

// Sort by semantic version number
func (v Versions) Sort() {
	slices.SortFunc(v, func(a, b *Version) int {
		return a.Version.Compare(b.Version)
	})
}

// Sort by installed first, than by semantic version number
func (v Versions) SortByInstalled() {
	slices.SortFunc(v, func(a, b *Version) int {
		if a.Installed && !b.Installed {
			return 1
		}

		if !a.Installed && b.Installed {
			return -1
		}

		return a.Version.Compare(b.Version)
	})
}

func (v Versions) Table() string {
	newV := v.Copy()
	newV.SortByInstalled()

	return table.New().
		Border(lipgloss.HiddenBorder()).
		BorderTop(false).
		BorderBottom(false).
		Rows(lo.Map(newV, func(row *Version, _ int) []string {
			indicator := ""

			version := row.Version.String()

			if len(row.Tags) > 0 {
				tags := strings.Join(row.Tags, ", ")
				version = fmt.Sprintf("%s\t(%s)", version, tags)
			}

			switch {
			case row.Active:
				indicator = "*"
				version = lipgloss.NewStyle().
					Foreground(lipgloss.Color("2")).
					Bold(true).
					Render(version)
			case !row.Installed:
				version = lipgloss.NewStyle().
					Faint(true).
					Italic(true).
					Render(version)
			}

			return []string{
				indicator,
				version,
			}
		})...).
		String()
}
