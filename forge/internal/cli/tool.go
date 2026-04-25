package cli

import (
	"errors"
	"sort"

	"github.com/orot/forge/internal/project"
	"github.com/orot/forge/internal/toolchain"
	"github.com/spf13/cobra"
)

func newToolCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tool",
		Short:   "Inspect and diagnose the project's toolchain",
		Aliases: []string{"toolchain"},
	}
	cmd.AddCommand(newToolShowCmd(), newToolDoctorCmd())
	return cmd
}

func sortedToolNames(m map[string]toolchain.Spec) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ---------- forge tool show ----------

func newToolShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show toolchain.yaml contents and resolved binary paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, t, err := loadCtxAndT()
			if err != nil {
				return err
			}
			if ctx.ProjectRoot == "" {
				return projectNotFound(t)
			}
			tc, err := toolchain.Load(project.ToolchainPath(ctx.ProjectRoot))
			if err != nil {
				return Coded(2, err)
			}

			type item struct {
				Name      string `json:"name"`
				Source    string `json:"source"`
				Version   string `json:"version"`
				Path      string `json:"path"`
				Available bool   `json:"available"`
			}
			payload := map[string]any{
				"policy": map[string]any{
					"mode":             tc.Policy.Mode,
					"fallbackToSystem": tc.Policy.FallbackToSystem,
				},
				"runtimes":        []item{},
				"packageManagers": []item{},
				"systemTools":     []item{},
			}
			collect := func(key string, src map[string]toolchain.Spec) {
				rows := make([]item, 0, len(src))
				for _, name := range sortedToolNames(src) {
					r := toolchain.Resolve(name, src[name])
					rows = append(rows, item{
						Name: r.Name, Source: r.Spec.Source,
						Version: r.Version, Path: r.Path, Available: r.Available,
					})
				}
				payload[key] = rows
			}
			collect("runtimes", tc.Runtimes)
			collect("packageManagers", tc.PackageManagers)
			collect("systemTools", tc.SystemTools)

			if flags.JSON {
				return writeJSON(cmd.OutOrStdout(), payload)
			}

			println(t.T("tool.showTitle"))
			println()
			printf("%s mode=%s, fallbackToSystem=%t\n", t.T("tool.policyLabel"),
				tc.Policy.Mode, tc.Policy.FallbackToSystem)
			println()
			renderToolCategory(t, "tool.runtimesLabel", tc.Runtimes)
			renderToolCategory(t, "tool.pmsLabel", tc.PackageManagers)
			renderToolCategory(t, "tool.systemLabel", tc.SystemTools)
			return nil
		},
	}
}

func renderToolCategory(t interface{ T(string, ...any) string }, key string, m map[string]toolchain.Spec) {
	println(t.T(key))
	if len(m) == 0 {
		printf("  %s\n\n", t.T("tool.none"))
		return
	}
	for _, name := range sortedToolNames(m) {
		r := toolchain.Resolve(name, m[name])
		state := t.T("tool.ok")
		if !r.Available {
			state = t.T("tool.missing")
		}
		ver := r.Version
		if ver == "" {
			ver = t.T("tool.versionUnknown")
		}
		printf("  %-12s %-7s %-30s %s\n", name, state, r.Path, ver)
	}
	println()
}

// ---------- forge tool doctor ----------

func newToolDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose declared toolchain entries against the host",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, t, err := loadCtxAndT()
			if err != nil {
				return err
			}
			if ctx.ProjectRoot == "" {
				return projectNotFound(t)
			}
			tc, err := toolchain.Load(project.ToolchainPath(ctx.ProjectRoot))
			if err != nil {
				return Coded(2, err)
			}

			type row struct {
				Name      string `json:"name"`
				Category  string `json:"category"`
				Available bool   `json:"available"`
				Path      string `json:"path"`
				Version   string `json:"version"`
			}
			missing := 0
			rows := []row{}
			collect := func(cat string, src map[string]toolchain.Spec) {
				for _, name := range sortedToolNames(src) {
					r := toolchain.Resolve(name, src[name])
					rows = append(rows, row{
						Name: name, Category: cat,
						Available: r.Available, Path: r.Path, Version: r.Version,
					})
					if !r.Available {
						missing++
					}
				}
			}
			collect("runtime", tc.Runtimes)
			collect("packageManager", tc.PackageManagers)
			collect("systemTool", tc.SystemTools)

			if flags.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"missingCount": missing,
					"results":      rows,
				})
			}

			println(t.T("tool.doctorTitle"))
			println()
			for _, r := range rows {
				state := t.T("tool.ok")
				if !r.Available {
					state = t.T("tool.missing")
				}
				printf("  %-15s %-12s %-8s %s\n", r.Category, r.Name, state, r.Path)
			}
			if missing > 0 {
				return Coded(6, errors.New("toolchain entries missing"))
			}
			return nil
		},
	}
}
