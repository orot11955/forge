package cli

import (
	"fmt"

	"github.com/orot/forge/internal/generator"
	"github.com/orot/forge/internal/project"
	"github.com/spf13/cobra"
)

func newGenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gen",
		Short: "Generate support files for the current project",
	}
	cmd.AddCommand(newGenBasicCmd(), newGenDockerCmd(), newGenComposeCmd(), newGenCICmd())
	return cmd
}

type genFlags struct {
	dryRun bool
	force  bool
}

func addGenFlags(cmd *cobra.Command, gf *genFlags) {
	cmd.Flags().BoolVar(&gf.dryRun, "dry-run", false, "Print plan without writing")
	cmd.Flags().BoolVar(&gf.force, "force", false, "Overwrite existing files")
}

func newGenBasicCmd() *cobra.Command {
	gf := &genFlags{}
	cmd := &cobra.Command{
		Use:   "basic",
		Short: "Generate README.md, .gitignore, .env.example, TODO.md",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, t, err := loadCtxAndT()
			if err != nil {
				return err
			}
			if ctx.ProjectRoot == "" {
				return projectNotFound(t)
			}
			pcfg, err := project.Load(ctx.ProjectRoot)
			if err != nil {
				return err
			}
			specs := generator.BasicFiles(pcfg)
			return runGen(cmd, t, ctx.ProjectRoot, specs, gf)
		},
	}
	addGenFlags(cmd, gf)
	return cmd
}

func newGenDockerCmd() *cobra.Command {
	gf := &genFlags{}
	cmd := &cobra.Command{
		Use:   "docker",
		Short: "Generate Dockerfile and .dockerignore for the project",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, t, err := loadCtxAndT()
			if err != nil {
				return err
			}
			if ctx.ProjectRoot == "" {
				return projectNotFound(t)
			}
			pcfg, err := project.Load(ctx.ProjectRoot)
			if err != nil {
				return err
			}
			content, err := generator.DockerfileFor(pcfg)
			if err != nil {
				return Coded(2, fmt.Errorf(t.T("gen.unsupportedType"), pcfg.Type))
			}
			specs := []generator.FileSpec{
				{Path: "Dockerfile", Content: content},
				{Path: ".dockerignore", Content: generator.DockerignoreFor(pcfg)},
			}
			return runGen(cmd, t, ctx.ProjectRoot, specs, gf)
		},
	}
	addGenFlags(cmd, gf)
	return cmd
}

func newGenComposeCmd() *cobra.Command {
	gf := &genFlags{}
	cmd := &cobra.Command{
		Use:   "compose",
		Short: "Generate docker-compose.yml for the project",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, t, err := loadCtxAndT()
			if err != nil {
				return err
			}
			if ctx.ProjectRoot == "" {
				return projectNotFound(t)
			}
			pcfg, err := project.Load(ctx.ProjectRoot)
			if err != nil {
				return err
			}
			specs := []generator.FileSpec{
				{Path: "docker-compose.yml", Content: generator.ComposeFor(pcfg)},
			}
			return runGen(cmd, t, ctx.ProjectRoot, specs, gf)
		},
	}
	addGenFlags(cmd, gf)
	return cmd
}

func newGenCICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ci",
		Short: "Generate CI pipeline files",
	}
	cmd.AddCommand(newGenCIJenkinsCmd())
	return cmd
}

func newGenCIJenkinsCmd() *cobra.Command {
	gf := &genFlags{}
	cmd := &cobra.Command{
		Use:   "jenkins",
		Short: "Generate a Jenkinsfile",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, t, err := loadCtxAndT()
			if err != nil {
				return err
			}
			if ctx.ProjectRoot == "" {
				return projectNotFound(t)
			}
			pcfg, err := project.Load(ctx.ProjectRoot)
			if err != nil {
				return err
			}
			specs := []generator.FileSpec{
				{Path: "Jenkinsfile", Content: generator.JenkinsfileFor(pcfg)},
			}
			return runGen(cmd, t, ctx.ProjectRoot, specs, gf)
		},
	}
	addGenFlags(cmd, gf)
	return cmd
}

func runGen(cmd *cobra.Command, t interface{ T(string, ...any) string },
	projectRoot string, specs []generator.FileSpec, gf *genFlags) error {

	if gf.dryRun && !flags.JSON {
		println(t.T("gen.dryRunHeader"))
		println()
	}
	results, err := generator.WriteFiles(projectRoot, specs, gf.force, gf.dryRun)
	if err != nil {
		return err
	}
	if flags.JSON {
		out := make([]map[string]any, 0, len(results))
		for _, r := range results {
			out = append(out, map[string]any{
				"path":    r.Path,
				"written": r.Written,
				"skipped": r.Skipped,
				"reason":  r.Reason,
			})
		}
		return writeJSON(cmd.OutOrStdout(), map[string]any{
			"projectRoot": projectRoot,
			"dryRun":      gf.dryRun,
			"results":     out,
		})
	}
	for _, r := range results {
		switch {
		case r.Skipped:
			println(t.T("gen.exists", r.Path))
		case r.Written:
			println(t.T("gen.wrote", r.Path))
		}
	}
	println()
	println(t.T("gen.done"))
	return nil
}
