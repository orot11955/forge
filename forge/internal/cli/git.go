package cli

import (
	"errors"
	"fmt"

	"github.com/orot/forge/internal/git"
	"github.com/spf13/cobra"
)

func newGitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "git",
		Short:              "Git wrapper (delegates to system git, with Forge status formatting)",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			args = extractGlobalFlags(args)
			ctx, t, err := loadCtxAndT()
			if err != nil {
				return err
			}
			if ctx.ProjectRoot == "" {
				return projectNotFound(t)
			}

			// Special-case: `forge git status` → Forge formatted summary
			// (use --raw to bypass and call system git status).
			if len(args) > 0 && args[0] == "status" && !hasArg(args, "--raw") {
				return printGitStatus(cmd, ctx.ProjectRoot, t)
			}

			// Otherwise delegate.
			if !git.IsRepo(ctx.ProjectRoot) && (len(args) == 0 || args[0] != "init") {
				return Coded(8, fmt.Errorf(t.T("git.notRepo"), ctx.ProjectRoot))
			}
			code, err := git.RunStreamed(ctx.ProjectRoot, args)
			if err != nil {
				if code == 0 {
					code = 8
				}
				return Coded(code, err)
			}
			return nil
		},
	}
	return cmd
}

func hasArg(args []string, s string) bool {
	for _, a := range args {
		if a == s {
			return true
		}
	}
	return false
}

func printGitStatus(cmd *cobra.Command, dir string, t interface{ T(string, ...any) string }) error {
	s, err := git.Inspect(dir)
	if err != nil {
		return Coded(8, err)
	}
	if !s.IsRepo {
		if flags.JSON {
			return writeJSON(cmd.OutOrStdout(), map[string]any{"isRepo": false})
		}
		return Coded(8, errors.New(fmt.Sprintf(t.T("git.notRepo"), dir)))
	}
	if flags.JSON {
		return writeJSON(cmd.OutOrStdout(), map[string]any{
			"isRepo":   true,
			"branch":   s.Branch,
			"upstream": s.Upstream,
			"clean":    s.Clean,
			"dirty":    s.Dirty,
			"remotes":  s.Remotes,
		})
	}
	println(t.T("git.statusTitle"))
	println()
	println(t.T("git.branchLabel"))
	upstream := s.Upstream
	if upstream == "" {
		upstream = t.T("git.noUpstream")
	}
	printf("  %s  ⟶  %s\n\n", s.Branch, upstream)

	println(t.T("git.remoteLabel"))
	if len(s.Remotes) == 0 {
		printf("  %s\n\n", t.T("git.noRemote"))
	} else {
		for _, r := range s.Remotes {
			printf("  %s  %s\n", r.Name, r.URL)
		}
		println()
	}

	if s.Clean {
		println(t.T("git.cleanLabel"))
	} else {
		println(t.T("git.dirtyLabel"))
		for _, line := range s.Dirty {
			printf("  %s\n", line)
		}
	}
	return nil
}
