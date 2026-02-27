package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	inputTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	inputHintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
	errorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
)

// textInputModel is the bubbletea model for single-line text input.
type textInputModel struct {
	input  textinput.Model
	title  string
	errMsg string
	done   bool
}

// CommitMessageInput shows a single-line text prompt for a commit message.
// Returns the entered message, or an error if the user cancels.
func CommitMessageInput(repoAlias string) (string, error) {
	ti := textinput.New()
	ti.Placeholder = "e.g. fix: correct null pointer in login handler"
	ti.Focus()
	ti.CharLimit = 200
	ti.Width = 72

	m := textInputModel{
		input: ti,
		title: fmt.Sprintf("Commit message for %s", repoAlias),
	}

	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("tui error: %w", err)
	}

	result := final.(textInputModel)
	if !result.done {
		return "", fmt.Errorf("cancelled")
	}
	return strings.TrimSpace(result.input.Value()), nil
}

// Init implements tea.Model.
func (m textInputModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (m textInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.done = false
			return m, tea.Quit

		case "enter":
			val := strings.TrimSpace(m.input.Value())
			if val == "" {
				m.errMsg = "Commit message cannot be empty"
				return m, nil
			}
			m.done = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m textInputModel) View() string {
	s := inputTitleStyle.Render(m.title) + "\n"
	s += inputHintStyle.Render("Type your commit message  •  enter to confirm  •  esc to cancel") + "\n\n"
	s += m.input.View() + "\n"
	if m.errMsg != "" {
		s += "\n" + errorStyle.Render(m.errMsg) + "\n"
	}
	return s
}
