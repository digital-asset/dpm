// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import "testing"

func TestConvertInlineRoles(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"double backtick", "Use ``dpm install`` to install.", "Use `dpm install` to install."},
		{"code role", "Use :code:`dpm install` to install.", "Use `dpm install` to install."},
		{"file role", "See :file:`daml.yaml`.", "See `daml.yaml`."},
		{"command role", "Run :command:`dpm build`.", "Run `dpm build`."},
		{"kbd role", "Press :kbd:`Ctrl+C`.", "Press `Ctrl+C`."},
		{"guilabel role", "Click :guilabel:`Save`.", "Click **Save**."},
		{"menuselection role", ":menuselection:`File --> Save`", "**File --> Save**"},
		{"term role", "A :term:`Participant`.", "A **Participant**."},
		{"abbr role", "Use :abbr:`DPM (Digital Asset Package Manager)`.", "Use DPM."},
		{"italic passthrough", "*italic*", "*italic*"},
		{"bold passthrough", "**bold**", "**bold**"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := convertInlineRoles(tc.in)
			if got != tc.want {
				t.Errorf("want %q got %q", tc.want, got)
			}
		})
	}
}
