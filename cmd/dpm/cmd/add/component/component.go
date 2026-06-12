package component

import (
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

			// TODO add support for multi-package too
			pkgPath, ok, err := assistantconfig.GetDamlPackageAbsolutePath()
			if err != nil {
				return err
			}
			if ok {
				return addComponent(pkgPath, component)
			}

			return nil
		},
	}

	return cmd
}

func addComponent(path, component string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	item, err := yaml.Marshal(componentlist.ComponentList{
		&componentlist.ComponentEntry{StringBased: &component},
	})

	if err != nil {
		return err
	}

	out := yamledit.AppendToYaml(b, "components", string(item))
	return os.WriteFile(path, []byte(out), 0644)
}
