package packagelock

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"daml.com/x/assistant/cmd/dpm/cmd/resolve/resolutionerrors"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/darpuller"
	"daml.com/x/assistant/pkg/multipackage"
	ociconsts "daml.com/x/assistant/pkg/oci"
	"daml.com/x/assistant/pkg/ociindex"
	"daml.com/x/assistant/pkg/ocilister"
	"daml.com/x/assistant/pkg/schema"
	"daml.com/x/assistant/pkg/versions"
	"github.com/goccy/go-yaml"
	"github.com/samber/lo"
	"oras.land/oras-go/v2/registry"
)

var ErrLockfileOutOfSync = resolutionerrors.NewOutdatedLockfileError(
	errors.New(assistantconfig.DpmLockFileName + " needs to be updated; please run 'dpm update'"),
)

type Locker struct {
	config *assistantconfig.Config
	op     Operation
	remote *assistantremote.Remote // Needed for resolving floaty versions
}

type Operation int

const (
	CheckOnly Operation = iota
	Regular             // resolves only, doesn't install!!
)

func New(config *assistantconfig.Config, remote *assistantremote.Remote, op Operation) *Locker {
	return &Locker{config: config, remote: remote, op: op}
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
		_, err = l.EnsureMultiPackageLockfile(ctx, filepath.Dir(multiPackagePath))
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
	expectedLockfile, err := l.computeExpectedLockfile(packageDirAbsPath)
	if err != nil {
		return nil, err
	}

	lockfilePath := filepath.Join(packageDirAbsPath, assistantconfig.DpmLockFileName)

	if l.op == CheckOnly {
		return nil, l.checkLockfile(expectedLockfile, lockfilePath)
	}
	return l.create(ctx, expectedLockfile, lockfilePath)
}

func (l *Locker) EnsureMultiPackageLockfile(ctx context.Context, multiPkgDirAbsPath string) (*PackageLock, error) {
	expectedLockfile, err := l.computeMultiExpectedLockfile()
	if err != nil {
		return nil, err
	}
	lockfilePath := filepath.Join(multiPkgDirAbsPath, assistantconfig.DpmMultiPackageLockFileName)
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

// create mutates expected floaty references (if any) to resolved references, and will populate the digests
func (l *Locker) create(ctx context.Context, expected *PackageLock, lockfilePath string) (*PackageLock, error) {
	// sdk-version
	sdkVersion, err := l.lockSdkVersion(ctx, expected.SdkVersion)
	if err != nil {
		return nil, err
	}
	expected.SdkVersion = sdkVersion

	// dars
	for _, d := range expected.Dars {
		if d.URI.Scheme == "builtin" {
			d.Path = d.URI.Host
			continue
		}

		// TODO resolve only, don't pull the full dar
		pulledDar, err := darpuller.New(l.config).PullDar(ctx, d.Dependency)
		if err != nil {
			return nil, err
		}
		d.Digest = pulledDar.Descriptor.Digest.String()

		ref, err := registry.ParseReference(strings.TrimPrefix(d.URI.String(), "oci://"))
		if err != nil {
			return nil, err
		}

		// TODO this doesn't work for @sha256 pinned refs
		resolvedRef := ":" + pulledDar.Version.String()
		d.URI = resolvedRefToURI(ref.Registry, ref.Repository, resolvedRef)
		d.Path = pulledDar.DarFilePath
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

// lockSdkVersion will resolve sdk version (if floaty) and return the populated (strict) semver
func (l *Locker) lockSdkVersion(ctx context.Context, expectedSdkVersion SdkVersion) (SdkVersion, error) {
	// the no-sdk case
	if expectedSdkVersion.Version == "" {
		return SdkVersion{
			Version: "",
		}, nil
	}

	if !ocilister.IsFloaty(expectedSdkVersion.Version) {
		return SdkVersion{
			Version: expectedSdkVersion.Version,
		}, nil
	}

	// resolve floaty version
	repoName, err := l.config.SdkManifestsRepo()
	if err != nil {
		return SdkVersion{}, err
	}

	artifact := &ociconsts.SdkManifestArtifact{
		SdkManifestsRepo: repoName,
	}
	resolvedVersion, err := ociindex.ResolveTag(ctx, l.remote, artifact, expectedSdkVersion.Version)
	if err != nil {
		return SdkVersion{}, err
	}

	return SdkVersion{
		Version: resolvedVersion.String(),
	}, nil
}

func resolvedRefToURI(refRegistry, refRepository, version string) *url.URL {
	u, _ := url.Parse(fmt.Sprintf("oci://%s/%s%s", refRegistry, refRepository, version))
	return u
}

func (l *Locker) computeExpectedLockfile(packageDirAbsPath string) (*PackageLock, error) {
	p, err := damlpackage.Read(filepath.Join(packageDirAbsPath, assistantconfig.DamlPackageFilename))
	if err != nil {
		return nil, err
	}

	// TODO de-duplicate p.ResolvedDependencies first
	expectedDars := lo.MapToSlice(p.ResolvedDependencies, func(_ string, d *damlpackage.ResolvedDependency) *Dar {
		return &Dar{
			URI:        d.FullUrl,
			Dependency: d,

			// TODO diff digests too
			// Digest:
		}
	})
	slices.SortFunc(expectedDars, func(a, b *Dar) int {
		return strings.Compare(a.URI.String(), b.URI.String())
	})

	lockSdkVersion, err := l.getExpectedSdkVersion(filepath.Join(packageDirAbsPath, assistantconfig.DamlPackageFilename))
	if err != nil {
		return nil, err
	}
	return &PackageLock{
		ManifestMeta: schema.ManifestMeta{
			APIVersion: PackageLockAPIVersion,
			Kind:       PackageLockKind,
		},
		SdkVersion: lockSdkVersion,
		Dars:       expectedDars,
	}, nil
}

func (l *Locker) computeMultiExpectedLockfile() (*PackageLock, error) {
	var expectedDars []*Dar

	lockSdkVersion, err := l.getExpectedSdkVersion("")
	if err != nil {
		return nil, err
	}
	return &PackageLock{
		ManifestMeta: schema.ManifestMeta{
			APIVersion: PackageLockAPIVersion,
			Kind:       PackageLockKind,
		},
		SdkVersion: lockSdkVersion,
		Dars:       expectedDars,
	}, nil
}

func (l *Locker) getExpectedSdkVersion(packageDirAbsPath string) (SdkVersion, error) {
	sdkVersion, _, err := versions.GetFloatyActiveVersion(l.config, packageDirAbsPath)
	if err != nil {
		return SdkVersion{}, err
	}

	// the no-sdk case
	if sdkVersion == "" {
		return SdkVersion{
			Version: "",
		}, nil
	}

	return SdkVersion{
		Version: sdkVersion,
	}, nil
}
