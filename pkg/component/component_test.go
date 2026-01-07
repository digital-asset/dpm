// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package component

import (
	"testing"

	"daml.com/x/assistant/pkg/component/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadComponentContents(t *testing.T) {
	c, err := ReadComponentContents(testdata.Valid)
	require.NoError(t, err)
	assert.NotEmpty(t, c.Spec.NativeCommands[0].Name)

	for _, y := range [][]byte{testdata.Empty, testdata.UnknownField, testdata.UnknownExportStrategy} {
		_, err = ReadComponentContents(y)
		assert.ErrorIs(t, err, ErrInvalidComponentManifest)
	}
}
