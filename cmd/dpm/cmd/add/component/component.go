package component

import (
	"fmt"
	"os"
	"strings"

	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/componentlist"
	"daml.com/x/assistant/pkg/yamledit"
	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
	"oras.land/oras-go/v2/registry"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "component <oci-uri>",
		Short:  "add a component to project",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			component := args[0]
			_, err := registry.ParseReference(strings.TrimPrefix(component, "oci://"))
			if err != nil {
				return err
			}

			projectManifest, err := getDamlYamlOrMultiPackageYaml()
			if err != nil {
				return err
			}

			return addComponent(projectManifest, component)
		},
	}

	return cmd
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

func addComponent(path, component string) error {
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
