// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import "testing"

func TestCleanupWhitespace(t *testing.T) {
	in := "line with trailing  \t\n\n\n\n\nafter many blanks\n\n\n"
	want := "line with trailing\n\nafter many blanks"
	if got := cleanupWhitespace(in); got != want {
		t.Errorf("want %q got %q", want, got)
	}
}
