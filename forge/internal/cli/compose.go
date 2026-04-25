package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/orot/forge/internal/compose"
	"github.com/orot/forge/internal/runner"
	"github.com/spf13/cobra"
)

func newComposeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "compose",
		Short:              "Docker Compose wrapper (auto-detects compose file, env warnings, safe defaults)",
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

			composeFile := compose.DetectFile(ctx.ProjectRoot)
			if composeFile == "" {
				return Coded(7, fmt.Errorf(t.T("compose.noFile"), ctx.ProjectRoot))
			}

			bin, lead, err := compose.Cmd()
			if err != nil {
				return Coded(7, err)
			}

			// destructive ops → confirm
			if isComposeDestructive(args) && !confirm(t.T("compose.destructiveConfirm")) {
				println(t.T("compose.cancelled"))
				return nil
			}

			// env warning when an `up` is being attempted and no .env
			if len(args) > 0 && args[0] == "up" && !compose.HasEnvFile(ctx.ProjectRoot) {
				fmt.Fprintln(os.Stderr, t.T("compose.envMissing"))
			}

			full := append([]string{}, lead...)
			full = append(full, "-f", composeFile)
			full = append(full, args...)

			if !flags.Quiet {
				printf("%s %s\n", t.T("compose.fileLabel"), composeFile)
			}

			res := runner.Run(runner.Spec{
				Name:       "forge compose",
				Executable: bin,
				Args:       full,
				Cwd:        ctx.ProjectRoot,
			})
			if res.ExitCode != 0 {
				return Coded(7, errors.New("compose exited non-zero"))
			}
			return nil
		},
	}
	return cmd
}

func isComposeDestructive(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "down", "rm":
		return true
	}
	for _, a := range args {
		if a == "--volumes" || a == "-v" || a == "prune" {
			return true
		}
	}
	return false
}
