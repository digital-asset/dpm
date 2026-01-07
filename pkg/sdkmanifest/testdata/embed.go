// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package testdata

import _ "embed"

// TODO auto-generate this file

//go:embed valid.yaml
var Valid []byte

//go:embed empty.yaml
var Empty []byte

//go:embed zeroComponents.yaml
var ZeroComponents []byte

//go:embed emptyComponent.yaml
var EmptyComponent []byte

//go:embed missingEdition.yaml
var MissingEdition []byte

//go:embed invalidEdition.yaml
var InvalidEdition []byte
