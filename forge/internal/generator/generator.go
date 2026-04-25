package generator

import (
	"errors"
	"os"
	"path/filepath"
)

type FileSpec struct {
	Path    string // relative to project root
	Content string
}

type WriteResult struct {
	Path    string
	Written bool
	Skipped bool
	Reason  string
}

// WriteFiles writes specs into projectRoot. When force is false, existing
// files are left untouched. When dryRun is true, nothing is written.
func WriteFiles(projectRoot string, specs []FileSpec, force, dryRun bool) ([]WriteResult, error) {
	out := make([]WriteResult, 0, len(specs))
	for _, s := range specs {
		full := filepath.Join(projectRoot, s.Path)
		exists := pathExists(full)
		if exists && !force {
			out = append(out, WriteResult{Path: s.Path, Skipped: true, Reason: "exists"})
			continue
		}
		if dryRun {
			out = append(out, WriteResult{Path: s.Path, Written: true, Reason: "dry-run"})
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return out, err
		}
		if err := os.WriteFile(full, []byte(s.Content), 0o644); err != nil {
			return out, err
		}
		out = append(out, WriteResult{Path: s.Path, Written: true})
	}
	return out, nil
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

var ErrUnsupported = errors.New("unsupported project type")
