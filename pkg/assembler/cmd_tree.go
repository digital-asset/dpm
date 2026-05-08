package assembler

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"daml.com/x/assistant/pkg/builtincommand"
	"daml.com/x/assistant/pkg/component"
	"daml.com/x/assistant/pkg/utils"
	"github.com/samber/lo"
)

// ValidatedCommandNode is a tree where every node is a command.
// The root node is a placeholder dummy command
type ValidatedCommandNode struct {
	Command  *ValidatedCommand
	Children []*ValidatedCommandNode
}

func (n *ValidatedCommandNode) path() []string {
	return n.Command.GetName()
}

// GroupByComponents returns the command tree as a  {<component name> -> <command>} map
func (n *ValidatedCommandNode) GroupByComponents() map[string][]*ValidatedCommand {
	return lo.GroupByMap(n.AsList(), func(cmd *ValidatedCommand) (string, *ValidatedCommand) {
		return cmd.ComponentName, cmd
	})
}

// AsList returns all the commands in the tree as a list
func (n *ValidatedCommandNode) AsList() []*ValidatedCommand {
	xs := lo.Map(flattenTree(n), func(item *ValidatedCommandNode, _ int) *ValidatedCommand {
		return item.Command
	})
	return xs[1:]
}

// BuildCommandTree builds a tree from all the raw component commands and runs some validations
func BuildCommandTree(comps map[string]*ResolvedComponent) (*ValidatedCommandNode, error) {
	flatCommands := lo.MapValues(comps, func(comp *ResolvedComponent, _ string) []*ValidatedCommand {
		return lo.Map(comp.Spec.AllCommands(), func(c component.Command, _ int) *ValidatedCommand {
			return &ValidatedCommand{
				Command:       c,
				AbsolutePath:  utils.ResolvePath(comp.AbsolutePath, c.GetPath()),
				ComponentName: comp.ComponentName,
			}
		})
	})

	return buildCommandTree(lo.Flatten(lo.Values(flatCommands)))
}

func buildCommandTree(entries []*ValidatedCommand) (*ValidatedCommandNode, error) {
	nodes := lo.Map(entries, func(e *ValidatedCommand, _ int) *ValidatedCommandNode {
		return &ValidatedCommandNode{
			Command: e,
		}
	})

	nodesByParent := lo.GroupBy(nodes, func(n *ValidatedCommandNode) string {
		return pathKey(parentPath(n.Command.GetName()))
	})

	root := &ValidatedCommandNode{
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

func buildChildren(path []string, nodesByParent map[string][]*ValidatedCommandNode) []*ValidatedCommandNode {
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

func validateNoDuplicates(nodes []*ValidatedCommandNode) []error {
	byPath := lo.GroupBy(nodes, func(n *ValidatedCommandNode) string {
		return pathKey(n.path())
	})

	dupeGroups := lo.Filter(lo.Values(byPath), func(group []*ValidatedCommandNode, _ int) bool {
		return len(group) > 1
	})

	return lo.Map(dupeGroups, func(group []*ValidatedCommandNode, _ int) error {
		return fmt.Errorf("command %v defined in multiple components", group[0].path())
	})
}

func flattenTree(root *ValidatedCommandNode) []*ValidatedCommandNode {
	out := []*ValidatedCommandNode{root}

	for _, child := range root.Children {
		out = append(out, flattenTree(child)...)
	}

	return out
}

func validateNoOrphanCommands(root *ValidatedCommandNode, nodes []*ValidatedCommandNode) []error {
	attached := lo.SliceToMap(flattenTree(root), func(n *ValidatedCommandNode) (string, bool) {
		return pathKey(n.path()), true
	})

	unattached := lo.Filter(nodes, func(n *ValidatedCommandNode, _ int) bool {
		return !attached[pathKey(n.path())]
	})

	return lo.Map(unattached, func(n *ValidatedCommandNode, _ int) error {
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
