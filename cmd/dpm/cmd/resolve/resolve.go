// Copyright (c) 2017-2025 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package resolve

import (
	"context"
	"log/slog"

	"daml.com/x/assistant/pkg/assembler"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/builtincommand"
	"daml.com/x/assistant/pkg/ocipuller/remotepuller"
	"daml.com/x/assistant/pkg/resolver"
	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

func getDeepResolutionOutput(ctx context.Context, config *assistantconfig.Config) (string, error) {
	puller, err := remotepuller.NewFromRemoteConfig(config)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create puller", "error", err)
		return "", err
	}

	deepResolution, err := resolver.New(config, assembler.New(config, puller)).RunDeepResolution(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to run deep resolution", "error", err)
		return "", err
	}

	bytes, err := yaml.Marshal(deepResolution)
	if err != nil {
		slog.ErrorContext(ctx, "failed marshal deep resolution", "error", err)
		return "", err
	}

	return string(bytes), nil
}

func Cmd(config *assistantconfig.Config) *cobra.Command {
	return &cobra.Command{
		Use:    string(builtincommand.Resolve),
		Long:   "completes deep resolution and generates the associated file",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			output, err := getDeepResolutionOutput(cmd.Context(), config)
			if err != nil {
				return err
			}
			cmd.Println(output)
			return nil
		},
	}
}
