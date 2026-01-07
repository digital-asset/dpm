// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package versions

import (
	"daml.com/x/assistant/cmd/dpm/cmd/resolve/resolutionerrors"
	"daml.com/x/assistant/pkg/damlpackage"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/builtincommand"
	"daml.com/x/assistant/pkg/ocilister"
	"daml.com/x/assistant/pkg/versions"
	"github.com/Masterminds/semver/v3"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

var ErrNoActiveSdk = fmt.Errorf("no SDK version is active")

func Cmd(config *assistantconfig.Config) *cobra.Command {
	var all, activeOnly bool
	var output string

	cmd := &cobra.Command{
		Use:   string(builtincommand.Version),
		Short: "show sdk versions",
		Long: `show sdk versions

	note that the active version in the output may or may not be installed.
`,
		Aliases: []string{string(builtincommand.Versions)},
		RunE: func(cmd *cobra.Command, args []string) error {
			var activeVersion *semver.Version
			installedVersions := []*semver.Version{}
			remoteVersions := []*semver.Version{}

			// get remote versions if applicable
			if all && !activeOnly {
				client, err := assistantremote.NewFromConfig(config)
				if err != nil {
					return err
				}

				edition, err := config.Edition.Get()
				if err != nil {
					return err
				}

				remoteVersions, err = ocilister.ListSDKVersions(cmd.Context(), edition, client)
				if err != nil {
					return err
				}
			}

			// get installed versions
			installedSDKs, err := assistantconfig.GetInstalledSDKsForEdition(config)
			if err == nil {
				v, err := getActiveVersion(config)
				if err != nil {
					return fmt.Errorf("error determining or parsing active SDK version, %w", err)
				}
				activeVersion = v

				installedVersions = lo.Map(installedSDKs, func(v *assistantconfig.InstalledSdkVersion, _ int) *semver.Version {
					return v.Version
				})
			}

			if activeOnly {
				if activeVersion == nil {
					return ErrNoActiveSdk
				} else {
					cmd.Println(activeVersion.String())
				}
				return nil
			}

			// assemble versions information
			v := versions.New(activeVersion, installedVersions, remoteVersions)

			// output
			switch output {
			case "table":
				cmd.Println(v.Table())
			case "json":
				data, err := json.MarshalIndent(v, "", "    ")
				if err != nil {
					return err
				}

				cmd.Println(string(data))
			default:
				return fmt.Errorf("output format not supported: %s", output)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&activeOnly, "active", "s", false, "display the active sdk version only")
	cmd.Flags().BoolVarP(&all, "all", "A", false, "display remote versions")
	cmd.Flags().StringVarP(&output, "output", "o", "table", "output format: json, table")
	return cmd
}

/*
	getActiveVersion

returns nil
- when sdk-version is null or "" in daml.yaml
- or when DPM_SDK_VERSION=""
- or when outside daml package context and there aren't any sdks installed
*/
func getActiveVersion(config *assistantconfig.Config) (*semver.Version, error) {
	// DPM_SDK_VERSION override
	versionOverride, ok := os.LookupEnv(assistantconfig.DpmSdkVersionEnvVar)
	if ok {
		if versionOverride == "" {
			return nil, nil
		}
		return semver.NewVersion(versionOverride)
	}

	// daml.yaml
	damlPackagePath, _, err := assistantconfig.GetDamlPackageAbsolutePath()
	if err != nil {
		return nil, err
	}
	if damlPackagePath != "" {
		damlPackage, err := damlpackage.Read(damlPackagePath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, resolutionerrors.NewDamlYamlNotFoundError(err)
			}
			return nil, resolutionerrors.NewMalformedDamlYamlError(err)
		}

		if damlPackage.SdkVersion == "" {
			return nil, nil
		}
		return semver.NewVersion(damlPackage.SdkVersion)
	}

	s, err := assistantconfig.GetInstalledSdkFromEnvOrDefault(config)
	if errors.Is(err, assistantconfig.ErrNoSdkInstalled) || errors.Is(err, assistantconfig.ErrTargetSdkNotInstalled) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return s.Version, nil
}
