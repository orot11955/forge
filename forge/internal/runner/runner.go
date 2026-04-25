package runner

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/orot/forge/internal/history"
	"github.com/orot/forge/internal/util"
)

type Spec struct {
	Name       string // human-readable label, e.g. "forge run build"
	Executable string
	Args       []string
	Cwd        string
	Env        []string
	Stdout     io.Writer
	Stderr     io.Writer
	Stdin      io.Reader
}

type Result struct {
	ExitCode   int
	StartedAt  string
	FinishedAt string
	Err        error
}

// Run executes a command synchronously, streaming output to the configured
// writers. Defaults to os.Stdout/os.Stderr if not provided.
func Run(s Spec) Result {
	r := Result{StartedAt: util.NowISO()}
	cmd := exec.Command(s.Executable, s.Args...)
	cmd.Dir = s.Cwd
	if s.Env != nil {
		cmd.Env = s.Env
	}
	cmd.Stdout = orDefault(s.Stdout, os.Stdout)
	cmd.Stderr = orDefault(s.Stderr, os.Stderr)
	cmd.Stdin = s.Stdin
	err := cmd.Run()
	r.FinishedAt = util.NowISO()
	if err != nil {
		var ee *exec.ExitError
		if asExitErr(err, &ee) {
			r.ExitCode = ee.ExitCode()
		} else {
			r.ExitCode = -1
		}
		r.Err = err
		return r
	}
	r.ExitCode = 0
	return r
}

func asExitErr(err error, dst **exec.ExitError) bool {
	if e, ok := err.(*exec.ExitError); ok {
		*dst = e
		return true
	}
	return false
}

func orDefault(w io.Writer, def io.Writer) io.Writer {
	if w == nil {
		return def
	}
	return w
}

// LogPath builds a log path under workbenchRoot/logs for a given project+script.
func LogPath(workbenchRoot, projectID, scriptID string, started time.Time) string {
	if workbenchRoot == "" {
		return ""
	}
	dir := filepath.Join(workbenchRoot, "logs", projectID)
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, fmt.Sprintf("%s_%s.log", started.Format("20060102_150405"), scriptID))
}

// AppendHistory writes a command event to the project's history.jsonl.
func AppendHistory(projectHistoryPath, command string, r Result, cwd, summary string) {
	if projectHistoryPath == "" {
		return
	}
	status := "success"
	if r.ExitCode != 0 {
		status = "failed"
	}
	_ = history.Append(projectHistoryPath, history.Event{
		ID:         history.NewID(time.Now()),
		Type:       "command",
		Command:    command,
		Status:     status,
		ExitCode:   r.ExitCode,
		StartedAt:  r.StartedAt,
		FinishedAt: r.FinishedAt,
		Cwd:        cwd,
		Summary:    summary,
	})
}
