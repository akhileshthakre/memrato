package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInjectOrderAndMissingFiles(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, memDir))
	write(t, filepath.Join(root, memDir, projectFile), "PROJECT")
	// local.md deliberately absent: a missing memory file is not an error.

	var out, errOut bytes.Buffer
	if err := runInject(root, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "PROJECT") {
		t.Errorf("project.md was not injected: %q", out.String())
	}
}

func TestInjectEmptyIsNotAnError(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := runInject(t.TempDir(), &out, &errOut); err != nil {
		t.Fatalf("empty repo should inject cleanly, got %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output, got %q", out.String())
	}
}

func TestTrimToBudget(t *testing.T) {
	tests := []struct {
		name   string
		bodies []string
		budget int
		want   []string
	}{
		{
			name:   "under budget is untouched",
			bodies: []string{"a", "b", "c"},
			budget: 100,
			want:   []string{"a", "b", "c"},
		},
		{
			name:   "trims the lowest-priority file last-section-first",
			bodies: []string{"global", "## A\naaa\n## B\nbbb", "## C\nccc"},
			budget: 30,
			want:   []string{"global", "## A\naaa\n## B\nbbb", ""},
		},
		{
			name:   "moves up a file once the lowest is exhausted",
			bodies: []string{"g", "## A\naaaaa\n## B\nbbbbb", "## C\nccccc"},
			budget: 12,
			want:   []string{"g", "## A\naaaaa", ""},
		},
		{
			name:   "terminates when nothing can be dropped",
			bodies: []string{"aaaaaaaaaa"},
			budget: 1,
			want:   []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodies := append([]string{}, tt.bodies...)
			trimmed := trimToBudget(bodies, tt.budget)

			for i := range bodies {
				if bodies[i] != tt.want[i] {
					t.Errorf("bodies[%d] = %q, want %q", i, bodies[i], tt.want[i])
				}
			}
			if want := total(tt.bodies) > tt.budget; trimmed != want {
				t.Errorf("trimmed = %v, want %v", trimmed, want)
			}
			if n := total(bodies); n > tt.budget {
				t.Errorf("still %d chars over a budget of %d", n, tt.budget)
			}
		})
	}
}

func TestInjectRespectsBudgetEnv(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, memDir))
	write(t, filepath.Join(root, memDir, projectFile), "## Keep\nkeep\n## Drop\n"+strings.Repeat("x", 500))
	t.Setenv(budgetEnvVar, "20") // 80 chars

	var out, errOut bytes.Buffer
	if err := runInject(root, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "xxxxx") {
		t.Error("over-budget section was injected anyway")
	}
	if !strings.Contains(errOut.String(), "budget") {
		t.Error("no warning on stderr when trimming")
	}
}

func TestStatusShowsEverySource(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, memDir))
	write(t, filepath.Join(root, memDir, projectFile), "## Stack\n- Go")

	var out bytes.Buffer
	if err := runStatus(root, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"global", "project", "local", "Total"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("status output is missing %q:\n%s", want, out.String())
		}
	}
}

func TestReadMissingFileIsEmpty(t *testing.T) {
	if got := read(filepath.Join(os.TempDir(), "definitely-not-here-memr.md")); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
