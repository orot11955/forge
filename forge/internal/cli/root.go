package cli

import (
	"os"

	"github.com/orot/forge/internal/config"
	"github.com/orot/forge/internal/i18n"
	"github.com/spf13/cobra"
)

type globalFlags struct {
	JSON      bool
	Lang      string
	Workbench string
	Verbose   bool
	Quiet     bool
	Yes       bool
	DryRun    bool
}

var flags = &globalFlags{}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "forge",
		Short:         "Forge — Workbench-based project lifecycle CLI",
		Long:          "Forge manages projects, toolchains, and lifecycle status from a single CLI.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.PersistentFlags().BoolVar(&flags.JSON, "json", false, "Output as JSON")
	cmd.PersistentFlags().StringVar(&flags.Lang, "lang", "", "Output language (en|ko)")
	cmd.PersistentFlags().StringVar(&flags.Workbench, "workbench", "", "Override Workbench Root")
	cmd.PersistentFlags().BoolVar(&flags.Verbose, "verbose", false, "Verbose output")
	cmd.PersistentFlags().BoolVar(&flags.Quiet, "quiet", false, "Suppress non-essential output")
	cmd.PersistentFlags().BoolVarP(&flags.Yes, "yes", "y", false, "Assume yes for confirmation prompts")

	cmd.AddCommand(
		newVersionCmd(),
		newConfigCmd(),
		newWorkCmd(),
		newProjectCmd(),
		newInitAliasCmd(),
		newStatusAliasCmd(),
		newCheckCmd(),
		newListAliasCmd(),
		newDoctorCmd(),
		newCreateAliasCmd(),
		newGenCmd(),
		newRunCmd(),
		newGitCmd(),
		newDockerCmd(),
		newComposeCmd(),
		newDeployCmd(),
		newToolCmd(),
		newLogsCmd(),
		newTUICmd(),
	)
	return cmd
}

func Execute() error {
	args := normalizeHelpArgs(os.Args[1:])
	cmd := newRootCmd()
	cmd.SetArgs(args)
	localizeHelp(cmd, resolveHelpLang(args))
	return cmd.Execute()
}

func normalizeHelpArgs(args []string) []string {
	hasHelp := false
	for _, a := range args {
		if a == "--help" || a == "-h" {
			hasHelp = true
			break
		}
	}
	if !hasHelp {
		return args
	}

	langArgs := []string{}
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--lang" && i+1 < len(args):
			langArgs = append(langArgs, a, args[i+1])
			i++
		case matchPrefix(a, "--lang="):
			langArgs = append(langArgs, a)
		default:
			rest = append(rest, a)
		}
	}
	if len(langArgs) == 0 {
		return args
	}
	return append(langArgs, rest...)
}

func matchPrefix(s, prefix string) bool {
	return len(s) > len(prefix) && s[:len(prefix)] == prefix
}

func resolveHelpLang(args []string) i18n.Lang {
	flagLang := ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--lang" && i+1 < len(args):
			flagLang = args[i+1]
			i++
		case matchPrefix(args[i], "--lang="):
			flagLang = args[i][len("--lang="):]
		}
	}
	g, _, _ := config.Load()
	cfgLang := ""
	if g != nil {
		cfgLang = g.Language
	}
	return i18n.ResolveLang(flagLang, os.Getenv("FORGE_LANG"), cfgLang)
}

func localizeHelp(cmd *cobra.Command, lang i18n.Lang) {
	if lang != i18n.LangKo {
		return
	}
	cmd.Short = "Forge — Workbench 기반 프로젝트 생명주기 CLI"
	cmd.Long = "Forge는 하나의 CLI에서 프로젝트, toolchain, lifecycle 상태를 관리합니다."

	if f := cmd.PersistentFlags().Lookup("json"); f != nil {
		f.Usage = "JSON으로 출력"
	}
	if f := cmd.PersistentFlags().Lookup("lang"); f != nil {
		f.Usage = "출력 언어 (en|ko)"
	}
	if f := cmd.PersistentFlags().Lookup("workbench"); f != nil {
		f.Usage = "Workbench Root 직접 지정"
	}
	if f := cmd.PersistentFlags().Lookup("verbose"); f != nil {
		f.Usage = "자세히 출력"
	}
	if f := cmd.PersistentFlags().Lookup("quiet"); f != nil {
		f.Usage = "필수 출력만 표시"
	}
	if f := cmd.PersistentFlags().Lookup("yes"); f != nil {
		f.Usage = "확인 질문에 yes로 응답"
	}

	shorts := map[string]string{
		"check":   "checks.yaml에 정의된 프로젝트 체크 실행",
		"compose": "Docker Compose 명령 래핑",
		"config":  "Forge 전역 설정 관리",
		"create":  "새 프로젝트 생성",
		"deploy":  ".forge/targets.yaml에 정의된 대상에 배포",
		"docker":  "Docker 명령 래핑",
		"doctor":  "Forge 실행 환경 진단",
		"gen":     "현재 프로젝트 보조 파일 생성",
		"git":     "Git 명령 래핑",
		"init":    "현재 디렉터리를 Forge 프로젝트로 등록",
		"list":    "등록된 프로젝트 목록 조회",
		"logs":    "현재 프로젝트 실행 로그 조회",
		"project": "프로젝트 관리",
		"run":     ".forge/scripts.yaml에 정의된 script 실행",
		"status":  "프로젝트 상태 조회",
		"tool":    "프로젝트 toolchain 조회 및 진단",
		"tui":     "프로젝트 탐색용 터미널 UI",
		"version": "Forge 버전 출력",
		"work":    "Workbench 관리",
	}
	for _, child := range cmd.Commands() {
		if s, ok := shorts[child.Name()]; ok {
			child.Short = s
		}
	}
}
