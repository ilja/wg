package cli_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"wg/internal/cli"
	"wg/internal/tui"
)

func TestSwitchPathOutputResolvesReferencesWithDataOnlyStdout(t *testing.T) {
	t.Run("exact reference", func(t *testing.T) {
		stdout, stderr, code := runSwitch(t, []string{"switch", "--path-output", "feature-alpha"}, cli.Options{})
		if code != 0 {
			t.Fatalf("expected success, code=%d stderr=%q", code, stderr)
		}
		if stdout != "/repo/demo.feature-alpha\n" {
			t.Fatalf("unexpected stdout %q", stdout)
		}
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}
	})

	t.Run("unique prefix", func(t *testing.T) {
		stdout, stderr, code := runSwitch(t, []string{"switch", "--path-output", "feature-b"}, cli.Options{})
		if code != 0 {
			t.Fatalf("expected success, code=%d stderr=%q", code, stderr)
		}
		if stdout != "/repo/demo.feature-beta\n" {
			t.Fatalf("unexpected stdout %q", stdout)
		}
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}
	})
}

func TestSwitchPathOutputFailuresKeepStdoutEmptyAndDiagnoseOnStderr(t *testing.T) {
	t.Run("missing reference", func(t *testing.T) {
		stdout, stderr, code := runSwitch(t, []string{"switch", "--path-output", "missing"}, cli.Options{})
		if code == 0 {
			t.Fatalf("expected missing reference to fail")
		}
		if stdout != "" {
			t.Fatalf("expected empty stdout, got %q", stdout)
		}
		if !strings.Contains(stderr, "missing") || !strings.Contains(stderr, "feature-alpha") || !strings.Contains(stderr, "feature-beta") {
			t.Fatalf("expected readable missing diagnostic with candidates, got %q", stderr)
		}
	})

	t.Run("ambiguous reference", func(t *testing.T) {
		stdout, stderr, code := runSwitch(t, []string{"switch", "--path-output", "feature"}, cli.Options{})
		if code == 0 {
			t.Fatalf("expected ambiguous reference to fail")
		}
		if stdout != "" {
			t.Fatalf("expected empty stdout, got %q", stdout)
		}
		if !strings.Contains(stderr, "ambiguous") || !strings.Contains(stderr, "feature-alpha") || !strings.Contains(stderr, "feature-beta") {
			t.Fatalf("expected ambiguity diagnostic with candidates, got %q", stderr)
		}
	})
}

func TestSwitchPathOutputWithoutReferenceUsesInjectedPicker(t *testing.T) {
	var pickerOptions []tui.PickerOption
	picker := func(ctx context.Context, options []tui.PickerOption, input io.Reader, output io.Writer) (tui.PickerOption, error) {
		pickerOptions = append([]tui.PickerOption(nil), options...)
		return tui.PickerOption{Label: "feature-beta", Branch: "feature-beta", Path: "/repo/demo.feature-beta"}, nil
	}

	var stdout, stderr bytes.Buffer
	fg := newFakeGit()
	code := cli.Run(context.Background(), []string{"switch", "--path-output"}, cli.Options{
		Cwd:       "/repo/demo.feature-alpha",
		Stdout:    &stdout,
		Stderr:    &stderr,
		GitRunner: fg,
		Picker:    cli.PickerFunc(picker),
	})
	if code != 0 {
		t.Fatalf("expected success, code=%d stderr=%q", code, stderr.String())
	}
	if stdout.String() != "/repo/demo.feature-beta\n" {
		t.Fatalf("unexpected stdout %q", stdout.String())
	}
	if stderr.String() != "" {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
	if len(pickerOptions) != 3 {
		t.Fatalf("expected picker to receive three options, got %#v", pickerOptions)
	}
	if pickerOptions[1].Label != "feature-alpha" || pickerOptions[1].Path != "/repo/demo.feature-alpha" {
		t.Fatalf("unexpected picker options: %#v", pickerOptions)
	}
}

func TestSwitchPickerMarksMergedBranchesAsIntegrated(t *testing.T) {
	var pickerOptions []tui.PickerOption
	picker := func(ctx context.Context, options []tui.PickerOption, input io.Reader, output io.Writer) (tui.PickerOption, error) {
		pickerOptions = append([]tui.PickerOption(nil), options...)
		return tui.PickerOption{Label: "feature-beta", Branch: "feature-beta", Path: "/repo/demo.feature-beta"}, nil
	}

	fg := newFakeGit()
	fg.mergedBranches = "main\nfeature-alpha\n"
	var stdout, stderr bytes.Buffer
	code := cli.Run(context.Background(), []string{"switch", "--path-output"}, cli.Options{
		Cwd:       "/repo/demo.feature-alpha",
		Stdout:    &stdout,
		Stderr:    &stderr,
		GitRunner: fg,
		Picker:    cli.PickerFunc(picker),
	})
	if code != 0 {
		t.Fatalf("expected success, code=%d stderr=%q", code, stderr.String())
	}
	if pickerOptions[0].Integrated {
		t.Fatalf("expected primary/default branch not to be marked integrated: %#v", pickerOptions)
	}
	if !pickerOptions[1].Integrated {
		t.Fatalf("expected merged feature branch to be marked integrated: %#v", pickerOptions)
	}
	if pickerOptions[2].Integrated {
		t.Fatalf("expected unmerged feature branch not to be marked integrated: %#v", pickerOptions)
	}
	for _, call := range fg.calls {
		if strings.Join(call.Args, " ") == "ls-remote --symref origin HEAD" {
			t.Fatalf("expected switch merged-status lookup to avoid network-capable ls-remote call; calls: %#v", fg.calls)
		}
	}
}

func TestSwitchPathOutputPickerCancellationDiagnosesOnStderr(t *testing.T) {
	picker := func(ctx context.Context, options []tui.PickerOption, input io.Reader, output io.Writer) (tui.PickerOption, error) {
		return tui.PickerOption{}, tui.ErrPickerCancelled
	}

	var stdout, stderr bytes.Buffer
	code := cli.Run(context.Background(), []string{"switch", "--path-output"}, cli.Options{
		Cwd:       "/repo/demo.feature-alpha",
		Stdout:    &stdout,
		Stderr:    &stderr,
		GitRunner: newFakeGit(),
		Picker:    cli.PickerFunc(picker),
	})
	if code == 0 {
		t.Fatalf("expected picker cancellation to fail")
	}
	if stdout.String() != "" {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "cancel") {
		t.Fatalf("expected cancellation diagnostic on stderr, got %q", stderr.String())
	}
}

func runSwitch(t *testing.T, args []string, opts cli.Options) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if opts.Cwd == "" {
		opts.Cwd = "/repo/demo.feature-alpha"
	}
	if opts.Stdout == nil {
		opts.Stdout = &stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = &stderr
	}
	if opts.GitRunner == nil {
		opts.GitRunner = newFakeGit()
	}
	code := cli.Run(context.Background(), args, opts)
	return stdout.String(), stderr.String(), code
}
