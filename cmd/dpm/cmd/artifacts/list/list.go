// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package list

import (
	"daml.com/x/assistant/cmd/dpm/cmd/artifacts/list/component"
	dartags "daml.com/x/assistant/cmd/dpm/cmd/artifacts/list/dar"
	"daml.com/x/assistant/pkg/assistantconfig"
	"github.com/spf13/cobra"
)

func Cmd(config *assistantconfig.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Commands for listing artifacts",
		Long:  "Commands for listing artifacts",
	}

	cmd.AddCommand(componenttags.Cmd(config))
	cmd.AddCommand(dartags.Cmd(config))

	return cmd
}
