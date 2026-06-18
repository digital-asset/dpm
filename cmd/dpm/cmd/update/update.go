package update

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"daml.com/x/assistant/cmd/dpm/cmd/add/dar"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/builtincommand"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/multipackage"
	"daml.com/x/assistant/pkg/ocilister"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

type updateCmd struct {
	forceInsecure bool
	config        *assistantconfig.Config
	printer       *cobra.Command
}

// TODO this currently only supports DARs and daml.yaml, but not components
func Cmd(config *assistantconfig.Config) *cobra.Command {
	c := updateCmd{}

	cmd := &cobra.Command{
		Use:    string(builtincommand.Update),
		Short:  "update project dependencies",
		Hidden: assistantconfig.ShaPinningEnabled(),
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			c.config = config
			c.printer = cmd

			pkgs, err := c.packagesToUpdate()
			if err != nil {
				return err
			}

			for _, p := range pkgs {
				if err := c.updatePackage(ctx, p); err != nil {
					return err
				}
			}

			fmt.Println("Successfully updated project.")
			return nil

		},
	}

	cmd.Flags().BoolVar(&c.forceInsecure, "force-insecure", false, "ignoring ArtifactLocations and force http instead of https for OCI registry")

	return cmd
}

func (c *updateCmd) packagesToUpdate() ([]string, error) {
	multiPackagePath, ok, err := assistantconfig.GetMultiPackageAbsolutePath()
	if err != nil {
		return nil, err
	}
	if ok {
		mPkg, err := multipackage.Read(multiPackagePath)
		if err != nil {
			return nil, err
		}
		return lo.Map(mPkg.AbsolutePackages(), func(p string, _ int) string {
			return filepath.Join(p, "daml.yaml")
		}), nil
	}

	damlPackagePath, ok, err := assistantconfig.GetDamlPackageAbsolutePath()
	if err != nil {
		return nil, err
	}
	if ok {
		return []string{damlPackagePath}, nil
	}

	return nil, fmt.Errorf("not in a (single-package or multi-package) project directory")
}

func (c *updateCmd) updatePackage(ctx context.Context, damlPackagePath string) error {
	c.printer.Printf("Updating package %s...\n", damlPackagePath)

	damlPackage, err := damlpackage.Read(damlPackagePath)
	if err != nil {
		return err
	}

	for _, dep := range damlPackage.ParsedDarDependencies.Dependencies {
		if err := c.updateDar(ctx, damlPackagePath, "dependencies", dep); err != nil {
			return err
		}
	}

	for _, dep := range damlPackage.ParsedDarDependencies.DataDependencies {
		if err := c.updateDar(ctx, damlPackagePath, "data-dependencies", dep); err != nil {
			return err
		}
	}

	return nil
}

func (c *updateCmd) updateDar(ctx context.Context, damlPackagePath, field string, dep *damlpackage.ParsedDarDependency) error {
	if dep.FullUrl.Scheme != "oci" {
		return nil
	}

	uri := dep.FullUrl.String()

	fmt.Printf("Updating %q...\n", uri)

	_, ref, err := dep.GetOciRemote()
	if err != nil {
		return err
	}

	// get rid of the sha256 pin on floaty tags because we're about to
	// re-resolve and update them
	tag := extractTag(uri)
	if ocilister.IsFloaty(tag) {
		uri = fmt.Sprintf("oci://%s/%s:%s", ref.Registry, ref.Repository, tag)
	}

	insecure := c.forceInsecure || dep.Location.Insecure
	if err := dar.AddOrUpdateDar(ctx, c.config, damlPackagePath, uri, field, insecure, dep.Index); err != nil {
		return err
	}

	fmt.Printf("Successfully updated %q\n\n", uri)
	return nil
}

// extractTag returns the tag if there is one, or "" otherwise.
// input uri is expected to begin with "oci://"
func extractTag(uri string) string {
	namePart, _, _ := strings.Cut(uri, "@")
	lastSlash := strings.LastIndexByte(namePart, '/')
	lastColon := strings.LastIndexByte(namePart, ':')

	if lastColon > lastSlash && lastColon+1 < len(namePart) {
		return namePart[lastColon+1:]
	}
	return ""
}
