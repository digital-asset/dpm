package update

import (
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/assistantconfig/assistantremote"
	"daml.com/x/assistant/pkg/builtincommand"
	"daml.com/x/assistant/pkg/packagelock"
	"github.com/spf13/cobra"
)

func Cmd(config *assistantconfig.Config) *cobra.Command {
	var checkOnly bool

	cmd := &cobra.Command{
		Use:    string(builtincommand.Update),
		Short:  "update (or create) package lockfile(s)",
		Hidden: !assistantconfig.DpmLockfileEnabled(),
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			op := packagelock.Regular
			if checkOnly {
				op = packagelock.CheckOnly
			}

			client, err := assistantremote.NewFromConfig(config)
			if err != nil {
				return err
			}
			locker := packagelock.New(config, client, op)
			_, err = locker.EnsureLockfiles(cmd.Context())
			return err
		},
	}
	cmd.Flags().BoolVar(&checkOnly, "check", false, "check existing lockfile but don't update it")

	return cmd
}
