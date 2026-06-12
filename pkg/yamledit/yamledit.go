package yamledit

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

// AppendToYaml adds item to the given target field.
// item can be a simple value or a whole object
func AppendToYaml(yamlFileContents []byte, targetFieldName, item string) (string, error) {
	// indent all lines of the item,
	// in case the item is not a single-line value but a whole object
	item = indentYaml(item)
	lines := strings.SplitAfter(string(yamlFileContents), "\n")

	file, err := parser.ParseBytes(yamlFileContents, 0)
	if err != nil {
		return "", err
	}

	field, _ := findField(file.Docs[0], targetFieldName)

	if field == nil {
		fieldLine := fmt.Sprintf("\n\n%s:\n", targetFieldName)
		return strings.Join(append(lines, fieldLine, item, "\n"), ""), nil
	}

	insertAt := field.GetToken().Position.Line
	lines = slices.Insert(lines, insertAt, item)
	return strings.Join(lines, ""), nil
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
	baseErr := fmt.Errorf("couldn't patch yaml field %q", targetFieldName)
	out, err := replaceItemInList(yamlContents, targetFieldName, itemIndex, item)
	if err != nil {
		return "", errors.Join(baseErr, err)
	}
	return out, nil
}

func replaceItemInList(yamlContents []byte, targetFieldName string, itemIndex int, item string) (string, error) {
	item = indentYaml(item)
	lines := strings.SplitAfter(string(yamlContents), "\n")

	file, err := parser.ParseBytes(yamlContents, 0)
	if err != nil {
		return "", err
	}

	field, nextField := findField(file.Docs[0], targetFieldName)
	if field == nil || field.Value == nil {
		return "", fmt.Errorf("couldn't patch yaml field %q: field does not exist", targetFieldName)
	}

	seq, ok := field.Value.(*ast.SequenceNode)
	if !ok {
		return "", fmt.Errorf("couldn't patch yaml field %q: field is not a list", targetFieldName)
	}
	if itemIndex >= len(seq.Entries) {
		return "", fmt.Errorf("couldn't patch yaml field %q: target item of index %d not present", targetFieldName, itemIndex)
	}

	entry := seq.Entries[itemIndex]
	startAt := entry.GetToken().Position.Line - 1

	endAt := len(lines)
	if itemIndex+1 < len(seq.Entries) {
		endAt = seq.Entries[itemIndex+1].GetToken().Position.Line - 1
	} else if nextField != nil {
		endAt = nextField.Key.GetToken().Position.Line - 1
	}

	// preserve comments/blank lines immediately before the next thing that appears in the yaml
	for endAt > startAt {
		line := strings.TrimSpace(lines[endAt-1])
		if line != "" && !strings.HasPrefix(line, "#") {
			break
		}
		endAt--
	}

	lines = slices.Replace(lines, startAt, endAt, item)
	return strings.Join(lines, ""), nil
}

// findField looks up the field with given name in the yaml doc's AST.
// Returns the both the field and next field's nodes (if any)
func findField(doc *ast.DocumentNode, field string) (*ast.MappingValueNode, *ast.MappingValueNode) {
	if doc == nil || doc.Body == nil {
		return nil, nil
	}

	body, ok := doc.Body.(*ast.MappingNode)
	if !ok {
		return nil, nil
	}

	for i, n := range body.Values {
		if n.Key.String() == field {
			if i+1 < len(body.Values) {
				return n, body.Values[i+1]
			}
			return n, nil
		}
	}

	return nil, nil
}
