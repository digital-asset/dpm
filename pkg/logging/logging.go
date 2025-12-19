// Copyright (c) 2017-2025 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"log/slog"
	"os"

	"daml.com/x/assistant/pkg/assistantconfig"
)

func InitLogging() error {
	logLevel, ok := os.LookupEnv(assistantconfig.LogLevelEnvVar)
	if !ok {
		return initLogging("info")
	}
	return initLogging(logLevel)
}

func initLogging(logLevel string) error {
	var l slog.Level
	if err := l.UnmarshalText([]byte(logLevel)); err != nil {
		return err
	}

	slogHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: l})
	slog.SetDefault(slog.New(slogHandler))
	return nil
}
