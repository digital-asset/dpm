// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package promote

import (
	"context"
	"fmt"
	"strings"

	"daml.com/x/assistant/pkg/assembler"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	ociconsts "daml.com/x/assistant/pkg/oci"
	"daml.com/x/assistant/pkg/ociindex"
	"daml.com/x/assistant/pkg/sdkbundle"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/simpleplatform"
	"daml.com/x/assistant/pkg/utils"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"oras.land/oras-go/v2"
)

func Cmd() *cobra.Command {
	var sourceRegistry, destinationRegistry, registryAuth string
	var insecure bool
	var publishConfigPath string
	var blobCache string

	cmd := &cobra.Command{
		Use:     "promote-components",
		Short:   "re-publish components from one OCI registry (public-unstable) to another (public)",
		Example: "  dpm repo promote-components --source-registry=gar.io/public-unstable --destination-registry=gar.io/public -f publish.yaml",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			sourceRegistry = strings.TrimRight(sourceRegistry, "/")
			destinationRegistry = strings.TrimRight(destinationRegistry, "/")

			if sourceRegistry == "" {
				return fmt.Errorf("source registry can't be the empty string")
			}
			if destinationRegistry == "" {
				return fmt.Errorf("destination registry can't be the empty string")
			}
			sourceClient, err := assistantremote.New(sourceRegistry, registryAuth, insecure)
			if err != nil {
				return err
			}

			destinationClient, err := assistantremote.New(destinationRegistry, registryAuth, insecure)
			if err != nil {
				return err
			}

			publishConfig, err := sdkbundle.ReadPublishConfig(publishConfigPath)
			if err != nil {
				return err
			}

			if err := promote(cmd.Context(), sourceClient, destinationClient, publishConfig, blobCache); err != nil {
				return err
			}

			fmt.Println("successfully promoted all components")
			return nil
		},
	}

	cmd.Flags().StringVar(&sourceRegistry, "source-registry", "", "source OCI registry to pull components from")
	cmd.MarkFlagRequired("source-registry")
	cmd.Flags().StringVar(&destinationRegistry, "destination-registry", "", "destination OCI registry to publish components to")
	cmd.MarkFlagRequired("destination-registry")
	cmd.Flags().BoolVar(&insecure, "insecure", false, "use http instead of https for OCI registry")
	cmd.Flags().StringVar(&registryAuth, "auth", "", "path to a config file similar to docker’s config.json to use for authenticating to the OCI registry. Defaults to docker's config.json")
	cmd.Flags().StringVarP(&publishConfigPath, "config-file", "f", "", `REQUIRED config file path"`)
	cmd.MarkFlagRequired("config-file")
	cmd.Flags().StringVar(&blobCache, "oci-cache", "", "use an oci-cache to speed up pulls")

	return cmd
}

// TODO re-publish components' extra-tags too
func promote(ctx context.Context, sourceClient, destinationClient *assistantremote.Remote, publishConfig *sdkbundle.PublishConfig, blobCache string) error {
	if blobCache == "" {
		tmp, deleteFn, err := utils.MkdirTemp("", "")
		if err != nil {
			return err
		}
		defer func() { _ = deleteFn() }()
		blobCache = tmp
	}
	if err := utils.EnsureDirs(blobCache); err != nil {
		return err
	}

	components := []*sdkmanifest.Component{
		publishConfig.Assistant.Component,
	}
	components = append(components, lo.Values(publishConfig.PlatformlessComponents())...)

	for _, comp := range components {
		repoName := ociconsts.ComponentRepoPrefix + comp.Name
		tag := assembler.ComputeTagOrDigest(comp)
		if err := copyComponent(ctx, sourceClient, destinationClient, repoName, tag, blobCache); err != nil {
			return err
		}
	}
	return nil
}

func copyComponent(ctx context.Context, sourceClient, destinationClient *assistantremote.Remote, repoName, tag string, blobCache string) error {
	index, indexBytes, err := ociindex.FetchIndex(ctx, sourceClient, repoName, tag)
	if err != nil {
		return err
	}

	dest, err := destinationClient.Repo(repoName)
	if err != nil {
		return err
	}

	repo, err := sourceClient.CachedRepo(repoName, blobCache)
	if err != nil {
		return err
	}

	for _, descriptor := range index.Manifests {
		platform := simpleplatform.FromOras(descriptor.Platform)
		platformTag := platform.ImageTag(tag)
		fmt.Printf("promoting %s:%s...\n", repoName, platformTag)
		_, err = oras.Copy(ctx, repo, descriptor.Digest.String(), dest, platformTag, oras.DefaultCopyOptions)
		if err != nil {
			return err
		}
	}

	fmt.Printf("promoting %s:%s index...\n", repoName, tag)
	_, err = oras.TagBytes(ctx, dest, v1.MediaTypeImageIndex, indexBytes, tag)
	return err
}
