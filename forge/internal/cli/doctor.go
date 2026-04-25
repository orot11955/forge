package cli

import (
	"os"
	"os/exec"

	"github.com/orot/forge/internal/project"
	"github.com/orot/forge/internal/workbench"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose Forge environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, t, err := loadCtxAndT()
			if err != nil {
				return err
			}

			cfgState := t.T("doctor.ok")
			if ctx.GlobalConfPath == "" {
				cfgState = t.T("doctor.missing")
			}

			workbenchState := t.T("doctor.notFound")
			if ctx.WorkbenchRoot != "" && workbench.IsRoot(ctx.WorkbenchRoot) {
				workbenchState = ctx.WorkbenchRoot
			}

			projectState := t.T("doctor.notFound")
			if ctx.ProjectRoot != "" {
				projectState = ctx.ProjectRoot
			}

			git := commandState(t, "git")
			docker := commandState(t, "docker")
			ssh := commandState(t, "ssh")
			workbenchParse, registryAccess := diagnoseWorkbench(ctx.WorkbenchRoot, t)
			projectParse := diagnoseProject(ctx.ProjectRoot, t)

			if flags.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"configPath":    ctx.GlobalConfPath,
					"configParse":   cfgState,
					"workbenchRoot": ctx.WorkbenchRoot,
					"workbenchYaml": workbenchParse,
					"registry":      registryAccess,
					"projectRoot":   ctx.ProjectRoot,
					"projectYaml":   projectParse,
					"git":           pathFor("git"),
					"docker":        pathFor("docker"),
					"ssh":           pathFor("ssh"),
				})
			}

			println(t.T("doctor.title"))
			println()
			printKV(t.T("doctor.configLabel"), ctx.GlobalConfPath+"  ("+cfgState+")")
			printKV(t.T("doctor.workbenchLabel"), workbenchState)
			printKV(t.T("doctor.workbenchYamlLabel"), workbenchParse)
			printKV(t.T("doctor.registryLabel"), registryAccess)
			printKV(t.T("doctor.projectLabel"), projectState)
			printKV(t.T("doctor.projectYamlLabel"), projectParse)
			printKV(t.T("doctor.gitLabel"), git)
			printKV(t.T("doctor.dockerLabel"), docker)
			printKV(t.T("doctor.sshLabel"), ssh)
			return nil
		},
	}
}

func commandState(t interface{ T(string, ...any) string }, name string) string {
	if p := pathFor(name); p != "" {
		return p
	}
	return t.T("doctor.notFound")
}

func pathFor(name string) string {
	p, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return p
}

func diagnoseWorkbench(root string, t interface{ T(string, ...any) string }) (string, string) {
	if root == "" {
		return t.T("doctor.notFound"), t.T("doctor.notFound")
	}
	if _, err := os.Stat(workbench.YAMLPath(root)); err != nil {
		return t.T("doctor.notFound"), t.T("doctor.notFound")
	}
	if _, err := workbench.Load(root); err != nil {
		return err.Error(), t.T("doctor.notFound")
	}
	if _, err := workbench.LoadRegistry(root); err != nil {
		return t.T("doctor.ok"), err.Error()
	}
	return t.T("doctor.ok"), t.T("doctor.ok")
}

func diagnoseProject(root string, t interface{ T(string, ...any) string }) string {
	if root == "" {
		return t.T("doctor.notFound")
	}
	if _, err := os.Stat(project.YAMLPath(root)); err != nil {
		return t.T("doctor.notFound")
	}
	if _, err := project.Load(root); err != nil {
		return err.Error()
	}
	return t.T("doctor.ok")
}
