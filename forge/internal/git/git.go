package git

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Status struct {
	IsRepo   bool
	Branch   string
	Upstream string
	Clean    bool
	Dirty    []string // porcelain lines
	Remotes  []Remote
}

type Remote struct {
	Name string
	URL  string
}

// IsRepo returns true if the directory is inside a git work tree.
func IsRepo(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return true
	}
	out, err := run(dir, "git", "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) == "true"
}

// Inspect returns a populated Status snapshot for dir.
func Inspect(dir string) (*Status, error) {
	s := &Status{}
	if !IsRepo(dir) {
		return s, nil
	}
	s.IsRepo = true
	if br, err := run(dir, "git", "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		s.Branch = strings.TrimSpace(br)
	}
	if up, err := run(dir, "git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"); err == nil {
		s.Upstream = strings.TrimSpace(up)
	}
	if porc, err := run(dir, "git", "status", "--porcelain"); err == nil {
		porc = strings.TrimRight(porc, "\n")
		if porc == "" {
			s.Clean = true
		} else {
			s.Dirty = strings.Split(porc, "\n")
		}
	}
	if remotes, err := run(dir, "git", "remote", "-v"); err == nil {
		seen := map[string]struct{}{}
		for _, line := range strings.Split(strings.TrimSpace(remotes), "\n") {
			if line == "" {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			key := fields[0]
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			s.Remotes = append(s.Remotes, Remote{Name: fields[0], URL: fields[1]})
		}
	}
	return s, nil
}

// Run executes git in dir with the given args, returning combined output.
// Streams to os.Stdout/os.Stderr if stream is true.
func RunStreamed(dir string, args []string) (int, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), err
	}
	return -1, err
}

func run(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return stdout.String(), nil
}
