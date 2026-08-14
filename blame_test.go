package main

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
	"time"
)

// A hand-built git blame --line-porcelain fixture. Real blame output is huge;
// the parser only cares about these four line shapes.
func blameFixture(oldTime, newTime int64) string {
	var b strings.Builder
	commit := func(hash, author string, when int64, content string) {
		b.WriteString(hash + " 1 1 1\n")
		b.WriteString("author " + author + "\n")
		b.WriteString("author-time " + strconv.FormatInt(when, 10) + "\n")
		b.WriteString("filename .memory/project.md\n")
		b.WriteString("\t" + content + "\n")
	}
	commit("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "Ada", oldTime, "## Stack")
	commit("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "Ada", oldTime, "- Postgres 16 is the datastore")
	commit("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "Grace", newTime, "## Gotchas")
	commit("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "Grace", newTime, "- CI needs CGO_ENABLED=0")
	commit("0000000000000000000000000000000000000000", "Not Committed Yet", newTime, "- freshly added, not committed")
	return b.String()
}

func TestParseBlame(t *testing.T) {
	old := time.Now().Add(-400 * 24 * time.Hour).Unix()
	recent := time.Now().Add(-24 * time.Hour).Unix()
	entries := parseBlame(blameFixture(old, recent))

	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3 (headings are not entries)", len(entries))
	}

	if entries[0].section != "Stack" || entries[0].author != "Ada" {
		t.Errorf("entry 0 = %+v, want section Stack by Ada", entries[0])
	}
	if !entries[0].stale() {
		t.Error("a 400-day-old entry should be stale")
	}
	if entries[1].section != "Gotchas" || entries[1].stale() {
		t.Errorf("entry 1 = %+v, want a fresh Gotchas entry", entries[1])
	}
	if entries[2].committed {
		t.Error("the all-zero hash marks an uncommitted line")
	}
	if entries[2].stale() {
		t.Error("uncommitted lines can never be stale")
	}
}

func TestRunBlameOutput(t *testing.T) {
	// No git repo here, so this exercises the failure path users hit first.
	var out bytes.Buffer
	if err := runBlame(t.TempDir(), &out); err == nil {
		t.Error("expected an error outside a git repo")
	}
}
