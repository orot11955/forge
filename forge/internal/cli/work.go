package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/orot/forge/internal/config"
	"github.com/orot/forge/internal/workbench"
	"github.com/spf13/cobra"
)

func newWorkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "work",
		Short:   "Manage Workbench",
		Aliases: []string{"workbench"},
	}
	cmd.AddCommand(newWorkInitCmd(), newWorkStatusCmd(), newWorkPathCmd())
	return cmd
}

func newWorkInitCmd() *cobra.Command {
	var idFlag string
	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a Workbench at the given path (default: cwd)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := ""
			if len(args) == 1 {
				target = config.ExpandPath(args[0])
			} else {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				target = cwd
			}
			_, t, err := loadCtxAndT()
			if err != nil {
				return err
			}
			cfg, err := workbench.Init(target, idFlag)
			if errors.Is(err, workbench.ErrExists) {
				if flags.JSON {
					return writeJSON(cmd.OutOrStdout(), map[string]any{
						"status": "exists",
						"root":   target,
					})
				}
				println(t.T("workbench.initExists", target))
				println(t.T("workbench.initNothingChanged"))
				return nil
			}
			if err != nil {
				return err
			}

			// Update global config: defaultWorkbench + recentWorkbenches.
			g, _, err := config.Load()
			defaultSetByThis := false
			previousDefault := ""
			if err == nil {
				previousDefault = g.DefaultWorkbench
				if g.DefaultWorkbench == "" {
					g.DefaultWorkbench = target
					defaultSetByThis = true
				}
				addRecent(g, target)
				_ = config.Save(g)
			}

			if flags.JSON {
				out := map[string]any{
					"status":           "created",
					"root":             target,
					"id":               cfg.ID,
					"defaultWorkbench": "",
					"defaultSetByInit": defaultSetByThis,
				}
				if defaultSetByThis {
					out["defaultWorkbench"] = target
				} else {
					out["defaultWorkbench"] = previousDefault
				}
				return writeJSON(cmd.OutOrStdout(), out)
			}
			println(t.T("workbench.initSuccess", target))
			println()
			if defaultSetByThis {
				println(t.T("workbench.initDefaultSet", target))
			} else if previousDefault != "" && previousDefault != target {
				println(t.T("workbench.initDefaultExists", previousDefault))
				println(t.T("workbench.initDefaultChangeHint", target))
			} else {
				println(t.T("workbench.initDefaultSet", previousDefault))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&idFlag, "id", "", "Workbench ID (defaults to slugified directory name)")
	return cmd
}

func addRecent(g *config.Global, path string) {
	for _, r := range g.RecentWorkbenches {
		if r == path {
			return
		}
	}
	g.RecentWorkbenches = append([]string{path}, g.RecentWorkbenches...)
	if len(g.RecentWorkbenches) > 10 {
		g.RecentWorkbenches = g.RecentWorkbenches[:10]
	}
}

func newWorkStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show Workbench status",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, t, err := loadCtxAndT()
			if err != nil {
				return err
			}
			if ctx.WorkbenchRoot == "" || !workbench.IsRoot(ctx.WorkbenchRoot) {
				return workbenchNotFound(t)
			}
			wcfg, err := workbench.Load(ctx.WorkbenchRoot)
			if err != nil {
				return Coded(2, fmt.Errorf("read workbench.yaml: %w", err))
			}
			reg, err := workbench.LoadRegistry(ctx.WorkbenchRoot)
			if err != nil {
				return err
			}
			isDefault := false
			if ctx.GlobalConfig != nil && ctx.GlobalConfig.DefaultWorkbench != "" {
				isDefault = config.ExpandPath(ctx.GlobalConfig.DefaultWorkbench) == ctx.WorkbenchRoot
			}

			if flags.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"root":     ctx.WorkbenchRoot,
					"id":       wcfg.ID,
					"name":     wcfg.Name,
					"projects": len(reg.Projects),
					"default":  isDefault,
				})
			}
			println(t.T("workbench.statusTitle"))
			println()
			println(t.T("workbench.rootLabel"))
			printf("  %s\n", ctx.WorkbenchRoot)
			println()
			println(t.T("workbench.idLabel"))
			printf("  %s\n", wcfg.ID)
			println()
			println(t.T("workbench.projectsLabel"))
			printf("  %d\n", len(reg.Projects))
			println()
			println(t.T("workbench.defaultLabel"))
			if isDefault {
				printf("  %s\n", t.T("workbench.defaultTrue"))
			} else {
				printf("  %s\n", t.T("workbench.defaultFalse"))
			}
			return nil
		},
	}
}

func newWorkPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print Workbench Root path",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, t, err := loadCtxAndT()
			if err != nil {
				return err
			}
			if ctx.WorkbenchRoot == "" || !workbench.IsRoot(ctx.WorkbenchRoot) {
				return workbenchNotFound(t)
			}
			if flags.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]string{"path": ctx.WorkbenchRoot})
			}
			println(ctx.WorkbenchRoot)
			return nil
		},
	}
}

func workbenchNotFound(t interface {
	T(string, ...any) string
}) error {
	msg := fmt.Sprintf("%s\n%s\n%s\n%s",
		t.T("workbench.notFound"),
		t.T("workbench.notFoundHint1"),
		t.T("workbench.notFoundHint2a"),
		t.T("workbench.notFoundHint2b"),
	)
	return Coded(3, errors.New(msg))
}
