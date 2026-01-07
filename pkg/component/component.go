// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package component

import (
	"errors"
	"fmt"
	"os"

	"daml.com/x/assistant/pkg/resolution"
	"daml.com/x/assistant/pkg/schema"
	"github.com/goccy/go-yaml"
	"github.com/samber/lo"
)

var ErrInvalidComponentManifest = fmt.Errorf("invalid component manifest")
var ErrMissingComponentField = fmt.Errorf("%w: a required field is missing", ErrInvalidComponentManifest)

const (
	ComponentKind          = "Component"
	ComponentSchemaVersion = "v1"
	ComponentAPIVersion    = schema.APIGroup + "/" + ComponentSchemaVersion
)

type Component struct {
	schema.ManifestMeta `yaml:",inline"`
	Spec                *Spec `yaml:"spec"`
}

type Spec struct {
	DependencyPaths map[string]string `yaml:"dependency-paths"`
	NativeCommands  []NativeCommand   `yaml:"commands"`
	JarCommands     []JarCommand      `yaml:"jar-commands"`
	Exports         Exports           `yaml:"exports"`
}

const (
	ExportConflictStrategyExtend = "extend"
	ExportConflictStrategyFail   = "fail"
)

// Exports is a Var -> []Export mapping
type Exports map[string]*Export

func (m *Exports) UnmarshalYAML(bytes []byte) error {
	raw := make(map[string]*Export)
	if err := yaml.UnmarshalWithOptions(bytes, &raw, yaml.Strict()); err != nil {
		return err
	}

	ExportStrategies := []string{ExportConflictStrategyExtend, ExportConflictStrategyFail}

	tmp := make(Exports)
	for k, e := range raw {
		e.Var = k
		tmp[k] = e
		if !lo.Contains(ExportStrategies, e.ConflictStrategy) {
			return fmt.Errorf("export %q has unknown type %q. Must be one of %q", k, e.ConflictStrategy, ExportStrategies)
		}
	}
	*m = tmp
	return nil
}

func (m *Exports) AsImports() resolution.Imports {
	imports := make(resolution.Imports)
	for k, e := range *m {
		imports[k] = e.Paths
	}
	return imports
}

type Export struct {
	ComponentName    string   `yaml:"-"`
	Var              string   `yaml:"-"`
	Paths            []string `yaml:"paths"`
	ConflictStrategy string   `yaml:"conflict-strategy"`
}

func (s *Spec) AllCommands() (all []Command) {
	commands := lo.Map(s.JarCommands, func(c JarCommand, _ int) Command {
		return &c
	})
	jarCommands := lo.Map(s.NativeCommands, func(c NativeCommand, _ int) Command {
		return &c
	})
	return append(commands, jarCommands...)
}

type Command interface {
	isComponentCommand() bool
	GetName() string
	GetPath() string
	GetAliases() []string
	GetDesc() *string
}

type JarCommand struct {
	Name    string   `yaml:"name"`
	Path    string   `yaml:"path"`
	Desc    *string  `yaml:"desc"`
	Aliases []string `yaml:"aliases"`
	JarArgs []string `yaml:"jar-args"`
	JvmArgs []string `yaml:"jvm-args"`
}

func (cmd *JarCommand) GetDesc() *string {
	return cmd.Desc
}

func (cmd *JarCommand) GetPath() string {
	return cmd.Path
}

func (cmd *JarCommand) GetName() string {
	return cmd.Name
}

func (cmd *JarCommand) GetAliases() []string {
	return cmd.Aliases
}

type NativeCommand struct {
	Name     string   `yaml:"name"`
	Path     string   `yaml:"path"`
	Desc     *string  `yaml:"desc"`
	Aliases  []string `yaml:"aliases"`
	ExecArgs []string `yaml:"exec-args"`
}

func (cmd *NativeCommand) GetDesc() *string {
	return cmd.Desc
}

func (cmd *NativeCommand) GetPath() string {
	return cmd.Path
}

func (cmd *NativeCommand) GetName() string {
	return cmd.Name
}

func (cmd *NativeCommand) GetAliases() []string {
	return cmd.Aliases
}

func (cmd *JarCommand) isComponentCommand() bool {
	return true
}
func (cmd *JarCommand) UnmarshalYAML(data []byte) error {
	type Alias JarCommand
	alias := Alias{}
	if err := yaml.UnmarshalWithOptions(data, &alias, yaml.Strict()); err != nil {
		return err
	}
	if alias.Path == "" {
		return fmt.Errorf("%w: 'jar-path'", ErrMissingComponentField)
	}
	if alias.Name == "" {
		return fmt.Errorf("%w: 'name'", ErrMissingComponentField)
	}
	*cmd = JarCommand(alias)
	return nil
}

func (cmd *NativeCommand) isComponentCommand() bool {
	return true
}
func (cmd *NativeCommand) UnmarshalYAML(data []byte) error {
	type Alias NativeCommand
	alias := Alias{}
	if err := yaml.UnmarshalWithOptions(data, &alias, yaml.Strict()); err != nil {
		return err
	}
	if alias.Path == "" {
		return fmt.Errorf("%w: 'path'", ErrMissingComponentField)
	}
	if alias.Name == "" {
		return fmt.Errorf("%w: 'name'", ErrMissingComponentField)
	}
	*cmd = NativeCommand(alias)
	return nil
}

func ReadComponent(filePath string) (*Component, error) {
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return ReadComponentContents(bytes)
}

func ReadComponentContents(contents []byte) (*Component, error) {
	var c Component
	if err := yaml.UnmarshalWithOptions(contents, &c, yaml.Strict()); err != nil {
		return nil, errors.Join(ErrInvalidComponentManifest, err)
	}

	s := schema.ManifestMeta{
		APIVersion: ComponentAPIVersion,
		Kind:       ComponentKind,
	}
	err := s.ValidateSchema(c.ManifestMeta)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidComponentManifest, err.Error())
	}

	if c.Spec == nil {
		return nil, fmt.Errorf("%w: 'spec'", ErrMissingComponentField)
	}

	return &c, nil
}

var _ yaml.BytesUnmarshaler = (*Exports)(nil)
var _ yaml.BytesUnmarshaler = (*NativeCommand)(nil)
var _ yaml.BytesUnmarshaler = (*JarCommand)(nil)
var _ Command = (*JarCommand)(nil)
var _ Command = (*NativeCommand)(nil)
