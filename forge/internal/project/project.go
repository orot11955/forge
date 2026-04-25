package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	DirName       = ".forge"
	YAMLName      = "project.yaml"
	ToolchainName = "toolchain.yaml"
	ChecksName    = "checks.yaml"
	ScriptsName   = "scripts.yaml"
	HistoryName   = "history.jsonl"
)

const (
	LifecycleInitialized = "initialized"
	LifecycleGenerated   = "generated"
	LifecycleChecked     = "checked"
	LifecycleDockerized  = "dockerized"
	LifecycleDeployed    = "deployed"

	LocationManaged  = "managed"
	LocationExternal = "external"
)

// lifecycleOrder defines the ascending lifecycle progression.
var lifecycleOrder = map[string]int{
	LifecycleInitialized: 1,
	LifecycleGenerated:   2,
	LifecycleChecked:     3,
	LifecycleDockerized:  4,
	LifecycleDeployed:    5,
}

func LifecycleAtLeast(current, target string) bool {
	c, ok1 := lifecycleOrder[current]
	t, ok2 := lifecycleOrder[target]
	if !ok1 || !ok2 {
		return false
	}
	return c >= t
}

type WorkbenchRef struct {
	ID string `yaml:"id,omitempty"`
}

type Runtime struct {
	Primary        string `yaml:"primary,omitempty"`
	PackageManager string `yaml:"packageManager,omitempty"`
}

type Lifecycle struct {
	Current string `yaml:"current"`
}

type Config struct {
	Version      int               `yaml:"version"`
	ID           string            `yaml:"id"`
	Name         string            `yaml:"name"`
	Description  string            `yaml:"description"`
	Type         string            `yaml:"type"`
	Template     *string           `yaml:"template"`
	LocationType string            `yaml:"locationType"`
	CreatedAt    string            `yaml:"createdAt"`
	UpdatedAt    string            `yaml:"updatedAt"`
	Workbench    WorkbenchRef      `yaml:"workbench,omitempty"`
	Paths        map[string]string `yaml:"paths,omitempty"`
	Runtime      Runtime           `yaml:"runtime,omitempty"`
	Lifecycle    Lifecycle         `yaml:"lifecycle"`
}

func YAMLPath(root string) string      { return filepath.Join(root, DirName, YAMLName) }
func ChecksPath(root string) string    { return filepath.Join(root, DirName, ChecksName) }
func ToolchainPath(root string) string { return filepath.Join(root, DirName, ToolchainName) }
func ScriptsPath(root string) string   { return filepath.Join(root, DirName, ScriptsName) }
func HistoryPath(root string) string   { return filepath.Join(root, DirName, HistoryName) }

func IsRoot(root string) bool {
	_, err := os.Stat(YAMLPath(root))
	return err == nil
}

func Load(root string) (*Config, error) {
	data, err := os.ReadFile(YAMLPath(root))
	if err != nil {
		return nil, err
	}
	c := &Config{}
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("parse project.yaml: %w", err)
	}
	return c, nil
}

func Save(root string, c *Config) error {
	if err := os.MkdirAll(filepath.Join(root, DirName), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(YAMLPath(root), data, 0o644)
}

// FindRoot walks upward from start looking for .forge/project.yaml.
func FindRoot(start string) (string, bool) {
	dir := start
	for {
		if IsRoot(dir) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

var ErrExists = errors.New("project already initialized")
