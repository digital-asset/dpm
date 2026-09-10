// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"daml.com/x/assistant/pkg/assembler"
	"daml.com/x/assistant/pkg/assembler/assemblyplan"
	"daml.com/x/assistant/pkg/multipackage"
	"daml.com/x/assistant/pkg/ocilister"
	"daml.com/x/assistant/pkg/ocipuller/remotepuller"
	"daml.com/x/assistant/pkg/utils"
	"daml.com/x/assistant/pkg/yamledit"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/samber/lo"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/gitparse"
	"daml.com/x/assistant/pkg/gitpuller"
	"daml.com/x/assistant/pkg/sdkinstall"
	"github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"
)

func Cmd(config *assistantconfig.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "package",
		Short:  "install the SDK and all opt-in components (if any) for the current project",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return InstallPackage(config, cmd)
		},
	}
	return cmd
}

func InstallPackage(config *assistantconfig.Config, cmd *cobra.Command) error {
	ctx := cmd.Context()

	cmd.SilenceUsage = true

	modifiedConfig := config
	modifiedConfig.AutoInstall = true
	multiPackagePath, hasMultiPackage, err := assistantconfig.GetMultiPackageAbsolutePath()
	if err != nil {
		return err
	}
	if hasMultiPackage {
		multiDamlPackage, err := multipackage.Read(multiPackagePath)
		if err != nil {
			return err
		}

		if multiDamlPackage.SdkVersion != "" {
			sdkVersion, err := semver.NewVersion(multiDamlPackage.SdkVersion)
			if err != nil {
				return err
			}
			if err := installSdk(ctx, cmd, config, sdkVersion); err != nil {
				return err
			}
		}

		if err := installMultiPackageYamlComponentsOnly(ctx, cmd, config); err != nil {
			return err
		}
		pkgs := multiDamlPackage.AbsolutePackages()

		for _, p := range pkgs {
			cmd.Printf("Processing package %q...\n", p)
			damlPackagePath := filepath.Join(p, assistantconfig.DamlPackageFilename)
			if err := processDamlPackage(ctx, cmd, modifiedConfig, damlPackagePath); err != nil {
				return err
			}
			if err := installOverridesForPackage(ctx, cmd, config, damlPackagePath); err != nil {
				return err
			}
		}

	} else {
		damlPackagePath, isDamlPackage, err := assistantconfig.GetDamlPackageAbsolutePath()
		if err != nil {
			return err
		}
		if !isDamlPackage {
			return fmt.Errorf("not in a package directory or subdirectory")
		}
		if err := processDamlPackage(ctx, cmd, modifiedConfig, damlPackagePath); err != nil {
			return err
		}
		return installOverridesForPackage(ctx, cmd, config, damlPackagePath)
	}

	return nil
}

func processDamlPackage(ctx context.Context, cmd *cobra.Command, config *assistantconfig.Config, damlPath string) error {
	damlPackage, err := damlpackage.Read(damlPath)
	if err != nil {
		return err
	}
	if damlPackage.SdkVersion != "" {
		sdkVersion, err := semver.NewVersion(damlPackage.SdkVersion)
		if err != nil {
			return err
		}
		if err := installSdk(ctx, cmd, config, sdkVersion); err != nil {
			return err
		}
	}

	damlPackage, fetched, err := gitpuller.PrepareGitDependencies(ctx, config, damlPath)
	if err != nil {
		return err
	}
	gitpuller.ReportPreparedGitDependencies(config, damlPackage, fetched, false)

	for _, field := range []string{"dependencies", "data-dependencies"} {
		raw, parsed := damlPackage.RawAndParsed(field)
		yamlTarget := yamledit.YamlTarget{
			YamlFilePath: damlPath,
			FieldName:    field,
		}
		if err := installDars(ctx, config, lo.Values(parsed), raw, yamlTarget); err != nil {
			return err
		}
	}

	return nil
}

func installDars(ctx context.Context, config *assistantconfig.Config, dars []*damlpackage.ParsedDarDependency, rawDars []*damlpackage.RawDependency, yamlTarget yamledit.YamlTarget) error {
	for _, d := range dars {
		updatedDar, version, err := InstallDar(ctx, config, d)
		if err != nil {
			return err
		}

		if updatedDar != nil {
			var uri string
			if updatedDar.Scheme() == "git" {
				uri = gitparse.FormatGitYamlLine(updatedDar.Git)
			} else {
				uri = updatedDar.StringWithAlias()
			}
			item := fmt.Sprintf("%q", uri)
			if updatedDar.Index >= 0 && updatedDar.Index < len(rawDars) {
				item, err = damlpackage.MarshalDependencyWithValue(rawDars[updatedDar.Index], uri)
				if err != nil {
					return err
				}
			}
			target := yamlTarget.Copy()
			target.Index = updatedDar.Index
			if updatedDar.Scheme() != "git" {
				target.LineComment = version
			}
			if err := yamledit.EditYaml(target, item); err != nil {
				return err
			}
		}
	}
	return nil
}

func InstallDar(ctx context.Context, config *assistantconfig.Config, dar *damlpackage.ParsedDarDependency) (updatedDar *damlpackage.ParsedDarDependency, version string, err error) {
	if dar.Scheme() == "git" {
		updatedDar, err = installGitDar(ctx, config, dar)
		return updatedDar, "", err
	}
	if dar.FullUrl == nil || dar.FullUrl.Scheme != "oci" {
		return nil, "", nil
	}
	fmt.Printf("installing dar %q...\n", dar.FullUrl.String())

	client, ref, err := dar.GetOciRemote()
	if err != nil {
		return nil, "", err
	}

	if !assistantconfig.ShaPinningEnabled() && ocilister.IsFloaty(ref.Reference) {
		return nil, "", fmt.Errorf("tag not allowed in %q: only strict semver OCI tags are supported currently", dar.FullUrl.String())
	}

	if assistantconfig.ShaPinningEnabled() && !strings.Contains(dar.FullUrl.String(), "@sha256:") {
		resolvedDigest, manifest, err := ocilister.FetchManifest(ctx, client, *ref)
		if err != nil {
			return nil, "", err
		}

		newUrl, err := url.Parse(dar.FullUrl.String() + "@" + resolvedDigest.String())
		if err != nil {
			return nil, "", err
		}
		updatedDar = &damlpackage.ParsedDarDependency{
			FullUrl:       newUrl,
			Location:      dar.Location,
			MainPackageId: dar.MainPackageId,
			Index:         dar.Index,
		}

		version = manifest.Annotations[v1.AnnotationVersion]

		client, ref, err = updatedDar.GetOciRemote()
		if err != nil {
			return nil, "", err
		}
	}

	puller := remotepuller.New(config.OciLayoutCache, client)
	darDir := config.CachePathForDar(ref)

	ok, err := utils.DirExists(darDir)
	if err != nil {
		return nil, "", err
	}
	if ok {
		fmt.Println("Dar already installed.")
		return updatedDar, version, nil
	}
	if _, err = puller.PullDarByFullPath(ctx, ref.Repository, ref.Reference, darDir); err != nil {
		return nil, version, err
	}

	return updatedDar, version, nil
}

func installGitDar(ctx context.Context, config *assistantconfig.Config, dar *damlpackage.ParsedDarDependency) (*damlpackage.ParsedDarDependency, error) {
	if dar.Git.Release {
		if gitpuller.DarIsCached(config, dar) {
			return nil, nil
		}
		fmt.Printf("installing git release asset %q...\n", dar.Git.DarPath)
	} else {
		fmt.Printf("installing git dar %q...\n", gitparse.FormatGitYamlLine(dar.Git))
		if gitpuller.DarIsCached(config, dar) {
			return nil, nil
		}
	}

	result, err := gitpuller.PullGitDar(ctx, config, dar)
	if err != nil {
		return nil, err
	}

	if result.Pinned != nil {
		return result.Pinned, nil
	}
	return nil, nil
}

func installMultiPackageYamlComponentsOnly(ctx context.Context, cmd *cobra.Command, config *assistantconfig.Config) error {
	puller, err := remotepuller.NewFromRemoteConfig(config)
	if err != nil {
		return err
	}
	a := assembler.New(config, puller)
	assemblyPlan, err := assemblyplan.New(ctx, config, a)
	if err != nil {
		return err
	}
	assemblyPlan.DamlPackage = nil
	if !assemblyPlan.HasOverrides() {
		return nil
	}
	cmd.Println("Installing multi-package.yaml components...")
	return utils.WithInstallLock(ctx, config.InstallLocalFilePath, func() error {
		_, err := assemblyPlan.Assemble(ctx)
		return err
	})
}

func installOverridesForPackage(ctx context.Context, cmd *cobra.Command, config *assistantconfig.Config, absPath string) error {
	puller, err := remotepuller.NewFromRemoteConfig(config)
	if err != nil {
		return err
	}
	a := assembler.New(config, puller)
	assemblyPlan, err := assemblyplan.NewShallow(ctx, config, a, absPath)
	if err != nil {
		return err
	}
	assemblyPlan.MultiPackage = nil
	if !assemblyPlan.HasOverrides() {
		cmd.Println("No opt-in components to install")
		return nil
	}
	cmd.Println("Installing components...")
	err = utils.WithInstallLock(ctx, config.InstallLocalFilePath, func() error {
		_, err := assemblyPlan.Assemble(ctx)
		return err
	})
	if err != nil {
		return err
	}
	cmd.Println("Successfully installed opt-in components")
	return nil
}

func installSdk(ctx context.Context, cmd *cobra.Command, config *assistantconfig.Config, sdkVersion *semver.Version) error {
	_, err := assistantconfig.GetInstalledSdkVersion(config, sdkVersion)
	if err == nil {
		cmd.Printf("SDK version %s is already installed\n", sdkVersion.String())
		return nil
	}
	if !errors.Is(err, assistantconfig.ErrTargetSdkNotInstalled) {
		return err
	}

	if _, err := sdkinstall.InstallSdkVersion(ctx, config, sdkVersion); err != nil {
		return err
	}
	cmd.Println("Successfully installed SDK " + sdkVersion.String())
	return nil
}
