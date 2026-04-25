package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Event struct {
	ID         string `json:"id"`
	Type       string `json:"type"` // "command" | "lifecycle"
	Command    string `json:"command,omitempty"`
	Status     string `json:"status,omitempty"`
	ExitCode   int    `json:"exitCode,omitempty"`
	StartedAt  string `json:"startedAt,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`
	Cwd        string `json:"cwd,omitempty"`
	Summary    string `json:"summary,omitempty"`
	From       string `json:"from,omitempty"`
	To         string `json:"to,omitempty"`
	Reason     string `json:"reason,omitempty"`
	At         string `json:"at,omitempty"`
}

func NewID(t time.Time) string {
	return fmt.Sprintf("evt_%s_%06d", t.Format("20060102_150405"), t.Nanosecond()/1000)
}

func Append(path string, e Event) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer f.Close()
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write history: %w", err)
	}
	return nil
}
