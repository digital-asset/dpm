// Copyright (c) 2017-2025 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package initcmd

import (
	"fmt"
	"os"

	"daml.com/x/assistant/cmd/dpm/cmd/component/init/manifests"
	"daml.com/x/assistant/pkg/assistantconfig"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "initialize a component in the current directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := writeFile("component.yaml", manifests.ComponentYaml, force); err != nil {
				return err
			}
			if err := writeFile(assistantconfig.DamlLocalFilename, manifests.Daml3LocalYaml, force); err != nil {
				return err
			}
			fmt.Printf("created component.yaml and %s in current directory\n", assistantconfig.DamlLocalFilename)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing component.yaml and dpm.local.yaml")

	return cmd
}

func writeFile(name string, data []byte, force bool) error {
	if _, err := os.Stat(name); err == nil && !force {
		return fmt.Errorf("file %s already exists. Use --force to overwrite", name)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(name, data, os.ModePerm)
}
