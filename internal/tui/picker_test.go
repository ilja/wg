package tui

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestPickerNavigationWrapsThroughOptions(t *testing.T) {
	model := newPickerModel(testPickerOptions())

	model = updatePicker(t, model, key(tea.KeyDown, ""))
	if model.selectedIndex != 1 {
		t.Fatalf("expected down to select index 1, got %d", model.selectedIndex)
	}

	model = updatePicker(t, model, key(tea.KeyDown, ""))
	if model.selectedIndex != 2 {
		t.Fatalf("expected second down to select index 2, got %d", model.selectedIndex)
	}

	model = updatePicker(t, model, key(tea.KeyDown, ""))
	if model.selectedIndex != 0 {
		t.Fatalf("expected down from end to wrap to index 0, got %d", model.selectedIndex)
	}

	model = updatePicker(t, model, key(tea.KeyUp, ""))
	if model.selectedIndex != 2 {
		t.Fatalf("expected up from start to wrap to index 2, got %d", model.selectedIndex)
	}

	model = updatePicker(t, model, key(0, "k"))
	if model.selectedIndex != 1 {
		t.Fatalf("expected k to select index 1, got %d", model.selectedIndex)
	}

	model = updatePicker(t, model, key(0, "j"))
	if model.selectedIndex != 2 {
		t.Fatalf("expected j to select index 2, got %d", model.selectedIndex)
	}
}

func TestPickerEnterRecordsSelectedPathAndExitsSuccessfully(t *testing.T) {
	model := newPickerModel(testPickerOptions())
	model = updatePicker(t, model, key(tea.KeyDown, ""))

	updated, cmd := model.Update(key(tea.KeyEnter, ""))
	model, ok := updated.(pickerModel)
	if !ok {
		t.Fatalf("expected pickerModel, got %T", updated)
	}
	if cmd == nil {
		t.Fatalf("expected enter to return a quit command")
	}
	if model.err != nil {
		t.Fatalf("expected no error, got %v", model.err)
	}
	if model.selected.Path != "/repo/demo.feature-alpha" {
		t.Fatalf("expected selected path, got %q", model.selected.Path)
	}
}

func TestPickerViewAlignsPathsInASecondColumn(t *testing.T) {
	model := newPickerModel(testPickerOptions())
	view := ansiEscapePattern.ReplaceAllString(model.View().Content, "")
	lines := strings.Split(view, "\n")

	mainPathColumn := strings.Index(lines[1], "/repo/demo")
	alphaPathColumn := strings.Index(lines[2], "/repo/demo.feature-alpha")
	betaPathColumn := strings.Index(lines[3], "/repo/demo.feature-beta")
	if mainPathColumn < 0 || alphaPathColumn < 0 || betaPathColumn < 0 {
		t.Fatalf("expected all paths in picker view, got:\n%s", view)
	}
	if mainPathColumn != alphaPathColumn || mainPathColumn != betaPathColumn {
		t.Fatalf("expected paths to align in one column, got main=%d alpha=%d beta=%d view:\n%s", mainPathColumn, alphaPathColumn, betaPathColumn, view)
	}
}

func TestPickerViewMutesIntegratedLabels(t *testing.T) {
	options := testPickerOptions()
	options[1].Integrated = true
	view := newPickerModel(options).View().Content

	if !strings.Contains(view, "\x1b[2mfeature-alpha") {
		t.Fatalf("expected integrated label to be faint, got:\n%s", view)
	}
}

func TestPickerCancellationReturnsErrorWithNoSelectedPath(t *testing.T) {
	for _, keyMsg := range []tea.KeyPressMsg{key(0, "q"), keyCtrlC()} {
		t.Run(keyMsg.String(), func(t *testing.T) {
			model := newPickerModel(testPickerOptions())
			updated, cmd := model.Update(keyMsg)
			model, ok := updated.(pickerModel)
			if !ok {
				t.Fatalf("expected pickerModel, got %T", updated)
			}
			if cmd == nil {
				t.Fatalf("expected cancellation to return a quit command")
			}
			if !errors.Is(model.err, ErrPickerCancelled) {
				t.Fatalf("expected cancellation error, got %v", model.err)
			}
			if model.selected.Path != "" {
				t.Fatalf("expected no selected path, got %q", model.selected.Path)
			}
		})
	}
}

func testPickerOptions() []PickerOption {
	return []PickerOption{
		{Label: "main", Branch: "main", Path: "/repo/demo"},
		{Label: "feature-alpha", Branch: "feature-alpha", Path: "/repo/demo.feature-alpha"},
		{Label: "feature-beta", Branch: "feature-beta", Path: "/repo/demo.feature-beta"},
	}
}

func updatePicker(t *testing.T, model pickerModel, msg tea.Msg) pickerModel {
	t.Helper()
	updated, _ := model.Update(msg)
	picker, ok := updated.(pickerModel)
	if !ok {
		t.Fatalf("expected pickerModel, got %T", updated)
	}
	return picker
}

func key(code rune, text string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code, Text: text})
}

func keyCtrlC() tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl})
}
