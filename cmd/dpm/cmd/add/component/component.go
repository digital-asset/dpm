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

	//item, err := yaml.Marshal(componentlist.ComponentList{
	//	&componentlist.ComponentEntry{StringBased: &component},
	//})

	item, err := yaml.Marshal(componentlist.ComponentList{
		&componentlist.ComponentEntry{FileBased: &componentlist.FileBased{
			Name: component,
			Path: "/dev/null",
		},
		},
	})
	if err != nil {
		return err
	}

	out := AppendToYaml(b, "components", string(item))
	return os.WriteFile(path, []byte(out), 0644)
}

// AppendToYaml adds item to targetField.
// item can be a simple value or a whole object
func AppendToYaml(b []byte, targetField, item string) string {
	// indent all lines of the item,
	// in case the item is not a single-line value but a whole object
	item = indentYaml(item)
	lines := strings.SplitAfter(string(b), "\n")

	file, _ := parser.ParseBytes(b, 0)
	components := findField(file.Docs[0], targetField)

	if components == nil {
		return strings.Join(append(lines, "\n\ncomponents:\n", item, "\n"), "")
	}

	insertAt := components.GetToken().Position.Line
	lines = slices.Insert(lines, insertAt, item)
	return strings.Join(lines, "")
}

func findField(doc *ast.DocumentNode, field string) *ast.MappingValueNode {
	if doc == nil || doc.Body == nil {
		return nil
	}

	body, ok := doc.Body.(*ast.MappingNode)
	if !ok {
		return nil
	}

	r, _ := lo.Find(body.Values, func(n *ast.MappingValueNode) bool {
		return n.Key.String() == field
	})
	return r
}

func indentYaml(item string) string {
	item = strings.TrimSpace(item)
	if item == "" {
		return ""
	}

	lines := strings.Split(item, "\n")
	for i := range lines {
		lines[i] = "  " + lines[i]
	}

	return strings.Join(lines, "\n") + "\n"
}
