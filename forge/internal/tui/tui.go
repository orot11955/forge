// Package tui provides the `forge tui` interactive UI built with Bubble Tea.
//
// The model intentionally re-uses internal Forge packages (workbench, project)
// rather than reimplementing logic. It is read-only: navigation and inspection
// only — no mutation. Mutating actions stay on the CLI side.
package tui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/orot/forge/internal/project"
	"github.com/orot/forge/internal/workbench"
)

type screen int

const (
	listScreen screen = iota
	detailScreen
	historyScreen
)

type Model struct {
	workbenchRoot string
	workbenchID   string
	projects      []workbench.RegistryEntry
	cursor        int
	screen        screen

	// Per-detail state (loaded lazily when user enters detailScreen).
	detail        *project.Config
	historyLines  []string
	historyFilter string // empty = all events; otherwise prefix to match command field

	width, height int
}

func New(root, id string, projects []workbench.RegistryEntry) Model {
	return Model{
		workbenchRoot: root,
		workbenchID:   id,
		projects:      projects,
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global keys.
	switch key {
	case "ctrl+c":
		return m, tea.Quit
	}

	switch m.screen {
	case listScreen:
		return m.updateList(key)
	case detailScreen:
		return m.updateDetail(key)
	case historyScreen:
		return m.updateHistory(key)
	}
	return m, nil
}

func (m Model) updateList(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.projects)-1 {
			m.cursor++
		}
	case "enter", "right", "l":
		if len(m.projects) == 0 {
			return m, nil
		}
		m.screen = detailScreen
		m.loadDetail()
	case "h":
		if len(m.projects) == 0 {
			return m, nil
		}
		m.screen = historyScreen
		m.historyFilter = ""
		m.loadHistory()
	case "c":
		if len(m.projects) == 0 {
			return m, nil
		}
		m.screen = historyScreen
		m.historyFilter = "forge check"
		m.loadHistory()
	}
	return m, nil
}

func (m Model) updateDetail(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "left", "q", "h":
		m.screen = listScreen
	case "H":
		// from detail, view full history
		m.screen = historyScreen
		m.historyFilter = ""
		m.loadHistory()
	}
	return m, nil
}

func (m Model) updateHistory(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "left", "q":
		m.screen = listScreen
	}
	return m, nil
}

// ---------- View ----------

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Faint(true)

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)

	cursorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("213"))

	headerRowStyle = lipgloss.NewStyle().
			Bold(true).
			Underline(true)

	staleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("203"))

	keyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("114")).
			Bold(true)
)

func (m Model) View() string {
	switch m.screen {
	case detailScreen:
		return m.viewDetail()
	case historyScreen:
		return m.viewHistory()
	}
	return m.viewList()
}

func (m Model) viewList() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("Forge — %s", m.workbenchID)))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render(m.workbenchRoot))
	b.WriteString("\n\n")

	if len(m.projects) == 0 {
		b.WriteString("No projects registered.\n")
		b.WriteString(footer(
			"q", "quit",
		))
		return b.String()
	}

	b.WriteString(headerRowStyle.Render(fmt.Sprintf("  %-20s %-8s %-12s %-10s %s",
		"NAME", "TYPE", "STATUS", "LOCATION", "PATH")))
	b.WriteString("\n")
	for i, p := range m.projects {
		prefix := "  "
		row := fmt.Sprintf("%-20s %-8s %-12s %-10s %s", p.Name, p.Type, p.Status, p.LocationType, p.Path)
		if !pathExists(p.Path) {
			row = staleStyle.Render(row + "  (stale)")
		}
		if i == m.cursor {
			prefix = cursorStyle.Render("▶ ")
			row = cursorStyle.Render(row)
		}
		b.WriteString(prefix + row + "\n")
	}
	b.WriteString(footer(
		"↑/↓", "navigate",
		"enter", "detail",
		"c", "check history",
		"h", "history",
		"q", "quit",
	))
	return b.String()
}

func (m Model) viewDetail() string {
	var b strings.Builder
	if m.detail == nil {
		b.WriteString(titleStyle.Render("Project Detail"))
		b.WriteString("\n\n")
		b.WriteString("Could not load project.yaml — has it been initialized?\n")
		b.WriteString(footer("esc", "back", "q", "back"))
		return b.String()
	}

	d := m.detail
	b.WriteString(titleStyle.Render(fmt.Sprintf("Project — %s", d.Name)))
	b.WriteString("\n\n")
	b.WriteString(headerRowStyle.Render("Context"))
	b.WriteString("\n")
	row := func(k, v string) {
		b.WriteString(fmt.Sprintf("  %-12s %s\n", k, v))
	}
	row("ID:", d.ID)
	row("Type:", d.Type)
	row("Location:", d.LocationType)
	row("Workbench:", d.Workbench.ID)
	if d.Description != "" {
		row("Description:", d.Description)
	}
	tmpl := "null"
	if d.Template != nil {
		tmpl = *d.Template
	}
	row("Template:", tmpl)
	row("Created:", d.CreatedAt)
	row("Updated:", d.UpdatedAt)

	b.WriteString("\n")
	b.WriteString(headerRowStyle.Render("Lifecycle"))
	b.WriteString("\n")
	row("Current:", d.Lifecycle.Current)

	if d.Runtime.Primary != "" {
		b.WriteString("\n")
		b.WriteString(headerRowStyle.Render("Runtime"))
		b.WriteString("\n")
		row("Primary:", d.Runtime.Primary)
		if d.Runtime.PackageManager != "" {
			row("Package mgr:", d.Runtime.PackageManager)
		}
	}

	b.WriteString(footer(
		"H", "full history",
		"esc", "back",
		"q", "back",
	))
	return b.String()
}

func (m Model) viewHistory() string {
	var b strings.Builder
	cur := m.projects[m.cursor]
	title := fmt.Sprintf("History — %s", cur.Name)
	if m.historyFilter != "" {
		title += fmt.Sprintf(" (filter: %s)", m.historyFilter)
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")
	if len(m.historyLines) == 0 {
		b.WriteString("(no events)\n")
	} else {
		for _, line := range m.historyLines {
			b.WriteString(line + "\n")
		}
	}
	b.WriteString(footer("esc", "back", "q", "back"))
	return b.String()
}

func footer(pairs ...string) string {
	if len(pairs)%2 != 0 {
		pairs = append(pairs, "")
	}
	var sb strings.Builder
	for i := 0; i < len(pairs); i += 2 {
		if i > 0 {
			sb.WriteString("  ")
		}
		sb.WriteString(keyStyle.Render(pairs[i]))
		sb.WriteString(" ")
		sb.WriteString(pairs[i+1])
	}
	return footerStyle.Render(sb.String())
}

// ---------- Detail / history loading (sync; cheap files) ----------

func (m *Model) loadDetail() {
	if len(m.projects) == 0 {
		return
	}
	cur := m.projects[m.cursor]
	cfg, err := project.Load(cur.Path)
	if err != nil {
		m.detail = nil
		return
	}
	m.detail = cfg
}

const maxHistoryLines = 30

func (m *Model) loadHistory() {
	m.historyLines = nil
	if len(m.projects) == 0 {
		return
	}
	cur := m.projects[m.cursor]
	path := project.HistoryPath(cur.Path)
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	// Read up to last N lines that match the filter. For Stage 10 we keep this
	// simple — slurp all lines, filter, then take the tail.
	var all []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if m.historyFilter != "" && !strings.Contains(line, `"command":"`+m.historyFilter) {
			continue
		}
		all = append(all, line)
	}
	if len(all) > maxHistoryLines {
		all = all[len(all)-maxHistoryLines:]
	}
	m.historyLines = all
}

// pathExists is duplicated here intentionally to avoid importing the cli
// package (would create a cycle).
func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// Run launches the TUI program against stdout/stdin.
func Run(m Model) error {
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
