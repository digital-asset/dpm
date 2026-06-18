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
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/content/oci"
)

type LocalOciPuller struct {
	config            *assistantconfig.Config
	localRegistryPath string
}

func (a *LocalOciPuller) PullDarByFullPath(ctx context.Context, darPath, tag, destPath string) (*v1.Descriptor, error) {
	return nil, fmt.Errorf("local oci-layout pulling of dars is not supported")
}

var _ ocipuller.OciPuller = (*LocalOciPuller)(nil)

func New(config *assistantconfig.Config, localRegistryPath string) *LocalOciPuller {
	return &LocalOciPuller{config, localRegistryPath}
}

func (a *LocalOciPuller) PullComponent(ctx context.Context, componentName, tag, destPath string, platform simpleplatform.Platform) (*v1.Descriptor, error) {
	return a.pull(ctx, ociconsts.ComponentRepoPrefix+componentName, tag, destPath, platform)
}

func (a *LocalOciPuller) PullComponentByFullPath(ctx context.Context, componentPath, tag, destPath string, platform simpleplatform.Platform) (*v1.Descriptor, error) {
	return a.pull(ctx, componentPath, tag, destPath, platform)
}

func (p *LocalOciPuller) PullAssembly(ctx context.Context, edition sdkmanifest.Edition, tag, destPath string, _ *simpleplatform.NonGeneric) (*v1.Descriptor, error) {
	repo, err := edition.SdkManifestsRepo()
	if err != nil {
		return nil, err
	}
	return p.pull(ctx, repo, tag, destPath, nil)
}

func (p *LocalOciPuller) pull(ctx context.Context, repo, tag, destPath string, platform simpleplatform.Platform) (*v1.Descriptor, error) {
	target, err := p.getLocalOciTarget(ctx, repo)
	if err != nil {
		return nil, err
	}
	src, err := ocicache.CachedTarget(target, p.config.OciLayoutCache)
	if err != nil {
		return nil, err
	}

	dest, err := file.New(destPath)
	if err != nil {
		return nil, err
	}
	dest.PreservePermissions = true

	opts := ocipuller.ApplyFileInfoCopyOptions(destPath)

	if nonGeneric, ok := platform.(*simpleplatform.NonGeneric); ok {
		index, _, err := ociindex.FetchIndexFromTarget(ctx, src, repo, tag)
		if err != nil {
			return nil, err
		}

		descriptor, err := ociindex.FindTargetPlatform(index.Manifests, nonGeneric)
		if err != nil {
			return nil, err
		}

		opts.WithTargetPlatform(descriptor.Platform)
	}

	desc, err := oras.Copy(ctx, src, tag, dest, tag, opts)
	return &desc, err
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

func (p *LocalOciPuller) GetManifest(ctx context.Context, compRepo string, tag string, platform simpleplatform.Platform) (*v1.Descriptor, error) {
	target, err := p.getLocalOciTarget(ctx, compRepo)
	if err != nil {
		return nil, err
	}

	src, err := ocicache.CachedTarget(target, p.config.OciLayoutCache)
	if err != nil {
		return nil, err
	}

	nonGeneric := platform.(*simpleplatform.NonGeneric)
	index, _, err := ociindex.FetchIndexFromTarget(ctx, src, compRepo, tag)
	if err != nil {
		return nil, err
	}

	manifestDesc, err := ociindex.FindTargetPlatform(index.Manifests, nonGeneric)
	if err != nil {
		return nil, err
	}

	return manifestDesc, nil

}
