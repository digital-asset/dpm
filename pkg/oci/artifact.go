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

type FirstPartyComponentArtifact struct {
	ComponentName string
}

type ComponentArtifact struct {
	ComponentRepo string
}

type DarArtifact struct {
	DarRepo string
}

func (a *ComponentArtifact) RepoName() string {
	return a.ComponentRepo
}
func (a *ComponentArtifact) ArtifactType() string  { return ComponentArtifactType }
func (a *ComponentArtifact) FileMediaType() string { return ComponentFileMediaType }

func (a *FirstPartyComponentArtifact) RepoName() string {
	return ComponentRepoPrefix + a.ComponentName
}
func (a *FirstPartyComponentArtifact) ArtifactType() string  { return ComponentArtifactType }
func (a *FirstPartyComponentArtifact) FileMediaType() string { return ComponentFileMediaType }

func (a *DarArtifact) RepoName() string {
	return a.DarRepo
}
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

type GenericArtifact struct {
	ArtifactName string
}

func (a *GenericArtifact) RepoName() string {
	return a.ArtifactName
}
func (a *GenericArtifact) ArtifactType() string  { return AssemblyArtifactType }
func (a *GenericArtifact) FileMediaType() string { return AssemblyFileMediaType }
