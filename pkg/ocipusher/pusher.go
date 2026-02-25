// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ocipusher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"

	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/oci"
	"daml.com/x/assistant/pkg/simpleplatform"
	"daml.com/x/assistant/pkg/utils/fileinfo"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
)

type PushOperation struct {
	fs                       *file.Store
	manifestDesc, configDesc *v1.Descriptor
	repoName                 string
	rawTag                   string // raw tag without platform. e.g. 1.2.3 not 1.2.3.linux_darwin

	platform simpleplatform.Platform // nil for generic
}

func (op *PushOperation) Tag() string {
	return op.platform.ImageTag(op.rawTag)
}

func (op *PushOperation) Platform() simpleplatform.Platform {
	return op.platform
}

func (op *PushOperation) Destination(registry string) string {
	return fmt.Sprintf("%s/%s:%s", registry, op.repoName, op.Tag())
}

func (op *PushOperation) DarDestination(registry string) string {
	return fmt.Sprintf("%s/%s:%s", registry, op.repoName, op.rawTag)
}

// Do pushes the content of dir to an oci registry
//
// mostly copied from
// https://pkg.go.dev/oras.land/oras-go/v2#example-package-PushFilesToRemoteRepository
func (op *PushOperation) Do(ctx context.Context, client *assistantremote.Remote) (*v1.Descriptor, error) {
	repo, err := remote.NewRepository(fmt.Sprintf("%s/%s", client.Registry, op.repoName))
	if err != nil {
		return nil, err
	}

	repo.Client = client
	repo.PlainHTTP = client.Insecure

	d, err := oras.Copy(ctx, op.fs, op.Tag(), repo, op.Tag(), oras.DefaultCopyOptions)
	if err != nil {
		return nil, err
	}

	if err := op.fs.Close(); err != nil {
		return nil, err
	}

	return &d, err
}

type Opts struct {
	Artifact            oci.Artifact
	RawTag, Dir         string
	RequiredAnnotations oci.DescriptorAnnotations
	ExtraAnnotations    map[string]string
	Platform            simpleplatform.Platform
}

func New(ctx context.Context, opts Opts) (*PushOperation, error) {
	repoName := opts.Artifact.RepoName()

	fs, err := file.New(opts.Dir)
	if err != nil {
		return nil, err
	}

	configDesc, err := appendConfig(ctx, fs, opts.Platform)
	if err != nil {
		return nil, err
	}

	dEntries, err := os.ReadDir(opts.Dir)
	if err != nil {
		return nil, err
	}

	var fileDescriptors []v1.Descriptor
	for _, de := range dEntries {
		fileDescriptor, err := fs.Add(ctx, de.Name(), opts.Artifact.FileMediaType(), "")
		if err != nil {
			return nil, err
		}

		osFileInfo, err := de.Info()
		if err != nil {
			return nil, err
		}
		fileInfoAnnotations := fileinfo.New(osFileInfo).AsAnnotations()
		appendAnnotations(fileDescriptor, fileInfoAnnotations)

		fileDescriptors = append(fileDescriptors, fileDescriptor)
	}

	annotations := map[string]string{}
	maps.Copy(annotations, opts.ExtraAnnotations)
	opts.RequiredAnnotations.AppendToMap(annotations)

	packOpts := oras.PackManifestOptions{
		Layers:              fileDescriptors,
		ManifestAnnotations: annotations,
		ConfigDescriptor:    configDesc,
	}

	manifestDescriptor, err := oras.PackManifest(ctx, fs, oras.PackManifestVersion1_1, opts.Artifact.ArtifactType(), packOpts)
	if err != nil {
		return nil, err
	}

	op := &PushOperation{
		repoName:     repoName,
		rawTag:       opts.RawTag,
		fs:           fs,
		manifestDesc: &manifestDescriptor,
		configDesc:   configDesc,
		platform:     opts.Platform,
	}

	if err := fs.Tag(ctx, manifestDescriptor, op.Tag()); err != nil {
		return nil, err
	}

	return op, nil
}

func appendAnnotations(descriptor v1.Descriptor, annotations map[string]string) {
	if descriptor.Annotations == nil {
		descriptor.Annotations = map[string]string{}
	}
	maps.Copy(descriptor.Annotations, annotations)
}

func appendConfig(ctx context.Context, store *file.Store, platform simpleplatform.Platform) (*v1.Descriptor, error) {
	var desc v1.Descriptor
	var blob []byte

	switch p := (platform).(type) {
	case *simpleplatform.NonGeneric:
		var err error
		blob, err = json.Marshal(p.ToOras())
		if err != nil {
			return nil, err
		}
	case *simpleplatform.Generic:
		blob = []byte(`{}`)
	default:
		return nil, fmt.Errorf("unknown platform type %t", platform)
	}

	desc = content.NewDescriptorFromBytes(oras.MediaTypeUnknownConfig, blob)
	if err := store.Push(ctx, desc, bytes.NewReader(blob)); err != nil {
		return nil, err
	}

	return &desc, nil
}
