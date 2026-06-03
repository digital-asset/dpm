// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package darpusher

import (
	"bytes"
	"context"
	"fmt"
	"github.com/opencontainers/go-digest"
	"maps"
	"oras.land/oras-go/v2/content/memory"
	"os"
	"path/filepath"

	consts "daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/oci"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"daml.com/x/assistant/pkg/utils/fileinfo"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
)

type DarOpts struct {
	Artifact            oci.Artifact
	RawTag, Dir         string
	RequiredAnnotations oci.DescriptorAnnotations
}

type DarPushOperation struct {
	fs           *file.Store
	ms           *memory.Store
	manifestDesc *v1.Descriptor
	repoName     string
	rawTag       string
}

func (op *DarPushOperation) Tag() string {
	return op.rawTag
}

func (op *DarPushOperation) DarDestination(registry string) string {
	return fmt.Sprintf("%s:%s", registry, op.Tag())
}

func DarNew(ctx context.Context, opts DarOpts) (*DarPushOperation, error) {
	repoName := opts.Artifact.RepoName()

	ms := memory.New()
	fs, err := file.New(opts.Dir)
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

		fileBytes, err := os.ReadFile(filepath.Join(opts.Dir, de.Name()))
		if err != nil {
			return nil, err
		}

		if err := ms.Push(ctx, fileDescriptor, bytes.NewReader(fileBytes)); err != nil {
			return nil, err
		}
		fileDescriptors = append(fileDescriptors, fileDescriptor)
	}

	darFilePath := filepath.Join(opts.Dir, consts.DarManifestName)
	_, err = os.Stat(darFilePath)
	if err != nil {
		DarManifest, err := darManifest(ctx, ms, opts)
		if err != nil {
			return nil, err
		}
		fileDescriptors = append(fileDescriptors, *DarManifest)
	}

	annotations := map[string]string{}

	packOpts := oras.PackManifestOptions{
		Layers:              fileDescriptors,
		ManifestAnnotations: annotations,
	}
	manifestDescriptor, err := oras.PackManifest(ctx, ms, oras.PackManifestVersion1_1, opts.Artifact.ArtifactType(), packOpts)
	if err != nil {
		return nil, err
	}

	op := &DarPushOperation{
		repoName:     repoName,
		rawTag:       opts.RawTag,
		fs:           fs,
		ms:           ms,
		manifestDesc: &manifestDescriptor,
	}

	if err := ms.Tag(ctx, manifestDescriptor, op.Tag()); err != nil {
		return nil, err
	}

	return op, nil
}

// DarDo pushes the content of dir to an oci registry
// mostly copied from
// https://pkg.go.dev/oras.land/oras-go/v2#example-package-PushFilesToRemoteRepository
func (op *DarPushOperation) DarDo(ctx context.Context, client *assistantremote.Remote) (*v1.Descriptor, error) {
	repo, err := remote.NewRepository(fmt.Sprintf("%s/%s", client.Registry, op.repoName))
	if err != nil {
		return nil, err
	}
	repo.Client = client
	repo.PlainHTTP = client.Insecure

	d, err := oras.Copy(ctx, op.ms, op.Tag(), repo, op.Tag(), oras.DefaultCopyOptions)

	if err != nil {
		return nil, err
	}

	if err := op.fs.Close(); err != nil {
		return nil, err
	}
	return &d, err
}

func createDarManifest(dir string) []byte {
	manifestData := fmt.Sprintf(`
apiVersion: digitalasset.com/v1
kind: Dar
spec: 
	paths:
		- %s
`, filepath.Join(dir, consts.DarManifestName))
	darByte := []byte(manifestData)
	return darByte
}

func appendAnnotations(descriptor v1.Descriptor, annotations map[string]string) {
	if descriptor.Annotations == nil {
		descriptor.Annotations = map[string]string{}
	}
	maps.Copy(descriptor.Annotations, annotations)
}

func darManifest(ctx context.Context, mem *memory.Store, opts DarOpts) (*v1.Descriptor, error) {
	darByte := createDarManifest(opts.Dir)
	blobReader := bytes.NewReader(darByte)

	desc := ocispec.Descriptor{
		MediaType: opts.Artifact.FileMediaType(),
		Digest:    digest.FromBytes(darByte),
		Size:      int64(len(darByte)),
		Annotations: map[string]string{
			ocispec.AnnotationTitle: consts.DarManifestName,
		},
	}

	if err := mem.Push(ctx, desc, blobReader); err != nil {
		return nil, err
	}

	return &desc, nil
}
