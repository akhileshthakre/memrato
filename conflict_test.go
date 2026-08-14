package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestOverlap(t *testing.T) {
	tests := []struct {
		name     string
		a, b     string
		conflict bool
	}{
		{
			name:     "same fact, different words",
			a:        "Postgres 16 is the primary datastore",
			b:        "We use Postgres 16 as the datastore",
			conflict: true,
		},
		{
			name:     "terse existing entry vs wordy proposal",
			a:        "pnpm only",
			b:        "The project uses pnpm only, npm breaks the workspace links",
			conflict: true,
		},
		{
			name:     "contradiction about the same subject still registers",
			a:        "Deploys run on Vercel",
			b:        "Deploys run on Fly.io, not Vercel",
			conflict: true,
		},
		{
			name:     "unrelated facts",
			a:        "Postgres 16 is the datastore",
			b:        "Tests are colocated next to the file they cover",
			conflict: false,
		},
		{
			name:     "one shared word is a coincidence, not a conflict",
			a:        "The build uses Go",
			b:        "Go read the contributing guide",
			conflict: false,
		},
		{
			name:     "stopwords alone never trigger",
			a:        "it is in the repo",
			b:        "we are on the branch",
			conflict: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := overlap(tt.a, tt.b)
			if (got >= conflictThreshold) != tt.conflict {
				t.Errorf("overlap = %.2f, want conflict=%v (threshold %.2f)", got, tt.conflict, conflictThreshold)
			}
		})
	}
}

func TestSectionEntries(t *testing.T) {
	doc := "# M\n\n## Stack\n- Go\n- Postgres\n\n## Gotchas\n- pnpm only\n\nsome prose\n"
	got := sectionEntries(doc)
	if len(got["Stack"]) != 2 || got["Stack"][0] != "Go" {
		t.Errorf("Stack = %v, want [Go Postgres]", got["Stack"])
	}
	if len(got["Gotchas"]) != 1 {
		t.Errorf("Gotchas = %v, want one entry", got["Gotchas"])
	}
}

func TestConflictsAreScopedToTheSection(t *testing.T) {
	doc := "## Stack\n- Postgres 16 is the datastore\n\n## Gotchas\n- Postgres 16 is the datastore\n"
	got := conflicts(doc, proposal{Section: "Stack", Text: "We use Postgres 16 as the datastore", Confidence: 1})
	if len(got) != 1 {
		t.Fatalf("got %d conflicts, want 1 — the identical Gotchas line is a different section", len(got))
	}
}

// The conflict must reach the human before they answer y/n, or it is useless.
func TestReviewSurfacesConflictBeforePrompting(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, memDir))
	write(t, filepath.Join(root, memDir, projectFile), "# M\n\n## Stack\n- Postgres 16 is the primary datastore\n")
	write(t, filepath.Join(root, memDir, proposedFile), proposedHeader+
		"- [ ] **Stack** — We use Postgres 16 as the datastore (0.80)\n")

	var out bytes.Buffer
	if err := runReview(root, strings.NewReader("n\n"), &out, false); err != nil {
		t.Fatal(err)
	}

	s := out.String()
	if !strings.Contains(s, "may conflict") {
		t.Errorf("no conflict warning shown:\n%s", s)
	}
	warn := strings.Index(s, "may conflict")
	prompt := strings.Index(s, "[y]es")
	if warn > prompt {
		t.Error("conflict warning printed after the prompt, too late to be useful")
	}
	// Surfaced, never resolved: the file is untouched because the human said no.
	if got := mustRead(t, filepath.Join(root, memDir, projectFile)); strings.Contains(got, "We use") {
		t.Error("conflicting entry was applied despite being rejected")
	}
}
