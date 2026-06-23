package update

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"daml.com/x/assistant/cmd/dpm/cmd/add/dar"
	"daml.com/x/assistant/pkg/assembler"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/builtincommand"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/multipackage"
	"daml.com/x/assistant/pkg/ocilister"
	"daml.com/x/assistant/pkg/ocipuller/remotepuller"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"github.com/spf13/cobra"
	"oras.land/oras-go/v2/registry"
)

type updateCmd struct {
	forceInsecure bool
	config        *assistantconfig.Config
	printer       *cobra.Command
}

func Cmd(config *assistantconfig.Config) *cobra.Command {
	c := updateCmd{}

	cmd := &cobra.Command{
		Use:   string(builtincommand.Update),
		Short: "update project dependencies",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			c.config = config
			c.printer = cmd

			pkgs, multiPkg, err := c.packagesToUpdate()
			if err != nil {
				return err
			}

			c.config.AutoInstall = true

			if err := c.updateMultiPackage(ctx, multiPkg); err != nil {
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

func (c *updateCmd) packagesToUpdate() (pkgs []*damlpackage.DamlPackage, mPkg *multipackage.MultiPackage, err error) {
	multiPackagePath, ok, err := assistantconfig.GetMultiPackageAbsolutePath()
	if err != nil {
		return nil, nil, err
	}
	if ok {
		mPkg, err = multipackage.Read(multiPackagePath)
		if err != nil {
			return nil, nil, err
		}

		for _, pkgPath := range mPkg.AbsolutePackages() {
			pkg, err := damlpackage.Read(filepath.Join(pkgPath, "daml.yaml"))
			if err != nil {
				return nil, nil, err
			}
			pkgs = append(pkgs, pkg)
		}

		return
	}

	damlPackagePath, ok, err := assistantconfig.GetDamlPackageAbsolutePath()
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return nil, nil, fmt.Errorf("not in a (single-package or multi-package) project directory")
	}

	pkg, err := damlpackage.Read(damlPackagePath)
	if err != nil {
		return nil, nil, err
	}

	return []*damlpackage.DamlPackage{pkg}, nil, nil
}

func (c *updateCmd) updateMultiPackage(ctx context.Context, mPkg *multipackage.MultiPackage) error {
	if mPkg == nil {
		return nil
	}

	c.printer.Printf("Updating multi-pacakge %s...\n", mPkg.AbsolutePath)

	client, err := assistantremote.NewFromConfig(c.config)
	if err != nil {
		return err
	}
	for _, comp := range mPkg.Components {
		if err := c.updateComponent(ctx, client, comp); err != nil {
			return err
		}
	}
	return nil
}

func (c *updateCmd) updatePackage(ctx context.Context, damlPackage *damlpackage.DamlPackage) error {
	c.printer.Printf("Updating package %s...\n", damlPackage.AbsolutePath)

	client, err := assistantremote.NewFromConfig(c.config)
	if err != nil {
		return err
	}
	for _, comp := range damlPackage.Components {
		if err := c.updateComponent(ctx, client, comp); err != nil {
			return err
		}
	}

	for _, dep := range damlPackage.ParsedDarDependencies.Dependencies {
		if err := c.updateDar(ctx, damlPackage.AbsolutePath, "dependencies", dep); err != nil {
			return err
		}
	}

	for _, dep := range damlPackage.ParsedDarDependencies.DataDependencies {
		if err := c.updateDar(ctx, damlPackage.AbsolutePath, "data-dependencies", dep); err != nil {
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

	fmt.Printf("Updating dar %q...\n", uri)

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

func (c *updateCmd) updateComponent(ctx context.Context, client *assistantremote.Remote, component *sdkmanifest.Component) error {
	if component.Uri == nil {
		return nil
	}

	fmt.Printf("Updating component %q...\n", component.String())

	puller := remotepuller.New(c.config.OciLayoutCache, client)
	assembler := assembler.New(c.config, puller)

	uri := *component.Uri

	ref, err := registry.ParseReference(strings.TrimPrefix(uri, "oci://"))
	if err != nil {
		return err
	}

	// get rid of the sha256 pin on floaty tags because we're about to
	// re-resolve and update them
	tag := extractTag(uri)
	if ocilister.IsFloaty(tag) {
		uri = fmt.Sprintf("oci://%s/%s:%s", ref.Registry, ref.Repository, tag)
	}
	component.Uri = &uri

	_, err = assembler.Assemble(ctx, &sdkmanifest.SdkManifest{
		Spec: &sdkmanifest.Spec{
			Components: map[string]*sdkmanifest.Component{component.Name: component},
		},
	})
	if err != nil {
		return err
	}

	fmt.Printf("Component updated: %q\n", component.String())
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
