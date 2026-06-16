// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package darpusher

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"

	"daml.com/x/assistant/pkg/darmanifest"
	"daml.com/x/assistant/pkg/schema"
	"github.com/Masterminds/semver/v3"
	"github.com/goccy/go-yaml"
	"github.com/opencontainers/go-digest"
	"oras.land/oras-go/v2/content/memory"

	consts "daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/oci"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"daml.com/x/assistant/pkg/utils/fileinfo"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
)

type DarOpts struct {
	Artifact    oci.Artifact
	Version     semver.Version
	Dars        []string
	LicenseFile string
}

type DarPushOperation struct {
	ms           *memory.Store
	manifestDesc *v1.Descriptor
	repoName     string
	rawTag       string
}

func (op *DarPushOperation) Tag() string {
	return op.rawTag
}

func (op *DarPushOperation) DarDestination(registry string) string {
	return fmt.Sprintf("%s:%s:%s", registry, op.repoName, op.Tag())
}

func DarNew(ctx context.Context, opts DarOpts) (*DarPushOperation, error) {
	repoName := opts.Artifact.RepoName()

	ms := memory.New()
	var fileDescriptors []v1.Descriptor
	var errs []error

	darManifest := darmanifest.DarManifest{
		ManifestMeta: schema.ManifestMeta{
			APIVersion: darmanifest.DarAPIVersion,
			Kind:       darmanifest.DarKind,
		},
		Spec: &darmanifest.Spec{
			Dars: []darmanifest.Dar{},
		},
	}

	for _, darPath := range opts.Dars {
		mainPackageId, err := GetMainPackageId(darPath)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		darManifest.Spec.Dars = append(darManifest.Spec.Dars, darmanifest.Dar{
			Path:          filepath.Base(darPath),
			MainPackageId: mainPackageId,
		})

		darFileDescriptor, err := opts.mkDescriptorForFile(ctx, ms, darPath)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		darFileDescriptor.Annotations[oci.DescriptorMainPackageIdAnnotation] = mainPackageId

		fileDescriptors = append(fileDescriptors, darFileDescriptor)
	}

	darManifestDescriptor, err := createDarManifestDescriptor(ctx, ms, opts, darManifest)
	if err != nil {
		return nil, err
	}
	fileDescriptors = append(fileDescriptors, *darManifestDescriptor)

	// license file
	if opts.LicenseFile != "" {
		desc, err := opts.mkDescriptorForFile(ctx, ms, opts.LicenseFile)
		if err != nil {
			return nil, err
		}
		fileDescriptors = append(fileDescriptors, desc)
	}

	annotations := map[string]string{}
	annotations[v1.AnnotationVersion] = opts.Version.String()

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
		rawTag:       opts.Version.String(),
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

	return &d, err
}

func appendAnnotations(descriptor v1.Descriptor, annotations map[string]string) {
	if descriptor.Annotations == nil {
		descriptor.Annotations = map[string]string{}
	}
	maps.Copy(descriptor.Annotations, annotations)
}

func createDarManifestDescriptor(ctx context.Context, mem *memory.Store, opts DarOpts, manifest darmanifest.DarManifest) (*v1.Descriptor, error) {
	darByte, err := yaml.Marshal(manifest)
	if err != nil {
		return nil, err
	}

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

func (opts DarOpts) mkDescriptorForFile(ctx context.Context, mem *memory.Store, filePath string) (v1.Descriptor, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return v1.Descriptor{}, err
	}
	defer func() { _ = f.Close() }()

	fileInfo, err := f.Stat()
	if err != nil {
		return v1.Descriptor{}, err
	}

	if !fileInfo.Mode().IsRegular() {
		return v1.Descriptor{}, fmt.Errorf("invalid file %q. Only regular files allowed", filePath)
	}

	content, err := io.ReadAll(f)
	if err != nil {
		return v1.Descriptor{}, err
	}

	desc := ocispec.Descriptor{
		MediaType: opts.Artifact.FileMediaType(),
		Digest:    digest.FromBytes(content),
		Size:      fileInfo.Size(),
		Annotations: map[string]string{
			ocispec.AnnotationTitle: filepath.Base(filePath),
		},
	}

	fileInfoAnnotations := fileinfo.New(fileInfo).AsAnnotations()
	appendAnnotations(desc, fileInfoAnnotations)

	if err := mem.Push(ctx, desc, bytes.NewReader(content)); err != nil {
		return v1.Descriptor{}, err
	}
	return desc, nil
}
