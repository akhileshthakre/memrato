package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendToSection(t *testing.T) {
	tests := []struct {
		name    string
		doc     string
		section string
		text    string
		want    string
	}{
		{
			name:    "into an empty section",
			doc:     "# M\n\n## Stack\n\n## Gotchas\n",
			section: "Stack",
			text:    "Go",
			want:    "# M\n\n## Stack\n- Go\n\n## Gotchas\n",
		},
		{
			name:    "after existing entries",
			doc:     "## Stack\n- Go\n- Postgres\n\n## Gotchas\n- x\n",
			section: "Stack",
			text:    "Redis",
			want:    "## Stack\n- Go\n- Postgres\n- Redis\n\n## Gotchas\n- x\n",
		},
		{
			name:    "last section in the file",
			doc:     "## Stack\n- Go\n",
			section: "Stack",
			text:    "Redis",
			want:    "## Stack\n- Go\n- Redis\n",
		},
		{
			name:    "creates a missing section",
			doc:     "# M\n\n## Stack\n- Go\n",
			section: "Decisions",
			text:    "No proxy",
			want:    "# M\n\n## Stack\n- Go\n\n## Decisions\n\n- No proxy",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := appendToSection(tt.doc, tt.section, tt.text); got != tt.want {
				t.Errorf("got:\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}

func setupReview(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mkdir(t, filepath.Join(root, memDir))
	write(t, filepath.Join(root, memDir, projectFile), "# M\n\n## Stack\n\n## Gotchas\n")
	write(t, filepath.Join(root, memDir, proposedFile), proposedHeader+
		"- [ ] **Stack** — Go 1.22 (0.90)\n"+
		"- [ ] **Gotchas** — CI needs CGO_ENABLED=0 (0.80)\n"+
		"- [ ] **Stack** — Redis for queues (0.60)\n")
	return root
}

func TestReviewAcceptRejectEdit(t *testing.T) {
	root := setupReview(t)
	var out bytes.Buffer
	// accept, reject, edit
	if err := runReview(root, strings.NewReader("y\nn\ne\nValkey for queues\n"), &out, false); err != nil {
		t.Fatal(err)
	}

	doc := mustRead(t, filepath.Join(root, memDir, projectFile))
	if !strings.Contains(doc, "- Go 1.22") {
		t.Errorf("accepted proposal missing:\n%s", doc)
	}
	if strings.Contains(doc, "CGO_ENABLED") {
		t.Errorf("rejected proposal was applied:\n%s", doc)
	}
	if !strings.Contains(doc, "- Valkey for queues") || strings.Contains(doc, "Redis") {
		t.Errorf("edited text not applied:\n%s", doc)
	}
	if fileExists(filepath.Join(root, memDir, proposedFile)) {
		t.Error("proposed.md should be gone once everything is decided")
	}
}

func TestReviewQuitLeavesTheRestPending(t *testing.T) {
	root := setupReview(t)
	var out bytes.Buffer
	if err := runReview(root, strings.NewReader("y\nq\n"), &out, false); err != nil {
		t.Fatal(err)
	}

	if doc := mustRead(t, filepath.Join(root, memDir, projectFile)); !strings.Contains(doc, "- Go 1.22") {
		t.Errorf("first acceptance was lost:\n%s", doc)
	}
	pending := parseProposals(mustRead(t, filepath.Join(root, memDir, proposedFile)))
	if len(pending) != 2 {
		t.Fatalf("got %d pending after quitting on item 2, want 2", len(pending))
	}
	if pending[0].Text != "CI needs CGO_ENABLED=0" {
		t.Errorf("wrong item left pending: %+v", pending[0])
	}
}

func TestReviewEofLeavesEverythingPending(t *testing.T) {
	root := setupReview(t)
	var out bytes.Buffer
	if err := runReview(root, strings.NewReader(""), &out, false); err != nil {
		t.Fatal(err)
	}
	if n := len(parseProposals(mustRead(t, filepath.Join(root, memDir, proposedFile)))); n != 3 {
		t.Errorf("got %d pending, want all 3 kept", n)
	}
}

func TestReviewWithNothingPending(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	if err := runReview(root, strings.NewReader(""), &out, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Nothing to review") {
		t.Errorf("unexpected output: %q", out.String())
	}
}
