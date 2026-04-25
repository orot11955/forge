package cli

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/orot/forge/internal/deploy"
	"github.com/orot/forge/internal/git"
	"github.com/orot/forge/internal/history"
	"github.com/orot/forge/internal/project"
	"github.com/orot/forge/internal/util"
	"github.com/spf13/cobra"
)

func newDeployCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "deploy <target>",
		Short: "Deploy the project to a target defined in .forge/targets.yaml",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, t, err := loadCtxAndT()
			if err != nil {
				return err
			}
			if ctx.ProjectRoot == "" {
				return projectNotFound(t)
			}

			tf, err := deploy.Load(ctx.ProjectRoot)
			if errors.Is(err, deploy.ErrNoTargets) {
				return Coded(2, errors.New(t.T("deploy.noTargets")))
			}
			if err != nil {
				return Coded(2, err)
			}

			if len(args) == 0 {
				return listTargets(cmd, t, tf)
			}
			name := args[0]
			tgt, ok := tf.Targets[name]
			if !ok {
				return Coded(9, fmt.Errorf(t.T("deploy.unknownTarget"), name))
			}
			if tgt.Type != "ssh-compose" {
				return Coded(9, fmt.Errorf(t.T("deploy.unsupportedType"), tgt.Type))
			}

			// Pre-flight: git dirty + branch check.
			if gs, err := git.Inspect(ctx.ProjectRoot); err == nil && gs.IsRepo {
				if !gs.Clean {
					fmt.Println(t.T("deploy.gitDirty"))
					if !dryRun && !flags.Yes && !confirm(t.T("compose.destructiveConfirm")) {
						return Coded(8, errors.New("deploy aborted: dirty tree"))
					}
				}
				if tgt.Branch != "" && gs.Branch != "" && gs.Branch != tgt.Branch {
					fmt.Println(t.T("deploy.branchMismatch", gs.Branch, tgt.Branch))
				}
			}

			steps := deploy.PlanSSHCompose(tgt)
			if len(steps) == 0 {
				return Coded(2, errors.New("invalid target: missing host/user"))
			}

			if dryRun {
				println(t.T("deploy.dryRunHeader"))
				println()
				println(t.T("deploy.planTitle"))
				for _, line := range deploy.FormatPlan(steps) {
					printf("  %s\n", line)
				}
				return nil
			}

			println(t.T("deploy.starting", name))
			started := util.NowISO()
			results := deploy.RunSteps(steps, tgt.Healthcheck.ExpectedStatus)
			finished := util.NowISO()

			var failed *deploy.StepResult
			for i := range results {
				r := results[i]
				println(t.T("deploy.step", r.Step.Description))
				if r.Err != nil {
					failed = &results[i]
					break
				}
				if r.Step.Kind == "healthcheck" {
					println(t.T("deploy.healthOk", r.Status))
				}
			}

			// Persist history.
			summary := "success"
			status := "success"
			exit := 0
			if failed != nil {
				summary = "failed at: " + failed.Step.Description
				status = "failed"
				exit = 9
			}
			_ = history.Append(project.HistoryPath(ctx.ProjectRoot), history.Event{
				ID:         history.NewID(time.Now()),
				Type:       "command",
				Command:    "forge deploy " + name,
				Status:     status,
				ExitCode:   exit,
				StartedAt:  started,
				FinishedAt: finished,
				Cwd:        ctx.ProjectRoot,
				Summary:    summary,
			})

			if failed != nil {
				println(t.T("deploy.failure", failed.Step.Description))
				return Coded(9, failed.Err)
			}
			println(t.T("deploy.success", name))
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print plan without executing")
	return cmd
}

func listTargets(cmd *cobra.Command, t interface{ T(string, ...any) string }, tf *deploy.File) error {
	names := make([]string, 0, len(tf.Targets))
	for k := range tf.Targets {
		names = append(names, k)
	}
	sort.Strings(names)
	if flags.JSON {
		return writeJSON(cmd.OutOrStdout(), map[string]any{"targets": names})
	}
	for _, n := range names {
		t := tf.Targets[n]
		printf("  %-12s %s  %s@%s:%s\n", n, t.Type, t.User, t.Host, t.Path)
	}
	return nil
}
