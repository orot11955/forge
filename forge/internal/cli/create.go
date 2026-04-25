package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/orot/forge/internal/config"
	"github.com/orot/forge/internal/project"
	"github.com/orot/forge/internal/runner"
	"github.com/orot/forge/internal/templates"
	"github.com/orot/forge/internal/util"
	"github.com/orot/forge/internal/workbench"
	"github.com/spf13/cobra"
)

func newCreateAliasCmd() *cobra.Command {
	cmd := newProjectCreateCmd()
	cmd.Use = "create <name>"
	cmd.Short = "Create a new project (alias for 'project create')"
	return cmd
}

func newProjectCreateCmd() *cobra.Command {
	var (
		typeFlag   string
		pathFlag   string
		pmFlag     string
		moduleFlag string
		dryRun     bool
		forceFlag  bool
	)
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new project from a template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			ctx, t, err := loadCtxAndT()
			if err != nil {
				return err
			}

			if typeFlag == "" {
				typeFlag = "generic"
			}
			tmpl, err := templates.Find(typeFlag)
			if err != nil {
				return Coded(2, fmt.Errorf(t.T("create.unknownType"), typeFlag))
			}

			// Resolve target path.
			var target string
			if pathFlag != "" {
				target = config.ExpandPath(pathFlag)
			} else if ctx.WorkbenchRoot != "" {
				target = filepath.Join(ctx.WorkbenchRoot, workbench.ProjectsDir, name)
			} else {
				cwd, _ := os.Getwd()
				target = filepath.Join(cwd, name)
			}

			// Path conflict check.
			if entries, err := os.ReadDir(target); err == nil && len(entries) > 0 {
				if !forceFlag {
					return Coded(1, fmt.Errorf(t.T("create.pathExists"), target))
				}
			}

			// Resolve template variables.
			pm := pmFlag
			if pm == "" {
				pm = tmpl.DefaultPackageManager
			}
			if pm == "" {
				pm = "yarn"
			}
			module := moduleFlag
			if module == "" {
				module = "example.com/" + name
			}
			vars := map[string]string{
				"projectName":    name,
				"packageManager": pm,
				"goModule":       module,
			}

			// Dry run summary.
			if dryRun {
				execPlan := []string{}
				if tmpl.Command != nil {
					execPlan = append([]string{tmpl.Command.Executable}, tmpl.Command.Render(vars)...)
				}
				if flags.JSON {
					return writeJSON(cmd.OutOrStdout(), map[string]any{
						"status": "dry-run",
						"type":   tmpl.Type,
						"target": target,
						"exec":   execPlan,
					})
				}
				println(t.T("create.dryRunHeader"))
				println()
				println(t.T("create.dryRunPlan"))
				printf("  type:    %s\n", tmpl.Type)
				printf("  target:  %s\n", target)
				if tmpl.Command != nil {
					printf("  exec:    %s %v\n", tmpl.Command.Executable, tmpl.Command.Render(vars))
				}
				return nil
			}

			// Run the template command (if any).
			if tmpl.Command != nil {
				runIn := tmpl.RunIn
				if runIn == "" {
					runIn = "parent"
				}
				var execCwd string
				switch runIn {
				case "project":
					if err := os.MkdirAll(target, 0o755); err != nil {
						return err
					}
					execCwd = target
				case "parent":
					if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
						return err
					}
					execCwd = filepath.Dir(target)
				}
				renderedArgs := tmpl.Command.Render(vars)
				stdout, stderr, _, closeLog := commandWriters(ctx.WorkbenchRoot, util.Slugify(name), "create")
				if closeLog != nil {
					defer closeLog()
				}
				if !flags.JSON && !flags.Quiet {
					printf("%s %s %v\n", t.T("create.running"), tmpl.Command.Executable, renderedArgs)
				}
				res := runner.Run(runner.Spec{
					Name:       "forge create",
					Executable: tmpl.Command.Executable,
					Args:       renderedArgs,
					Cwd:        execCwd,
					Stdout:     stdout,
					Stderr:     stderr,
				})
				if res.ExitCode != 0 {
					rollback(target, t)
					return Coded(5, fmt.Errorf("template command failed (exit %d)", res.ExitCode))
				}
			} else {
				// No external command (generic): just make the dir.
				if err := os.MkdirAll(target, 0o755); err != nil {
					return err
				}
			}

			// Verify target dir actually exists.
			if _, err := os.Stat(target); err != nil {
				return Coded(5, fmt.Errorf("target directory not created: %s", target))
			}

			// Materialize Forge metadata, marking lifecycle as generated.
			pcfg, err := project.Materialize(target, project.MaterializeOptions{
				WorkbenchRoot: ctx.WorkbenchRoot,
				WorkbenchID:   ctx.WorkbenchID,
				InitialState:  project.LifecycleGenerated,
				HistoryNote:   fmt.Sprintf("project created from template '%s'", tmpl.ID),
				Command:       "forge create",
			})
			if err != nil {
				return err
			}

			if flags.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"status":       "created",
					"id":           pcfg.ID,
					"path":         target,
					"type":         pcfg.Type,
					"locationType": pcfg.LocationType,
					"template":     tmpl.ID,
				})
			}
			println(t.T("create.success", pcfg.ID, target))
			return nil
		},
	}
	cmd.Flags().StringVar(&typeFlag, "type", "", "Project type (generic|next|nest|go|java)")
	cmd.Flags().StringVar(&pathFlag, "path", "", "Target path (overrides Workbench/projects)")
	cmd.Flags().StringVar(&pmFlag, "package-manager", "", "Override package manager (yarn|npm|pnpm)")
	cmd.Flags().StringVar(&moduleFlag, "module", "", "Go module path (for type=go)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print plan without executing")
	cmd.Flags().BoolVar(&forceFlag, "force", false, "Allow non-empty target directory")
	return cmd
}

func rollback(target string, t interface{ T(string, ...any) string }) {
	entries, err := os.ReadDir(target)
	if err != nil {
		return
	}
	if len(entries) == 0 {
		_ = os.Remove(target)
		fmt.Fprintln(os.Stderr, t.T("create.rolledBack", target))
		return
	}
	fmt.Fprintln(os.Stderr, t.T("create.rollbackKept", target))
}

// keep unused-import suppression friendly
var _ = errors.New
