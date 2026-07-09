// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package pathmap

import "testing"

func TestDerive(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{
			name: "canton participant tutorial",
			in:   "/x/docs-website/docs/replicated/canton/3.5/participant/tutorials/getting_started.rst",
			want: "appdev/tutorials/canton-getting-started",
			ok:   true,
		},
		{
			name: "canton participant howto",
			in:   "/x/docs-website/docs/replicated/canton/3.5/participant/howtos/install.rst",
			want: "appdev/howtos/canton-install",
			ok:   true,
		},
		{
			name: "canton synchronizer",
			in:   "/x/docs-website/docs/replicated/canton/3.5/synchronizer/tutorials/setup.rst",
			want: "global-synchronizer/canton-setup",
			ok:   true,
		},
		{
			name: "canton sdk",
			in:   "/x/docs-website/docs/replicated/canton/3.5/sdk/intro.rst",
			want: "appdev/sdk/canton-intro",
			ok:   true,
		},
		{
			name: "daml tutorial",
			in:   "/x/docs-website/docs/replicated/daml/3.5/sdk/tutorials/first-steps.rst",
			want: "appdev/tutorials/daml-first-steps",
			ok:   true,
		},
		{
			name: "daml module",
			in:   "/x/docs-website/docs/replicated/daml/3.5/sdk/modules/contracts.rst",
			want: "appdev/modules/daml-contracts",
			ok:   true,
		},
		{
			name: "pqs",
			in:   "/x/docs-website/docs/replicated/pqs/3.5/reference/sql.rst",
			want: "integrations/pqs/sql",
			ok:   true,
		},
		{
			name: "splice wallet kernel",
			in:   "/x/docs-website/docs/replicated/splice-wallet-kernel/devnet/kernel-intro.rst",
			want: "integrations/splice/kernel-intro",
			ok:   true,
		},
		{
			name: "quickstart",
			in:   "/x/docs-website/docs/replicated/quickstart/3.5/sdk/quickstart/download/cnqs_installation.rst",
			want: "appdev/quickstart/cnqs-installation",
			ok:   true,
		},
		{
			name: "kebab-cases filename",
			in:   "/x/docs-website/docs/replicated/canton/3.5/participant/tutorials/GETTING_STARTED.rst",
			want: "appdev/tutorials/canton-getting-started",
			ok:   true,
		},
		{
			name: "unknown subtree returns false",
			in:   "/x/docs-website/docs/replicated/unknown-thing/file.rst",
			ok:   false,
		},
		{
			name: "path outside docs-website returns false",
			in:   "/random/place/foo.rst",
			ok:   false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Derive(tc.in)
			if ok != tc.ok {
				t.Fatalf("want ok=%v got ok=%v (path=%q)", tc.ok, ok, got)
			}
			if tc.ok && string(got) != tc.want {
				t.Errorf("want %q got %q", tc.want, got)
			}
		})
	}
}

func TestDerivedPath_URL(t *testing.T) {
	d := DerivedPath("appdev/tutorials/canton-getting-started")
	// Mintlify serves docs-main/ as site root, so the URL must NOT
	// include the docs-main/ segment.
	want := "/appdev/tutorials/canton-getting-started"
	if got := d.URL(); got != want {
		t.Errorf("want %q got %q", want, got)
	}
}
