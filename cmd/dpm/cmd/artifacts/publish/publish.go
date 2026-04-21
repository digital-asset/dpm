// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package publish

import (
	publishcomponent "daml.com/x/assistant/cmd/dpm/cmd/artifacts/publish/component"
	publishdar "daml.com/x/assistant/cmd/dpm/cmd/artifacts/publish/dar"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "publish",
		Long: "Commands for publishing artifacts",
	}

	cmd.AddCommand(publishcomponent.Cmd())
	cmd.AddCommand(publishdar.Cmd())

	return cmd
}
