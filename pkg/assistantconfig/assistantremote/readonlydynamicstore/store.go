// Copyright (c) 2017-2025 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package readonlydynamicstore

import (
	"context"
	"fmt"

	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

type ReadOnlyDynamicStore struct {
	ds *credentials.DynamicStore
}

var _ credentials.Store = (*ReadOnlyDynamicStore)(nil)

func New(dynamicStore *credentials.DynamicStore) *ReadOnlyDynamicStore {
	return &ReadOnlyDynamicStore{dynamicStore}
}

func (r ReadOnlyDynamicStore) Get(ctx context.Context, serverAddress string) (auth.Credential, error) {
	return r.ds.Get(ctx, serverAddress)
}

func (r ReadOnlyDynamicStore) Put(ctx context.Context, serverAddress string, cred auth.Credential) error {
	return fmt.Errorf("ReadOnlyDynamicStore credential store does not allow put operations")
}

func (r ReadOnlyDynamicStore) Delete(ctx context.Context, serverAddress string) error {
	return fmt.Errorf("ReadOnlyDynamicStore credential store does not allow delete operations")
}
