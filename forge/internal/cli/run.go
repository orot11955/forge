package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/orot/forge/internal/project"
	"github.com/orot/forge/internal/runner"
	"github.com/orot/forge/internal/scripts"
	"github.com/orot/forge/internal/toolchain"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run [script]",
		Short: "Run a script defined in .forge/scripts.yaml",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, t, err := loadCtxAndT()
			if err != nil {
				return err
			}
			if ctx.ProjectRoot == "" {
				return projectNotFound(t)
			}
			sf, err := loadScripts(project.ScriptsPath(ctx.ProjectRoot))
			if err != nil {
				return Coded(2, fmt.Errorf("read scripts.yaml: %w", err))
			}
			tc, err := toolchain.Load(project.ToolchainPath(ctx.ProjectRoot))
			if err != nil {
				return Coded(2, fmt.Errorf("read toolchain.yaml: %w", err))
			}

			if len(args) == 0 {
				return listScripts(cmd, t, sf)
			}
			name := args[0]
			s, ok := sf.Scripts[name]
			if !ok {
				return Coded(5, fmt.Errorf(t.T("run.scriptNotFound"), name))
			}
			cwd := s.Cwd
			if cwd == "" {
				cwd = "."
			}
			if filepath.IsAbs(cwd) {
				return Coded(2, errors.New("absolute cwd in scripts.yaml is not supported"))
			}
			if s.Shell {
				return Coded(2, errors.New("scripts.yaml shell=true is not supported in MVP"))
			}
			fullCwd := filepath.Join(ctx.ProjectRoot, cwd)
			executable, err := toolchain.ResolveExecutable(s.Command, tc, ctx.WorkbenchRoot)
			if err != nil {
				return Coded(6, err)
			}

			env := envWithToolchainPath(toolchain.PathEntries(ctx.WorkbenchRoot, tc))
			stdout, stderr, logPath, closeLog := commandWriters(ctx.WorkbenchRoot, ctx.ProjectID, name)
			if closeLog != nil {
				defer closeLog()
			}

			if !flags.Quiet && !flags.JSON {
				println(t.T("run.starting", name))
				printf("  %s %v\n", executable, s.Args)
			}

			res := runner.Run(runner.Spec{
				Name:       "forge run " + name,
				Executable: executable,
				Args:       s.Args,
				Cwd:        fullCwd,
				Env:        env,
				Stdout:     stdout,
				Stderr:     stderr,
			})
			runner.AppendHistory(project.HistoryPath(ctx.ProjectRoot),
				"forge run "+name, res, fullCwd,
				fmt.Sprintf("script=%s exit=%d log=%s", name, res.ExitCode, logPath))

			if flags.JSON {
				_ = writeJSON(cmd.OutOrStdout(), map[string]any{
					"script":   name,
					"exitCode": res.ExitCode,
					"logPath":  logPath,
				})
			}
			if res.ExitCode != 0 {
				return Coded(5, fmt.Errorf("script exited with %d", res.ExitCode))
			}
			return nil
		},
	}
}

func envWithToolchainPath(entries []string) []string {
	if len(entries) == 0 {
		return nil
	}
	current := os.Getenv("PATH")
	parts := append([]string{}, entries...)
	if current != "" {
		parts = append(parts, current)
	}
	nextPath := "PATH=" + strings.Join(parts, string(os.PathListSeparator))
	env := os.Environ()
	replaced := false
	for i, v := range env {
		if strings.HasPrefix(v, "PATH=") {
			env[i] = nextPath
			replaced = true
			break
		}
	}
	if !replaced {
		env = append(env, nextPath)
	}
	return env
}

func commandWriters(workbenchRoot, projectID, scriptID string) (io.Writer, io.Writer, string, func()) {
	if workbenchRoot == "" || projectID == "" {
		if flags.JSON {
			return io.Discard, io.Discard, "", nil
		}
		return os.Stdout, os.Stderr, "", nil
	}
	logPath := runner.LogPath(workbenchRoot, projectID, scriptID, time.Now())
	f, err := os.Create(logPath)
	if err != nil {
		if flags.JSON {
			return io.Discard, io.Discard, "", nil
		}
		return os.Stdout, os.Stderr, "", nil
	}
	closeLog := func() { _ = f.Close() }
	if flags.JSON {
		return f, f, logPath, closeLog
	}
	return io.MultiWriter(os.Stdout, f), io.MultiWriter(os.Stderr, f), logPath, closeLog
}

func listScripts(cmd *cobra.Command, t interface{ T(string, ...any) string }, sf *scripts.File) error {
	names := make([]string, 0, len(sf.Scripts))
	for k := range sf.Scripts {
		names = append(names, k)
	}
	sort.Strings(names)
	if flags.JSON {
		return writeJSON(cmd.OutOrStdout(), map[string]any{
			"scripts": names,
		})
	}
	println(t.T("run.scriptListHeader"))
	for _, n := range names {
		s := sf.Scripts[n]
		printf("  %-12s %s %v\n", n, s.Command, s.Args)
	}
	return nil
}

func loadScripts(path string) (*scripts.File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f := &scripts.File{}
	if err := yaml.Unmarshal(data, f); err != nil {
		return nil, err
	}
	if f.Scripts == nil {
		f.Scripts = map[string]scripts.Script{}
	}
	return f, nil
}
