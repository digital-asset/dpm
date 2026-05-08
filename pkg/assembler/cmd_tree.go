package assembler

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"daml.com/x/assistant/pkg/builtincommand"
	"daml.com/x/assistant/pkg/component"
	"github.com/samber/lo"
)

type Node struct {
	Command  *ValidatedCommand
	Children []*Node
}

func (n *Node) path() []string {
	return n.Command.GetName()
}

func (n *Node) GroupByComponents() map[string][]*ValidatedCommand {
	return lo.GroupByMap(n.AsList(), func(cmd *ValidatedCommand) (string, *ValidatedCommand) {
		return cmd.ComponentName, cmd
	})
}

func (n *Node) AsList() []*ValidatedCommand {
	xs := lo.Map(flattenTree(n), func(item *Node, _ int) *ValidatedCommand {
		return item.Command
	})
	return xs[1:]
}

// BuildTree return a tree containing all the commands. The root node is a dummy
func BuildTree(entries []*ValidatedCommand) (*Node, error) {
	nodes := lo.Map(entries, func(e *ValidatedCommand, _ int) *Node {
		return &Node{
			Command: e,
		}
	})

	nodesByParent := lo.GroupBy(nodes, func(n *Node) string {
		return pathKey(parentPath(n.Command.GetName()))
	})

	root := &Node{
		// using a dummy command for root node
		Command: &ValidatedCommand{Command: &component.NativeCommand{}},
	}
	root.Children = buildChildren(root.path(), nodesByParent)

	// validations
	var errs []error
	commands := root.AsList()
	errs = append(errs, validateNoDuplicates(nodes)...)
	errs = append(errs, validateNoOrphanCommands(root, nodes)...)
	errs = append(errs, validateAliases(commands)...)
	errs = append(errs, validateConflictWithBuiltinCommands(commands)...)
	errs = append(errs, validateExecutablePaths(commands)...)
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return root, nil
}

func buildChildren(path []string, nodesByParent map[string][]*Node) []*Node {
	children := nodesByParent[pathKey(path)]

	for _, child := range children {
		child.Children = buildChildren(child.path(), nodesByParent)
	}

	return children
}

func pathKey(parts []string) string {
	return strings.Join(parts, " ")
}

func parentPath(path []string) []string {
	if len(path) == 0 {
		return nil
	}
	return path[:len(path)-1]
}

func validateNoDuplicates(nodes []*Node) []error {
	byPath := lo.GroupBy(nodes, func(n *Node) string {
		return pathKey(n.path())
	})

	dupeGroups := lo.Filter(lo.Values(byPath), func(group []*Node, _ int) bool {
		return len(group) > 1
	})

	return lo.Map(dupeGroups, func(group []*Node, _ int) error {
		return fmt.Errorf("command %v defined in multiple components", group[0].path())
	})
}

func flattenTree(root *Node) []*Node {
	out := []*Node{root}

	for _, child := range root.Children {
		out = append(out, flattenTree(child)...)
	}

	return out
}

func validateNoOrphanCommands(root *Node, nodes []*Node) []error {
	attached := lo.SliceToMap(flattenTree(root), func(n *Node) (string, bool) {
		return pathKey(n.path()), true
	})

	unattached := lo.Filter(nodes, func(n *Node, _ int) bool {
		return !attached[pathKey(n.path())]
	})

	return lo.Map(unattached, func(n *Node, _ int) error {
		return fmt.Errorf(
			"missing parent %v for path %v",
			parentPath(n.path()),
			n.path(),
		)
	})
}

func validateAliases(commands []*ValidatedCommand) []error {
	var errs []error

	aliases := lo.FlatMap(commands, func(c *ValidatedCommand, _ int) []lo.Entry[string, string] {
		return lo.Map(c.GetAliases(), func(alias string, _ int) lo.Entry[string, string] {
			return lo.Entry[string, string]{
				Key:   alias,
				Value: c.ComponentName,
			}
		})
	})
	groupedByAlias := lo.GroupByMap(aliases, func(p lo.Entry[string, string]) (string, string) {
		return p.Key, p.Value
	})
	for alias, comps := range groupedByAlias {
		if len(comps) > 1 {
			errs = append(errs, fmt.Errorf("command alias %q is used by multiple components %v", alias, comps))
		}
	}

	return errs
}

func validateConflictWithBuiltinCommands(commands []*ValidatedCommand) []error {
	var errs []error
	builtin := lo.SliceToMap(builtincommand.BuiltinCommands, func(b builtincommand.BuiltinCommand) (string, struct{}) {
		return string(b), struct{}{}
	})
	for _, cmd := range commands {
		_, ok := builtin[cmd.String()]
		if ok {
			errs = append(errs, fmt.Errorf("command named %q (from component %q) conflicts with the assistant's built-in commands", cmd.GetName(), cmd.ComponentName))
		}
	}
	return errs
}

func validateExecutablePaths(commands []*ValidatedCommand) []error {
	var errs []error

	uniqueByPath := lo.UniqBy(commands, func(cmd *ValidatedCommand) string { return cmd.AbsolutePath })
	for _, c := range uniqueByPath {
		errMsg := fmt.Sprintf("component %q command validation failed for command %q", c.ComponentName, c.GetName())
		f, err := os.Stat(c.AbsolutePath)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", errMsg, err))
			continue
		}
		if f.IsDir() {
			errs = append(errs, fmt.Errorf("%s: %q is a directory", errMsg, c.AbsolutePath))
		}
	}

	return errs
}
