// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"strings"
	"testing"
)

func TestConvertYoutube(t *testing.T) {
	in := `.. youtube:: xsuMDLED6gI
`
	got := convertYoutube(in)
	for _, want := range []string{
		`<iframe`,
		`src="https://www.youtube.com/embed/xsuMDLED6gI"`,
		`title="YouTube video"`,
		`frameBorder="0"`,
		`allowFullScreen`,
		`/>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, ".. youtube::") {
		t.Errorf("directive line was not consumed:\n%s", got)
	}
}

func TestConvertYoutube_WithTitleOption(t *testing.T) {
	in := `.. youtube:: xsuMDLED6gI
   :title: Build Quickstart
`
	got := convertYoutube(in)
	if !strings.Contains(got, `title="Build Quickstart"`) {
		t.Errorf(":title: option not honored:\n%s", got)
	}
}

func TestConvertYoutube_PreservesIndent(t *testing.T) {
	in := `   .. youtube:: abc123
`
	got := convertYoutube(in)
	if !strings.Contains(got, `   <iframe`) {
		t.Errorf("indent dropped:\n%s", got)
	}
}
