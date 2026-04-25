package cli

import (
	"fmt"

	"github.com/orot/forge/internal/config"
	"github.com/orot/forge/internal/i18n"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage Forge global config",
	}
	cmd.AddCommand(newConfigPathCmd(), newConfigLanguageCmd())
	return cmd
}

func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Show global config path",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := config.Path()
			if err != nil {
				return err
			}
			if flags.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]string{"path": p})
			}
			_, t, err := loadCtxAndT()
			if err != nil {
				return err
			}
			println(t.T("config.pathLabel"))
			println(" ", p)
			return nil
		},
	}
}

func newConfigLanguageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "language <en|ko>",
		Short: "Set output language",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			lang := args[0]
			if !i18n.IsValid(lang) {
				return Coded(2, fmt.Errorf("invalid language: %s (supported: en, ko)", lang))
			}
			g, _, err := config.Load()
			if err != nil {
				return err
			}
			g.Language = lang
			if err := config.Save(g); err != nil {
				return err
			}
			if flags.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]string{"language": lang})
			}
			t, err := i18n.New(i18n.Lang(lang))
			if err != nil {
				return err
			}
			println(t.T("config.languageSet", lang))
			return nil
		},
	}
}
