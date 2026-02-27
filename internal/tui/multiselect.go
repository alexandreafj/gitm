// Package tui provides interactive terminal UI components built with bubbletea.
package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/alexandreferreira/gitm/internal/db"
)

// Styles
var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	normalStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	hintStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
	countStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
	disabledStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	protectStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

// multiSelectModel is the bubbletea model for multi-select.
type multiSelectModel struct {
	repos    []*db.Repository
	cursor   int
	selected map[int]bool
	disabled map[int]bool // indices of repos that cannot be selected
	title    string
	done     bool
}

// MultiSelect displays an interactive checkbox list and returns the selected repositories.
// If preSelectAll is true, all non-disabled repos are pre-selected.
// disabledIdxs lists the indices of repos that should be shown but not toggleable.
// Returns nil if the user cancels.
func MultiSelect(repos []*db.Repository, title string, preSelectAll bool, disabledIdxs []int) ([]*db.Repository, error) {
	if len(repos) == 0 {
		return nil, fmt.Errorf("no repositories registered — run `gitm repo add <path>` first")
	}

	disabledMap := make(map[int]bool, len(disabledIdxs))
	for _, i := range disabledIdxs {
		disabledMap[i] = true
	}

	selected := make(map[int]bool)
	if preSelectAll {
		for i := range repos {
			if !disabledMap[i] {
				selected[i] = true
			}
		}
	}

	m := multiSelectModel{
		repos:    repos,
		selected: selected,
		disabled: disabledMap,
		title:    title,
	}

	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("tui error: %w", err)
	}

	result := final.(multiSelectModel)
	if !result.done {
		// User quit without confirming.
		return nil, fmt.Errorf("cancelled")
	}

	var chosen []*db.Repository
	for i, repo := range result.repos {
		if result.selected[i] {
			chosen = append(chosen, repo)
		}
	}

	if len(chosen) == 0 {
		return nil, fmt.Errorf("no repositories selected")
	}
	return chosen, nil
}

// Init implements tea.Model.
func (m multiSelectModel) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m multiSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if m.cursor < len(m.repos)-1 {
				m.cursor++
			}

		case " ":
			// No-op for disabled items.
			if !m.disabled[m.cursor] {
				m.selected[m.cursor] = !m.selected[m.cursor]
			}

		case "a":
			// Count only selectable (non-disabled) repos.
			selectableCount := 0
			selectedSelectableCount := 0
			for i := range m.repos {
				if !m.disabled[i] {
					selectableCount++
					if m.selected[i] {
						selectedSelectableCount++
					}
				}
			}
			if selectedSelectableCount == selectableCount {
				// Deselect all selectable.
				for i := range m.repos {
					if !m.disabled[i] {
						delete(m.selected, i)
					}
				}
			} else {
				// Select all selectable.
				for i := range m.repos {
					if !m.disabled[i] {
						m.selected[i] = true
					}
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
func (m multiSelectModel) View() string {
	s := titleStyle.Render(m.title) + "\n"
	s += hintStyle.Render("↑/↓ or j/k to move  •  space to toggle  •  a to select all  •  enter to confirm  •  q/esc to cancel") + "\n\n"

	selectableCount := 0
	selectedCount := 0
	for i := range m.repos {
		if !m.disabled[i] {
			selectableCount++
			if m.selected[i] {
				selectedCount++
			}
		}
	}

	for i, repo := range m.repos {
		cursor := "  "
		if m.cursor == i {
			cursor = cursorStyle.Render("▶ ")
		}

		if m.disabled[i] {
			checkbox := disabledStyle.Render("[ ]")
			name := disabledStyle.Render(repo.Alias)
			label := protectStyle.Render("⛔ protected branch")
			s += fmt.Sprintf("%s%s %s  %s  %s\n",
				cursor,
				checkbox,
				name,
				hintStyle.Render(repo.Path),
				label,
			)
		} else {
			checkbox := "[ ]"
			name := normalStyle.Render(repo.Alias)
			if m.selected[i] {
				checkbox = selectedStyle.Render("[✓]")
				name = selectedStyle.Render(repo.Alias)
			}
			s += fmt.Sprintf("%s%s %s  %s\n",
				cursor,
				checkbox,
				name,
				hintStyle.Render(repo.Path),
			)
		}
	}

	s += "\n" + countStyle.Render(fmt.Sprintf("%d/%d selected", selectedCount, selectableCount)) + "\n"
	return s
}
