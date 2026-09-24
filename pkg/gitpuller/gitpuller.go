package gitpuller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/githubrelease"
	"daml.com/x/assistant/pkg/gitparse"
	"daml.com/x/assistant/pkg/utils"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
)

type refKind int

const (
	refKindBranch refKind = iota
	refKindTag
)

type GitDarPuller struct {
	config *assistantconfig.Config
}

type PulledGitDar struct {
	ResolvedRef string
	DarFilePath string
	Digest      string
}

func New(config *assistantconfig.Config) *GitDarPuller {
	return &GitDarPuller{config: config}
}

// DarIsCached reports whether the local cache already contains the .dar for this dependency.
func DarIsCached(config *assistantconfig.Config, dep *damlpackage.ParsedDarDependency) bool {
	if config == nil || dep == nil || dep.Git.CloneURL == nil {
		return false
	}
	var cachedPath string
	var err error
	if dep.Git.Release {
		cachedPath, err = config.CachePathForGitRelease(dep.Git.CloneURL, dep.Git.Ref, dep.Git.DarPath)
	} else {
		cachedPath, err = config.CachePathForGitDependency(dep.Git.CloneURL, dep.Git.DarPath, dep.Git.Ref)
	}
	if err != nil {
		return false
	}
	info, err := os.Stat(cachedPath)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular() && info.Size() > 0
}

// PullGitDarResult holds the outcome of resolving a git dependency.
type PullGitDarResult struct {
	Pinned *damlpackage.ParsedDarDependency
	Pulled *PulledGitDar
}

// PullGitDar pulls the dar and validates or computes commit pins.
func PullGitDar(ctx context.Context, config *assistantconfig.Config, dep *damlpackage.ParsedDarDependency) (*PullGitDarResult, error) {
	pulled, err := New(config).PullDar(ctx, dep)
	if err != nil {
		return nil, err
	}

	if !gitparse.GitRefIsMutable(dep.Git.Ref) && dep.Git.Ref != pulled.ResolvedRef {
		return nil, gitparse.GitPinMismatchError(dep.Git, pulled.ResolvedRef)
	}

	var pinned *damlpackage.ParsedDarDependency
	if gitparse.GitRefIsMutable(dep.Git.Ref) && !dep.Git.Release {
		pinned = dep.WithGitRef(pulled.ResolvedRef)
	}

	return &PullGitDarResult{Pinned: pinned, Pulled: pulled}, nil
}

func (p *GitDarPuller) PullDar(ctx context.Context, dep *damlpackage.ParsedDarDependency) (*PulledGitDar, error) {
	if dep == nil || dep.Git.CloneURL == nil {
		return nil, fmt.Errorf("invalid git dependency: missing clone URL")
	}
	if dep.Git.Release {
		_, _ = fmt.Fprintf(os.Stderr, "Resolving git release: downloading %s\n", gitparse.DescribeGitFetch(dep.Git))
		return p.pullReleaseDar(ctx, dep)
	}

	if !gitparse.GitRefIsMutable(dep.Git.Ref) {
		cachedDar, err := p.config.CachePathForGitDependency(dep.Git.CloneURL, dep.Git.DarPath, dep.Git.Ref)
		if err != nil {
			return nil, err
		}
		if DarIsCached(p.config, dep) {
			digest, err := fileDigest(cachedDar)
			if err != nil {
				return nil, err
			}
			return &PulledGitDar{
				ResolvedRef: dep.Git.Ref,
				DarFilePath: cachedDar,
				Digest:      digest,
			}, nil
		}
	}

	_, _ = fmt.Fprintf(os.Stderr, "Resolving git dependency: fetching %s\n", gitparse.DescribeGitFetch(dep.Git))

	cloneURL := dep.Git.CloneURL.String()
	workBase, err := p.config.GitWorkPathForRepo(dep.Git.CloneURL)
	if err != nil {
		return nil, err
	}

	if err := utils.EnsureDirs(workBase); err != nil {
		return nil, err
	}

	repo, commitSHA, err := p.cloneOrOpen(ctx, workBase, cloneURL, dep.Git.Ref)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch git repository %s (ref %q): %w", cloneURL, dep.Git.Ref, err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return nil, err
	}

	sourceDar, err := gitparse.JoinRepoRelativeDarPath(worktree.Filesystem.Root(), dep.Git.DarPath)
	if err != nil {
		return nil, fmt.Errorf("git dependency %q: %w", dep.Git.DarPath, err)
	}
	if err := damlpackage.RejectSymlinkOutsideRoot(worktree.Filesystem.Root(), sourceDar); err != nil {
		return nil, fmt.Errorf("git dependency %q: %w", dep.Git.DarPath, err)
	}
	info, err := os.Stat(sourceDar)
	if err != nil {
		return nil, fmt.Errorf(
			"dar file %q not found at commit %s in %s: %w; ensure the repository contains a pre-built .dar at that path",
			dep.Git.DarPath, commitSHA, cloneURL, err,
		)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("dar path %q at commit %s in %s is a directory, expected a .dar file", dep.Git.DarPath, commitSHA, cloneURL)
	}
	if info.Size() == 0 {
		return nil, fmt.Errorf(
			"dar file %q at commit %s in %s is empty; ensure the repository contains a non-empty pre-built .dar at that path",
			dep.Git.DarPath, commitSHA, cloneURL,
		)
	}

	cachedDar, err := p.config.CachePathForGitDependency(dep.Git.CloneURL, dep.Git.DarPath, commitSHA)
	if err != nil {
		return nil, err
	}
	if err := utils.EnsureDirs(filepath.Dir(cachedDar)); err != nil {
		return nil, err
	}
	if err := utils.AtomicCopyFile(sourceDar, cachedDar); err != nil {
		return nil, fmt.Errorf("failed to cache dar from git: %w", err)
	}

	digest, err := fileDigest(cachedDar)
	if err != nil {
		return nil, err
	}

	return &PulledGitDar{
		ResolvedRef: commitSHA,
		DarFilePath: cachedDar,
		Digest:      digest,
	}, nil
}

func (p *GitDarPuller) pullReleaseDar(ctx context.Context, dep *damlpackage.ParsedDarDependency) (*PulledGitDar, error) {
	asset := strings.TrimSpace(dep.Git.DarPath)
	if asset == "" {
		return nil, fmt.Errorf(
			"git release %q has no asset; run dpm update to expand release dependencies",
			dep.Git.Ref,
		)
	}

	cachedDar, err := p.config.CachePathForGitRelease(dep.Git.CloneURL, dep.Git.Ref, asset)
	if err != nil {
		return nil, err
	}
	if err := utils.EnsureDirs(filepath.Dir(cachedDar)); err != nil {
		return nil, err
	}

	if DarIsCached(p.config, dep) {
		digest, err := fileDigest(cachedDar)
		if err != nil {
			return nil, err
		}
		return &PulledGitDar{
			ResolvedRef: dep.Git.Ref,
			DarFilePath: cachedDar,
			Digest:      digest,
		}, nil
	}

	darPath, err := githubrelease.DownloadAsset(ctx, dep.Git.CloneURL, dep.Git.Ref, asset, filepath.Dir(cachedDar))
	if err != nil {
		return nil, fmt.Errorf("failed to download release asset %q (release %q): %w", asset, dep.Git.Ref, err)
	}
	if darPath != cachedDar {
		if err := utils.AtomicCopyFile(darPath, cachedDar); err != nil {
			return nil, err
		}
	}

	digest, err := fileDigest(cachedDar)
	if err != nil {
		return nil, err
	}

	return &PulledGitDar{
		ResolvedRef: dep.Git.Ref,
		DarFilePath: cachedDar,
		Digest:      digest,
	}, nil
}

func (p *GitDarPuller) cloneOrOpen(ctx context.Context, workBase, cloneURL, ref string) (*git.Repository, string, error) {
	repoPath := filepath.Join(workBase, "repo")
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err == nil {
		repo, err := git.PlainOpen(repoPath)
		if err != nil {
			return nil, "", err
		}
		if !strings.HasPrefix(cloneURL, "file://") {
			if plumbing.IsHash(ref) {
				if err := p.ensureCommitPresent(ctx, repo, cloneURL, ref); err != nil {
					return nil, "", err
				}
			} else if err := p.fetchRef(ctx, repo, cloneURL, ref); err != nil {
				return nil, "", err
			}
		}
		commitSHA, err := resolveCommit(repo, ref)
		if err != nil {
			return nil, "", err
		}
		if err := checkoutCommit(repo, commitSHA); err != nil {
			return nil, "", err
		}
		return repo, commitSHA, nil
	}

	cloneOpts := &git.CloneOptions{URL: cloneURL, Depth: 1}
	if !plumbing.IsHash(ref) && ref != "HEAD" {
		kind, err := resolveRefType(ctx, cloneURL, ref)
		if err != nil {
			return nil, "", err
		}
		cloneOpts.SingleBranch = true
		switch kind {
		case refKindBranch:
			cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(ref)
		case refKindTag:
			cloneOpts.ReferenceName = plumbing.NewTagReferenceName(ref)
		}
	}

	repo, err := git.PlainCloneContext(ctx, repoPath, false, cloneOpts)
	if err != nil {
		_ = os.RemoveAll(repoPath)
		return nil, "", err
	}
	if !strings.HasPrefix(cloneURL, "file://") && plumbing.IsHash(ref) {
		if err := p.ensureCommitPresent(ctx, repo, cloneURL, ref); err != nil {
			return nil, "", err
		}
	}

	commitSHA, err := resolveCommit(repo, ref)
	if err != nil {
		return nil, "", err
	}
	if err := checkoutCommit(repo, commitSHA); err != nil {
		return nil, "", err
	}

	return repo, commitSHA, nil
}

// resolveRefType classifies a remote ref as a branch or tag (like git ls-remote).
func resolveRefType(ctx context.Context, cloneURL, ref string) (refKind, error) {
	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{cloneURL},
	})

	refs, err := remote.ListContext(ctx, &git.ListOptions{})
	if err != nil {
		return 0, fmt.Errorf("couldn't list remote refs for %q: %w", ref, err)
	}

	branchName := plumbing.NewBranchReferenceName(ref)
	tagName := plumbing.NewTagReferenceName(ref)

	var hasBranch, hasTag bool
	for _, r := range refs {
		switch r.Name() {
		case branchName:
			hasBranch = true
		case tagName:
			hasTag = true
		}
	}

	if hasBranch {
		return refKindBranch, nil
	}
	if hasTag {
		return refKindTag, nil
	}
	return 0, fmt.Errorf("couldn't find remote ref %q as branch or tag", ref)
}

func (p *GitDarPuller) ensureCommitPresent(ctx context.Context, repo *git.Repository, cloneURL, commit string) error {
	if _, err := repo.CommitObject(plumbing.NewHash(commit)); err == nil {
		return nil
	}
	return p.fetchCommitHash(ctx, repo, cloneURL, commit)
}

func (p *GitDarPuller) fetchCommitHash(ctx context.Context, repo *git.Repository, cloneURL, commit string) error {
	remote, err := repo.Remote("origin")
	if err != nil {
		return err
	}
	spec := config.RefSpec(fmt.Sprintf("+%s:refs/dpm/pinned/%s", commit, commit))
	err = remote.FetchContext(ctx, &git.FetchOptions{
		RemoteURL: cloneURL,
		RefSpecs:  []config.RefSpec{spec},
		Depth:     1,
	})
	if !isFetchOK(err) {
		return fmt.Errorf("couldn't fetch commit %q: %w", commit, err)
	}
	return nil
}

func (p *GitDarPuller) fetchRef(ctx context.Context, repo *git.Repository, cloneURL, ref string) error {
	if plumbing.IsHash(ref) {
		return p.fetchCommitHash(ctx, repo, cloneURL, ref)
	}

	kind, err := resolveRefType(ctx, cloneURL, ref)
	if err != nil {
		return err
	}

	var spec config.RefSpec
	switch kind {
	case refKindBranch:
		spec = config.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/heads/%s", ref, ref))
	case refKindTag:
		spec = config.RefSpec(fmt.Sprintf("+refs/tags/%s:refs/tags/%s", ref, ref))
	default:
		return fmt.Errorf("couldn't classify remote ref %q", ref)
	}

	remote, err := repo.Remote("origin")
	if err != nil {
		return err
	}
	err = remote.FetchContext(ctx, &git.FetchOptions{
		RemoteURL: cloneURL,
		RefSpecs:  []config.RefSpec{spec},
		Depth:     1,
	})
	if !isFetchOK(err) {
		return fmt.Errorf("couldn't fetch remote ref %q: %w", ref, err)
	}
	return nil
}

func isFetchOK(err error) bool {
	return err == nil || errors.Is(err, git.NoErrAlreadyUpToDate)
}

func resolveCommit(repo *git.Repository, ref string) (string, error) {
	if plumbing.IsHash(ref) {
		return ref, nil
	}

	hash, err := repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return "", fmt.Errorf("couldn't resolve ref %q: %w", ref, err)
	}
	return hash.String(), nil
}

func checkoutCommit(repo *git.Repository, commitSHA string) error {
	worktree, err := repo.Worktree()
	if err != nil {
		return err
	}
	hash := plumbing.NewHash(commitSHA)
	return worktree.Checkout(&git.CheckoutOptions{Hash: hash, Force: true})
}

func fileDigest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
