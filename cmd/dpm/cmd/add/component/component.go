package component

import (
	"cmp"
	"context"
	"fmt"
	"strings"

	"daml.com/x/assistant/pkg/assembler"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/componentlist"
	"daml.com/x/assistant/pkg/damlpackage"
	"daml.com/x/assistant/pkg/multipackage"
	"daml.com/x/assistant/pkg/ocilister"
	"daml.com/x/assistant/pkg/ocipuller/remotepuller"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/yamledit"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"oras.land/oras-go/v2/registry"
)

func Cmd(config *assistantconfig.Config) *cobra.Command {
	var insecure bool

	cmd := &cobra.Command{
		Use:   "component <oci-uri>",
		Short: "add a component to project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			uri := args[0]

			damlPackagePath, multiPackagePath, err := getDamlYamlOrMultiPackageYaml()
			if err != nil {
				return err
			}
			projectManifest := cmp.Or(damlPackagePath, multiPackagePath)

			var components componentlist.ComponentList
			if damlPackagePath != "" {
				obj, err := damlpackage.Read(damlPackagePath)
				if err != nil {
					return err
				}
				components = obj.ComponentsList
			} else {
				obj, err := multipackage.Read(multiPackagePath)
				if err != nil {
					return err
				}
				components = obj.ComponentsList
			}

			index, err := findExistingComponent(components, uri)
			if err != nil {
				return err
			}
			if index != -1 {
				fmt.Printf("component %q already exists, will be updated...\n", uri)
			}

			return AddOrUpdateComponent(ctx, config, projectManifest, uri, insecure, index)
		},
	}

	cmd.Flags().BoolVar(&insecure, "insecure", false, "use http instead of https for OCI registry")

	return cmd
}

func AddOrUpdateComponent(ctx context.Context, config *assistantconfig.Config, projectManifest, uri string, insecure bool, index int) error {
	ref, err := registry.ParseReference(strings.TrimPrefix(uri, "oci://"))
	if err != nil {
		return err
	}
	client, err := assistantremote.New(ref.Registry, "", insecure)
	if err != nil {
		return err
	}

	// Resolve to sha256
	sha, manifest, err := ocilister.FetchManifest(ctx, client, ref)
	if err != nil {
		return err
	}
	resolvedUri := uri + "@" + sha.String()

	// Pull
	if err := PullComponent(ctx, resolvedUri, config, client); err != nil {
		return err
	}

	// Edit daml.yaml / multi-package.yaml
	yamlTarget := yamledit.YamlTarget{
		YamlFilePath: projectManifest,
		FieldName:    "components",
		Index:        index,
	}
	if version := manifest.Annotations[v1.AnnotationVersion]; version != "" {
		yamlTarget.LineComment = "# " + version
	}
	if err := yamledit.EditYaml(yamlTarget, resolvedUri); err != nil {
		return err
	}

	fmt.Printf("Successfully installed and added component %q to %q\n", resolvedUri, projectManifest)
	return nil
}

func PullComponent(ctx context.Context, resolvedUri string, config *assistantconfig.Config, client *assistantremote.Remote) error {
	fmt.Println("Pulling...")
	m, err := asSdkManifest(resolvedUri)
	if err != nil {
		return err
	}
	config.AutoInstall = true
	puller := remotepuller.New(config.OciLayoutCache, client)
	_, err = assembler.New(config, puller).Assemble(ctx, m)
	return err
}

func asSdkManifest(uri string) (*sdkmanifest.SdkManifest, error) {
	components, err := componentlist.ComponentList{
		&componentlist.ComponentEntry{
			StringBased: &uri,
		},
	}.ToMap(nil)
	if err != nil {
		return nil, err
	}

	return &sdkmanifest.SdkManifest{
		Spec: &sdkmanifest.Spec{
			Components: components,
		},
	}, nil
}

func getDamlYamlOrMultiPackageYaml() (string, string, error) {
	p, ok, err := assistantconfig.GetDamlPackageAbsolutePath()
	if err != nil {
		return "", "", err
	}
	if ok {
		return p, "", nil
	}

	p, ok, err = assistantconfig.GetMultiPackageAbsolutePath()
	if err != nil {
		return "", "", err
	}
	if ok {
		return "", p, nil
	}

	return "", "", fmt.Errorf("not in a (single-package or multi-package) project directory")
}

func findExistingComponent(components componentlist.ComponentList, uri string) (int, error) {
	uriRef, err := registry.ParseReference(strings.TrimPrefix(uri, "oci://"))
	if err != nil {
		return 0, err
	}

	for i, compEntry := range components {
		if compEntry.StringBased == nil {
			continue
		}
		comp := *compEntry.StringBased

		if !strings.HasPrefix(comp, "oci://") {
			continue
		}

		compRef, err := registry.ParseReference(strings.TrimPrefix(comp, "oci://"))
		if err != nil {
			return 0, fmt.Errorf("invalid uri %q in daml.yaml or multi-package.yaml: %w", comp, err)
		}

		if uriRef.Registry == compRef.Registry && uriRef.Repository == compRef.Repository {
			return i, nil
		}
	}
	return -1, nil
}
