package cli

import (
	"github.com/spf13/cobra"
)

// Version can be overridden by release builds through go build -ldflags -X.
var Version = "0.1.0"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show Forge version",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, t, err := loadCtxAndT()
			if err != nil {
				return err
			}
			if flags.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]string{"version": Version})
			}
			println(t.T("version.current", Version))
			return nil
		},
	}
}
