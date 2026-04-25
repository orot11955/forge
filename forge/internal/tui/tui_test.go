package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/orot/forge/internal/workbench"
)

func makeModel() Model {
	return New("/tmp/forge-smoke/Workbench-sql", "workbench-sql", []workbench.RegistryEntry{
		{ID: "alpha", Name: "alpha", Type: "node", Status: "checked", LocationType: "external", Path: "/tmp/forge-smoke/proj-sql"},
		{ID: "bravo", Name: "bravo", Type: "go", Status: "generated", LocationType: "managed", Path: "/nope/bravo"},
	})
}

func TestListView_RendersHeaderAndRows(t *testing.T) {
	m := makeModel()
	out := m.View()
	for _, want := range []string{"Forge — workbench-sql", "NAME", "alpha", "bravo", "navigate", "quit"} {
		if !strings.Contains(out, want) {
			t.Errorf("list view missing %q\n--- output ---\n%s", want, out)
		}
	}
}

func TestNavigation_DownAndEnter(t *testing.T) {
	m := makeModel()
	// Down
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	mv := updated.(Model)
	if mv.cursor != 1 {
		t.Fatalf("expected cursor=1 after Down, got %d", mv.cursor)
	}
	// Down again should clamp at 1 (only 2 entries)
	updated, _ = mv.Update(tea.KeyMsg{Type: tea.KeyDown})
	mv = updated.(Model)
	if mv.cursor != 1 {
		t.Fatalf("expected cursor clamped at 1, got %d", mv.cursor)
	}
	// Up returns to 0
	updated, _ = mv.Update(tea.KeyMsg{Type: tea.KeyUp})
	mv = updated.(Model)
	if mv.cursor != 0 {
		t.Fatalf("expected cursor=0 after Up, got %d", mv.cursor)
	}
}

func TestQuit(t *testing.T) {
	m := makeModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatalf("expected tea.Quit cmd from 'q', got nil")
	}
	// We don't compare the cmd against tea.Quit directly because they are
	// closures, but we can call it — it returns tea.QuitMsg.
	if msg := cmd(); msg == nil {
		t.Fatalf("expected non-nil quit msg")
	}
}

func TestStaleProject_RendersBadge(t *testing.T) {
	m := makeModel()
	out := m.View()
	if !strings.Contains(out, "(stale)") {
		t.Errorf("expected (stale) badge for missing path; got:\n%s", out)
	}
}

func TestHistoryScreen_FromCKey(t *testing.T) {
	m := makeModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	mv := updated.(Model)
	if mv.screen != historyScreen {
		t.Fatalf("expected historyScreen after 'c', got %v", mv.screen)
	}
	if mv.historyFilter != "forge check" {
		t.Fatalf("expected filter 'forge check', got %q", mv.historyFilter)
	}
	out := mv.View()
	if !strings.Contains(out, "filter: forge check") {
		t.Errorf("history view missing filter label:\n%s", out)
	}
}
