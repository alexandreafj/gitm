package cli

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/fatih/color"
)

// columnGap separates two columns of a rendered table.
const columnGap = "  "

var (
	headerColor = color.New(color.Bold, color.Underline)
	aliasColor  = color.New(color.FgCyan)
	dimColor    = color.New(color.FgWhite)
	okColor     = color.New(color.FgGreen)
	warnColor   = color.New(color.FgYellow)
	errColor    = color.New(color.FgRed)
)

// table collects rows and renders them as aligned columns.
//
// Columns are sized by each cell's *visible* width. Padding a colored cell
// with %-20s or text/tabwriter instead counts its ANSI escape bytes as
// characters, which pads short cells too little and misaligns every column
// after the first.
type table struct {
	w     io.Writer
	cells [][]string
}

// newTable returns a table that renders to w when flushed.
func newTable(w io.Writer) *table {
	return &table{w: w}
}

// cell colors s for display in a table. A nil color, or disabled color
// output, returns s unchanged.
func cell(c *color.Color, s string) string {
	if c == nil {
		return s
	}
	return c.Sprint(s)
}

// headerRow adds the bold, underlined title row shared by every table.
func headerRow(t *table, titles ...string) {
	cells := make([]string, len(titles))
	for i, title := range titles {
		cells[i] = cell(headerColor, title)
	}
	row(t, cells...)
}

// row adds one row to the table.
func row(t *table, cells ...string) {
	t.cells = append(t.cells, cells)
}

// flushTable writes the aligned table and wraps any write failure.
func flushTable(t *table, name string) error {
	widths := make([]int, 0, 8)
	for _, cells := range t.cells {
		for i, c := range cells {
			for len(widths) <= i {
				widths = append(widths, 0)
			}
			if w := lipgloss.Width(c); w > widths[i] {
				widths[i] = w
			}
		}
	}

	var out strings.Builder
	for _, cells := range t.cells {
		var line strings.Builder
		for i, c := range cells {
			if i > 0 {
				line.WriteString(columnGap)
			}
			line.WriteString(c)
			// The last column is never padded, so rows do not end in spaces.
			if i < len(cells)-1 {
				line.WriteString(strings.Repeat(" ", widths[i]-lipgloss.Width(c)))
			}
		}
		out.WriteString(strings.TrimRight(line.String(), " "))
		out.WriteByte('\n')
	}

	if _, err := io.WriteString(t.w, out.String()); err != nil {
		return fmt.Errorf("write %s table: %w", name, err)
	}
	return nil
}

// truncate shortens s to at most max runes, adding an ellipsis when cut.
// Counting runes rather than bytes keeps multi-byte branch names intact.
func truncate(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	if max == 1 {
		return string(runes[:1])
	}
	return string(runes[:max-1]) + "…"
}
