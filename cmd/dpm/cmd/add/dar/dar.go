package dar

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	project "daml.com/x/assistant/cmd/dpm/cmd/install/package"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/ocilister"
	"daml.com/x/assistant/pkg/yamledit"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"oras.land/oras-go/v2/registry"
)

func Cmd(config *assistantconfig.Config) *cobra.Command {
	var insecure bool
	var dependencies, dataDependencies bool

	cmd := &cobra.Command{
		Use:   "dar <oci-uri> <--dependencies | --data-dependencies>",
		Short: "add or update a dar in the project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			uri := args[0]

			depsFieldName, err := dependenciesFieldFromArgs(dependencies, dataDependencies)
			if err != nil {
				return err
			}

			damlPackagePath, ok, err := assistantconfig.GetDamlPackageAbsolutePath()
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("must be in daml.yaml directory or sub-directory")
			}

			// figure out if we need to update rather than add
			existingDep, err := findExistingDependency(uri, depsFieldName)
			if err != nil {
				return err
			}

			yamlTarget := yamledit.YamlTarget{
				YamlFilePath: damlPackagePath,
				FieldName:    depsFieldName,
				Index:        -1,
			}

			// update
			if existingDep != nil {
				fmt.Printf("dependency %q already exists in daml.yaml, will be updated...\n", uri)
				yamlTarget.Index = existingDep.Index
				return AddOrUpdateDar(ctx, config, uri, insecure, yamlTarget)
			}

			// add
			return AddOrUpdateDar(ctx, config, uri, insecure, yamlTarget)
		},
	}

	cmd.Flags().BoolVar(&insecure, "insecure", false, "use http instead of https for OCI registry")
	cmd.Flags().BoolVar(&dependencies, "dependencies", false, "add the dar to the dependencies field")
	cmd.Flags().BoolVar(&dataDependencies, "data-dependencies", false, "add the dar to the data-dependencies field")

	return cmd
}

func findExistingDependency(uri, depsFieldName string) (*damlpackage.ParsedDarDependency, error) {
	damlPackagePath, ok, err := assistantconfig.GetDamlPackageAbsolutePath()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("must be in daml.yaml directory or sub-directory")
	}

	damlPackage, err := damlpackage.Read(damlPackagePath)
	if err != nil {
		return nil, err
	}

	deps := damlPackage.ParsedDarDependencies.Dependencies
	if depsFieldName == "data-dependencies" {
		deps = damlPackage.ParsedDarDependencies.DataDependencies
	}

	uriRef, err := registry.ParseReference(strings.TrimPrefix(uri, "oci://"))
	if err != nil {
		return nil, err
	}

	for _, dep := range deps {
		if dep.FullUrl.Scheme != "oci" {
			continue
		}

		depUrl := dep.FullUrl.String()

		depRef, err := registry.ParseReference(strings.TrimPrefix(depUrl, "oci://"))
		if err != nil {
			return nil, fmt.Errorf("invalid uri %q in daml.yaml or multi-package.yaml: %w", depUrl, err)
		}

		if uriRef.Registry == depRef.Registry && uriRef.Repository == depRef.Repository {
			return dep, nil
		}
	}

	return nil, nil
}

// AddOrUpdateDar will add when the passed index is -1, otherwise it will update at that index
func AddOrUpdateDar(ctx context.Context, config *assistantconfig.Config, uri string, insecure bool, yamlTarget yamledit.YamlTarget) error {
	ref, err := registry.ParseReference(strings.TrimPrefix(uri, "oci://"))
	if err != nil {
		return err
	}
	client, err := assistantremote.New(ref.Registry, "", insecure)
	if err != nil {
		return err
	}

	// Resolve to sha256
	resolvedDigest, manifest, err := ocilister.FetchManifest(ctx, client, ref)
	if err != nil {
		return err
	}
	yamlTarget.LineComment = manifest.Annotations[v1.AnnotationVersion]
	resolvedUri := uri + "@" + resolvedDigest.String()

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
	if _, _, err := project.InstallDar(ctx, config, parsedDarDep); err != nil {
		return err
	}

	// Edit daml.yaml
	if err := yamledit.EditYaml(yamlTarget, resolvedUri); err != nil {
		return err
	}

	fmt.Printf("Successfully installed and added dar %q to %q\n", resolvedUri, yamlTarget.YamlFilePath)
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
