package add

import (
	"daml.com/x/assistant/cmd/dpm/cmd/add/component"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "add",
		Long: "Add components and dars to project",
	}

	cmd.AddCommand(component.Cmd())
	return cmd
}
