// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package oci

import (
	"fmt"

	"daml.com/x/assistant/pkg/utils"
	"github.com/Masterminds/semver/v3"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	ComponentArtifactType  = "application/vnd.component.artifact"
	ComponentFileMediaType = "application/vnd.component.file"
	AssemblyArtifactType   = "application/vnd.assembly.artifact"
	AssemblyFileMediaType  = "application/vnd.assembly.file"
	ComponentRepoPrefix    = "components/"

	DarArtifactType  = "application/vnd.dar.artifact"
	DarFileMediaType = "application/vnd.dar.file"
	DarRepoPrefix    = "dars/"

	sdkManifestsRepoPrefix     = "sdk-manifests/"
	SdkManifestsOpenSourceRepo = sdkManifestsRepoPrefix + "open-source"
	SdkManifestsEnterpriseRepo = sdkManifestsRepoPrefix + "enterprise"
	SdkManifestsPrivateRepo    = sdkManifestsRepoPrefix + "private"

	// deprecated annotations
	LegacyDpmAnnotationPrefix = "com.digitalasset."
	LegacyNameAnnotation      = LegacyDpmAnnotationPrefix + "name"
	LegacyVersionAnnotation   = LegacyDpmAnnotationPrefix + "version"

	DpmAnnotationPrefix      = "com.dpm."
	DescriptorNameAnnotation = DpmAnnotationPrefix + "name"
)

// DescriptorAnnotations are required annotations to be appended onto image and index manifests.
// These will facilitate resolving "latest" floaty tags to corresponding component or assembly semver
type DescriptorAnnotations struct {
	Name    string
	Version *semver.Version
}

func (d DescriptorAnnotations) AppendToMap(annotations map[string]string) {
	annotations[DescriptorNameAnnotation] = d.Name
	annotations[v1.AnnotationVersion] = d.Version.String()

	// deprecated but keeping for backwards compatibility with older dpm binaries
	annotations[LegacyNameAnnotation] = d.Name
	annotations[LegacyVersionAnnotation] = d.Version.String()
}

func LegacyDpmAnnotation(annotation string) string {
	return LegacyDpmAnnotationPrefix + annotation
}

func DpmAnnotation(annotation string) string {
	return DpmAnnotationPrefix + annotation
}

func VersionFromDescriptorAnnotations(descriptorAnnotations map[string]string) (*semver.Version, error) {
	err := fmt.Errorf("descriptor missing required (%q, or %q) annotations", v1.AnnotationVersion, LegacyVersionAnnotation)
	if descriptorAnnotations == nil {
		return nil, err
	}

	version, ok := utils.GetWithFallback(descriptorAnnotations, v1.AnnotationVersion, LegacyVersionAnnotation)
	if !ok {
		return nil, err
	}

	return semver.NewVersion(version)
}
