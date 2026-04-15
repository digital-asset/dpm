package publish

import (
	"fmt"

	ociconsts "daml.com/x/assistant/pkg/oci"
)

//
//type Destination interface {
//	GetRegistry() string
//	Artifact() ociconsts.Artifact
//	String() string
//}
//
//type ThirdPartyDestination struct {
//	reference *registry.Reference
//}
//
//func (t ThirdPartyDestination) IsThirdParty() bool {
//	return true
//}
//
//func (t ThirdPartyDestination) GetRef() *registry.Reference {
//	return t.reference
//}
//
//func (t ThirdPartyDestination) Artifact() ociconsts.Artifact {
//	return &ociconsts.ThirdPartyComponentArtifact{
//		ComponentRepo: t.reference.Repository,
//	}
//}
//
//type FirstPartyDestination struct {
//	reference *registry.Reference
//}
//
//func (f FirstPartyDestination) IsThirdParty() bool {
//	return false
//}
//
//func (f FirstPartyDestination) GetRef() *registry.Reference {
//	return f.reference
//}
//
//func (f FirstPartyDestination) Artifact() ociconsts.Artifact {
//	return &ociconsts.ComponentArtifact{
//		ComponentName:
//	}
//}
//
//var _ Destination = (*FirstPartyDestination)(nil)
//var _ Destination = (*ThirdPartyDestination)(nil)

type Destination struct {
	Registry string
	Artifact ociconsts.Artifact
}

func (d *Destination) String() string {
	return fmt.Sprintf("%s/%s", d.Registry, d.Artifact.RepoName())
}
