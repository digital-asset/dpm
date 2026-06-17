package update

import (
	"context"
	"fmt"
	"strings"

	"daml.com/x/assistant/cmd/dpm/cmd/add/dar"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/builtincommand"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/ocilister"
	"github.com/spf13/cobra"
)

type updateCmd struct {
	forceInsecure bool
	config        *assistantconfig.Config
}

// TODO this currently only supports DARs and daml.yaml, but not components
func Cmd(config *assistantconfig.Config) *cobra.Command {
	c := updateCmd{}
	c.config = config

	cmd := &cobra.Command{
		Use:    string(builtincommand.Update),
		Short:  "update project dependencies",
		Hidden: assistantconfig.ShaPinningEnabled(),
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			damlPackagePath, ok, err := assistantconfig.GetDamlPackageAbsolutePath()
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("must be in daml.yaml directory or sub-directory")
			}

			damlPackage, err := damlpackage.Read(damlPackagePath)
			if err != nil {
				return err
			}

			for _, dep := range damlPackage.ParsedDarDependencies.Dependencies {
				if err := c.updateDar(ctx, "dependencies", dep); err != nil {
					return err
				}
			}

			for _, dep := range damlPackage.ParsedDarDependencies.DataDependencies {
				if err := c.updateDar(ctx, "data-dependencies", dep); err != nil {
					return err
				}
			}

			fmt.Println("Successfully updated project at " + damlPackagePath)
			return nil

		},
	}

	cmd.Flags().BoolVar(&c.forceInsecure, "force-insecure", false, "ignoring ArtifactLocations and force http instead of https for OCI registry")

	return cmd
}

func (c *updateCmd) updateDar(ctx context.Context, field string, dep *damlpackage.ParsedDarDependency) error {
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
	if err := dar.AddOrUpdateDar(ctx, c.config, uri, field, insecure, dep.Index); err != nil {
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
