package add

import (
	"daml.com/x/assistant/cmd/dpm/cmd/add/component"
	"daml.com/x/assistant/pkg/assistantconfig"
	"github.com/spf13/cobra"
)

func Cmd(config *assistantconfig.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:  "add",
		Long: "Add components and dars to project",
	}

	cmd.AddCommand(component.Cmd(config))
	return cmd
}
