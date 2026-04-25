package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/orot/forge/internal/context"
	"github.com/orot/forge/internal/i18n"
	"github.com/orot/forge/internal/project"
	"github.com/orot/forge/internal/workbench"
	"github.com/spf13/cobra"
)

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "project",
		Short:   "Manage projects",
		Aliases: []string{"pro"},
	}
	cmd.AddCommand(
		newProjectInitCmd(),
		newProjectStatusCmd(),
		newProjectListCmd(),
		newProjectCreateCmd(),
	)
	return cmd
}

func newInitAliasCmd() *cobra.Command {
	c := newProjectInitCmd()
	c.Use = "init"
	c.Short = "Register the current directory as a Forge project (alias for 'project init')"
	return c
}

func newStatusAliasCmd() *cobra.Command {
	c := newProjectStatusCmd()
	c.Use = "status"
	c.Short = "Show project status (alias for 'project status')"
	return c
}

func newListAliasCmd() *cobra.Command {
	c := newProjectListCmd()
	c.Use = "list"
	c.Short = "List registered projects (alias for 'project list')"
	return c
}

// ---------- project init ----------

func newProjectInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Register an existing project as a Forge project",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, t, err := loadCtxAndT()
			if err != nil {
				return err
			}
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			if project.IsRoot(cwd) {
				if flags.JSON {
					return writeJSON(cmd.OutOrStdout(), map[string]any{
						"status": "exists",
						"path":   cwd,
					})
				}
				println(t.T("project.initExists", cwd))
				println(t.T("project.initRunStatus"))
				return nil
			}

			pcfg, err := project.Materialize(cwd, project.MaterializeOptions{
				WorkbenchRoot: ctx.WorkbenchRoot,
				WorkbenchID:   ctx.WorkbenchID,
				InitialState:  project.LifecycleInitialized,
				HistoryNote:   "project initialized",
				Command:       "forge init",
			})
			if err != nil {
				return err
			}
			if flags.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"status":       "created",
					"id":           pcfg.ID,
					"path":         cwd,
					"type":         pcfg.Type,
					"locationType": pcfg.LocationType,
				})
			}
			for _, w := range project.DetectorWarnings(cwd) {
				fmt.Fprintln(os.Stderr, w)
			}
			println(t.T("project.initSuccess", pcfg.ID))
			println(t.T("project.initRunStatus"))
			return nil
		},
	}
}

// ---------- project status ----------

func newProjectStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show project status",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, t, err := loadCtxAndT()
			if err != nil {
				return err
			}
			if ctx.ProjectRoot == "" {
				return projectNotFound(t)
			}
			pcfg, err := project.Load(ctx.ProjectRoot)
			if err != nil {
				return err
			}
			if flags.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"context": map[string]any{
						"workbenchRoot": ctx.WorkbenchRoot,
						"projectRoot":   ctx.ProjectRoot,
						"projectType":   pcfg.Type,
						"locationType":  pcfg.LocationType,
						"language":      string(ctx.Lang),
					},
					"project": map[string]any{
						"id":   pcfg.ID,
						"name": pcfg.Name,
					},
					"lifecycle": map[string]string{
						"current": pcfg.Lifecycle.Current,
					},
				})
			}
			println(t.T("project.statusTitle"))
			println()
			println(t.T("project.contextSection"))
			printf("  %-10s %s\n", t.T("project.workbenchLabel"), ctx.WorkbenchRoot)
			printf("  %-10s %s\n", t.T("project.pathLabel"), ctx.ProjectRoot)
			printf("  %-10s %s\n", t.T("project.typeLabel"), pcfg.Type)
			tmpl := "null"
			if pcfg.Template != nil {
				tmpl = *pcfg.Template
			}
			printf("  %-10s %s\n", t.T("project.templateLabel"), tmpl)
			printf("  %-10s %s\n", t.T("project.locationLabel"), pcfg.LocationType)
			println()
			println(t.T("project.lifecycleSection"))
			printf("  %-10s %s\n", t.T("project.currentLabel"), pcfg.Lifecycle.Current)
			return nil
		},
	}
}

func printKV(label, value string) {
	printf("%s\n  %s\n\n", label, value)
}

// ---------- project list ----------

func newProjectListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List registered projects in the Workbench",
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
				return err
			}

			type entry struct {
				ID           string `json:"id"`
				Name         string `json:"name"`
				Path         string `json:"path"`
				Type         string `json:"type"`
				Status       string `json:"status"`
				LocationType string `json:"location_type"`
				State        string `json:"state"`
			}
			out := make([]entry, 0, len(reg.Projects))
			for _, p := range reg.Projects {
				state := "ok"
				if !pathExists(p.Path) {
					state = "stale"
				}
				out = append(out, entry{
					ID: p.ID, Name: p.Name, Path: p.Path, Type: p.Type,
					Status: p.Status, LocationType: p.LocationType, State: state,
				})
			}

			if flags.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"workbench": ctx.WorkbenchRoot,
					"projects":  out,
				})
			}

			println(t.T("list.title", ctx.WorkbenchRoot))
			if len(out) == 0 {
				println()
				println(t.T("list.empty"))
				return nil
			}
			println()
			fmt.Printf("%-16s %-8s %-12s %-10s %-40s %s\n",
				t.T("list.headerName"), t.T("list.headerType"), t.T("list.headerStatus"),
				t.T("list.headerLocation"), t.T("list.headerPath"), t.T("list.headerState"))
			for _, e := range out {
				fmt.Printf("%-16s %-8s %-12s %-10s %-40s %s\n",
					e.Name, e.Type, e.Status, e.LocationType, e.Path, e.State)
			}
			return nil
		},
	}
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func projectNotFound(t *i18n.Translator) error {
	msg := fmt.Sprintf("%s\n%s\n%s\n%s",
		t.T("project.notFound"),
		t.T("project.notFoundHint1"),
		t.T("project.notFoundHint2a"),
		t.T("project.notFoundHint2b"),
	)
	return Coded(3, errors.New(msg))
}

// ensure context import retained
var _ = context.Resolve
