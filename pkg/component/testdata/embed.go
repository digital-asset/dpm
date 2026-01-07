// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package testdata

import _ "embed"

// TODO auto-generate this file
// TODO double check this don't get included in the final binary

//go:embed valid.yaml
var Valid []byte

//go:embed empty.yaml
var Empty []byte

//go:embed unknown-field.yaml
var UnknownField []byte

//go:embed unknown-strategy-type.yaml
var UnknownExportStrategy []byte
