package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// File status colour styles (porcelain prefix).
var (
	statusMStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true) // M  modified   → yellow
	statusAStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true) // A  added      → green
	statusDStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true) // D  deleted    → red
	statusUStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))            // ?? untracked  → dim
	statusRStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true) // R  renamed    → magenta
)

// fileLine holds a parsed porcelain entry.
type fileLine struct {
	raw    string // original porcelain line e.g. " M src/foo.php"
	status string // two-char XY status code
	path   string // file path portion
}

func parsePorcelainLine(raw string) fileLine {
	if len(raw) < 4 {
		return fileLine{raw: raw, status: "??", path: strings.TrimSpace(raw)}
	}
	status := raw[:2]
	path := strings.TrimSpace(raw[3:])
	return fileLine{raw: raw, status: status, path: path}
}

func renderStatus(status string) string {
	s := strings.TrimSpace(status)
	if s == "" {
		s = "?"
	}
	padded := fmt.Sprintf("%-2s", s)
	switch {
	case strings.Contains(s, "M"):
		return statusMStyle.Render(padded)
	case strings.Contains(s, "A"):
		return statusAStyle.Render(padded)
	case strings.Contains(s, "D"):
		return statusDStyle.Render(padded)
	case strings.Contains(s, "R"):
		return statusRStyle.Render(padded)
	case s == "??":
		return statusUStyle.Render(padded)
	default:
		return hintStyle.Render(padded)
	}
}

// fileSelectModel is the bubbletea model for file selection.
type fileSelectModel struct {
	title    string
	lines    []fileLine
	cursor   int
	selected map[int]bool
	done     bool
}

// FileSelect displays an interactive checkbox list of dirty files (porcelain format)
// and returns the raw porcelain lines for the user's selection.
// Nothing is pre-selected. Returns nil if the user cancels.
func FileSelect(porcelainLines []string, title string) ([]string, error) {
	if len(porcelainLines) == 0 {
		return nil, fmt.Errorf("no dirty files")
	}

	lines := make([]fileLine, len(porcelainLines))
	for i, l := range porcelainLines {
		lines[i] = parsePorcelainLine(l)
	}

	m := fileSelectModel{
		title:    title,
		lines:    lines,
		selected: make(map[int]bool),
	}

	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("tui error: %w", err)
	}

	result := final.(fileSelectModel)
	if !result.done {
		return nil, fmt.Errorf("cancelled")
	}

	var chosen []string
	for i, fl := range result.lines {
		if result.selected[i] {
			chosen = append(chosen, fl.raw)
		}
	}
	if len(chosen) == 0 {
		return nil, fmt.Errorf("no files selected")
	}
	return chosen, nil
}

// Init implements tea.Model.
func (m fileSelectModel) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m fileSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.done = false
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.lines)-1 {
				m.cursor++
			}

		case " ":
			m.selected[m.cursor] = !m.selected[m.cursor]

		case "a":
			if len(m.selected) == len(m.lines) {
				m.selected = make(map[int]bool)
			} else {
				for i := range m.lines {
					m.selected[i] = true
				}
			}

		case "enter":
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m fileSelectModel) View() string {
	s := titleStyle.Render(m.title) + "\n"
	s += hintStyle.Render("↑/↓ or j/k to move  •  space to toggle  •  a to select all  •  enter to confirm  •  q/esc to cancel") + "\n\n"

	for i, fl := range m.lines {
		cursor := "  "
		if m.cursor == i {
			cursor = cursorStyle.Render("▶ ")
		}

		checkbox := "[ ]"
		pathStr := normalStyle.Render(fl.path)
		if m.selected[i] {
			checkbox = selectedStyle.Render("[✓]")
			pathStr = selectedStyle.Render(fl.path)
		}

		s += fmt.Sprintf("%s%s %s%s\n",
			cursor,
			checkbox,
			renderStatus(fl.status),
			pathStr,
		)
	}

	s += "\n" + countStyle.Render(fmt.Sprintf("%d/%d selected", len(m.selected), len(m.lines))) + "\n"
	return s
}
