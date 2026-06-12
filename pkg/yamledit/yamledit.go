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

// ReplaceItemInList replaces the itemIndex-th item in targetFieldName.
// item can be a simple value or a whole object
func ReplaceItemInList(yamlContents []byte, targetFieldName string, itemIndex int, item string) (string, error) {
	item = indentYaml(item)
	lines := strings.SplitAfter(string(yamlContents), "\n")

	file, _ := parser.ParseBytes(yamlContents, 0)
	components := findField(file.Docs[0], targetFieldName)
	if components == nil || components.Value == nil {
		return "", fmt.Errorf("couldn't update the `%s` yaml field: field is missing or empty", targetFieldName)
	}

	seq, ok := components.Value.(*ast.SequenceNode)
	if !ok || itemIndex < 0 || itemIndex >= len(seq.Entries) {
		return "", fmt.Errorf("couldn't update the `%s` yaml field: target entry of index %d does't exist", targetFieldName, itemIndex)
	}

	entry := seq.Entries[itemIndex]
	startAt := entry.GetToken().Position.Line - 1

	endAt := len(lines)

	if itemIndex+1 < len(seq.Entries) {
		endAt = seq.Entries[itemIndex+1].GetToken().Position.Line - 1

		// preserve comments/blank lines immediately before the next item.
		for endAt > startAt {
			line := strings.TrimSpace(lines[endAt-1])
			if line != "" && !strings.HasPrefix(line, "#") {
				break
			}
			endAt--
		}
	} else {
		// last item: stop when another field at the same or lower indentation starts.
		itemIndent := entry.GetToken().Position.Column - 1

		for i := startAt + 1; i < len(lines); i++ {
			line := strings.TrimRight(lines[i], "\r\n")
			trimmed := strings.TrimSpace(line)

			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}

			if strings.HasSuffix(trimmed, ":") && leadingSpaces(line) <= itemIndent {
				endAt = i
				break
			}
		}
	}

	lines = slices.Replace(lines, startAt, endAt, item)
	return strings.Join(lines, ""), nil
}

func leadingSpaces(s string) int {
	count := 0
	for _, r := range s {
		if r != ' ' {
			break
		}
		count++
	}
	return count
}
