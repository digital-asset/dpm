// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package publishdar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/licenseutils"
	ociconsts "daml.com/x/assistant/pkg/oci"
	"daml.com/x/assistant/pkg/ociindex"
	"daml.com/x/assistant/pkg/ocilister"
	"daml.com/x/assistant/pkg/ocipusher/darpusher"
	"daml.com/x/assistant/pkg/utils"
	"github.com/Masterminds/semver/v3"
	"github.com/fatih/color"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/samber/lo"
	"oras.land/oras-go/v2/errdef"
)

type DarConfig struct {
	Name                   string
	File                   string
	Version                *semver.Version
	DryRun, IncludeGitInfo bool
	Annotations            map[string]string
	ExtraTags              []string

	Registry     string
	AuthFilePath string
	Insecure     bool
}

type DarPublisher struct {
	config  *DarConfig
	printer utils.RawPrinter
}

func New(config *DarConfig, printer utils.RawPrinter) *DarPublisher {
	return &DarPublisher{config: config, printer: printer}
}

func (p *DarPublisher) PublishDar(ctx context.Context) (err error) {
	var pushOp *darpusher.DarPushOperation
	if assistantconfig.DarPublishEnabled() {
		pushOp, err = p.prepareDar(ctx, p.config.File)

		if p.config.DryRun {
			p.printer.Println("Skipping push due to --dry-run")
			return nil
		}

		if p.config.Registry == "" {
			return fmt.Errorf("--registy must be provided when not in dry-run mode")
		}

		client, err := assistantremote.New(p.config.Registry, p.config.AuthFilePath, p.config.Insecure)
		if err != nil {
			return err
		}

		versionExists, err := p.checkVersionExists(ctx, pushOp, client)
		if err != nil {
			return err
		}

		if versionExists {
			p.printer.Println("skipped pushing because dar's version already exists in remote")
		} else {
			_, err := p.push(ctx, client, pushOp)
			if err != nil {
				return err
			}
		}
		if p.config.ExtraTags != nil && len(p.config.ExtraTags) > 0 {
			p.printer.Println("pushing extra tags...")
			// Function below is not specifically for a generated index, can be utilized to setting tags to artifacts in general
			err := ociindex.Tag(ctx, client, &ociconsts.DarArtifact{DarName: p.config.Name}, p.config.Version, p.config.ExtraTags)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *DarPublisher) prepareDar(ctx context.Context, dir string) (*darpusher.DarPushOperation, error) {
	p.printer.Printf("📦 Checking %q includes license file...\n", dir)
	if err := checkHasLicense(dir); err != nil {
		return nil, err
	}
	p.printer.Printf("License file included ✅\n")
	p.printer.Println()

	return p.prepare(ctx, dir)
}

func (p *DarPublisher) prepare(ctx context.Context, dir string) (*darpusher.DarPushOperation, error) {
	annotations := maps.Clone(p.config.Annotations)
	if p.config.IncludeGitInfo {
		gitAnnotations, err := collectGitAnnotations()
		if err != nil {
			return nil, err
		}
		maps.Copy(annotations, gitAnnotations)
	}
	var artifact ociconsts.Artifact
	artifact = &ociconsts.DarArtifact{DarName: p.config.Name}

	opts := darpusher.DarOpts{
		Artifact: artifact,
		RawTag:   p.config.Version.String(),
		Dir:      dir,
	}

	pushOp, err := darpusher.DarNew(ctx, opts)
	if err != nil {
		if errors.Is(err, errdef.ErrSizeExceedsLimit) {
			p.printer.PrintErrln(`Failed to construct OCI manifest due to size limit.
Consider reducing the number of files at the root by moving them to subdirectories`)
		}
		return nil, err
	}

	return pushOp, nil
}

func (p *DarPublisher) push(ctx context.Context, client *assistantremote.Remote, pushOp *darpusher.DarPushOperation) (*v1.Descriptor, error) {
	coloredDest := color.GreenString(pushOp.DarDestination(client.Registry))

	p.printer.Printf("Pushing %q...\n", coloredDest)
	descriptor, err := pushOp.DarDo(ctx, client)
	if err != nil {
		return nil, err
	}
	descriptorJson, err := json.MarshalIndent(descriptor, "", "  ")
	if err != nil {
		return nil, err
	}
	p.printer.Printf("\n%s\n", string(descriptorJson))
	p.printer.Println("successfully published " + coloredDest)
	return descriptor, nil
}

func checkHasLicense(dir string) error {
	des, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	_, ok := lo.Find(des, func(de os.DirEntry) bool {
		return de.Name() == licenseutils.ComponentLicenseFilename && de.Type().IsRegular()
	})
	if !ok {
		return fmt.Errorf("required %s file is missing at component root (%q)", licenseutils.ComponentLicenseFilename, dir)
	}
	return nil
}

func (p *DarPublisher) checkVersionExists(ctx context.Context, op *darpusher.DarPushOperation, client *assistantremote.Remote) (bool, error) {
	var tags []string

	tags, found, err := ocilister.ListTags(ctx, client, ociconsts.DarRepoPrefix+p.config.Name)
	if err != nil {
		return false, err
	}
	if found {
		for _, tag := range tags {
			if tag == op.Tag() {
				return true, nil
			}
		}
	}
	return false, nil
}

// TODO : Currently not attaching annotations below, to be added in follow up PR
func (config *DarConfig) RequiredAnnotations() ociconsts.DescriptorAnnotations {
	return ociconsts.DescriptorAnnotations{
		Name:    config.Name,
		Version: config.Version,
	}
}

func collectGitAnnotations() (map[string]string, error) {
	r, err := git.PlainOpenWithOptions(".", &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, err
	}
	head, err := r.Head()
	if err != nil {
		return nil, err
	}

	result := map[string]string{
		"git.commit": head.Hash().String(),
	}

	tag, err := r.TagObject(head.Hash())
	if err == nil {
		result["git.tag"] = tag.Name
	} else if !errors.Is(err, plumbing.ErrObjectNotFound) {
		return nil, err
	}

	return result, nil
}
