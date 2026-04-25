package workbench

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/orot/forge/internal/util"
	"gopkg.in/yaml.v3"
)

const (
	DirName        = ".forge"
	YAMLName       = "workbench.yaml"
	ConfigName     = "config.yaml"
	RegistryName   = "registry.json" // legacy, kept for migration
	RegistryDBName = "registry.db"
	TargetsName    = "targets.yaml"
	ProjectsDir    = "projects"
	RuntimesDir    = "runtimes"
	SDKsDir        = "sdks"
	PMsDir         = "package-managers"
	ToolsDir       = "tools"
	TemplatesDir   = "templates"
	CacheDir       = "cache"
	LogsDir        = "logs"
)

type Paths struct {
	Projects        string `yaml:"projects"`
	Runtimes        string `yaml:"runtimes"`
	SDKs            string `yaml:"sdks"`
	PackageManagers string `yaml:"packageManagers"`
	Tools           string `yaml:"tools"`
	Templates       string `yaml:"templates"`
	Cache           string `yaml:"cache"`
	Logs            string `yaml:"logs"`
}

type Defaults struct {
	ProjectType    string `yaml:"projectType,omitempty"`
	PackageManager string `yaml:"packageManager,omitempty"`
	NodeVersion    string `yaml:"nodeVersion,omitempty"`
}

type Policy struct {
	ToolchainMode    string `yaml:"toolchainMode,omitempty"`
	FallbackToSystem bool   `yaml:"fallbackToSystem,omitempty"`
}

type Config struct {
	Version   int      `yaml:"version"`
	ID        string   `yaml:"id"`
	Name      string   `yaml:"name"`
	CreatedAt string   `yaml:"createdAt"`
	UpdatedAt string   `yaml:"updatedAt"`
	Paths     Paths    `yaml:"paths"`
	Defaults  Defaults `yaml:"defaults"`
	Policy    Policy   `yaml:"policy"`
}

func DefaultPaths() Paths {
	return Paths{
		Projects:        ProjectsDir,
		Runtimes:        RuntimesDir,
		SDKs:            SDKsDir,
		PackageManagers: PMsDir,
		Tools:           ToolsDir,
		Templates:       TemplatesDir,
		Cache:           CacheDir,
		Logs:            LogsDir,
	}
}

func YAMLPath(root string) string       { return filepath.Join(root, DirName, YAMLName) }
func ConfigPath(root string) string     { return filepath.Join(root, DirName, ConfigName) }
func TargetsPath(root string) string    { return filepath.Join(root, DirName, TargetsName) }
func RegistryPath(root string) string   { return filepath.Join(root, DirName, RegistryName) }
func RegistryDBPath(root string) string { return filepath.Join(root, DirName, RegistryDBName) }

// IsRoot returns true if root contains a Workbench config.
func IsRoot(root string) bool {
	_, err := os.Stat(YAMLPath(root))
	return err == nil
}

// Load reads workbench.yaml from root.
func Load(root string) (*Config, error) {
	data, err := os.ReadFile(YAMLPath(root))
	if err != nil {
		return nil, err
	}
	c := &Config{}
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("parse workbench.yaml: %w", err)
	}
	return c, nil
}

// Init creates the Workbench directory structure at root.
// Returns ErrExists if .forge/workbench.yaml already exists.
func Init(root, id string) (*Config, error) {
	if IsRoot(root) {
		return nil, ErrExists
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir workbench root: %w", err)
	}
	for _, sub := range []string{
		DirName,
		ProjectsDir, RuntimesDir, SDKsDir, PMsDir,
		ToolsDir, TemplatesDir, CacheDir, LogsDir,
	} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", sub, err)
		}
	}
	if id == "" {
		id = util.Slugify(filepath.Base(root))
	}
	now := util.NowISO()
	cfg := &Config{
		Version:   1,
		ID:        id,
		Name:      filepath.Base(root),
		CreatedAt: now,
		UpdatedAt: now,
		Paths:     DefaultPaths(),
		Defaults:  Defaults{ProjectType: "generic", PackageManager: "yarn"},
		Policy:    Policy{ToolchainMode: "auto", FallbackToSystem: true},
	}
	if err := Save(root, cfg); err != nil {
		return nil, err
	}
	if err := writeIfMissing(RegistryPath(root), EmptyRegistryJSON()); err != nil {
		return nil, err
	}
	if err := writeIfMissing(ConfigPath(root),
		"# Workbench-local config (overrides global where supported).\n"); err != nil {
		return nil, err
	}
	if err := writeIfMissing(TargetsPath(root),
		"# Workbench-level deploy targets shared across projects.\ntargets: {}\n"); err != nil {
		return nil, err
	}
	return cfg, nil
}

func writeIfMissing(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func Save(root string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal workbench.yaml: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(root, DirName), 0o755); err != nil {
		return err
	}
	return os.WriteFile(YAMLPath(root), data, 0o644)
}

// FindRoot searches start and its ancestors for a Workbench root.
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

var ErrExists = errors.New("workbench already exists")
