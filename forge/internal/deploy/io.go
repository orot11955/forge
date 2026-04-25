package deploy

import (
	"io"
	"os"
)

// Indirection so tests/CLI can swap streams; defaults to os.Stdout/os.Stderr.
var (
	stdoutWriter = func() io.Writer { return os.Stdout }
	stderrWriter = func() io.Writer { return os.Stderr }
)
