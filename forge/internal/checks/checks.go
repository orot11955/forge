package checks

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type LocalizedTitle struct {
	En string `yaml:"en"`
	Ko string `yaml:"ko"`
}

type Check struct {
	ID       string         `yaml:"id"`
	Title    LocalizedTitle `yaml:"title"`
	Type     string         `yaml:"type"`
	Path     string         `yaml:"path,omitempty"`
	Command  string         `yaml:"command,omitempty"`
	Field    string         `yaml:"field,omitempty"` // dotted path for json/yaml field checks
	Required bool           `yaml:"required"`
	Phase    string         `yaml:"phase"`
}

type File struct {
	Version int     `yaml:"version"`
	Checks  []Check `yaml:"checks"`
}

func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f := &File{}
	if err := yaml.Unmarshal(data, f); err != nil {
		return nil, fmt.Errorf("parse checks.yaml: %w", err)
	}
	return f, nil
}

func Save(path string, f *File) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(f)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

type Result struct {
	Check  Check
	Passed bool
	Detail string
}

func Run(projectRoot string, f *File) []Result {
	out := make([]Result, 0, len(f.Checks))
	for _, c := range f.Checks {
		r := Result{Check: c}
		switch c.Type {
		case "file_exists":
			r.Passed = isFile(filepath.Join(projectRoot, c.Path))
		case "directory_exists":
			r.Passed = isDir(filepath.Join(projectRoot, c.Path))
		case "git_initialized":
			r.Passed = isDir(filepath.Join(projectRoot, ".git"))
		case "docker_available":
			_, err := exec.LookPath("docker")
			r.Passed = err == nil
		case "command_exists":
			_, err := exec.LookPath(c.Command)
			r.Passed = err == nil
		case "json_field_exists":
			r.Passed = jsonFieldExists(filepath.Join(projectRoot, c.Path), c.Field)
		case "yaml_field_exists":
			r.Passed = yamlFieldExists(filepath.Join(projectRoot, c.Path), c.Field)
		default:
			r.Passed = false
			r.Detail = fmt.Sprintf("unsupported check type: %s", c.Type)
		}
		out = append(out, r)
	}
	return out
}

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func jsonFieldExists(path, field string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return false
	}
	return walkField(root, splitField(field))
}

func yamlFieldExists(path, field string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var root any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return false
	}
	return walkField(root, splitField(field))
}

func splitField(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ".")
}

// walkField checks whether `parts` resolves into root.
// Supports nested map access; numeric segments index into arrays.
func walkField(root any, parts []string) bool {
	cur := root
	for _, p := range parts {
		switch v := cur.(type) {
		case map[string]any:
			next, ok := v[p]
			if !ok {
				return false
			}
			cur = next
		case map[any]any:
			next, ok := v[p]
			if !ok {
				return false
			}
			cur = next
		case []any:
			idx := -1
			fmt.Sscanf(p, "%d", &idx)
			if idx < 0 || idx >= len(v) {
				return false
			}
			cur = v[idx]
		default:
			return false
		}
	}
	return cur != nil
}

// DefaultsForType returns the initial checks.yaml contents for a project type.
func DefaultsForType(t string) *File {
	base := []Check{
		{
			ID: "has_readme", Required: true, Phase: "base",
			Title: LocalizedTitle{En: "README.md exists", Ko: "README.md 존재"},
			Type:  "file_exists", Path: "README.md",
		},
		{
			ID: "has_gitignore", Required: false, Phase: "base",
			Title: LocalizedTitle{En: ".gitignore exists", Ko: ".gitignore 존재"},
			Type:  "file_exists", Path: ".gitignore",
		},
		{
			ID: "has_env_example", Required: false, Phase: "base",
			Title: LocalizedTitle{En: ".env.example exists", Ko: ".env.example 존재"},
			Type:  "file_exists", Path: ".env.example",
		},
		{
			ID: "git_initialized", Required: false, Phase: "git",
			Title: LocalizedTitle{En: "Git repository initialized", Ko: "Git 저장소 초기화 여부"},
			Type:  "git_initialized",
		},
	}
	return &File{Version: 1, Checks: base}
}
