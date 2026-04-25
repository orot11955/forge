package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Global struct {
	DefaultWorkbench  string   `yaml:"defaultWorkbench,omitempty"`
	Language          string   `yaml:"language,omitempty"`
	RecentWorkbenches []string `yaml:"recentWorkbenches,omitempty"`
}

func Path() (string, error) {
	if env := os.Getenv("FORGE_CONFIG"); env != "" {
		return env, nil
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "forge", "config.yaml"), nil
}

func Load() (*Global, string, error) {
	p, err := Path()
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Global{}, p, nil
		}
		return nil, p, fmt.Errorf("read config: %w", err)
	}
	g := &Global{}
	if err := yaml.Unmarshal(data, g); err != nil {
		return nil, p, fmt.Errorf("parse config: %w", err)
	}
	return g, p, nil
}

func Save(g *Global) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}
	data, err := yaml.Marshal(g)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func ExpandPath(p string) string {
	if p == "" {
		return p
	}
	if p == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			return home
		}
	}
	if len(p) >= 2 && p[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	abs, err := filepath.Abs(p)
	if err == nil {
		return abs
	}
	return p
}
