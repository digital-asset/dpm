// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package component

import (
	initcmd "daml.com/x/assistant/cmd/dpm/cmd/component/init"
	"daml.com/x/assistant/cmd/dpm/cmd/component/run"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/builtincommand"
	"github.com/spf13/cobra"
)

func Cmd(config *assistantconfig.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:    string(builtincommand.Component),
		Short:  "commands for component development",
		Hidden: true,
	}
	cmd.AddCommand(
		initcmd.Cmd(),
		run.Cmd(config),
	)

	return cmd
}
