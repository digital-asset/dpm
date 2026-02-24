package darpuller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/ocicache"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
)

type OciDarPuller struct {
	config *assistantconfig.Config
}

func New(config *assistantconfig.Config) *OciDarPuller {
	return &OciDarPuller{
		config: config,
	}
}

func (a *OciDarPuller) PullDar(ctx context.Context, dar *damlpackage.ResolvedDependency) (*v1.Descriptor, string, error) {
	repo, ref, err := dar.GetOciRepo()
	if err != nil {
		return nil, "", err
	}

	src, err := ocicache.CachedTarget(repo, a.config.OciLayoutCache)
	if err != nil {
		return nil, "", err
	}

	destPath := a.getPath(dar.FullUrl.String())
	dest, err := file.New(destPath)
	if err != nil {
		return nil, "", err
	}
	dest.PreservePermissions = true
	// errors out if dest already exists
	dest.DisableOverwrite = true

	desc, err := oras.Copy(ctx, src, ref.Reference, dest, ref.Reference, oras.CopyOptions{})
	if err != nil {
		return nil, "", err
	}

	// TODO validate the pulled DAR is actually a DAR (?)
	return &desc, destPath, err
}

func (a *OciDarPuller) getPath(rawUrl string) string {
	// TODO maybe use a more human-readable path (but be sure to sanitize to not fail on Windows)
	sha := sha256.Sum256([]byte(rawUrl))
	return filepath.Join(
		a.config.CachePath,
		"dars",
		hex.EncodeToString(sha[:]),
	)
}
