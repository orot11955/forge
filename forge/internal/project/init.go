package project

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/orot/forge/internal/checks"
	"github.com/orot/forge/internal/detector"
	"github.com/orot/forge/internal/history"
	"github.com/orot/forge/internal/scripts"
	"github.com/orot/forge/internal/toolchain"
	"github.com/orot/forge/internal/util"
	"github.com/orot/forge/internal/workbench"
)

// MaterializeOptions configures how to write .forge metadata into a project.
type MaterializeOptions struct {
	WorkbenchRoot string // optional, used for managed/external classification
	WorkbenchID   string
	InitialState  string // defaults to LifecycleInitialized
	HistoryNote   string // initial history summary, e.g. "project initialized" or "project created"
	Command       string // command label for history, e.g. "forge init"
}

// Materialize writes project.yaml/toolchain.yaml/checks.yaml/scripts.yaml/history.jsonl
// into projectRoot based on detector results, and registers the project in
// the workbench registry when WorkbenchRoot is set.
//
// Returns the persisted *Config.
func Materialize(projectRoot string, opts MaterializeOptions) (*Config, error) {
	det := detector.Detect(projectRoot)
	id := util.Slugify(filepath.Base(projectRoot))
	location := LocationExternal
	if opts.WorkbenchRoot != "" {
		rel, err := filepath.Rel(filepath.Join(opts.WorkbenchRoot, workbench.ProjectsDir), projectRoot)
		if err == nil && !strings.HasPrefix(rel, "..") {
			location = LocationManaged
		}
	}
	now := util.NowISO()
	state := opts.InitialState
	if state == "" {
		state = LifecycleInitialized
	}
	pcfg := &Config{
		Version:      1,
		ID:           id,
		Name:         filepath.Base(projectRoot),
		Description:  "",
		Type:         det.Type,
		Template:     nil,
		LocationType: location,
		CreatedAt:    now,
		UpdatedAt:    now,
		Lifecycle:    Lifecycle{Current: state},
		Paths:        map[string]string{"docs": "docs"},
	}
	switch det.Type {
	case "node", "next", "nest":
		pcfg.Runtime = Runtime{Primary: "node", PackageManager: det.PackageManager}
	case "go":
		pcfg.Runtime = Runtime{Primary: "go"}
	case "java":
		pcfg.Runtime = Runtime{Primary: "java"}
	}
	if opts.WorkbenchID != "" {
		pcfg.Workbench = WorkbenchRef{ID: opts.WorkbenchID}
	}

	if err := Save(projectRoot, pcfg); err != nil {
		return nil, err
	}
	if err := toolchain.Save(ToolchainPath(projectRoot),
		toolchain.DefaultsForType(det.Type, det.PackageManager)); err != nil {
		return nil, err
	}
	if err := checks.Save(ChecksPath(projectRoot),
		checks.DefaultsForType(det.Type)); err != nil {
		return nil, err
	}
	scriptsFile := scripts.Empty()
	if len(det.NodeScripts) > 0 {
		scriptsFile = scripts.FromNodeScripts(det.PackageManager, det.NodeScripts)
	}
	if err := scripts.Save(ScriptsPath(projectRoot), scriptsFile); err != nil {
		return nil, err
	}

	cmdLabel := opts.Command
	if cmdLabel == "" {
		cmdLabel = "forge init"
	}
	note := opts.HistoryNote
	if note == "" {
		note = "project initialized"
	}
	_ = history.Append(HistoryPath(projectRoot), history.Event{
		ID:         history.NewID(time.Now()),
		Type:       "command",
		Command:    cmdLabel,
		Status:     "success",
		ExitCode:   0,
		StartedAt:  now,
		FinishedAt: util.NowISO(),
		Cwd:        projectRoot,
		Summary:    note,
	})

	if opts.WorkbenchRoot != "" {
		reg, err := workbench.LoadRegistry(opts.WorkbenchRoot)
		if err == nil {
			reg.Upsert(workbench.RegistryEntry{
				ID:           pcfg.ID,
				Name:         pcfg.Name,
				Path:         projectRoot,
				Type:         pcfg.Type,
				Status:       pcfg.Lifecycle.Current,
				LocationType: pcfg.LocationType,
				CreatedAt:    pcfg.CreatedAt,
				UpdatedAt:    pcfg.UpdatedAt,
			})
			_ = workbench.SaveRegistry(opts.WorkbenchRoot, reg)
		}
	}

	return pcfg, nil
}

// DetectorWarnings exposes detector warnings for callers that want to surface
// them to the user (e.g. multi-framework conflicts).
func DetectorWarnings(projectRoot string) []string {
	return detector.Detect(projectRoot).Warnings
}
