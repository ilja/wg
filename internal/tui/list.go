package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

type WorktreeListRow struct {
	Marker     string
	Name       string
	Path       string
	Integrated bool
}

func RenderWorktreeList(rows []WorktreeListRow) string {
	var builder strings.Builder
	mutedStyle := lipgloss.NewStyle().Faint(true)
	nameWidth := worktreeListNameWidth(rows)
	nameStyle := lipgloss.NewStyle().Width(nameWidth)

	for _, row := range rows {
		name := nameStyle.Render(row.Name)
		if row.Integrated {
			name = mutedStyle.Render(name)
		}
		_, _ = fmt.Fprintf(&builder, "%s %s %s\n", row.Marker, name, row.Path)
	}
	return builder.String()
}

func worktreeListNameWidth(rows []WorktreeListRow) int {
	width := 0
	for _, row := range rows {
		width = max(width, lipgloss.Width(row.Name))
	}
	return width
}
