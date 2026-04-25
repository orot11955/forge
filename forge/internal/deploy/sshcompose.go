package deploy

import (
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Step represents a single planned action in the deploy flow.
type Step struct {
	Description string   // human label
	Kind        string   // "ssh", "healthcheck"
	Args        []string // SSH: ["ssh", "user@host", "cmd"]
	URL         string   // for healthcheck
}

// PlanSSHCompose builds the ordered step list for an ssh-compose deploy.
func PlanSSHCompose(target Target) []Step {
	user := target.User
	host := target.Host
	if user == "" || host == "" {
		return nil
	}
	dest := user + "@" + host
	composeFile := target.ComposeFile
	if composeFile == "" {
		composeFile = "docker-compose.yml"
	}
	branch := target.Branch
	if branch == "" {
		branch = "main"
	}

	sshBase := []string{}
	if target.Port != 0 {
		sshBase = append(sshBase, "-p", strconv.Itoa(target.Port))
	}
	sshBase = append(sshBase, dest)

	remote := func(label, cmd string) Step {
		return Step{
			Description: label,
			Kind:        "ssh",
			Args:        append(append([]string{}, sshBase...), cmd),
		}
	}

	steps := []Step{
		remote("connection probe",
			"true"),
		remote(fmt.Sprintf("cd %s && git fetch && git checkout %s && git pull --ff-only", target.Path, branch),
			fmt.Sprintf("cd %s && git fetch && git checkout %s && git pull --ff-only", target.Path, branch)),
		remote(fmt.Sprintf("verify .env exists at %s", target.Path),
			fmt.Sprintf("test -f %s/.env", target.Path)),
		remote(fmt.Sprintf("docker compose -f %s config", composeFile),
			fmt.Sprintf("cd %s && docker compose -f %s config >/dev/null", target.Path, composeFile)),
		remote(fmt.Sprintf("docker compose -f %s up -d --build", composeFile),
			fmt.Sprintf("cd %s && docker compose -f %s up -d --build", target.Path, composeFile)),
	}

	if target.Healthcheck.URL != "" {
		steps = append(steps, Step{
			Description: "healthcheck " + target.Healthcheck.URL,
			Kind:        "healthcheck",
			URL:         target.Healthcheck.URL,
		})
	}
	return steps
}

// ExecStep runs a single step. For ssh steps it streams to stdout/stderr
// and returns the exit code. For healthcheck it performs an HTTP GET.
type StepResult struct {
	Step     Step
	ExitCode int
	Status   int // for healthcheck
	Err      error
}

// RunSteps executes plan steps sequentially, stopping on first error.
// streamCmd controls whether ssh exec output is streamed (true) or captured (false).
func RunSteps(steps []Step, expectedStatus int) []StepResult {
	out := make([]StepResult, 0, len(steps))
	for _, s := range steps {
		r := StepResult{Step: s}
		switch s.Kind {
		case "ssh":
			cmd := exec.Command("ssh", s.Args...)
			cmd.Stdout = stdoutWriter()
			cmd.Stderr = stderrWriter()
			err := cmd.Run()
			if err != nil {
				if ee, ok := err.(*exec.ExitError); ok {
					r.ExitCode = ee.ExitCode()
				} else {
					r.ExitCode = -1
				}
				r.Err = err
			}
		case "healthcheck":
			expected := expectedStatus
			if expected == 0 {
				expected = 200
			}
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Get(s.URL)
			if err != nil {
				r.Err = err
				r.ExitCode = -1
			} else {
				_ = resp.Body.Close()
				r.Status = resp.StatusCode
				if resp.StatusCode != expected {
					r.Err = fmt.Errorf("status %d (expected %d)", resp.StatusCode, expected)
					r.ExitCode = 1
				}
			}
		}
		out = append(out, r)
		if r.Err != nil {
			break
		}
	}
	return out
}

// FormatPlan returns one line per step suitable for human dry-run output.
func FormatPlan(steps []Step) []string {
	out := make([]string, 0, len(steps))
	for _, s := range steps {
		switch s.Kind {
		case "ssh":
			out = append(out, "ssh "+strings.Join(s.Args, " "))
		case "healthcheck":
			out = append(out, "GET "+s.URL)
		}
	}
	return out
}
