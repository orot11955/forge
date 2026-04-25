package deploy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const TargetsFileName = "targets.yaml"

type Healthcheck struct {
	URL            string `yaml:"url"`
	ExpectedStatus int    `yaml:"expectedStatus,omitempty"`
}

type Target struct {
	Type        string      `yaml:"type"` // "ssh-compose"
	Host        string      `yaml:"host"`
	User        string      `yaml:"user"`
	Port        int         `yaml:"port,omitempty"`
	Path        string      `yaml:"path"`
	Branch      string      `yaml:"branch,omitempty"`
	ComposeFile string      `yaml:"composeFile,omitempty"`
	Healthcheck Healthcheck `yaml:"healthcheck,omitempty"`
}

type File struct {
	Targets map[string]Target `yaml:"targets"`
}

func Path(projectRoot string) string {
	return filepath.Join(projectRoot, ".forge", TargetsFileName)
}

func Load(projectRoot string) (*File, error) {
	p := Path(projectRoot)
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNoTargets
		}
		return nil, fmt.Errorf("read %s: %w", p, err)
	}
	f := &File{}
	if err := yaml.Unmarshal(data, f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", p, err)
	}
	if len(f.Targets) == 0 {
		return nil, ErrNoTargets
	}
	return f, nil
}

var ErrNoTargets = errors.New("no targets defined")
