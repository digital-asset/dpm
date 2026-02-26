package darpuller

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/damlpackage"
	ociconsts "daml.com/x/assistant/pkg/oci"
	"daml.com/x/assistant/pkg/ocicache"
	"daml.com/x/assistant/pkg/utils"
	"github.com/Masterminds/semver/v3"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry"
)

type OciDarPuller struct {
	config *assistantconfig.Config
}

func New(config *assistantconfig.Config) *OciDarPuller {
	return &OciDarPuller{
		config: config,
	}
}

func (a *OciDarPuller) PullDar(ctx context.Context, dar *damlpackage.ResolvedDependency) (*v1.Descriptor, *semver.Version, string, error) {
	repo, ref, err := dar.GetOciRepo()
	if err != nil {
		return nil, nil, "", err
	}

	src, err := ocicache.CachedTarget(repo, a.config.OciLayoutCache)
	if err != nil {
		return nil, nil, "", err
	}

	desc, err := repo.Resolve(ctx, ref.Reference)
	if err != nil {
		return nil, nil, "", err
	}
	version, err := a.getVersion(ctx, repo, desc)
	if err != nil {
		return nil, nil, "", err
	}

	destPath := a.getPath(ref, version)

	ok, err := utils.DirExists(destPath)
	if err != nil {
		return nil, nil, "", err
	}
	if ok {
		return &desc, version, destPath, nil
	}

	dest, err := file.New(destPath)
	if err != nil {
		return nil, nil, "", err
	}
	dest.PreservePermissions = true
	// errors out if dest already exists
	dest.DisableOverwrite = true

	_, err = oras.Copy(ctx, src, ref.Reference, dest, ref.Reference, oras.CopyOptions{})
	if err != nil {
		return nil, nil, "", err
	}

	// TODO validate the pulled DAR is actually a DAR (?)
	return &desc, version, destPath, err
}

// figure out the dar's non-floaty semver

func (a *OciDarPuller) getVersion(ctx context.Context, repo oras.ReadOnlyTarget, desc v1.Descriptor) (*semver.Version, error) {
	annotations, err := getAnnotations(ctx, repo, desc)
	if err != nil {
		return nil, err
	}

	v := cmp.Or(
		annotations[v1.AnnotationVersion],
		// fallback
		annotations[ociconsts.DescriptorVersionAnnotation],
	)
	if v == "" {
		return nil, fmt.Errorf("missing version annotation in image manifest")
	}
	ver, err := semver.StrictNewVersion(v)
	if err != nil {
		return nil, fmt.Errorf("version annotation %q in image manifest isn't a strict semver: %w", v, err)
	}
	return ver, nil
}

func (a *OciDarPuller) getPath(ref *registry.Reference, version *semver.Version) string {
	// TODO maybe use a more human-readable path (but be sure to sanitize to not fail on Windows)
	sha := sha256.Sum256([]byte(fmt.Sprintf("%s/%s:%s", ref.Registry, ref.Repository, version.String())))
	return filepath.Join(
		a.config.CachePath,
		"dars",
		hex.EncodeToString(sha[:]),
	)
}

func getAnnotations(ctx context.Context, repo oras.ReadOnlyTarget, desc v1.Descriptor) (map[string]string, error) {
	rc, err := repo.Fetch(ctx, desc)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	// extract just the annotations
	manifest := struct {
		Annotations map[string]string `json:"annotations"`
	}{}
	if err := json.NewDecoder(rc).Decode(&manifest); err != nil {
		return nil, err
	}
	return manifest.Annotations, nil
}
