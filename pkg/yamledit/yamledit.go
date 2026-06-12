package yamledit

import (
	"slices"
	"strings"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/samber/lo"
)

// AppendToYaml adds item to targetField.
// item can be a simple value or a whole object
func AppendToYaml(yamlFileContents []byte, targetField, item string) string {
	// indent all lines of the item,
	// in case the item is not a single-line value but a whole object
	item = indentYaml(item)
	lines := strings.SplitAfter(string(yamlFileContents), "\n")

	file, _ := parser.ParseBytes(yamlFileContents, 0)
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
