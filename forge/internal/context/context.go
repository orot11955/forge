package context

import (
	"os"

	"github.com/orot/forge/internal/config"
	"github.com/orot/forge/internal/i18n"
	"github.com/orot/forge/internal/project"
	"github.com/orot/forge/internal/workbench"
)

type Context struct {
	Cwd            string
	WorkbenchRoot  string
	WorkbenchID    string
	ProjectRoot    string
	ProjectID      string
	ProjectType    string
	LocationType   string
	Lifecycle      string
	Lang           i18n.Lang
	GlobalConfig   *config.Global
	GlobalConfPath string
}

// Resolve constructs the execution context.
//   - flagWorkbench: value of --workbench (empty if not set)
//   - flagLang: value of --lang (empty if not set)
func Resolve(flagWorkbench, flagLang string) (*Context, error) {
	ctx := &Context{}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	ctx.Cwd = cwd

	g, gpath, err := config.Load()
	if err != nil {
		return nil, err
	}
	ctx.GlobalConfig = g
	ctx.GlobalConfPath = gpath
	ctx.Lang = i18n.ResolveLang(flagLang, os.Getenv("FORGE_LANG"), g.Language)

	// Project Root: walk up from cwd
	if pRoot, ok := project.FindRoot(cwd); ok {
		ctx.ProjectRoot = pRoot
		if cfg, err := project.Load(pRoot); err == nil {
			ctx.ProjectID = cfg.ID
			ctx.ProjectType = cfg.Type
			ctx.LocationType = cfg.LocationType
			ctx.Lifecycle = cfg.Lifecycle.Current
		}
	}

	// Workbench Root resolution order:
	// 1. --workbench
	// 2. FORGE_WORKBENCH
	// 3. Walk up from project root (if exists)
	// 4. Walk up from cwd
	// 5. global config defaultWorkbench
	if flagWorkbench != "" {
		w := config.ExpandPath(flagWorkbench)
		if workbench.IsRoot(w) {
			ctx.WorkbenchRoot = w
		} else if root, ok := workbench.FindRoot(w); ok {
			ctx.WorkbenchRoot = root
		} else {
			ctx.WorkbenchRoot = w
		}
	}
	if ctx.WorkbenchRoot == "" {
		if env := os.Getenv("FORGE_WORKBENCH"); env != "" {
			w := config.ExpandPath(env)
			if root, ok := workbench.FindRoot(w); ok {
				ctx.WorkbenchRoot = root
			} else {
				ctx.WorkbenchRoot = w
			}
		}
	}
	if ctx.WorkbenchRoot == "" && ctx.ProjectRoot != "" {
		if root, ok := workbench.FindRoot(ctx.ProjectRoot); ok {
			ctx.WorkbenchRoot = root
		}
	}
	if ctx.WorkbenchRoot == "" {
		if root, ok := workbench.FindRoot(cwd); ok {
			ctx.WorkbenchRoot = root
		}
	}
	if ctx.WorkbenchRoot == "" && g.DefaultWorkbench != "" {
		w := config.ExpandPath(g.DefaultWorkbench)
		if workbench.IsRoot(w) {
			ctx.WorkbenchRoot = w
		}
	}

	if ctx.WorkbenchRoot != "" {
		if wcfg, err := workbench.Load(ctx.WorkbenchRoot); err == nil {
			ctx.WorkbenchID = wcfg.ID
		}
	}

	return ctx, nil
}
