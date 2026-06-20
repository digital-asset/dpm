// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sdkinstall

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"daml.com/x/assistant/pkg/assembler"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/ocipuller"
	"daml.com/x/assistant/pkg/ocipuller/remotepuller"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/simpleplatform"
	"daml.com/x/assistant/pkg/utils"
	"github.com/Masterminds/semver/v3"
)

const (
	assistantVersionCommentPrefix = ":: assistant version: "
)

// InstallSdkVersion installs the given sdk version by pulling the assembly and the components defined in it
// from the remote OCI registry
func InstallSdkVersion(ctx context.Context, config *assistantconfig.Config, sdkVersion *semver.Version) (*assistantconfig.InstalledSdkVersion, error) {
	if config.AutoInstall == false {
		return nil, fmt.Errorf("invalid assistantconfig object: AutoInstall must be set to true when attempting to install an SDk")
	}

	puller, err := remotepuller.NewFromRemoteConfig(config)
	if err != nil {
		return nil, err
	}

	err = utils.WithInstallLock(ctx, config.InstallLocalFilePath, func() error {
		return installSdkVersion(ctx, config, puller, sdkVersion)
	})
	if err != nil {
		return nil, err
	}

	return assistantconfig.GetInstalledSdkVersion(config, sdkVersion)
}

func installSdkVersion(ctx context.Context, config *assistantconfig.Config, puller ocipuller.OciPuller, sdkVersion *semver.Version) error {
	tag := sdkVersion.String() // TODO flesh this out further

	tmpDir, deleteFn, err := utils.MkdirTemp("", "")
	if err != nil {
		return err
	}
	defer func() { _ = deleteFn() }()

	edition, err := config.Edition.Get()
	if err != nil {
		return err
	}

	err = puller.PullAssembly(ctx, edition, tag, tmpDir, simpleplatform.CurrentPlatform())
	if err != nil {
		return err
	}

	tmpAssemblyPath := filepath.Join(tmpDir, sdkVersion.String()+".yaml")
	manifest, err := sdkmanifest.ReadSdkManifest(tmpAssemblyPath)
	if err != nil {
		return err
	}

	if *manifest.Spec.Edition != edition {
		return fmt.Errorf("pulled assembly has unexpected edition %q", manifest.Spec.Edition.String())
	}
	assemblyPath := filepath.Join(config.InstalledSdkManifestsPath, edition.String(), sdkVersion.String()+".yaml")
	if err := utils.CopyFile(tmpAssemblyPath, assemblyPath); err != nil {
		return err
	}

	assemblyManifest, err := sdkmanifest.ReadSdkManifest(assemblyPath)
	if err != nil {
		return err
	}

	if assemblyManifest.Spec.Assistant == nil {
		return fmt.Errorf("sdk missing the assistant")
	}

	assemblyResult, err := assembler.New(config, puller).Assemble(ctx, assemblyManifest)
	if err != nil {
		return err
	}

	_, err = LinkAssistantIfNewerSdk(config, *assemblyResult.AssistantAbsolutePath, sdkVersion)
	return err
}

func LinkAssistant(config *assistantconfig.Config, binSourcePath string) (symlinkPath string, err error) {
	symlinkPath, err = linkAssistant(config, binSourcePath)
	if err != nil {
		return "", fmt.Errorf("failed to set up assistant link: %w", err)
	}
	return
}

func LinkAssistantIfNewerSdk(config *assistantconfig.Config, binSourcePath string, sdkVersion *semver.Version) (symlinkPath string, err error) {
	installedSdkVersion, err := assistantconfig.GetInstalledSdk(config)
	// note: treating an unknown edition similarly to no-sdk-installed
	// because when installing an SDK for the first time, the edition on the system might not have been set yet
	// e.g. first-time `dpm bootstrap`
	isNoSdk := errors.Is(err, assistantconfig.ErrNoSdkInstalled)
	if err != nil && !isNoSdk {
		return "", err
	}

	if isNoSdk || !sdkVersion.LessThan(installedSdkVersion.Version) {
		return LinkAssistant(config, binSourcePath)
	}

	return GetLinkTarget(config, binSourcePath), nil
}

func linkAssistant(config *assistantconfig.Config, binSourcePath string) (string, error) {
	symlinkPath := GetLinkTarget(config, binSourcePath)
	if err := utils.EnsureDirs(filepath.Dir(symlinkPath)); err != nil {
		return "", err
	}

	if err := os.RemoveAll(symlinkPath); err != nil {
		return "", fmt.Errorf("failed to delete existing dpm binary link. %w", err)
	}

	if runtime.GOOS == "windows" {
		content := assistantVersionCommentPrefix + filepath.Base(filepath.Dir(binSourcePath)) + "\r\n"
		content += "@echo off\r\n"
		content += binSourcePath + " %*\r\n"
		return symlinkPath, os.WriteFile(symlinkPath, []byte(content), 0755)
	}

	rel, err := filepath.Rel(config.DamlHomePath, binSourcePath)
	if err != nil {
		return "", err
	}
	binRelativeSourcePath := path.Join("..", rel)
	return symlinkPath, os.Symlink(binRelativeSourcePath, symlinkPath)
}

func GetLinkTarget(config *assistantconfig.Config, binSourcePath string) string {
	binTargetDir := filepath.Join(config.DamlHomePath, "bin")
	binTargetName := strings.ReplaceAll(filepath.Base(binSourcePath), ".exe", ".cmd")
	return filepath.Join(binTargetDir, binTargetName)
}
