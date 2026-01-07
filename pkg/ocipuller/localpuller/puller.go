// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package localpuller

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"daml.com/x/assistant/pkg/assistantconfig"
	ociconsts "daml.com/x/assistant/pkg/oci"
	"daml.com/x/assistant/pkg/ocicache"
	"daml.com/x/assistant/pkg/ociindex"
	"daml.com/x/assistant/pkg/ocipuller"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/simpleplatform"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/content/oci"
)

type LocalOciPuller struct {
	config            *assistantconfig.Config
	localRegistryPath string
}

var _ ocipuller.OciPuller = (*LocalOciPuller)(nil)

func New(config *assistantconfig.Config, localRegistryPath string) *LocalOciPuller {
	return &LocalOciPuller{config, localRegistryPath}
}

func (p *LocalOciPuller) PullComponent(ctx context.Context, componentName, tag, destPath string, platform simpleplatform.Platform) error {
	return p.pull(ctx, ociconsts.ComponentRepoPrefix+componentName, tag, destPath, platform)
}

func (p *LocalOciPuller) PullAssembly(ctx context.Context, edition sdkmanifest.Edition, tag, destPath string, _ *simpleplatform.NonGeneric) error {
	repo, err := edition.SdkManifestsRepo()
	if err != nil {
		return err
	}
	return p.pull(ctx, repo, tag, destPath, nil)
}

func (p *LocalOciPuller) pull(ctx context.Context, repo, tag, destPath string, platform simpleplatform.Platform) error {
	target, err := p.getLocalOciTarget(ctx, repo)
	if err != nil {
		return err
	}
	src, err := ocicache.CachedTarget(target, p.config.OciLayoutCache)
	if err != nil {
		return err
	}

	dest, err := file.New(destPath)
	if err != nil {
		return err
	}
	dest.PreservePermissions = true

	opts := ocipuller.ApplyFileInfoCopyOptions(destPath)

	if nonGeneric, ok := platform.(*simpleplatform.NonGeneric); ok {
		index, _, err := ociindex.FetchIndexFromTarget(ctx, src, repo, tag)
		if err != nil {
			return err
		}

		descriptor, err := ociindex.FindTargetPlatform(index.Manifests, nonGeneric)
		if err != nil {
			return err
		}

		opts.WithTargetPlatform(descriptor.Platform)
	}

	_, err = oras.Copy(ctx, src, tag, dest, tag, opts)
	return err
}

func (p *LocalOciPuller) getLocalOciTarget(ctx context.Context, repo string) (oras.ReadOnlyTarget, error) {
	d := filepath.Join(p.localRegistryPath, repo)
	info, err := os.Stat(d)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("expected %s to be a directory", d)
	}
	return oci.NewFromFS(ctx, os.DirFS(d))
}
