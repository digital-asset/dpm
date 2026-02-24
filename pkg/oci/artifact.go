// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package oci

type Artifact interface {
	RepoName() string
	ArtifactType() string
	FileMediaType() string
}

type Dar interface {
	DarRepoName() string
}

type ComponentArtifact struct {
	ComponentName string
}

type DarArtifact struct {
	DarName string
}

func (a *ComponentArtifact) RepoName() string {
	return ComponentRepoPrefix + a.ComponentName
}

func (a *DarArtifact) RepoName() string {
	return DarRepoPrefix + a.DarName
}

func (a *ComponentArtifact) ArtifactType() string  { return ComponentArtifactType }
func (a *ComponentArtifact) FileMediaType() string { return ComponentFileMediaType }

func (a *DarArtifact) ArtifactType() string  { return DarArtifactType }
func (a *DarArtifact) FileMediaType() string { return DarFileMediaType }

type SdkManifestArtifact struct {
	SdkManifestsRepo string
}

func (a *SdkManifestArtifact) RepoName() string {
	return a.SdkManifestsRepo
}
func (a *SdkManifestArtifact) ArtifactType() string  { return AssemblyArtifactType }
func (a *SdkManifestArtifact) FileMediaType() string { return AssemblyFileMediaType }
