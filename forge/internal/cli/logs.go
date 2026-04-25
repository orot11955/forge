package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/orot/forge/internal/workbench"
	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	var tail int
	cmd := &cobra.Command{
		Use:   "logs [latest|file]",
		Short: "Show command logs for the current project",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, t, err := loadCtxAndT()
			if err != nil {
				return err
			}
			if ctx.WorkbenchRoot == "" {
				return workbenchNotFound(t)
			}
			if ctx.ProjectRoot == "" || ctx.ProjectID == "" {
				return projectNotFound(t)
			}
			dir := filepath.Join(ctx.WorkbenchRoot, workbench.LogsDir, ctx.ProjectID)
			logs, err := logFiles(dir)
			if err != nil {
				return err
			}
			if len(args) == 0 {
				return printLogList(cmd, t, logs)
			}
			name := args[0]
			if name == "latest" {
				if len(logs) == 0 {
					return Coded(1, errors.New(t.T("logs.empty")))
				}
				name = logs[len(logs)-1].Name
			}
			if filepath.Base(name) != name {
				return Coded(2, errors.New("log file must be a filename, not a path"))
			}
			p := filepath.Join(dir, name)
			lines, err := readTailLines(p, tail)
			if err != nil {
				return err
			}
			if flags.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"path":  p,
					"lines": lines,
				})
			}
			for _, line := range lines {
				println(line)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&tail, "tail", 200, "Number of lines to print")
	return cmd
}

type logEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
}

func logFiles(dir string) ([]logEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []logEntry{}, nil
		}
		return nil, err
	}
	out := []logEntry{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, logEntry{
			Name:    e.Name(),
			Path:    filepath.Join(dir, e.Name()),
			Size:    info.Size(),
			ModTime: info.ModTime().Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func printLogList(cmd *cobra.Command, t interface{ T(string, ...any) string }, logs []logEntry) error {
	if flags.JSON {
		return writeJSON(cmd.OutOrStdout(), map[string]any{"logs": logs})
	}
	if len(logs) == 0 {
		println(t.T("logs.empty"))
		return nil
	}
	println(t.T("logs.title"))
	for _, e := range logs {
		fmt.Printf("  %-32s %8d  %s\n", e.Name, e.Size, e.ModTime)
	}
	return nil
}

func readTailLines(path string, tail int) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := strings.TrimRight(string(data), "\n")
	if content == "" {
		return []string{}, nil
	}
	lines := strings.Split(content, "\n")
	if tail > 0 && len(lines) > tail {
		lines = lines[len(lines)-tail:]
	}
	return lines, nil
}
