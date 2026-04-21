// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package publish

import (
	publishcomponent "daml.com/x/assistant/cmd/dpm/cmd/artifacts/publish/component"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "publish",
		Long:   "Internal set of commands for publishing artifacts with OCI URIs",
		Hidden: true,
	}

	cmd.AddCommand(publishcomponent.Cmd())

	return cmd
}
