// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package publish

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_collectGitAnnotations(t *testing.T) {
	annotations, err := collectGitAnnotations()
	require.NoError(t, err)
	assert.Contains(t, annotations, "git.commit")
}
