package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/orot/forge/internal/context"
	"github.com/orot/forge/internal/i18n"
)

func loadCtxAndT() (*context.Context, *i18n.Translator, error) {
	ctx, err := context.Resolve(flags.Workbench, flags.Lang)
	if err != nil {
		return nil, nil, err
	}
	t, err := i18n.New(ctx.Lang)
	if err != nil {
		return nil, nil, err
	}
	return ctx, t, nil
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printlnIfNotQuiet(args ...any) {
	if flags.Quiet {
		return
	}
	fmt.Fprintln(os.Stdout, args...)
}

func printf(format string, args ...any) {
	fmt.Fprintf(os.Stdout, format, args...)
}

func println(args ...any) {
	fmt.Fprintln(os.Stdout, args...)
}
