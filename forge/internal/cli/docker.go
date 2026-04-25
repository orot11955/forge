package cli

import (
	"errors"
	"os"
	"os/exec"

	"github.com/orot/forge/internal/runner"
	"github.com/spf13/cobra"
)

func newDockerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "docker",
		Short:              "Docker wrapper (delegates to system docker, with safety checks)",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			args = extractGlobalFlags(args)
			_, t, err := loadCtxAndT()
			if err != nil {
				return err
			}
			if _, err := exec.LookPath("docker"); err != nil {
				return Coded(7, errors.New(t.T("docker.notInstalled")))
			}

			// `forge docker check` → daemon ping
			if len(args) == 1 && args[0] == "check" {
				return dockerCheck(cmd, t)
			}

			// destructive ops → confirm unless --yes
			if isDockerDestructive(args) && !confirm(t.T("docker.destructiveConfirm")) {
				println(t.T("docker.cancelled"))
				return nil
			}

			res := runner.Run(runner.Spec{
				Name:       "forge docker",
				Executable: "docker",
				Args:       args,
				Cwd:        mustGetwd(),
			})
			if res.ExitCode != 0 {
				return Coded(7, errors.New("docker exited non-zero"))
			}
			return nil
		},
	}
	return cmd
}

func dockerCheck(cmd *cobra.Command, t interface{ T(string, ...any) string }) error {
	out, err := exec.Command("docker", "info", "--format", "{{.ServerVersion}}").CombinedOutput()
	if err != nil {
		if flags.JSON {
			return writeJSON(cmd.OutOrStdout(), map[string]any{
				"daemon":    "down",
				"installed": true,
				"detail":    string(out),
			})
		}
		println(t.T("docker.daemonDown"))
		return Coded(7, errors.New("docker daemon unreachable"))
	}
	if flags.JSON {
		return writeJSON(cmd.OutOrStdout(), map[string]any{
			"daemon":  "up",
			"version": trimNL(string(out)),
		})
	}
	println(t.T("docker.daemonUp"))
	printf("  ServerVersion: %s\n", trimNL(string(out)))
	return nil
}

func isDockerDestructive(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "rm", "rmi", "system", "volume", "network", "image":
		// docker system prune, docker volume rm, etc. are destructive
		return true
	case "container":
		if len(args) > 1 && args[1] == "prune" {
			return true
		}
	}
	// "prune" anywhere
	for _, a := range args {
		if a == "prune" {
			return true
		}
	}
	return false
}

func mustGetwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func trimNL(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
