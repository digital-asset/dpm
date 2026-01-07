// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ociindex

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/oci"
	"daml.com/x/assistant/pkg/simpleplatform"
	"github.com/Masterminds/semver/v3"
	"github.com/opencontainers/image-spec/specs-go"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/samber/lo"
	"oras.land/oras-go/v2"
)

type Opts struct {
	Artifact            oci.Artifact
	Tag                 string
	Manifests           []v1.Descriptor
	ExtraAnnotations    map[string]string
	RequiredAnnotations oci.DescriptorAnnotations
}

func Tag(ctx context.Context, client *assistantremote.Remote, artifact oci.Artifact, version *semver.Version, tags []string) error {
	repo, err := client.Repo(artifact.RepoName())
	if err != nil {
		return err
	}
	_, err = oras.TagN(ctx, repo, version.String(), tags, oras.DefaultTagNOptions)
	return err
}

func PushIndex(ctx context.Context, client *assistantremote.Remote, opts Opts) (*v1.Descriptor, error) {
	repo, err := client.Repo(opts.Artifact.RepoName())
	if err != nil {
		return nil, err
	}

	annotations := map[string]string{}
	maps.Copy(annotations, opts.ExtraAnnotations)
	opts.RequiredAnnotations.AppendToMap(annotations)

	return pushIndex(ctx, repo, opts.Tag, opts.Manifests, opts.Artifact.ArtifactType(), annotations)
}

func pushIndex(ctx context.Context, repo oras.Target, tag string, manifests []v1.Descriptor, artifactType string, annotations map[string]string) (*v1.Descriptor, error) {
	index := v1.Index{
		ArtifactType: artifactType,
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		MediaType:   v1.MediaTypeImageIndex,
		Manifests:   manifests,
		Annotations: annotations,
	}
	indexBytes, err := json.Marshal(index)
	if err != nil {
		return nil, err
	}

	indexDesc, err := oras.TagBytes(ctx, repo, v1.MediaTypeImageIndex, indexBytes, tag)
	if err != nil {
		return nil, err
	}

	return &indexDesc, err
}

func FetchIndex(ctx context.Context, client *assistantremote.Remote, repoName, tag string) (*v1.Index, []byte, error) {
	repo, err := client.Repo(repoName)
	if err != nil {
		return nil, nil, err
	}

	return FetchIndexFromTarget(ctx, repo, repoName, tag)
}

func FetchIndexFromTarget(ctx context.Context, repo oras.ReadOnlyTarget, repoName, tag string) (*v1.Index, []byte, error) {
	desc, bytes, err := oras.FetchBytes(ctx, repo, tag, oras.DefaultFetchBytesOptions)
	if err != nil {
		return nil, nil, err
	}

	if desc.MediaType != v1.MediaTypeImageIndex {
		return nil, nil, fmt.Errorf("reference \"%s:%s\" is %q and not an image index", repoName, tag, desc.MediaType)
	}

	index := v1.Index{}
	if err := json.Unmarshal(bytes, &index); err != nil {
		return nil, nil, err
	}
	return &index, bytes, nil
}

// FindTargetPlatform selects a descriptor matching the given preferred platform, or a generic platform as fallback,
// returns an error if neither are available
func FindTargetPlatform(descriptors []v1.Descriptor, preferred *simpleplatform.NonGeneric) (*v1.Descriptor, error) {
	targetDesc, ok := lo.Find(descriptors, func(d v1.Descriptor) bool {
		return d.Platform != nil && d.Platform.OS == preferred.OS && d.Platform.Architecture == preferred.Architecture
	})
	if ok {
		return &targetDesc, nil
	}

	// generic platform
	targetDesc, ok = lo.Find(descriptors, func(d v1.Descriptor) bool {
		return d.Platform == nil
	})
	if !ok {
		platforms := lo.Map(descriptors, func(d v1.Descriptor, _ int) string {
			return simpleplatform.FromOras(d.Platform).String()
		})
		return nil, fmt.Errorf("no matching platform %s/%s found among manifests in index: %v", preferred.OS, preferred.Architecture, platforms)
	}

	return &targetDesc, nil
}

func ResolveTag(ctx context.Context, client *assistantremote.Remote, artifact oci.Artifact, tag string) (*semver.Version, error) {
	index, _, err := FetchIndex(ctx, client, artifact.RepoName(), tag)
	if err != nil {
		return nil, err
	}
	v, err := oci.VersionFromDescriptorAnnotations(index.Annotations)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve '%s:%s': %w", artifact.RepoName(), tag, err)
	}
	return v, err
}
