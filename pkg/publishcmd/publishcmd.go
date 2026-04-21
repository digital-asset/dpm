// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package publishcmd

import (
	"path/filepath"

	"daml.com/x/assistant/pkg/simpleplatform"
)

const PlatformFlagName = "platform"
const FileFlagName = "file"
const RegistryFlagName = "registry"
const ComponentNameFlagName = "name"
const DarNameFlagName = "name"
const VersionFlagName = "version"

type PublishCmd struct {
	DryRun, IncludeGitInfo bool
	Name                   string
	Version                string
	Annotations            map[string]string
	Platforms              map[string]string
	ExtraTags              []string

	Insecure     bool
	Registry     string
	RegistryAuth string
}

type PublishDarCmd struct {
	DryRun, IncludeGitInfo bool
	Name                   string
	Version                string
	Annotations            map[string]string
	File                   string
	ExtraTags              []string

	Insecure     bool
	Registry     string
	RegistryAuth string
}

func (c *PublishCmd) ParsePlatforms() (map[simpleplatform.Platform]string, error) {
	parsed := map[simpleplatform.Platform]string{}
	for platformStr, dir := range c.Platforms {
		p, err := simpleplatform.ParsePlatform(platformStr)
		if err != nil {
			return nil, err
		}

		d, err := filepath.Abs(dir)
		if err != nil {
			return nil, err
		}

		parsed[p] = d
	}

	return parsed, nil
}
