// Copyright (c) 2017-2025 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	dpm "daml.com/x/assistant/cmd/dpm/cmd"
	"daml.com/x/assistant/pkg/assistant"
)

func main() {
	ctx, cancelFn := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancelFn()

	da := assistant.DamlAssistant{
		Stderr: os.Stderr,
		Stdout: os.Stdout,
		Stdin:  os.Stdin,
		ExitFn: os.Exit,
		OsArgs: os.Args,
	}
	cmd, err := dpm.RootCmd(ctx, &da)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if err := cmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}

}
