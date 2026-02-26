package packagelock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/darpuller"
	"daml.com/x/assistant/pkg/multipackage"
	"daml.com/x/assistant/pkg/schema"
	"github.com/goccy/go-yaml"
	"github.com/samber/lo"
	"oras.land/oras-go/v2/registry"
)

var ErrLockfileOutOfSync = errors.New(assistantconfig.DpmLockFileName + " needs to be updated; please run 'dpm update'")

type Locker struct {
	config *assistantconfig.Config
	op     Operation
}

type Operation int

const (
	CheckOnly Operation = iota
	Regular
)

func New(config *assistantconfig.Config, op Operation) *Locker {
	return &Locker{config: config, op: op}
}

func (l *Locker) EnsureLockfiles(ctx context.Context) (map[string]*PackageLock, error) {
	// multi-package
	multiPackagePath, isMultiPackage, err := assistantconfig.GetMultiPackageAbsolutePath()
	if err != nil {
		return nil, fmt.Errorf("failed to determine whether a multi-package is in scope: %w", err)
	}
	if isMultiPackage {
		multiPackage, err := multipackage.Read(multiPackagePath)
		if err != nil {
			return nil, err
		}
		return l.ensureLockfiles(ctx, multiPackage.AbsolutePackages()...)
	}

	// single package
	damlPackagePath, isDamlPackage, err := assistantconfig.GetDamlPackageAbsolutePath()
	if err != nil {
		return nil, err
	}
	if isDamlPackage {
		return l.ensureLockfiles(ctx, filepath.Dir(damlPackagePath))
	}

	// no packages
	return make(map[string]*PackageLock), nil
}

func (l *Locker) ensureLockfiles(ctx context.Context, packages ...string) (map[string]*PackageLock, error) {
	m := map[string]*PackageLock{}
	var errs []error

	for _, p := range packages {
		result, err := l.EnsureLockfile(ctx, p)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		m[p] = result
	}

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return m, nil
}

func (l *Locker) EnsureLockfile(ctx context.Context, packageDirAbsPath string) (*PackageLock, error) {
	expectedLockfile, err := computeExpectedLockfile(packageDirAbsPath)
	if err != nil {
		return nil, err
	}
	lockfilePath := filepath.Join(packageDirAbsPath, assistantconfig.DpmLockFileName)

	if l.op == CheckOnly {
		return nil, l.checkLockfile(expectedLockfile, lockfilePath)
	}

	return l.create(ctx, expectedLockfile, lockfilePath)
}

func (l *Locker) checkLockfile(expectedLockfile *PackageLock, lockfilePath string) error {
	existingLockfile, err := ReadPackageLock(lockfilePath)
	if os.IsNotExist(err) {
		return fmt.Errorf("%w: %w", ErrLockfileOutOfSync, err)
	}
	if err != nil {
		return err
	}

	inSync, err := existingLockfile.isInSync(expectedLockfile)
	if err != nil {
		return err
	}

	if inSync {
		return nil
	}

	return ErrLockfileOutOfSync
}

func (l *Locker) create(ctx context.Context, expected *PackageLock, lockfilePath string) (*PackageLock, error) {
	for _, d := range expected.Dars {
		descriptor, version, _, err := darpuller.New(l.config).PullDar(ctx, d.Dependency)
		if err != nil {
			return nil, err
		}
		d.Digest = descriptor.Digest.String()

		ref, err := registry.ParseReference(strings.TrimPrefix(d.URI, "oci://"))
		if err != nil {
			return nil, err
		}

		// TODO this doesn't work for @sha256 pinned refs
		resolvedRef := ":" + version.String()
		d.URI = fmt.Sprintf("oci://%s/%s%s", ref.Registry, ref.Repository, resolvedRef)
	}

	data, err := yaml.Marshal(expected)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(lockfilePath, data, 0644); err != nil {
		return nil, err
	}
	return expected, nil
}

func computeExpectedLockfile(packageDirAbsPath string) (*PackageLock, error) {
	p, err := damlpackage.Read(filepath.Join(packageDirAbsPath, assistantconfig.DamlPackageFilename))
	if err != nil {
		return nil, err
	}

	// TODO de-duplicate p.ResolvedDependencies first
	expectedDars := lo.MapToSlice(p.ResolvedDependencies, func(_ string, d *damlpackage.ResolvedDependency) *Dar {
		return &Dar{
			URI:        d.FullUrl.String(),
			Dependency: d,

			// TODO diff digests too
			// Digest:
		}
	})
	slices.SortFunc(expectedDars, func(a, b *Dar) int {
		return strings.Compare(a.URI, b.URI)
	})

	return &PackageLock{
		ManifestMeta: schema.ManifestMeta{
			APIVersion: PackageLockAPIVersion,
			Kind:       PackageLockKind,
		},
		Dars: expectedDars,
	}, nil
}
