// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package resolve

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"daml.com/x/assistant/pkg/assembler"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/builtincommand"
	"daml.com/x/assistant/pkg/ocipuller/remotepuller"
	"daml.com/x/assistant/pkg/resolver"
	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

var output string

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

	var bytes []byte

	switch output {
	case "json":
		bytes, err = json.MarshalIndent(deepResolution, "", "  ")
	case "yaml":
		bytes, err = yaml.Marshal(deepResolution)
	default:
		return "", fmt.Errorf("output format not supported: %s", output)
	}

	if err != nil {
		return "", fmt.Errorf("failed marshal deep resolution: %w", err)
	}

	return string(bytes), nil
}

func Cmd(config *assistantconfig.Config) *cobra.Command {
	cmd := &cobra.Command{
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

	cmd.Flags().StringVarP(&output, "output", "o", "yaml", "output format: json, yaml")
	return cmd
}
