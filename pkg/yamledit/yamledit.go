package yamledit

import (
	"fmt"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/parser"
)

type YamlTarget struct {
	YamlFilePath string
	FieldName    string
	Index        int

	LineComment string
}

func (t *YamlTarget) Copy() YamlTarget {
	return YamlTarget{
		YamlFilePath: t.YamlFilePath,
		FieldName:    t.FieldName,
		Index:        t.Index,
		LineComment:  t.LineComment,
	}
}

// EditYaml adds an item to list in yaml file
// or replace the given index with it
func EditYaml(info YamlTarget, item string) error {
	b, err := os.ReadFile(info.YamlFilePath)
	if err != nil {
		return err
	}

	item = attachLineComment(item, info.LineComment)

	var out string
	if info.Index != -1 {
		out, err = ReplaceItemInList(b, info.FieldName, info.Index, item)
	} else {
		out, err = AddToList(b, info.FieldName, item)
	}
	if err != nil {
		return err
	}

	return os.WriteFile(info.YamlFilePath, []byte(out), 0644)
}

// AddToList adds item to the given target field.
// item can be a simple value or a YAML object.
func AddToList(raw []byte, field string, item string) (string, error) {
	f, err := parser.ParseBytes(raw, parser.ParseComments)
	if err != nil {
		return "", err
	}

	itemListYAML, err := marshalOneItemList(item)
	if err != nil {
		return "", err
	}

	itemListFile, err := parser.ParseBytes(itemListYAML, parser.ParseComments)
	if err != nil {
		return "", err
	}

	path, err := yaml.PathString("$." + field)
	if err != nil {
		return "", err
	}

	err = path.MergeFromFile(f, itemListFile)
	if err == nil {
		return f.String(), nil
	}

	if !yaml.IsNotFoundNodeError(err) {
		return "", err
	}

	// field does not exist yet. Add:
	//
	// field:
	//   - <item> # <comment>
	rootFragment := field + ":\n" + indentYaml(string(itemListYAML))

	rootFragmentFile, err := parser.ParseBytes([]byte(rootFragment), parser.ParseComments)
	if err != nil {
		return "", err
	}

	root, err := yaml.PathString("$")
	if err != nil {
		return "", err
	}

	if err := root.MergeFromFile(f, rootFragmentFile); err != nil {
		return "", err
	}

	return f.String(), nil
}

func marshalOneItemList(item string) ([]byte, error) {
	item = normalizeYAMLFragment(item)
	if strings.TrimSpace(item) == "" {
		return nil, fmt.Errorf("empty YAML item")
	}

	// Validate the raw item first.
	if _, err := parser.ParseBytes([]byte(item), parser.ParseComments); err != nil {
		return nil, fmt.Errorf("invalid YAML item: %w", err)
	}

	lines := strings.Split(strings.TrimRight(item, "\n"), "\n")
	out := make([]string, len(lines))

	out[0] = "- " + lines[0]
	for i := 1; i < len(lines); i++ {
		out[i] = "  " + lines[i]
	}

	seq := []byte(strings.Join(out, "\n") + "\n")

	// Validate the generated one-item sequence too.
	if _, err := parser.ParseBytes(seq, parser.ParseComments); err != nil {
		return nil, fmt.Errorf("invalid YAML list item: %w", err)
	}

	return seq, nil
}

func normalizeYAMLFragment(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.Trim(s, "\n")
	return s + "\n"
}

func indentYaml(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := range lines {
		lines[i] = "  " + lines[i]
	}
	return strings.Join(lines, "\n") + "\n"
}

func attachLineComment(item, comment string) string {
	comment = strings.TrimSpace(comment)
	if comment == "" {
		return item
	}

	comment = strings.TrimSpace(strings.TrimPrefix(comment, "#"))
	if comment == "" {
		return item
	}

	item = strings.TrimRight(item, "\n")
	lines := strings.Split(item, "\n")
	lines[0] = strings.TrimRight(lines[0], " \t") + " # " + comment
	return strings.Join(lines, "\n")
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

	replacement = normalizeYAMLFragment(replacement)

	replacementFile, err := parser.ParseBytes([]byte(replacement), parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("invalid YAML replacement: %w", err)
	}

	if err := p.ReplaceWithFile(f, replacementFile); err != nil {
		return "", err
	}

	return f.String(), nil
}
