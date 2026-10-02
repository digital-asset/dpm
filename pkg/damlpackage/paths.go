package damlpackage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RejectSymlinkOutsideRoot rejects paths that resolve outside root via symlinks.
func RejectSymlinkOutsideRoot(root, path string) error {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("failed to resolve symlink %s: %w", path, err)
	}

	resolvedAbs, err := filepath.Abs(resolved)
	if err != nil {
		return err
	}
	if !pathWithinRoot(resolvedAbs, root) {
		return fmt.Errorf("symlink points outside the repository: %s -> %s", path, resolvedAbs)
	}
	return nil
}

func pathWithinRoot(target, root string) bool {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	rootResolved, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		rootResolved = rootAbs
	}

	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	targetResolved, err := filepath.EvalSymlinks(targetAbs)
	if err != nil {
		targetResolved = targetAbs
	}

	if targetResolved == rootResolved {
		return true
	}
	return strings.HasPrefix(targetResolved, rootResolved+string(os.PathSeparator))
}
