package dar

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	project "daml.com/x/assistant/cmd/dpm/cmd/install/package"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/ocilister"
	"daml.com/x/assistant/pkg/yamledit"
	"github.com/spf13/cobra"
	"oras.land/oras-go/v2/registry"
)

func Cmd(config *assistantconfig.Config) *cobra.Command {
	var insecure bool

	cmd := &cobra.Command{
		Use:    "dar <oci-uri>",
		Short:  "add a dar to project",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			uri := args[0]

			damlPackage, ok, err := assistantconfig.GetDamlPackageAbsolutePath()
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("must be in daml.yaml directory or sub-directory")
			}

			ref, err := registry.ParseReference(strings.TrimPrefix(uri, "oci://"))
			if err != nil {
				return err
			}
			client, err := assistantremote.New(ref.Registry, "", insecure)
			if err != nil {
				return err
			}

			// Resolve to sha256
			ociManifest, err := ocilister.FetchManifest(ctx, client, ref)
			if err != nil {
				return err
			}
			resolvedUri := uri + "@" + ociManifest.Digest.String()

			// Pull
			parsedUrl, err := url.Parse(resolvedUri)
			if err != nil {
				return err
			}
			parsedDarDep := &damlpackage.ParsedDarDependency{
				FullUrl: parsedUrl,
				Location: &damlpackage.ArtifactLocation{
					Insecure: insecure,
				},
			}
			if err := project.InstallDar(ctx, config, parsedDarDep); err != nil {
				return err
			}

			// Edit daml.yaml
			if err := appendDarToYaml(damlPackage, resolvedUri); err != nil {
				return err
			}

			fmt.Printf("Successfully installed and added dar %q to %q\n", resolvedUri, damlPackage)
			return nil
		},
	}

	cmd.Flags().BoolVar(&insecure, "insecure", false, "use http instead of https for OCI registry")

	return cmd
}

func appendDarToYaml(path, dar string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	out, err := yamledit.AddToList(b, "data-dependencies", dar)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(out), 0644)
}
