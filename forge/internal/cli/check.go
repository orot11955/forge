package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/orot/forge/internal/checks"
	"github.com/orot/forge/internal/history"
	"github.com/orot/forge/internal/i18n"
	"github.com/orot/forge/internal/project"
	"github.com/orot/forge/internal/util"
	"github.com/orot/forge/internal/workbench"
	"github.com/spf13/cobra"
)

func newCheckCmd() *cobra.Command {
	var noUpdate bool
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run project checks defined in checks.yaml",
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
			cf, err := checks.Load(project.ChecksPath(ctx.ProjectRoot))
			if err != nil {
				return Coded(2, fmt.Errorf("read checks.yaml: %w", err))
			}
			results := checks.Run(ctx.ProjectRoot, cf)

			requiredFailedCount := 0
			optionalFailedCount := 0
			for _, r := range results {
				if !r.Passed {
					if r.Check.Required {
						requiredFailedCount++
					} else {
						optionalFailedCount++
					}
				}
			}
			requiredFailed := requiredFailedCount > 0
			optionalFailed := optionalFailedCount > 0

			// Lifecycle transition.
			fromState := pcfg.Lifecycle.Current
			toState := fromState
			if !requiredFailed && !noUpdate {
				if fromState == project.LifecycleInitialized || fromState == project.LifecycleGenerated {
					toState = project.LifecycleChecked
				}
			}
			lifecycleChanged := toState != fromState

			// Output (JSON path)
			if flags.JSON {
				rs := make([]map[string]any, 0, len(results))
				for _, r := range results {
					rs = append(rs, map[string]any{
						"id":       r.Check.ID,
						"type":     r.Check.Type,
						"required": r.Check.Required,
						"phase":    r.Check.Phase,
						"passed":   r.Passed,
					})
				}
				payload := map[string]any{
					"context": map[string]any{
						"workbenchRoot": ctx.WorkbenchRoot,
						"projectRoot":   ctx.ProjectRoot,
						"projectType":   pcfg.Type,
						"locationType":  pcfg.LocationType,
						"language":      string(ctx.Lang),
					},
					"projectRoot":    ctx.ProjectRoot,
					"results":        rs,
					"requiredPassed": !requiredFailed,
					"optionalFailed": optionalFailed,
					"checks": map[string]int{
						"total":          len(results),
						"requiredFailed": requiredFailedCount,
						"optionalFailed": optionalFailedCount,
					},
					"lifecycle": map[string]string{
						"from": fromState,
						"to":   toState,
					},
				}
				if err := writeJSON(cmd.OutOrStdout(), payload); err != nil {
					return err
				}
			} else {
				printCheckHuman(t, ctx.Lang, results, requiredFailed, lifecycleChanged, fromState, toState)
			}

			// Persist lifecycle transition.
			if lifecycleChanged {
				now := util.NowISO()
				pcfg.Lifecycle.Current = toState
				pcfg.UpdatedAt = now
				if err := project.Save(ctx.ProjectRoot, pcfg); err != nil {
					return err
				}
				_ = history.Append(project.HistoryPath(ctx.ProjectRoot), history.Event{
					ID:     history.NewID(time.Now()),
					Type:   "lifecycle",
					From:   fromState,
					To:     toState,
					Reason: "required checks passed",
					At:     now,
				})
				if ctx.WorkbenchRoot != "" {
					reg, err := workbench.LoadRegistry(ctx.WorkbenchRoot)
					if err == nil {
						for i := range reg.Projects {
							if reg.Projects[i].Path == ctx.ProjectRoot {
								reg.Projects[i].Status = toState
								reg.Projects[i].UpdatedAt = now
								lc := now
								reg.Projects[i].LastCheckedAt = &lc
								_ = workbench.SaveRegistry(ctx.WorkbenchRoot, reg)
								break
							}
						}
					}
				}
			}

			// command history
			finishedAt := util.NowISO()
			summary := "all checks passed"
			status := "success"
			exit := 0
			if requiredFailed {
				summary = "required checks failed"
				status = "failed"
				exit = 4
			} else if optionalFailed {
				summary = "optional checks failed"
			}
			_ = history.Append(project.HistoryPath(ctx.ProjectRoot), history.Event{
				ID:         history.NewID(time.Now()),
				Type:       "command",
				Command:    "forge check",
				Status:     status,
				ExitCode:   exit,
				StartedAt:  finishedAt,
				FinishedAt: finishedAt,
				Cwd:        ctx.Cwd,
				Summary:    summary,
			})

			if requiredFailed {
				return Coded(4, errors.New("required checks failed"))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&noUpdate, "no-update", false, "Do not update lifecycle even if required checks pass")
	return cmd
}

func printCheckHuman(t *i18n.Translator, lang i18n.Lang, results []checks.Result,
	requiredFailed, lifecycleChanged bool, from, to string) {

	println(t.T("check.title"))
	println()

	currentPhase := ""
	for _, r := range results {
		if r.Check.Phase != currentPhase {
			currentPhase = r.Check.Phase
			printf("[%s]\n", currentPhase)
		}
		mark := "✓"
		if !r.Passed {
			if r.Check.Required {
				mark = "✗"
			} else {
				mark = "!"
			}
		}
		title := titleFor(r.Check.Title, lang)
		printf("%s %s\n", mark, title)
	}
	println()
	switch {
	case requiredFailed:
		println(t.T("check.requiredFailed"))
	default:
		println(t.T("check.requiredPassed"))
	}
	if lifecycleChanged {
		println()
		println(t.T("check.lifecycleTransition"))
		printf("  %s\n", t.T("check.lifecycleArrow", from, to))
	} else if !requiredFailed {
		println(t.T("check.lifecycleUnchanged"))
	}
}

func titleFor(t checks.LocalizedTitle, lang i18n.Lang) string {
	if lang == i18n.LangKo && t.Ko != "" {
		return t.Ko
	}
	if t.En != "" {
		return t.En
	}
	return t.Ko
}
