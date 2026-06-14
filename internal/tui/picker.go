package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var ErrPickerCancelled = errors.New("picker cancelled")

type PickerOption struct {
	Label  string
	Branch string
	Path   string
}

func RunPicker(ctx context.Context, options []PickerOption, input io.Reader, output io.Writer) (PickerOption, error) {
	if len(options) == 0 {
		return PickerOption{}, fmt.Errorf("no worktrees available to select")
	}
	if input == nil {
		input = strings.NewReader("")
	}
	if output == nil {
		output = io.Discard
	}

	program := tea.NewProgram(
		newPickerModel(options),
		tea.WithContext(ctx),
		tea.WithInput(input),
		tea.WithOutput(output),
	)
	finalModel, err := program.Run()
	if err != nil {
		return PickerOption{}, err
	}
	model, ok := finalModel.(pickerModel)
	if !ok {
		return PickerOption{}, fmt.Errorf("unexpected picker model %T", finalModel)
	}
	if model.err != nil {
		return PickerOption{}, model.err
	}
	return model.selected, nil
}

type pickerModel struct {
	options       []PickerOption
	selectedIndex int
	selected      PickerOption
	err           error
}

func newPickerModel(options []PickerOption) pickerModel {
	return pickerModel{options: options}
}

func (m pickerModel) Init() tea.Cmd {
	return nil
}

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "up", "k":
		m.moveUp()
	case "down", "j":
		m.moveDown()
	case "enter":
		if len(m.options) > 0 {
			m.selected = m.options[m.selectedIndex]
		}
		return m, tea.Quit
	case "q", "ctrl+c":
		m.err = ErrPickerCancelled
		return m, tea.Quit
	}
	return m, nil
}

func (m pickerModel) View() tea.View {
	var builder strings.Builder
	selectedStyle := lipgloss.NewStyle().Bold(true)
	mutedStyle := lipgloss.NewStyle().Faint(true)

	builder.WriteString("Select worktree:\n")
	for i, option := range m.options {
		cursor := "  "
		label := option.Label
		if option.Branch != "" && option.Branch != option.Label {
			label += " (" + option.Branch + ")"
		}
		line := fmt.Sprintf("%s%s %s", cursor, label, mutedStyle.Render(option.Path))
		if i == m.selectedIndex {
			line = selectedStyle.Render("> " + label + " " + mutedStyle.Render(option.Path))
		}
		builder.WriteString(line)
		builder.WriteByte('\n')
	}

	return tea.NewView(builder.String())
}

func (m *pickerModel) moveUp() {
	if len(m.options) == 0 {
		return
	}
	m.selectedIndex--
	if m.selectedIndex < 0 {
		m.selectedIndex = len(m.options) - 1
	}
}

func (m *pickerModel) moveDown() {
	if len(m.options) == 0 {
		return
	}
	m.selectedIndex++
	if m.selectedIndex >= len(m.options) {
		m.selectedIndex = 0
	}
}
