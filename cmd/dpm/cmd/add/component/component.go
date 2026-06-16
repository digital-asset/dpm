package component

import (
	"context"
	"fmt"
	"os"
	"strings"

	"daml.com/x/assistant/pkg/assembler"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/componentlist"
	"daml.com/x/assistant/pkg/ocipuller/remotepuller"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/yamledit"
	"github.com/goccy/go-yaml"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"oras.land/oras-go/v2/registry"
)

func Cmd(config *assistantconfig.Config) *cobra.Command {
	var insecure bool

	cmd := &cobra.Command{
		Use:    "component <oci-uri>",
		Short:  "add a component to project",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			uri := args[0]

			projectManifest, err := getDamlYamlOrMultiPackageYaml()
			if err != nil {
				return err
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
			sha, err := GetDigest(ctx, client, ref)
			if err != nil {
				return err
			}
			resolvedUri := uri + "@" + sha.String()

			// Pull
			if err := PullComponent(ctx, resolvedUri, config, client); err != nil {
				return err
			}

			// Edit daml.yaml / multi-package.yaml
			if err := appendComponentToYaml(projectManifest, resolvedUri); err != nil {
				return err
			}

			fmt.Printf("Successfully installed and added component %q to %q\n", resolvedUri, projectManifest)
			return nil
		},
	}

	cmd.Flags().BoolVar(&insecure, "insecure", false, "use http instead of https for OCI registry")

	return cmd
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

func GetDigest(ctx context.Context, client *assistantremote.Remote, ref registry.Reference) (*digest.Digest, error) {
	// TODO this function does a HEAD instead of GET
	// and so the returned OCI descriptor isn't the full one would include all the annotations

	fmt.Printf("Resolving %q...\n", ref.String())

	repo, err := client.Repo(ref.Repository)
	if err != nil {
		return nil, err
	}
	desc, err := repo.Resolve(ctx, ref.Reference)
	if err != nil {
		return nil, err
	}

	fmt.Println("resolved sha256: " + desc.Digest)
	fmt.Println("resolved version: " + desc.Annotations[v1.AnnotationVersion])
	return &desc.Digest, nil
}

func asSdkManifest(uri string) (*sdkmanifest.SdkManifest, error) {
	components, err := componentlist.ComponentList{
		&componentlist.ComponentEntry{
			StringBased: &uri,
		},
	}.ToMap()
	if err != nil {
		return nil, err
	}

	return &sdkmanifest.SdkManifest{
		Spec: &sdkmanifest.Spec{
			Components: components,
		},
	}, nil
}

func getDamlYamlOrMultiPackageYaml() (string, error) {
	p, ok, err := assistantconfig.GetDamlPackageAbsolutePath()
	if err != nil {
		return "", err
	}
	if ok {
		return p, nil
	}

	p, ok, err = assistantconfig.GetMultiPackageAbsolutePath()
	if err != nil {
		return "", err
	}
	if ok {
		return p, nil
	}

	return "", fmt.Errorf("not in a (single-package or multi-package) project directory")
}

func appendComponentToYaml(path, component string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	item, err := yaml.Marshal(&componentlist.ComponentEntry{StringBased: &component})
	if err != nil {
		return err
	}

	out, err := yamledit.AddToList(b, "components", string(item))
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(out), 0644)
}
