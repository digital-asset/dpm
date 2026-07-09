// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"strings"
	"testing"
)

func TestConvertTabs(t *testing.T) {
	in := `.. tabs::

   .. tab:: First Tab

      Content for the first tab.

   .. tab:: Second Tab

      Content for the second tab.
`
	got := convertTabs(in)
	for _, want := range []string{
		"<Tabs>",
		`<Tab title="First Tab">`,
		"Content for the first tab.",
		"</Tab>",
		`<Tab title="Second Tab">`,
		"Content for the second tab.",
		"</Tabs>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, ".. tabs::") || strings.Contains(got, ".. tab::") {
		t.Errorf("RST tabs directive survived in:\n%s", got)
	}
}

func TestConvertTabs_PreservesNestedRST(t *testing.T) {
	// Tab content has a code block — it should still run through the
	// rest of the pipeline once the tab wrapper is in place.
	in := `.. tabs::

   .. tab:: bash

      .. code-block:: bash

         make build
`
	got := convertTabs(in)
	if !strings.Contains(got, ".. code-block:: bash") {
		t.Errorf("nested directive should survive convertTabs (codeblocks runs later):\n%s", got)
	}
}

func TestConvertTabs_TitleEscaped(t *testing.T) {
	in := `.. tabs::

   .. tab:: Said "hello"

      ok
`
	got := convertTabs(in)
	if !strings.Contains(got, `title="Said \"hello\""`) {
		t.Errorf("quote in title not escaped:\n%s", got)
	}
}
