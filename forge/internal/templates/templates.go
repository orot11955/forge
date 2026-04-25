package templates

import (
	"embed"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed embedded/*.yaml
var embeddedFS embed.FS

type Localized struct {
	En string `yaml:"en"`
	Ko string `yaml:"ko"`
}

type Command struct {
	Executable string   `yaml:"executable"`
	Args       []string `yaml:"args"`
}

type Spec struct {
	ID                    string    `yaml:"id"`
	Label                 Localized `yaml:"label"`
	Description           Localized `yaml:"description"`
	Type                  string    `yaml:"type"`
	DefaultPackageManager string    `yaml:"defaultPackageManager,omitempty"`
	Command               *Command  `yaml:"command,omitempty"`
	// runIn: "parent" runs the command in the parent of target dir (e.g. create-next-app);
	// "project" runs inside the target dir; default is "parent" when a command is set.
	RunIn string `yaml:"runIn,omitempty"`
}

// All returns embedded templates keyed by id.
func All() (map[string]*Spec, error) {
	out := map[string]*Spec{}
	entries, err := embeddedFS.ReadDir("embedded")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		data, err := embeddedFS.ReadFile("embedded/" + e.Name())
		if err != nil {
			return nil, err
		}
		s := &Spec{}
		if err := yaml.Unmarshal(data, s); err != nil {
			return nil, fmt.Errorf("parse template %s: %w", e.Name(), err)
		}
		out[s.ID] = s
	}
	return out, nil
}

// Find returns the embedded template by id.
func Find(id string) (*Spec, error) {
	all, err := All()
	if err != nil {
		return nil, err
	}
	s, ok := all[id]
	if !ok {
		return nil, fmt.Errorf("unknown template: %s", id)
	}
	return s, nil
}

// Render substitutes {{ var }} placeholders in args using vars.
func (c *Command) Render(vars map[string]string) []string {
	out := make([]string, len(c.Args))
	for i, a := range c.Args {
		out[i] = renderString(a, vars)
	}
	return out
}

func renderString(s string, vars map[string]string) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, "{{ "+k+" }}", v)
		s = strings.ReplaceAll(s, "{{"+k+"}}", v)
	}
	return s
}
