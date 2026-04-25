package scripts

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Script struct {
	Cwd     string   `yaml:"cwd"`
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
	Shell   bool     `yaml:"shell"`
}

type File struct {
	Version int               `yaml:"version"`
	Scripts map[string]Script `yaml:"scripts"`
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

// FromNodeScripts converts package.json scripts into a scripts.yaml using the
// chosen package manager (yarn/npm/pnpm/bun).
func FromNodeScripts(pm string, nodeScripts map[string]string) *File {
	f := &File{Version: 1, Scripts: map[string]Script{}}
	if pm == "" {
		pm = "npm"
	}
	for name := range nodeScripts {
		f.Scripts[name] = Script{
			Cwd:     ".",
			Command: pm,
			Args:    nodeScriptArgs(pm, name),
			Shell:   false,
		}
	}
	return f
}

func nodeScriptArgs(pm, name string) []string {
	switch pm {
	case "npm":
		return []string{"run", name}
	default:
		// yarn/pnpm/bun all accept the script name directly.
		return []string{name}
	}
}

func Empty() *File {
	return &File{Version: 1, Scripts: map[string]Script{}}
}
