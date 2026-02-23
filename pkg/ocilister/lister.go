// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ocilister

import (
	"context"
	"errors"

	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	ociconsts "daml.com/x/assistant/pkg/oci"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"github.com/Masterminds/semver/v3"
	"oras.land/oras-go/v2/registry/remote/errcode"
)

// TODO this file should be a Lister interface, implemented by be methods on assistantremote.Remote

func ListTags(ctx context.Context, client *assistantremote.Remote, repoName string) ([]string, bool, error) {
	var result []string

	repo, err := client.Repo(repoName)
	if err != nil {
		return nil, false, err
	}

	err = repo.Tags(ctx, "", func(tags []string) error {
		result = append(result, tags...)
		return nil
	})
	if isErrorCode(err, errcode.ErrorCodeNameUnknown) {
		// repo doesn't even exist...
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}
	return result, true, nil
}

func ListComponentVersions(ctx context.Context, name string, client *assistantremote.Remote) (map[*semver.Version][]string, error) {
	return listSemverTags(ctx, ociconsts.ComponentRepoPrefix+name, client)
}

func ListSDKVersions(ctx context.Context, edition sdkmanifest.Edition, client *assistantremote.Remote) (map[*semver.Version][]string, error) {
	repo, err := edition.SdkManifestsRepo()
	if err != nil {
		return nil, err
	}
	return listSemverTags(ctx, repo, client)
}

func listSemverTags(ctx context.Context, repoName string, client *assistantremote.Remote) (map[*semver.Version][]string, error) {
	repo, err := client.Repo(repoName)
	if err != nil {
		return nil, err
	}
	
	repo.Manifests()

	//results := map[*semver.Version][]string{}
	nonFloatyToFloaty := map[*semver.Version][]string{}
	tags, found, err := ListTags(ctx, client, repoName)
	if err != nil {
		return nil, err
	}

	//if !found {
	//	return results, nil
	//}
	if !found {
		return nonFloatyToFloaty, nil
	}

	//floaty, NonFloaty := lo.FilterReject(tags, func(t string, _ int) bool {
	//	return isFloaty(t)
	//})

	var floaty, nonFloaty map[string]string

	digestToTags := map[string][]string{}
	for _, tag := range tags {
		desc, err := repo.Resolve(ctx, tag)
		if err != nil {
			return nil, err
		}
		digestToTags[desc.Digest.String()] =
			append(digestToTags[desc.Digest.String()], tag)

		if isFloaty(tag) {
			floaty[tag] = desc.Digest.String()
		} else {
			nonFloaty[tag] = desc.Digest.String()
		}
	}

	for nonFloatyTag, digest := range nonFloaty {
		v, _ := semver.NewVersion(nonFloatyTag)
		nonFloatyToFloaty[v] = append(nonFloatyToFloaty[v], digestToTags[digest]...)
	}

	//result := lo.FilterMap(tags, func(t string, _ int) (*semver.Version, bool) {
	//	v, err := semver.NewVersion(t)
	//	if err != nil {
	//		return nil, false
	//	}
	//	return v, true
	//})
	//
	//slices.SortFunc(result, func(a, b *semver.Version) int {
	//	return a.Compare(b)
	//})
	//
	//return result, nil

	return nonFloatyToFloaty, nil
}

func isFloaty(tag string) bool {
	v, err := semver.NewVersion(tag)
	if err != nil {
		return true
	}
	return v.String() == tag
}

func Cmp(a, b *semver.Version) int {
	return a.Compare(b)
}

// IsErrorCode returns true if err is an oras Error and its Code equals to code.
func isErrorCode(err error, code string) bool {
	var ec errcode.Error
	return errors.As(err, &ec) && ec.Code == code
}
