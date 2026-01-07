// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ocicache

import (
	"daml.com/x/assistant/pkg/ocicache/cache"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
)

func CachedTarget(src oras.ReadOnlyTarget, ociLayoutCache string) (oras.ReadOnlyTarget, error) {
	ociStore, err := oci.New(ociLayoutCache)
	if err != nil {
		return nil, err
	}
	return cache.New(src, ociStore), nil
}
