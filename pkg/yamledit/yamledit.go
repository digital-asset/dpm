package yamledit

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/parser"
)

type YamlTarget struct {
	YamlFilePath string
	FieldName    string
}

// EditYaml adds an item to list in yaml file
// or replace the given index with it
func EditYaml(info YamlTarget, item string, index int) error {
	b, err := os.ReadFile(info.YamlFilePath)
	if err != nil {
		return err
	}

	var out string
	if index != -1 {
		out, err = ReplaceItemInList(b, info.FieldName, index, item)
	} else {
		out, err = AddToList(b, info.FieldName, item)
	}
	if err != nil {
		return err
	}

	return os.WriteFile(info.YamlFilePath, []byte(out), 0644)
}

// AddToList adds item to the given target field.
// item can be a simple value or a whole object
func AddToList(raw []byte, field string, item string) (string, error) {
	f, err := parser.ParseBytes(raw, parser.ParseComments)
	if err != nil {
		return "", err
	}

	itemListYAML, err := marshalOneItemList(item)
	if err != nil {
		return "", err
	}

	path, err := yaml.PathString("$." + field)
	if err != nil {
		return "", err
	}

	err = path.MergeFromReader(f, bytes.NewReader(itemListYAML))
	if err == nil {
		return f.String(), nil
	}

	if !yaml.IsNotFoundNodeError(err) {
		return "", err
	}

	// field does not exist yet. Add:
	//
	// field:
	//   - <item>
	rootFragment := field + ":\n" + indentYaml(string(itemListYAML))

	root, err := yaml.PathString("$")
	if err != nil {
		return "", err
	}

	if err := root.MergeFromReader(f, strings.NewReader(rootFragment)); err != nil {
		return "", err
	}

	return f.String(), nil
}

func marshalOneItemList(item string) ([]byte, error) {
	var value any
	if err := yaml.Unmarshal([]byte(item), &value); err != nil {
		return nil, err
	}
	return yaml.MarshalWithOptions([]any{value}, yaml.Indent(2))
}

func indentYaml(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := range lines {
		lines[i] = "  " + lines[i]
	}
	return strings.Join(lines, "\n") + "\n"
}

// ReplaceItemInList replaces the specified item in given field.
// item can be a simple value or a whole object
func ReplaceItemInList(raw []byte, field string, index int, replacement string) (string, error) {
	return replace(raw, fmt.Sprintf("$.%s[%d]", field, index), replacement)
}

func replace(raw []byte, path string, replacement string) (string, error) {
	f, err := parser.ParseBytes(raw, parser.ParseComments)
	if err != nil {
		return "", err
	}

	p, err := yaml.PathString(path)
	if err != nil {
		return "", err
	}

	if err := p.ReplaceWithReader(f, bytes.NewBufferString(replacement)); err != nil {
		return "", err
	}

	return f.String(), nil
}
