// Copyright (c) 2017-2025 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package builtincommand

import (
	"github.com/samber/lo"
)

type BuiltinCommand string

const (
	Versions  BuiltinCommand = "versions"
	Version   BuiltinCommand = "version"
	Bootstrap BuiltinCommand = "bootstrap"
	Install   BuiltinCommand = "install"
	UnInstall BuiltinCommand = "uninstall"
	Component BuiltinCommand = "component"
	Repo      BuiltinCommand = "repo"
	Resolve   BuiltinCommand = "resolve"
	Login     BuiltinCommand = "login"
)

var BuiltinCommands = []BuiltinCommand{Versions, Version, Bootstrap, Install, UnInstall, Component, Repo, Resolve, Login}

func IsBuiltinCommand(args []string) bool {
	if len(args) > 1 {
		elems := lo.Map(BuiltinCommands, func(item BuiltinCommand, _ int) string {
			return string(item)
		})
		return lo.Contains(elems, args[1])
	}
	return false
}
