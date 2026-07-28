package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"wg/internal/cli"
	"wg/internal/git"
)

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

type gitCall struct {
	Dir  string
	Args []string
}

type fakeGit struct {
	top            string
	porcelain      string
	mergedBranches string
	calls          []gitCall
}

func (f *fakeGit) Run(ctx context.Context, dir string, args ...string) (git.Result, error) {
	f.calls = append(f.calls, gitCall{Dir: dir, Args: append([]string(nil), args...)})
	switch strings.Join(args, " ") {
	case "rev-parse --show-toplevel":
		return git.Result{Stdout: f.top + "\n", ExitCode: 0}, nil
	case "worktree list --porcelain":
		return git.Result{Stdout: f.porcelain, ExitCode: 0}, nil
	case "for-each-ref --format=%(refname:short) --merged=main refs/heads":
		return git.Result{Stdout: f.mergedBranches, ExitCode: 0}, nil
	default:
		return git.Result{Stderr: "unexpected git command", ExitCode: 1}, nil
	}
}

func TestReadOnlyCommandsUseOnlyFastMetadataGitCalls(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want [][]string
	}{
		{
			name: "list",
			args: []string{"list"},
			want: [][]string{
				{"rev-parse", "--show-toplevel"},
				{"worktree", "list", "--porcelain"},
				{"symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"},
				{"for-each-ref", "--format=%(refname:short)", "--merged=main", "refs/heads"},
			},
		},
		{
			name: "path",
			args: []string{"path", "feature-alpha"},
			want: [][]string{{"rev-parse", "--show-toplevel"}, {"worktree", "list", "--porcelain"}},
		},
		{
			name: "env",
			args: []string{"env"},
			want: [][]string{
				{"rev-parse", "--show-toplevel"},
				{"worktree", "list", "--porcelain"},
				{"symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"},
				{"ls-remote", "--symref", "origin", "HEAD"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fg := newFakeGit()
			var stdout, stderr bytes.Buffer
			code := cli.Run(context.Background(), tc.args, cli.Options{
				Cwd:       "/repo/demo.feature-alpha/subdir",
				Stdout:    &stdout,
				Stderr:    &stderr,
				GitRunner: fg,
			})
			if code != 0 {
				t.Fatalf("expected success, code=%d stderr=%q", code, stderr.String())
			}
			if got := callArgs(fg.calls); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("git calls mismatch\nwant: %#v\n got: %#v", tc.want, got)
			}
			forbidden := []string{"status", "url", "port", "ci", "summary"}
			for _, call := range fg.calls {
				joined := strings.ToLower(strings.Join(call.Args, " "))
				for _, word := range forbidden {
					if strings.Contains(joined, word) {
						t.Fatalf("read-only command used forbidden probe %q in git call %v", word, call.Args)
					}
				}
			}
		})
	}
}

func TestListHumanOutputAlignsColumnsAndMutesIntegratedBranches(t *testing.T) {
	fg := newFakeGit()
	fg.mergedBranches = "main\nfeature-alpha\n"
	var stdout, stderr bytes.Buffer
	code := cli.Run(context.Background(), []string{"list"}, cli.Options{
		Cwd:       "/repo/demo.feature-alpha/subdir",
		Stdout:    &stdout,
		Stderr:    &stderr,
		GitRunner: fg,
	})
	if code != 0 {
		t.Fatalf("expected success, code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\x1b[2mfeature-alpha") {
		t.Fatalf("expected integrated branch name to be faint, got:\n%s", stdout.String())
	}

	plain := ansiEscapePattern.ReplaceAllString(stdout.String(), "")
	if strings.Contains(plain, "main main") || strings.Contains(plain, "feature-alpha feature-alpha") {
		t.Fatalf("expected human list not to repeat branch as state, got:\n%s", plain)
	}
	lines := strings.Split(strings.TrimSuffix(plain, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected three list rows, got %#v", lines)
	}
	pathColumn := strings.Index(lines[0], "/repo/demo")
	if pathColumn < 0 {
		t.Fatalf("expected path in first row, got %q", lines[0])
	}
	for _, line := range lines[1:] {
		if got := strings.Index(line, "/repo/"); got != pathColumn {
			t.Fatalf("expected paths to align at column %d, got %d in:\n%s", pathColumn, got, plain)
		}
	}
	for _, call := range fg.calls {
		if strings.Join(call.Args, " ") == "ls-remote --symref origin HEAD" {
			t.Fatalf("expected list merged-status lookup to avoid network-capable ls-remote call; calls: %#v", fg.calls)
		}
	}
}

func TestListJSONSkipsMergedStatusLookup(t *testing.T) {
	fg := newFakeGit()
	var stdout, stderr bytes.Buffer
	code := cli.Run(context.Background(), []string{"list", "--json"}, cli.Options{
		Cwd:       "/repo/demo.feature-alpha/subdir",
		Stdout:    &stdout,
		Stderr:    &stderr,
		GitRunner: fg,
	})
	if code != 0 {
		t.Fatalf("expected success, code=%d stderr=%q", code, stderr.String())
	}
	if got := callArgs(fg.calls); !reflect.DeepEqual(got, [][]string{{"rev-parse", "--show-toplevel"}, {"worktree", "list", "--porcelain"}}) {
		t.Fatalf("expected json list to skip merged lookup, got calls %#v", got)
	}

	var entries []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &entries); err != nil {
		t.Fatalf("expected valid json, got %v from %q", err, stdout.String())
	}
	if len(entries) != 3 {
		t.Fatalf("expected three json entries, got %#v", entries)
	}
	if entries[1]["name"] != "feature-alpha" || entries[1]["branch"] != "feature-alpha" || entries[1]["current"] != true {
		t.Fatalf("unexpected json entry: %#v", entries[1])
	}
	if _, ok := entries[1]["state"]; ok {
		t.Fatalf("json list should not include redundant state: %#v", entries[1])
	}
	if _, ok := entries[1]["integrated"]; ok {
		t.Fatalf("json list should not include integrated status: %#v", entries[1])
	}
	if _, ok := entries[1]["merged"]; ok {
		t.Fatalf("json list should not include merged status: %#v", entries[1])
	}
}

func TestPathAndEnvStdoutDataContracts(t *testing.T) {
	t.Run("path success writes only data to stdout", func(t *testing.T) {
		fg := newFakeGit()
		var stdout, stderr bytes.Buffer
		code := cli.Run(context.Background(), []string{"path", "feature-alpha"}, cli.Options{
			Cwd:       "/repo/demo.feature-alpha",
			Stdout:    &stdout,
			Stderr:    &stderr,
			GitRunner: fg,
		})
		if code != 0 {
			t.Fatalf("expected success, code=%d stderr=%q", code, stderr.String())
		}
		if stdout.String() != "/repo/demo.feature-alpha\n" {
			t.Fatalf("unexpected stdout %q", stdout.String())
		}
		if stderr.String() != "" {
			t.Fatalf("expected empty stderr, got %q", stderr.String())
		}
	})

	t.Run("env success writes only data to stdout", func(t *testing.T) {
		fg := newFakeGit()
		var stdout, stderr bytes.Buffer
		code := cli.Run(context.Background(), []string{"env", "feature-beta"}, cli.Options{
			Cwd:       "/repo/demo.feature-alpha",
			Stdout:    &stdout,
			Stderr:    &stderr,
			GitRunner: fg,
		})
		if code != 0 {
			t.Fatalf("expected success, code=%d stderr=%q", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "WG_WORKTREE_PATH=/repo/demo.feature-beta\n") {
			t.Fatalf("env stdout missing target path:\n%s", stdout.String())
		}
		if stderr.String() != "" {
			t.Fatalf("expected empty stderr, got %q", stderr.String())
		}
	})

	t.Run("missing names diagnose only on stderr", func(t *testing.T) {
		fg := newFakeGit()
		var stdout, stderr bytes.Buffer
		code := cli.Run(context.Background(), []string{"path", "missing"}, cli.Options{
			Cwd:       "/repo/demo.feature-alpha",
			Stdout:    &stdout,
			Stderr:    &stderr,
			GitRunner: fg,
		})
		if code == 0 {
			t.Fatalf("expected missing path to fail")
		}
		if stdout.String() != "" {
			t.Fatalf("expected empty stdout, got %q", stdout.String())
		}
		if !strings.Contains(stderr.String(), "missing") {
			t.Fatalf("expected missing diagnostic on stderr, got %q", stderr.String())
		}
	})

	t.Run("ambiguous names diagnose only on stderr with candidates", func(t *testing.T) {
		fg := newFakeGit()
		var stdout, stderr bytes.Buffer
		code := cli.Run(context.Background(), []string{"env", "feature"}, cli.Options{
			Cwd:       "/repo/demo.feature-alpha",
			Stdout:    &stdout,
			Stderr:    &stderr,
			GitRunner: fg,
		})
		if code == 0 {
			t.Fatalf("expected ambiguity to fail")
		}
		if stdout.String() != "" {
			t.Fatalf("expected empty stdout, got %q", stdout.String())
		}
		if !strings.Contains(stderr.String(), "feature-alpha") || !strings.Contains(stderr.String(), "feature-beta") {
			t.Fatalf("expected candidates on stderr, got %q", stderr.String())
		}
	})
}

func TestRemoveCompletionOutputsEligibleBranchMatches(t *testing.T) {
	cases := []struct {
		name      string
		prefix    string
		porcelain string
		want      string
	}{
		{
			name:   "unique branch prefix",
			prefix: "feature-a",
			want:   "feature-alpha\n",
		},
		{
			name:   "ambiguous branch prefix",
			prefix: "feature",
			want:   "feature-alpha\nfeature-beta\n",
		},
		{
			name:   "missing branch prefix",
			prefix: "missing",
		},
		{
			name:   "case sensitive prefix",
			prefix: "Feature-a",
		},
		{
			name:   "primary branch excluded",
			prefix: "ma",
		},
		{
			name:   "detached worktree excluded",
			prefix: "detached",
			porcelain: strings.Join([]string{
				"worktree /repo/demo",
				"HEAD 1111111111111111111111111111111111111111",
				"branch refs/heads/main",
				"",
				"worktree /repo/demo.detached",
				"HEAD 2222222222222222222222222222222222222222",
				"detached",
				"",
			}, "\n"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fg := newFakeGit()
			if tc.porcelain != "" {
				fg.porcelain = tc.porcelain
			}
			var stdout, stderr bytes.Buffer
			code := cli.Run(context.Background(), []string{"config", "shell", "complete", "remove", tc.prefix}, cli.Options{
				Cwd:       "/repo/demo.feature-alpha",
				Stdout:    &stdout,
				Stderr:    &stderr,
				GitRunner: fg,
			})
			if code != 0 {
				t.Fatalf("expected success, code=%d stderr=%q", code, stderr.String())
			}
			if stdout.String() != tc.want {
				t.Fatalf("unexpected stdout: want %q got %q", tc.want, stdout.String())
			}
			if stderr.String() != "" {
				t.Fatalf("expected empty stderr, got %q", stderr.String())
			}
			if got := callArgs(fg.calls); !reflect.DeepEqual(got, [][]string{{"rev-parse", "--show-toplevel"}, {"worktree", "list", "--porcelain"}}) {
				t.Fatalf("git calls mismatch: %#v", got)
			}
		})
	}
}

func TestEnvStableOrderedExistingWorktreeOutput(t *testing.T) {
	fg := newFakeGit()
	var firstStdout, firstStderr bytes.Buffer
	firstCode := cli.Run(context.Background(), []string{"env"}, cli.Options{
		Cwd:       "/repo/demo.feature-alpha/subdir",
		Stdout:    &firstStdout,
		Stderr:    &firstStderr,
		GitRunner: fg,
	})
	if firstCode != 0 {
		t.Fatalf("expected first env success, code=%d stderr=%q", firstCode, firstStderr.String())
	}

	fg = newFakeGit()
	var secondStdout, secondStderr bytes.Buffer
	secondCode := cli.Run(context.Background(), []string{"env"}, cli.Options{
		Cwd:       "/repo/demo.feature-alpha/subdir",
		Stdout:    &secondStdout,
		Stderr:    &secondStderr,
		GitRunner: fg,
	})
	if secondCode != 0 {
		t.Fatalf("expected second env success, code=%d stderr=%q", secondCode, secondStderr.String())
	}
	if firstStdout.String() != secondStdout.String() {
		t.Fatalf("expected repeated env output to be byte-for-byte stable\nfirst:\n%s\nsecond:\n%s", firstStdout.String(), secondStdout.String())
	}

	lines := strings.Split(strings.TrimSpace(firstStdout.String()), "\n")
	wantKeys := []string{
		"WG_BRANCH",
		"WG_WORKTREE_PATH",
		"WG_WORKTREE_NAME",
		"WG_REPO",
		"WG_PRIMARY_WORKTREE_PATH",
		"WG_DEFAULT_BRANCH",
		"WG_BASE",
		"WG_PORT",
	}
	if len(lines) != len(wantKeys) {
		t.Fatalf("expected %d env lines, got %d: %#v", len(wantKeys), len(lines), lines)
	}
	values := make(map[string]string)
	for i, line := range lines {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("invalid env line %q", line)
		}
		if key != wantKeys[i] {
			t.Fatalf("env key order mismatch at %d: want %s got %s", i, wantKeys[i], key)
		}
		values[key] = value
	}
	if values["WG_WORKTREE_PATH"] != "/repo/demo.feature-alpha" || values["WG_WORKTREE_NAME"] != "feature-alpha" {
		t.Fatalf("unexpected current worktree env values: %#v", values)
	}
	if values["WG_DEFAULT_BRANCH"] != "main" {
		t.Fatalf("expected default branch main, got %q", values["WG_DEFAULT_BRANCH"])
	}
	if lines[6] != "WG_BASE=" {
		t.Fatalf("existing worktree should render exactly WG_BASE=, got %q", lines[6])
	}
	port, err := strconv.Atoi(values["WG_PORT"])
	if err != nil || port < 10000 || port > 19999 {
		t.Fatalf("expected stable WG_PORT in range, got %q", values["WG_PORT"])
	}

	fg = newFakeGit()
	var namedStdout, namedStderr bytes.Buffer
	namedCode := cli.Run(context.Background(), []string{"env", "feature-beta"}, cli.Options{
		Cwd:       "/repo/demo.feature-alpha/subdir",
		Stdout:    &namedStdout,
		Stderr:    &namedStderr,
		GitRunner: fg,
	})
	if namedCode != 0 {
		t.Fatalf("expected named env success, code=%d stderr=%q", namedCode, namedStderr.String())
	}
	if !strings.Contains(namedStdout.String(), "WG_WORKTREE_PATH=/repo/demo.feature-beta\n") {
		t.Fatalf("expected named worktree path in env output, got:\n%s", namedStdout.String())
	}
	if !strings.Contains(namedStdout.String(), "WG_BASE=\n") {
		t.Fatalf("expected named existing worktree to render empty base, got:\n%s", namedStdout.String())
	}
}

func newFakeGit() *fakeGit {
	return &fakeGit{
		top:            "/repo/demo.feature-alpha",
		mergedBranches: "main\n",
		porcelain: strings.Join([]string{
			"worktree /repo/demo",
			"HEAD 1111111111111111111111111111111111111111",
			"branch refs/heads/main",
			"",
			"worktree /repo/demo.feature-alpha",
			"HEAD 2222222222222222222222222222222222222222",
			"branch refs/heads/feature-alpha",
			"",
			"worktree /repo/demo.feature-beta",
			"HEAD 3333333333333333333333333333333333333333",
			"branch refs/heads/feature-beta",
			"",
		}, "\n"),
	}
}

func callArgs(calls []gitCall) [][]string {
	out := make([][]string, 0, len(calls))
	for _, call := range calls {
		out = append(out, call.Args)
	}
	return out
}
