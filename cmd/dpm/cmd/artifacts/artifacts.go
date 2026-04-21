// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package artifacts

import (
	"daml.com/x/assistant/cmd/dpm/cmd/artifacts/publish"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/builtincommand"
	"github.com/spf13/cobra"
)

func Cmd(config *assistantconfig.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:  string(builtincommand.Artifacts),
		Long: "Commands for managing artifacts",
	}

	cmd.AddCommand(publish.Cmd())

	return cmd
}
