package packagelock

import (
	"context"
	"fmt"
	"strings"

	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/gitparse"
	"daml.com/x/assistant/pkg/gitpuller"
)

// gitPinReuse maps a ref-free GitLockKey to one locked commit.
// A key is ambiguous when that path has more than one locked commit, or when
// daml.yaml declares more than one ref that would reuse the only commit.
// Ambiguous keys are not reused.
type gitPinReuse struct {
	pins      map[string]string
	ambiguous map[string]struct{}
}

func newGitPinReuse() gitPinReuse {
	return gitPinReuse{
		pins:      map[string]string{},
		ambiguous: map[string]struct{}{},
	}
}

func gitPinReuseFor(lockfilePath string, expected []*Dar) gitPinReuse {
	reuse := gitPinsFromExistingLock(lockfilePath)
	reuse.disallowSharedMutablePins(expected)
	return reuse
}

func gitPinsFromExistingLock(lockfilePath string) gitPinReuse {
	lock, err := ReadPackageLock(lockfilePath)
	if err != nil {
		return newGitPinReuse()
	}
	return gitPinsFromLock(lock)
}

func gitPinsFromLock(lock *PackageLock) gitPinReuse {
	reuse := newGitPinReuse()
	if lock == nil {
		return reuse
	}
	for _, d := range lock.Dars {
		if d == nil || d.URI == nil || d.URI.Scheme != "git" {
			continue
		}
		ref := gitparse.GitRefFromURI(d.URI)
		if gitparse.GitRefIsMutable(ref) {
			continue
		}
		reuse.add(gitparse.GitLockKey(d.URI), ref)
	}
	return reuse
}

func (r gitPinReuse) add(key, ref string) {
	if _, ok := r.ambiguous[key]; ok {
		return
	}
	if prev, ok := r.pins[key]; ok && prev != ref {
		delete(r.pins, key)
		r.ambiguous[key] = struct{}{}
		return
	}
	r.pins[key] = ref
}

// disallowSharedMutablePins drops a reusable pin when daml.yaml declares more
// than one ref for the same repository path. With no pin recorded, each ref is
// left to resolve on its own.
func (r gitPinReuse) disallowSharedMutablePins(expected []*Dar) {
	refsByKey := map[string]map[string]struct{}{}
	for _, d := range expected {
		if d == nil || d.Dependency == nil || d.Dependency.FullUrl == nil || d.Dependency.FullUrl.Scheme != "git" {
			continue
		}
		key := gitparse.GitLockKey(d.Dependency.FullUrl)
		if refsByKey[key] == nil {
			refsByKey[key] = map[string]struct{}{}
		}
		refsByKey[key][d.Dependency.Git.Ref] = struct{}{}
	}
	for key, refs := range refsByKey {
		if len(refs) < 2 {
			continue
		}
		_, pinned := r.pins[key]
		_, ambiguous := r.ambiguous[key]
		if !pinned && !ambiguous {
			continue
		}
		delete(r.pins, key)
		r.ambiguous[key] = struct{}{}
	}
}

func ambiguousGitPinsError(expected []*Dar, reuse gitPinReuse) error {
	for _, d := range expected {
		if d == nil {
			continue
		}
		if _, err := gitDependencyForUpdate(d.Dependency, reuse); err != nil {
			return err
		}
	}
	return nil
}

func gitDependencyForUpdate(dep *damlpackage.ParsedDarDependency, reuse gitPinReuse) (*damlpackage.ParsedDarDependency, error) {
	if dep == nil || dep.FullUrl == nil || !gitparse.GitRefIsMutable(dep.Git.Ref) {
		return dep, nil
	}
	key := gitparse.GitLockKey(dep.FullUrl)
	if _, ambiguous := reuse.ambiguous[key]; ambiguous {
		return nil, ambiguousGitPinError(dep)
	}
	pinned, ok := reuse.pins[key]
	if !ok || gitparse.GitRefIsMutable(pinned) {
		return dep, nil
	}
	return dep.WithGitRef(pinned), nil
}

func ambiguousGitPinError(dep *damlpackage.ParsedDarDependency) error {
	return fmt.Errorf(
		"git dependency %q shares %s in %s with another ref; pin each ref to a commit SHA in daml.yaml",
		gitparse.FormatGitYamlLine(dep.Git),
		dep.Git.DarPath,
		gitRepoLabel(dep),
	)
}

func gitRepoLabel(dep *damlpackage.ParsedDarDependency) string {
	if dep == nil || dep.Git.CloneURL == nil {
		return ""
	}
	u := dep.Git.CloneURL
	return strings.TrimSuffix(u.Host+u.EscapedPath(), ".git")
}

func (l *Locker) resolveGitDar(ctx context.Context, d *Dar, reuse gitPinReuse) error {
	dep, err := gitDependencyForUpdate(d.Dependency, reuse)
	if err != nil {
		return err
	}
	pulled, err := gitpuller.New(l.config).PullDar(ctx, dep)
	if err != nil {
		return err
	}
	d.Digest = pulled.Digest
	d.Path = pulled.DarFilePath
	pinned, err := gitparse.PinnedGitURI(dep.FullUrl, dep.Git, pulled.ResolvedRef)
	if err != nil {
		return err
	}
	d.URI = pinned
	return nil
}
