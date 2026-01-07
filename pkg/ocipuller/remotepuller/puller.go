// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package remotepuller

import (
	"context"
	"fmt"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	ociconsts "daml.com/x/assistant/pkg/oci"
	"daml.com/x/assistant/pkg/ocicache"
	"daml.com/x/assistant/pkg/ociindex"
	"daml.com/x/assistant/pkg/ocipuller"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/simpleplatform"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
)

type RemoteOciPuller struct {
	config *assistantconfig.Config
	remote *assistantremote.Remote
}

var _ ocipuller.OciPuller = (*RemoteOciPuller)(nil)

func New(config *assistantconfig.Config, remote *assistantremote.Remote) *RemoteOciPuller {
	return &RemoteOciPuller{
		config: config,
		remote: remote,
	}
}

func NewFromRemoteConfig(config *assistantconfig.Config) (*RemoteOciPuller, error) {
	remote, err := assistantremote.NewFromConfig(config)
	if err != nil {
		return nil, err
	}
	return New(config, remote), nil
}

func (a *RemoteOciPuller) PullComponent(ctx context.Context, componentName, tag, destPath string, platform simpleplatform.Platform) error {
	return a.pull(ctx, ociconsts.ComponentRepoPrefix+componentName, tag, destPath, platform)
}

func (a *RemoteOciPuller) PullAssembly(ctx context.Context, edition sdkmanifest.Edition, tag, destPath string, platform *simpleplatform.NonGeneric) error {
	repo, err := edition.SdkManifestsRepo()
	if err != nil {
		return err
	}
	return a.pull(ctx, repo, tag, destPath, platform)
}

func (a *RemoteOciPuller) pull(ctx context.Context, repo, tag, destPath string, platform simpleplatform.Platform) error {
	src, err := a.cachedRepo(fmt.Sprintf("%s/%s", a.remote.Registry, repo))
	if err != nil {
		return err
	}

	dest, err := file.New(destPath)
	if err != nil {
		return err
	}
	dest.PreservePermissions = true
	// errors out if dest already exists
	dest.DisableOverwrite = true

	opts := ocipuller.ApplyFileInfoCopyOptions(destPath)

	if nonGeneric, ok := platform.(*simpleplatform.NonGeneric); ok {
		index, _, err := ociindex.FetchIndex(ctx, a.remote, repo, tag)
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

func (a *RemoteOciPuller) cachedRepo(url string) (oras.ReadOnlyTarget, error) {
	repo, err := remote.NewRepository(url)
	if err != nil {
		return nil, err
	}
	repo.Client = a.remote
	repo.PlainHTTP = a.remote.Insecure
	return ocicache.CachedTarget(repo, a.config.OciLayoutCache)
}
