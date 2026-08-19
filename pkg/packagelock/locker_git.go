package packagelock

import (
	"context"

	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/gitpuller"
)

func gitPinsFromExistingLock(lockfilePath string) map[string]string {
	lock, err := ReadPackageLock(lockfilePath)
	if err != nil {
		return nil
	}
	pins := make(map[string]string)
	for _, d := range lock.Dars {
		if d.URI == nil || d.URI.Scheme != "git" {
			continue
		}
		ref := damlpackage.GitRefFromURI(d.URI)
		if damlpackage.GitRefIsMutable(ref) {
			continue
		}
		pins[damlpackage.GitLockKey(d.URI)] = ref
	}
	return pins
}

func gitDependencyForUpdate(dep *damlpackage.ParsedDarDependency, existingPins map[string]string) *damlpackage.ParsedDarDependency {
	if dep == nil || dep.FullUrl == nil || !damlpackage.GitRefIsMutable(dep.GitRef) {
		return dep
	}
	pinned, ok := existingPins[damlpackage.GitLockKey(dep.FullUrl)]
	if !ok || damlpackage.GitRefIsMutable(pinned) {
		return dep
	}
	return dep.WithGitRef(pinned)
}

func (l *Locker) resolveGitDar(ctx context.Context, d *Dar, existingPins map[string]string) error {
	dep := gitDependencyForUpdate(d.Dependency, existingPins)
	pulled, err := gitpuller.New(l.config).PullDar(ctx, dep)
	if err != nil {
		return err
	}
	d.Digest = pulled.Digest
	d.Path = pulled.DarFilePath
	pinned, err := damlpackage.PinnedGitURI(dep, pulled.ResolvedRef)
	if err != nil {
		return err
	}
	d.URI = pinned
	return nil
}
