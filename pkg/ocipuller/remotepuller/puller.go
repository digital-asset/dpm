// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package remotepuller

import (
	"context"
	"fmt"
	"github.com/opencontainers/go-digest"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	ociconsts "daml.com/x/assistant/pkg/oci"
	"daml.com/x/assistant/pkg/ocicache"
	"daml.com/x/assistant/pkg/ociindex"
	"daml.com/x/assistant/pkg/ocipuller"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/simpleplatform"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
)

type RemoteOciPuller struct {
	ociLayoutCache string
	remote         *assistantremote.Remote
}

var _ ocipuller.OciPuller = (*RemoteOciPuller)(nil)

func New(ociLayoutCache string, remote *assistantremote.Remote) *RemoteOciPuller {
	return &RemoteOciPuller{
		ociLayoutCache: ociLayoutCache,
		remote:         remote,
	}
}

func NewFromRemoteConfig(config *assistantconfig.Config) (*RemoteOciPuller, error) {
	remote, err := assistantremote.NewFromConfig(config)
	if err != nil {
		return nil, err
	}
	return New(config.OciLayoutCache, remote), nil
}

func (a *RemoteOciPuller) PullComponent(ctx context.Context, componentName, tag, destPath string, platform simpleplatform.Platform) (*v1.Descriptor, error) {
	return a.pull(ctx, ociconsts.ComponentRepoPrefix+componentName, tag, destPath, platform)
}

func (a *RemoteOciPuller) PullComponentByFullPath(ctx context.Context, componentPath, reference, destPath string, platform simpleplatform.Platform) (*v1.Descriptor, error) {
	return a.pull(ctx, componentPath, reference, destPath, platform)
}

func (a *RemoteOciPuller) PullDarByFullPath(ctx context.Context, darPath, tag, destPath string) (*v1.Descriptor, error) {
	return a.pull(ctx, darPath, tag, destPath, &simpleplatform.Generic{})
}

func (a *RemoteOciPuller) PullAssembly(ctx context.Context, edition sdkmanifest.Edition, tag, destPath string, platform *simpleplatform.NonGeneric) (*v1.Descriptor, error) {
	repo, err := edition.SdkManifestsRepo()
	if err != nil {
		return nil, err
	}
	return a.pull(ctx, repo, tag, destPath, platform)
}

func (a *RemoteOciPuller) pull(ctx context.Context, repo, reference, destPath string, platform simpleplatform.Platform) (*v1.Descriptor, error) {
	src, err := a.cachedRepo(fmt.Sprintf("%s/%s", a.remote.Registry, repo))
	if err != nil {
		return nil, err
	}
	dest, err := file.New(destPath)
	if err != nil {
		return nil, err
	}
	dest.PreservePermissions = true
	// errors out if dest already exists
	dest.DisableOverwrite = true
	opts := ocipuller.ApplyFileInfoCopyOptions(destPath)
	if nonGeneric, ok := platform.(*simpleplatform.NonGeneric); ok {
		index, _, err := ociindex.FetchIndex(ctx, a.remote, repo, reference)
		if err != nil {
			return nil, err
		}

		descriptor, err := ociindex.FindTargetPlatform(index.Manifests, nonGeneric)
		if err != nil {
			return nil, err
		}

		opts.WithTargetPlatform(descriptor.Platform)
	}

	desc, err := oras.Copy(ctx, src, reference, dest, reference, opts)
	if err != nil {
		return nil, err
	}
	return &desc, err
}

func (a *RemoteOciPuller) cachedRepo(url string) (oras.ReadOnlyTarget, error) {
	repo, err := remote.NewRepository(url)
	if err != nil {
		return nil, err
	}
	repo.Client = a.remote
	repo.PlainHTTP = a.remote.Insecure
	return ocicache.CachedTarget(repo, a.ociLayoutCache)
}

func (a *RemoteOciPuller) GetDigest(ctx context.Context, compRepo string, tag string) (*digest.Digest, error) {
	url := fmt.Sprintf("%s/%s", a.remote.Registry, compRepo)
	repo, err := remote.NewRepository(url)
	if err != nil {
		return nil, err
	}
	repo.Client = a.remote
	repo.PlainHTTP = a.remote.Insecure
	desc, err := repo.Resolve(ctx, tag)
	if err != nil {
		return nil, err
	}
	return &desc.Digest, nil
}
