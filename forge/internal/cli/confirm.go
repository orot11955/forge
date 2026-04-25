package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// confirm prompts the user with `prompt` and returns true if they accept.
// Honors --yes (always true) and --quiet (always false; non-interactive must
// pass --yes explicitly to confirm).
func confirm(prompt string) bool {
	if flags.Yes {
		return true
	}
	if flags.Quiet {
		return false
	}
	fmt.Fprintf(os.Stderr, "%s [y/N]: ", prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}
