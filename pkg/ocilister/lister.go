// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ocilister

import (
	"context"
	"errors"
	"log/slog"
	"regexp"

	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	ociconsts "daml.com/x/assistant/pkg/oci"
	"daml.com/x/assistant/pkg/ociindex"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"github.com/Masterminds/semver/v3"
	"github.com/samber/lo"
	"oras.land/oras-go/v2/registry/remote/errcode"
)

var platformTagRegex = regexp.MustCompile(
	`^\d+\.\d+\.\d+.*\.(?:generic|[^._]+_[^._]+)$`,
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
	return listTags(ctx, ociconsts.ComponentRepoPrefix+name, client)
}

func ListSDKVersions(ctx context.Context, edition sdkmanifest.Edition, client *assistantremote.Remote) (map[*semver.Version][]string, error) {
	repo, err := edition.SdkManifestsRepo()
	if err != nil {
		return nil, err
	}
	return listTags(ctx, repo, client)
}

func listTags(ctx context.Context, repoName string, client *assistantremote.Remote) (map[*semver.Version][]string, error) {
	tags, found, err := ListTags(ctx, client, repoName)
	if err != nil {
		return nil, err
	}

	if !found {
		return map[*semver.Version][]string{}, nil
	}

	nonFloaty := map[string][]string{}
	for _, tag := range tags {
		if IsPlatformTag(tag) {
			// skip 1.2.3.<platform>
			continue
		}

		if IsFloaty(tag) {
			version, err := ociindex.ResolveTag(ctx, client, &ociconsts.SdkManifestArtifact{repoName}, tag)
			if err != nil {
				slog.Warn("failed to resolve floaty tag to semver",
					slog.String("repo", repoName),
					slog.String("tag", tag),
					slog.Any("err", err),
				)
				continue
			}
			nonFloaty[version.String()] = append(nonFloaty[version.String()], tag)
		} else if _, ok := nonFloaty[tag]; !ok {
			nonFloaty[tag] = []string{}
		}
	}

	return lo.MapKeys(nonFloaty, func(_ []string, tag string) *semver.Version {
		v, _ := semver.NewVersion(tag)
		return v
	}), nil
}

func IsFloaty(tag string) bool {
	_, err := semver.StrictNewVersion(tag)
	return err != nil
}

func IsPlatformTag(tag string) bool {
	return platformTagRegex.MatchString(tag)
}

func Cmp(a, b *semver.Version) int {
	return a.Compare(b)
}

// IsErrorCode returns true if err is an oras Error and its Code equals to code.
func isErrorCode(err error, code string) bool {
	var ec errcode.Error
	return errors.As(err, &ec) && ec.Code == code
}
