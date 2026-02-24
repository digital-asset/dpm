// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package oci

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
)

const (
	ComponentArtifactType  = "application/vnd.component.artifact"
	ComponentFileMediaType = "application/vnd.component.file"
	AssemblyArtifactType   = "application/vnd.assembly.artifact"
	AssemblyFileMediaType  = "application/vnd.assembly.file"
	ComponentRepoPrefix    = "components/"

	DarArtifactType  = "application/vnd.dar.artifact"
	DarFileMediaType = "application/vnd.dar.file"
	DarRepoPrefix    = "dar/"

	sdkManifestsRepoPrefix     = "sdk-manifests/"
	SdkManifestsOpenSourceRepo = sdkManifestsRepoPrefix + "open-source"
	SdkManifestsEnterpriseRepo = sdkManifestsRepoPrefix + "enterprise"
	SdkManifestsPrivateRepo    = sdkManifestsRepoPrefix + "private"

	DAAnnotationPrefix          = "com.digitalasset."
	DescriptorNameAnnotation    = DAAnnotationPrefix + "name"
	DescriptorVersionAnnotation = DAAnnotationPrefix + "version"
)

// DescriptorAnnotations are required annotations to be appended onto image and index manifests.
// These will facilitate resolving "latest" floaty tags to corresponding component or assembly semver
type DescriptorAnnotations struct {
	Name    string
	Version *semver.Version
}

func (d DescriptorAnnotations) AppendToMap(annotations map[string]string) {
	annotations[DescriptorNameAnnotation] = d.Name
	annotations[DescriptorVersionAnnotation] = d.Version.String()
}

func DAAnnotation(annotation string) string {
	return DAAnnotationPrefix + annotation
}

func VersionFromDescriptorAnnotations(descriptorAnnotations map[string]string) (*semver.Version, error) {
	err := fmt.Errorf("descriptor missing required %q annotations", DescriptorVersionAnnotation)
	if descriptorAnnotations == nil {
		return nil, err
	}
	version, ok := descriptorAnnotations[DescriptorVersionAnnotation]
	if !ok {
		return nil, err
	}

	return semver.NewVersion(version)
}
