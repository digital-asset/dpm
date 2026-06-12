package yamledit

import (
	"fmt"
	"slices"
	"strings"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/samber/lo"
)

// AppendToYaml adds item to the given target field.
// item can be a simple value or a whole object
func AppendToYaml(yamlFileContents []byte, targetFieldName, item string) string {
	// indent all lines of the item,
	// in case the item is not a single-line value but a whole object
	item = indentYaml(item)
	lines := strings.SplitAfter(string(yamlFileContents), "\n")

	file, _ := parser.ParseBytes(yamlFileContents, 0)
	components := findField(file.Docs[0], targetFieldName)

	if components == nil {
		fieldLine := fmt.Sprintf("\n\n%s:\n", targetFieldName)
		return strings.Join(append(lines, fieldLine, item, "\n"), "")
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
