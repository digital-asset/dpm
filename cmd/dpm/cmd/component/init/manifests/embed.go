// Copyright (c) 2017-2025 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package manifests

import _ "embed"

//go:embed component.yaml
var ComponentYaml []byte

//go:embed dpm.local.yaml
var Daml3LocalYaml []byte
