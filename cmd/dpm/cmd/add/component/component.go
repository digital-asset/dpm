package component

import (
	"os"
	"slices"
	"strings"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/componentlist"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/samber/lo"
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

	out := appendToYaml(b, strings.TrimRight(string(item), "\n"))
	return os.WriteFile(path, out, 0644)
}

func appendToYaml(b []byte, item string) []byte {
	item = "  " + item + "\n"

	file, _ := parser.ParseBytes(b, 0)
	components := findField(file.Docs[0], "components")
	lines := strings.SplitAfter(string(b), "\n")

	if components == nil {
		return []byte(strings.Join(append(lines, "components:\n", item), ""))
	}

	insertAt := components.GetToken().Position.Line
	if len(components.Entries) > 0 {
		last := components.Entries[len(components.Entries)-1]
		insertAt = last.GetToken().Position.Line
	}

	lines = slices.Insert(lines, insertAt, item)
	return []byte(strings.Join(lines, ""))
}

func findField(doc *ast.DocumentNode, field string) *ast.SequenceNode {
	body := doc.Body.(*ast.MappingNode)
	r, _ := lo.Find(body.Values, func(n *ast.MappingValueNode) bool {
		return n.Key.String() == field
	})
	return r.Value.(*ast.SequenceNode)
}
