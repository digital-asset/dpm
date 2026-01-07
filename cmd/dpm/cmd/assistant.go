// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"daml.com/x/assistant/cmd/dpm/cmd/uninstall"
	"errors"
	"fmt"
	"log/slog"

	"daml.com/x/assistant/cmd/dpm/cmd/bootstrap"
	componentCmd "daml.com/x/assistant/cmd/dpm/cmd/component"
	"daml.com/x/assistant/cmd/dpm/cmd/install"
	"daml.com/x/assistant/cmd/dpm/cmd/login"
	"daml.com/x/assistant/cmd/dpm/cmd/repo"
	"daml.com/x/assistant/cmd/dpm/cmd/resolve"
	"daml.com/x/assistant/cmd/dpm/cmd/versions"
	"daml.com/x/assistant/pkg/assistant"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/assistantversion"
	"daml.com/x/assistant/pkg/builtincommand"
	"daml.com/x/assistant/pkg/logging"
	"github.com/goccy/go-yaml"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

const (
	builtinGroupId = "builtin"
	sdkGroupId     = "sdk"
	DpmName        = "dpm"
)

func RootCmd(ctx context.Context, da *assistant.DamlAssistant) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use: DpmName,
	}

	defer da.SetOutputStreams(cmd)

	if len(da.OsArgs) == 0 {
		return nil, fmt.Errorf("DamlAssistant.OsArgs must contain at least one entry similar to os.Args")
	}

	cmd.SetArgs(da.OsArgs[1:])
	cmd.AddGroup(&cobra.Group{
		ID:    builtinGroupId,
		Title: "Meta Commands",
	})
	cmd.AddGroup(&cobra.Group{
		ID:    sdkGroupId,
		Title: "Dpm-SDK Commands",
	})

	if err := logging.InitLogging(); err != nil {
		return nil, err
	}

	config, err := assistantconfig.Get()
	if err != nil {
		return nil, err
	}
	if err := config.EnsureDirs(); err != nil {
		return nil, err
	}
	if lo.Contains(da.OsArgs, cobra.ShellCompRequestCmd) || lo.Contains(da.OsArgs, cobra.ShellCompNoDescRequestCmd) {
		config.AutoInstall = false // auto-install is normally disabled by default, but be explicit about it in this case still.
	}

	cmd.AddCommand(
		setCmdMetaGroup(versions.Cmd(config)),
		setCmdMetaGroup(bootstrap.Cmd(config)),
		setCmdMetaGroup(install.Cmd(config)),
		setCmdMetaGroup(uninstall.Cmd(config)),
		setCmdMetaGroup(login.Cmd(config)),
		setCmdMetaGroup(repo.Cmd(config)),
		setCmdMetaGroup(resolve.Cmd(config)),
		componentCmd.Cmd(config),
	)

	resolutionType := lo.Ternary(
		isHelp(da.OsArgs),
		assistant.ShallowResolution,
		assistant.DeepResolution,
	)

	if shouldAddSdkCommands(da.OsArgs) {
		sdkCommands, err := da.ComputeSdkCommandsFromAssemblyPlan(ctx, config, resolutionType)
		if errors.Is(err, assistantconfig.ErrNoSdkInstalled) {
			slog.Warn("No SDK has been installed. To use the bare assistant without an SDK, use a 'dpm.local.yaml' or see the docs for more info")
		} else if err != nil {
			return nil, err
		} else {
			for _, c := range sdkCommands {
				c.GroupID = sdkGroupId
			}
			cmd.AddCommand(sdkCommands...)
		}
	}

	version, err := yaml.Marshal(assistantversion.Get())
	if err != nil {
		return nil, err
	}
	cmd.Version = string(version)
	cmd.VersionTemplate()
	cmd.SetVersionTemplate("{{.Version}}")

	return cmd, nil
}

func setCmdMetaGroup(cmd *cobra.Command) *cobra.Command {
	cmd.GroupID = builtinGroupId
	return cmd
}

func shouldAddSdkCommands(osArgs []string) bool {
	return len(osArgs) == 1 || !builtincommand.IsBuiltinCommand(osArgs)
}

func isHelp(osArgs []string) bool {
	return len(osArgs) == 1 || (len(osArgs) == 2 &&
		lo.Contains([]string{"--help", "-h", "help"}, osArgs[1]))
}
