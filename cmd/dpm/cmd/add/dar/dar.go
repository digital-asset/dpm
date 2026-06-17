package dar

import (
	"context"
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
	var dependencies, dataDependencies bool

	cmd := &cobra.Command{
		Use:    "dar <oci-uri> <--dependencies | --data-dependencies>",
		Short:  "add a dar to project",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			uri := args[0]

			depsFieldName, err := dependenciesFieldFromArgs(dependencies, dataDependencies)
			if err != nil {
				return err
			}

			return AddOrUpdateDar(ctx, config, uri, depsFieldName, insecure, -1)
		},
	}

	cmd.Flags().BoolVar(&insecure, "insecure", false, "use http instead of https for OCI registry")
	cmd.Flags().BoolVar(&dependencies, "dependencies", false, "add the dar to the dependencies field")
	cmd.Flags().BoolVar(&dataDependencies, "data-dependencies", false, "add the dar to the data-dependencies field")

	return cmd
}

func AddOrUpdateDar(ctx context.Context, config *assistantconfig.Config, uri, depsFieldName string, insecure bool, index int) error {
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
	if err := appendDarToYaml(damlPackage, depsFieldName, resolvedUri, index); err != nil {
		return err
	}

	fmt.Printf("Successfully installed and added dar %q to %q\n", resolvedUri, damlPackage)
	return nil
}

func dependenciesFieldFromArgs(dependencies, dataDependencies bool) (string, error) {
	if dataDependencies && dependencies {
		return "", fmt.Errorf("--dependencies and --data-dependencies cannot both be provided")
	}
	if dependencies {
		return "dependencies", nil
	}
	if dataDependencies {
		return "data-dependencies", nil
	}
	return "", fmt.Errorf("a --dependencies or --data-dependencies is required")
}

func appendDarToYaml(path, fieldName, dar string, index int) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var out string
	if index != -1 {
		out, err = yamledit.ReplaceItemInList(b, fieldName, index, dar)
	} else {
		out, err = yamledit.AddToList(b, fieldName, dar)
	}
	if err != nil {
		return err
	}

	return os.WriteFile(path, []byte(out), 0644)
}
