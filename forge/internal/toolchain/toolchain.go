package toolchain

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Spec struct {
	Version string `yaml:"version"`
	Source  string `yaml:"source"`
}

type Policy struct {
	Mode             string `yaml:"mode"`
	FallbackToSystem bool   `yaml:"fallbackToSystem"`
}

type Config struct {
	Version         int             `yaml:"version"`
	Policy          Policy          `yaml:"policy"`
	Runtimes        map[string]Spec `yaml:"runtimes,omitempty"`
	PackageManagers map[string]Spec `yaml:"packageManagers,omitempty"`
	SystemTools     map[string]Spec `yaml:"systemTools,omitempty"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c := &Config{}
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("parse toolchain.yaml: %w", err)
	}
	return c, nil
}

// Resolved represents a single tool entry after resolution against the host.
type Resolved struct {
	Name      string
	Spec      Spec
	Path      string // empty if not found
	Version   string // best-effort, may be empty
	Available bool
}

// Resolve looks up the binary in PATH and probes its version.
// Returns Resolved with Available=false when not found.
func Resolve(name string, spec Spec) Resolved {
	r := Resolved{Name: name, Spec: spec}
	p, err := exec.LookPath(name)
	if err != nil {
		return r
	}
	r.Path = p
	r.Available = true
	r.Version = probeVersion(name)
	return r
}

func ResolveExecutable(name string, c *Config, workbenchRoot string) (string, error) {
	spec, declared := specFor(name, c)
	searchWorkbench := declared && spec.Source == "workbench"
	if declared && spec.Source == "" {
		searchWorkbench = policyMode(c) == "workbench-first"
	}
	if searchWorkbench || policyMode(c) == "workbench-first" {
		if p := findInPaths(name, PathEntries(workbenchRoot, c)); p != "" {
			return p, nil
		}
	}
	if canFallbackToSystem(c) || !declared || spec.Source == "system" {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("tool not available: %s", name)
}

func PathEntries(workbenchRoot string, c *Config) []string {
	if workbenchRoot == "" || c == nil {
		return nil
	}
	mode := policyMode(c)
	if mode == "system" {
		return nil
	}
	seen := map[string]struct{}{}
	out := []string{}
	add := func(p string) {
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	for name := range c.Runtimes {
		add(filepath.Join(workbenchRoot, "runtimes", name, "bin"))
	}
	for name := range c.PackageManagers {
		add(filepath.Join(workbenchRoot, "package-managers", name, "bin"))
	}
	add(filepath.Join(workbenchRoot, "tools", "bin"))
	for name := range c.SystemTools {
		add(filepath.Join(workbenchRoot, "tools", name, "bin"))
	}
	return out
}

func specFor(name string, c *Config) (Spec, bool) {
	if c == nil {
		return Spec{}, false
	}
	for _, m := range []map[string]Spec{c.Runtimes, c.PackageManagers, c.SystemTools} {
		if spec, ok := m[name]; ok {
			return spec, true
		}
	}
	return Spec{}, false
}

func policyMode(c *Config) string {
	if c == nil || c.Policy.Mode == "" {
		return "auto"
	}
	return c.Policy.Mode
}

func canFallbackToSystem(c *Config) bool {
	if c == nil {
		return true
	}
	if c.Policy.Mode == "" {
		return true
	}
	return c.Policy.FallbackToSystem
}

func findInPaths(name string, paths []string) string {
	for _, dir := range paths {
		p := filepath.Join(dir, name)
		if isExecutable(p) {
			return p
		}
	}
	return ""
}

func isExecutable(p string) bool {
	st, err := os.Stat(p)
	if err != nil || st.IsDir() {
		return false
	}
	return st.Mode()&0o111 != 0
}

func probeVersion(name string) string {
	args, ok := versionArgs[name]
	if !ok {
		args = []string{"--version"}
	}
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	line := strings.TrimSpace(out.String())
	if line == "" {
		return ""
	}
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	return line
}

var versionArgs = map[string][]string{
	"go":   {"version"},
	"java": {"-version"}, // outputs to stderr; we capture both
	"ssh":  {"-V"},       // also stderr
}

func Save(path string, c *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func DefaultsForType(projectType, packageManager string) *Config {
	c := &Config{
		Version: 1,
		Policy:  Policy{Mode: "auto", FallbackToSystem: true},
		SystemTools: map[string]Spec{
			"git":    {Version: "any", Source: "system"},
			"docker": {Version: "any", Source: "system"},
			"ssh":    {Version: "any", Source: "system"},
		},
	}
	switch projectType {
	case "node", "next", "nest":
		c.Runtimes = map[string]Spec{
			"node": {Version: "any", Source: "system"},
		}
		pm := packageManager
		if pm == "" {
			pm = "npm"
		}
		c.PackageManagers = map[string]Spec{
			pm: {Version: "any", Source: "system"},
		}
	case "go":
		c.Runtimes = map[string]Spec{
			"go": {Version: "any", Source: "system"},
		}
	case "java":
		c.Runtimes = map[string]Spec{
			"java": {Version: "any", Source: "system"},
		}
	}
	return c
}
