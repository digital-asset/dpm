// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	cmd "daml.com/x/assistant/cmd/dpm/cmd"
	"daml.com/x/assistant/pkg/assistant"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func main() {
	ctx, cancelFn := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancelFn()

	if err := getDocsCmd().ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}

}

func getDocsCmd() *cobra.Command {
	var format string

	docsCmd := &cobra.Command{
		Use:   "docs <output dir>",
		Short: "generate assistant CLI commands reference",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := os.Args[1]

			var useRst bool
			switch format {
			case "rst":
				useRst = true
			case "md":
				useRst = false
			default:
				return fmt.Errorf("only --rst or --md are supported")
			}

			if err := genDocs(context.Background(), dir, useRst); err != nil {
				cmd.SilenceUsage = true
				return err
			}

			fmt.Printf("successfully generated at %s\n", dir)
			return nil
		},
	}

	docsCmd.Flags().StringVar(&format, "format", "", "(required) --md or -rst")
	docsCmd.MarkFlagRequired("format")

	return docsCmd
}

func genDocs(ctx context.Context, dir string, useRst bool) error {
	tmp, deleteFn, err := utils.MkdirTemp("", "")
	if err != nil {
		return err
	}
	defer func() { _ = deleteFn() }()

	if err := os.Setenv(assistantconfig.EditionEnvVar, sdkmanifest.OpenSource.String()); err != nil {
		return err
	}
	if err := os.Setenv(assistantconfig.DpmHomeEnvVar, tmp); err != nil {
		return err
	}

	da := &assistant.DamlAssistant{OsArgs: []string{os.Args[0]}}
	root, err := cmd.RootCmd(ctx, da)
	if err != nil {
		return err
	}
	root.DisableAutoGenTag = true

	_, err = os.ReadDir(dir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	root.IsAdditionalHelpTopicCommand()
	for _, c := range root.Commands() {
		c.Hidden = false
	}

	if useRst {
		if err := doc.GenReSTTreeCustom(root, dir, prependRSTHeader, linkHandler); err != nil {
			return err
		}
		fmt.Println("generating index.rst...")
		return generateTOC(dir)

	} else {
		return doc.GenMarkdownTreeCustom(root, dir, prependFrontMatter, func(s string) string {
			return s
		})
	}
}

// add a Jekyll/Just-the-Docs front-matter block
func prependFrontMatter(filename string) string {
	base := filepath.Base(filename)
	cmdKey := strings.TrimSuffix(base, ".md")
	title := strings.Title(strings.ReplaceAll(cmdKey, "_", " "))
	return fmt.Sprintf(`---
layout: default
title: %s
parent: CLI reference
---

`, title)
}

func prependRSTHeader(filename string) string {
	base := filepath.Base(filename)
	cmdKey := strings.TrimSuffix(base, ".rst")
	title := strings.Title(strings.ReplaceAll(cmdKey, "_", " "))
	return fmt.Sprintf("%s\n%s\n\n", title, strings.Repeat("=", len(title)))
}

func linkHandler(name, ref string) string {
	return fmt.Sprintf(":ref:`%s <%s>`", name, ref)
}

func generateTOC(outputDir string) error {
	tocHeader := `.. toctree::
   :maxdepth: 2
   :caption: CLI Reference:

`

	f, err := os.Create(filepath.Join(outputDir, "index.rst"))
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	if _, err := f.WriteString(tocHeader); err != nil {
		return err
	}

	commands, err := os.ReadDir(outputDir)
	if err != nil {
		return fmt.Errorf("error reading output directory: %v", err)
	}

	for _, c := range commands {
		if filepath.Ext(c.Name()) == ".rst" && c.Name() != "index.rst" {
			line := fmt.Sprintf("   %s\n", strings.TrimSuffix(c.Name(), ".rst"))
			if _, err := f.WriteString(line); err != nil {
				return err
			}
		}
	}

	return nil
}
