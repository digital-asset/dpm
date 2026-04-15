// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package publish

import (
	"fmt"

	ociconsts "daml.com/x/assistant/pkg/oci"
)

type Destination struct {
	Registry string
	Artifact ociconsts.Artifact
}

func (d *Destination) String() string {
	return fmt.Sprintf("%s/%s", d.Registry, d.Artifact.RepoName())
}
