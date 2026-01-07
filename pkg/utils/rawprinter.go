// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

type RawPrinter interface {
	Print(i ...interface{})
	Println(i ...interface{})
	Printf(format string, i ...interface{})
	PrintErr(i ...interface{})
	PrintErrln(i ...interface{})
	PrintErrf(format string, i ...interface{})
}

type StdPrinter struct{}

func (s StdPrinter) Print(i ...interface{}) {
	fmt.Print(i...)
}

func (s StdPrinter) Println(i ...interface{}) {
	fmt.Println(i...)
}

func (s StdPrinter) Printf(format string, i ...interface{}) {
	fmt.Printf(format, i...)
}

func (s StdPrinter) PrintErr(i ...interface{}) {
	fmt.Fprint(os.Stderr, i...)
}

func (s StdPrinter) PrintErrln(i ...interface{}) {
	fmt.Fprintln(os.Stderr, i...)
}

func (s StdPrinter) PrintErrf(format string, i ...interface{}) {
	fmt.Fprintf(os.Stderr, format, i...)
}

var _ RawPrinter = (*StdPrinter)(nil)
var _ RawPrinter = (*cobra.Command)(nil)
