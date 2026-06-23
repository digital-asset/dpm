package add

import (
	"daml.com/x/assistant/cmd/dpm/cmd/add/component"
	"daml.com/x/assistant/cmd/dpm/cmd/add/dar"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/builtincommand"
	"github.com/spf13/cobra"
)

func Cmd(config *assistantconfig.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   string(builtincommand.Add),
		Short: "Add components and dars to project",
	}

	cmd.AddCommand(component.Cmd(config))
	cmd.AddCommand(dar.Cmd(config))
	return cmd
}
