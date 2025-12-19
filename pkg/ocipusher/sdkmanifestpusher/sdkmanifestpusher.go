// Copyright (c) 2017-2025 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sdkmanifestpusher

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/oci"
	"daml.com/x/assistant/pkg/ociindex"
	"daml.com/x/assistant/pkg/ocilister"
	"daml.com/x/assistant/pkg/ocipusher"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/simpleplatform"
	"daml.com/x/assistant/pkg/utils"
	"github.com/Masterminds/semver/v3"
	"github.com/fatih/color"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/samber/lo"
)

type AssemblyPusher struct {
	printer utils.RawPrinter
	config  *PushArgs
}

type PushArgs struct {
	Edition     sdkmanifest.Edition
	Version     *semver.Version
	Annotations map[string]string
	ExtraTags   []string
}

func New(printer utils.RawPrinter, config *PushArgs) *AssemblyPusher {
	return &AssemblyPusher{
		config:  config,
		printer: printer,
	}
}

func (p *AssemblyPusher) PushSdkManifest(ctx context.Context, client *assistantremote.Remote, sdkManifests map[simpleplatform.NonGeneric]string) (*v1.Descriptor, error) {
	existingVersions, err := ocilister.ListSDKVersions(ctx, p.config.Edition, client)
	if err != nil {
		return nil, err
	}

	alreadyExists := lo.Contains(lo.Map(existingVersions, func(v *semver.Version, _ int) string {
		return v.String()
	}), p.config.Version.String())

	if alreadyExists {
		return nil, fmt.Errorf("sdk version %s already exists in remote registry. refusing to push", p.config.Version.String())
	}

	repo, err := p.config.Edition.SdkManifestsRepo()
	if err != nil {
		return nil, err
	}

	var pushOps []*ocipusher.PushOperation

	var deleteFns []func() error
	closeFn := func() error {
		lo.ForEach(deleteFns, func(close func() error, _ int) {
			_ = close()
		})
		return nil
	}
	defer func() { _ = closeFn() }()

	for platform, manifestPath := range sdkManifests {
		pushOp, deleteFn, err := p.prepare(ctx, repo, manifestPath, &platform)
		deleteFns = append(deleteFns, deleteFn)
		if err != nil {
			return nil, err
		}
		pushOps = append(pushOps, pushOp)
	}

	var descriptors []v1.Descriptor
	for _, pushOp := range pushOps {
		desc, err := p.push(ctx, client, pushOp)
		if err != nil {
			return nil, err
		}
		switch p := pushOp.Platform().(type) {
		case *simpleplatform.NonGeneric:
			desc.Platform = p.ToOras()
		default:
			return nil, fmt.Errorf("platform type %t not supported for assembly OCI index", p)
		}
		descriptors = append(descriptors, *desc)
	}

	coloredDest := color.GreenString(fmt.Sprintf("%s/%s", repo, p.config.Version.String()))
	p.printer.Println("📖 Pushing index " + coloredDest)
	tag := p.config.Version.String()

	indexDesc, err := ociindex.PushIndex(ctx, client, p.config.indexOpts(repo, tag, descriptors))
	if err != nil {
		return nil, err
	}
	descriptorJson, err := json.MarshalIndent(indexDesc, "", "  ")
	if err != nil {
		return nil, err
	}
	p.printer.Printf("\n%s\n", string(descriptorJson))
	p.printer.Println("successfully published index " + coloredDest)

	if p.config.ExtraTags != nil && len(p.config.ExtraTags) > 0 {
		p.printer.Println("pushing extra tags...")
		err := ociindex.Tag(ctx, client, &oci.SdkManifestArtifact{SdkManifestsRepo: repo}, p.config.Version, p.config.ExtraTags)
		if err != nil {
			return nil, err
		}
	}

	return indexDesc, nil
}

func (p *AssemblyPusher) prepare(ctx context.Context, repoName string, pathToAssembly string, platform *simpleplatform.NonGeneric) (*ocipusher.PushOperation, func() error, error) {
	dir, deleteFn, err := utils.MkdirTemp("", "")
	if err != nil {
		return nil, deleteFn, err
	}
	//defer func() { _ = deleteFn() }()
	filename := p.config.Version.String() + ".yaml"
	if err := utils.CopyFile(pathToAssembly, filepath.Join(dir, filename)); err != nil {
		return nil, deleteFn, err
	}

	opts := ocipusher.Opts{
		Artifact: &oci.SdkManifestArtifact{
			SdkManifestsRepo: repoName,
		},
		RawTag:              p.config.Version.String(),
		Dir:                 dir,
		RequiredAnnotations: p.config.RequiredAnnotations(),
		ExtraAnnotations:    p.config.Annotations,
		Platform:            platform,
	}

	pushOp, err := ocipusher.New(ctx, opts)
	if err != nil {
		return nil, deleteFn, err
	}
	return pushOp, deleteFn, err
}

func (p *AssemblyPusher) push(ctx context.Context, client *assistantremote.Remote, pushOp *ocipusher.PushOperation) (*v1.Descriptor, error) {
	coloredDest := color.GreenString(pushOp.Destination(client.Registry))

	p.printer.Printf("Pushing %q...\n", coloredDest)
	descriptor, err := pushOp.Do(ctx, client)
	if err != nil {
		return nil, err
	}
	descriptorJson, err := json.MarshalIndent(descriptor, "", "  ")
	if err != nil {
		return nil, err
	}
	p.printer.Printf("\n%s\n", string(descriptorJson))
	p.printer.Println("successfully published " + coloredDest)
	return descriptor, nil
}

func (config *PushArgs) indexOpts(repo, tag string, descriptors []v1.Descriptor) ociindex.Opts {
	return ociindex.Opts{
		Artifact:            &oci.SdkManifestArtifact{SdkManifestsRepo: repo},
		Tag:                 tag,
		Manifests:           descriptors,
		ExtraAnnotations:    config.Annotations,
		RequiredAnnotations: config.RequiredAnnotations(),
	}
}

func (config *PushArgs) RequiredAnnotations() oci.DescriptorAnnotations {
	return oci.DescriptorAnnotations{
		Name:    "assembly",
		Version: config.Version,
	}
}
