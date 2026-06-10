package component

import (
	"os"
	"strings"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/componentlist"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/spf13/cobra"
	"oras.land/oras-go/v2/registry"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "component <component>",
		Short: "add a component to project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			component := args[0]
			_, err := registry.ParseReference(strings.TrimPrefix(component, "oci://"))
			if err != nil {
				return err
			}

			pkgPath, ok, err := assistantconfig.GetDamlPackageAbsolutePath()
			if err != nil {
				return err
			}
			if ok {
				return addToPackage(pkgPath, component)
			}

			return nil
		},
	}

	return cmd
}

func addToPackage(path, component string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	item, err := yaml.Marshal(componentlist.ComponentList{
		&componentlist.ComponentEntry{StringBased: &component},
	})
	if err != nil {
		return err
	}

	out := insertAfterLastComponent(b, strings.TrimRight(string(item), "\n"))
	return os.WriteFile(path, out, 0644)
}

func insertAfterLastComponent(b []byte, item string) []byte {
	file, _ := parser.ParseBytes(b, 0)

	components := file.Docs[0].
		Body.(*ast.MappingNode).
		Values[1].
		Value.(*ast.SequenceNode)

	insertAt := components.GetToken().Position.Line
	indent := strings.Repeat(" ", components.GetToken().Position.Column)

	if len(components.Entries) > 0 {
		last := components.Entries[len(components.Entries)-1]
		insertAt = last.GetToken().Position.Line

		line := strings.SplitAfter(string(b), "\n")[insertAt-1]
		indent = line[:len(line)-len(strings.TrimLeft(line, " "))]
	}

	lines := strings.SplitAfter(string(b), "\n")

	lines = append(
		lines[:insertAt],
		append([]string{indent + item + "\n"}, lines[insertAt:]...)...,
	)

	return []byte(strings.Join(lines, ""))
}
