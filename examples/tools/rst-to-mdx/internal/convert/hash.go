// Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"crypto/sha256"
	"encoding/hex"
)

// hash8 returns the first 8 hex characters of the SHA-256 of the input.
// Used for the COPIED_START provenance marker to signal drift when the
// source RST changes.
func hash8(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:8]
}
