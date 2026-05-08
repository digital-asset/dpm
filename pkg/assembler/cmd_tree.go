package assembler

import (
	"fmt"
	"strings"
)

type Node struct {
	Command  *ValidatedCommand
	Path     []string
	Children []*Node
}

func BuildTree(entries []*ValidatedCommand) (*Node, error) {
	root := &Node{
		Path: []string{},
	}

	nodesByKey := map[string]*Node{
		pathKey(nil): root,
	}

	maxDepth := 0

	for _, e := range entries {
		path := clone(e.GetName())

		if len(path) == 0 {
			return nil, fmt.Errorf("empty path is reserved for root")
		}

		k := pathKey(path)
		if _, exists := nodesByKey[k]; exists {
			return nil, fmt.Errorf("duplicate entry for path %v", path)
		}

		nodesByKey[k] = &Node{
			Command: e,
			Path:    path,
		}

		if len(path) > maxDepth {
			maxDepth = len(path)
		}
	}

	for depth := 1; depth <= maxDepth; depth++ {
		for _, e := range entries {
			path := e.GetName()
			if len(path) != depth {
				continue
			}

			parentPath := path[:len(path)-1]

			parent, ok := nodesByKey[pathKey(parentPath)]
			if !ok {
				return nil, fmt.Errorf("missing parent %v for path %v", parentPath, path)
			}

			node := nodesByKey[pathKey(path)]
			parent.Children = append(parent.Children, node)
		}
	}

	return root, nil
}

func pathKey(parts []string) string {
	return strings.Join(parts, "\x00")
}

func clone(xs []string) []string {
	if xs == nil {
		return nil
	}
	out := make([]string, len(xs))
	copy(out, xs)
	return out
}
