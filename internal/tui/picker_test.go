package tui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
)

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
