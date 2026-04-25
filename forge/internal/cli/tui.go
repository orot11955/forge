package cli

import (
	"github.com/orot/forge/internal/tui"
	"github.com/orot/forge/internal/workbench"
	"github.com/spf13/cobra"
)

func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Interactive terminal UI for browsing projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, t, err := loadCtxAndT()
			if err != nil {
				return err
			}
			if ctx.WorkbenchRoot == "" {
				return workbenchNotFound(t)
			}
			reg, err := workbench.LoadRegistry(ctx.WorkbenchRoot)
			if err != nil {
				return Coded(2, err)
			}
			m := tui.New(ctx.WorkbenchRoot, ctx.WorkbenchID, reg.Projects)
			return tui.Run(m)
		},
	}
}
