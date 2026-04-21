// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"daml.com/x/assistant/cmd/dpm/cmd/repo/assistant"
	componentPublish "daml.com/x/assistant/cmd/dpm/cmd/repo/component"
	"daml.com/x/assistant/cmd/dpm/cmd/repo/promote"
	"daml.com/x/assistant/cmd/dpm/cmd/repo/resolve"
	"daml.com/x/assistant/cmd/dpm/cmd/repo/sdkmanifest"
	"daml.com/x/assistant/cmd/dpm/cmd/repo/tags"
	"daml.com/x/assistant/cmd/dpm/cmd/repo/tarball"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/builtincommand"
	"github.com/spf13/cobra"
)

func Cmd(config *assistantconfig.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:    string(builtincommand.Repo),
		Long:   "Internal set of commands for working with OCI release-drive",
		Hidden: true,
	}

	cmd.AddCommand(sdkmanifest.Cmd())
	cmd.AddCommand(tarball.Cmd())
	cmd.AddCommand(componentPublish.Cmd())
	cmd.AddCommand(assistant.Cmd())
	cmd.AddCommand(resolve.Cmd())
	cmd.AddCommand(promote.Cmd())
	cmd.AddCommand(tags.Cmd(config))

	return cmd
}
