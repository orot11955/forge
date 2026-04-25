package cli

// extractGlobalFlags scans args for Forge's well-known global flags and applies
// them to the global `flags` struct, returning the remaining args (with the
// global flags removed) for delegation to system tools (git, docker, compose).
//
// Wrappers use cobra's DisableFlagParsing to forward unknown flags as-is to
// the underlying tool, but that also bypasses Forge's own --json/--yes/etc.
// This helper bridges the gap.
func extractGlobalFlags(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--json":
			flags.JSON = true
		case "--yes", "-y":
			flags.Yes = true
		case "--quiet":
			flags.Quiet = true
		case "--verbose":
			flags.Verbose = true
		case "--lang":
			if i+1 < len(args) {
				flags.Lang = args[i+1]
				i++
			}
		case "--workbench":
			if i+1 < len(args) {
				flags.Workbench = args[i+1]
				i++
			}
		default:
			// support --lang=ko / --workbench=path forms
			if v, ok := matchEqual(a, "--lang="); ok {
				flags.Lang = v
				continue
			}
			if v, ok := matchEqual(a, "--workbench="); ok {
				flags.Workbench = v
				continue
			}
			out = append(out, a)
		}
	}
	return out
}

func matchEqual(arg, prefix string) (string, bool) {
	if len(arg) <= len(prefix) {
		return "", false
	}
	if arg[:len(prefix)] != prefix {
		return "", false
	}
	return arg[len(prefix):], true
}
