// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ocipuller

import (
	"context"

	"daml.com/x/assistant/pkg/oci"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/simpleplatform"
	"daml.com/x/assistant/pkg/utils/fileinfo"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
)

type OciPuller interface {
	PullAssembly(ctx context.Context, edition sdkmanifest.Edition, tag, destPath string, platform *simpleplatform.NonGeneric) error
	PullComponent(ctx context.Context, componentName, tag, destPath string, platform simpleplatform.Platform) error
}

// ApplyFileInfoCopyOptions returns an oras.CopyOptions that applies
// the fileinfo.FileInfo annotations present on a component's layers upon copying
func ApplyFileInfoCopyOptions(rootPath string) oras.CopyOptions {
	opts := oras.DefaultCopyOptions
	defaultPostCopy := opts.PostCopy

	opts.PostCopy = func(ctx context.Context, desc ocispec.Descriptor) error {
		if defaultPostCopy != nil {
			if err := defaultPostCopy(ctx, desc); err != nil {
				return nil
			}
		}
		if desc.MediaType != oci.ComponentFileMediaType {
			return nil
		}

		fi, err := fileinfo.NewFromAnnotations(desc.Annotations)
		if err != nil {
			return err
		}
		return fi.Apply(rootPath)
	}
	return opts
}
