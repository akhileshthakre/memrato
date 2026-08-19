package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// detachProbeVar turns this test binary into a stand-in for the detached
// distiller. Under `go test` the re-exec target detach picks is the test binary
// itself, so there is nothing else it could usefully start.
const detachProbeVar = "MEMR_TEST_DETACH_PROBE"

// TestMain records what detach handed us and exits, when it is us that detach
// started. Renamed into place because the parent polls for the file.
func TestMain(m *testing.M) {
	if probe := os.Getenv(detachProbeVar); probe != "" {
		raw, _ := io.ReadAll(os.Stdin)
		_ = os.WriteFile(probe+".tmp", append([]byte(os.Getenv(detachEnvVar)+"\n"), raw...), 0o644)
		_ = os.Rename(probe+".tmp", probe)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestReadTranscriptKeepsProseDropsNoise(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "t.jsonl")
	write(t, path, strings.Join([]string{
		`{"type":"user","message":{"role":"user","content":"KEEP_USER_STRING"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"KEEP_ASSISTANT"}]}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","thinking":"DROP_THINKING"}]}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"DROP_TOOL"}}]}}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","content":"DROP_RESULT"}]}}`,
		`{"type":"queue-operation","content":"DROP_QUEUE"}`,
		`not json at all`,
		`{"type":"user","message":{"role":"user","content":[{"type":"text","text":"KEEP_BLOCK"}]}}`,
	}, "\n"))

	got, err := readTranscript(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"KEEP_USER_STRING", "KEEP_ASSISTANT", "KEEP_BLOCK"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q from:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"DROP_THINKING", "DROP_TOOL", "DROP_RESULT", "DROP_QUEUE"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("leaked %q into the distiller prompt:\n%s", unwanted, got)
		}
	}
}

// Golden test over the shared fixture. Extend testdata/session.jsonl when you
// find a transcript shape the extractor mishandles.
func TestReadTranscriptMatchesGolden(t *testing.T) {
	got, err := readTranscript(filepath.Join("testdata", "session.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	want := mustRead(t, filepath.Join("testdata", "session.expected.txt"))
	if got != want {
		t.Errorf("extraction drifted from the golden file.\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestReadTranscriptKeepsTheTail(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "t.jsonl")
	var lines []string
	lines = append(lines, `{"type":"user","message":{"role":"user","content":"OLDEST"}}`)
	for i := 0; i < 400; i++ {
		lines = append(lines, `{"type":"user","message":{"role":"user","content":"`+strings.Repeat("x", 500)+`"}}`)
	}
	lines = append(lines, `{"type":"user","message":{"role":"user","content":"NEWEST"}}`)
	write(t, path, strings.Join(lines, "\n"))

	got, err := readTranscript(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) > maxTranscript+100 {
		t.Errorf("transcript not capped: %d chars", len(got))
	}
	if !strings.Contains(got, "NEWEST") {
		t.Error("dropped the most recent turn")
	}
	if strings.Contains(got, "OLDEST") {
		t.Error("kept the head instead of the tail")
	}
}

func TestParseModelJSON(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int
	}{
		{"bare array", `[{"section":"Stack","text":"Go 1.22","confidence":0.9}]`, 1},
		{"fenced", "Here you go:\n```json\n[{\"section\":\"Stack\",\"text\":\"Go\",\"confidence\":1}]\n```\nHope that helps!", 1},
		{"empty array", `[]`, 0},
		{"no array at all", `I could not find anything durable.`, 0},
		{"garbage", `[not json`, 0},
		{"drops entries with no text", `[{"section":"Stack","text":"  ","confidence":1},{"section":"Stack","text":"ok","confidence":1}]`, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := len(parseModelJSON(tt.raw)); got != tt.want {
				t.Errorf("got %d proposals, want %d", got, tt.want)
			}
		})
	}
}

func TestProposalsRoundTrip(t *testing.T) {
	in := []proposal{{"Stack", "Postgres 16 is the datastore", 0.9}, {"Gotchas", "CI needs CGO_ENABLED=0", 0.75}}
	path := filepath.Join(t.TempDir(), proposedFile)
	if err := appendProposals(path, in); err != nil {
		t.Fatal(err)
	}

	got := parseProposals(mustRead(t, path))
	if len(got) != len(in) {
		t.Fatalf("got %d proposals back, want %d", len(got), len(in))
	}
	for i := range in {
		if got[i] != in[i] {
			t.Errorf("proposal %d = %+v, want %+v", i, got[i], in[i])
		}
	}
}

func TestAppendProposalsDedupesAndKeepsEarlierOnes(t *testing.T) {
	path := filepath.Join(t.TempDir(), proposedFile)
	if err := appendProposals(path, []proposal{{"Stack", "Go 1.22", 0.9}}); err != nil {
		t.Fatal(err)
	}
	if err := appendProposals(path, []proposal{{"Stack", "go 1.22", 0.8}, {"Gotchas", "new one", 0.7}}); err != nil {
		t.Fatal(err)
	}

	got := parseProposals(mustRead(t, path))
	if len(got) != 2 {
		t.Fatalf("got %d proposals, want 2 (deduped, earlier kept):\n%s", len(got), mustRead(t, path))
	}
	if got[0].Text != "Go 1.22" || got[1].Text != "new one" {
		t.Errorf("unexpected contents: %+v", got)
	}
}

// The distiller shells out to `claude`, which fires SessionEnd, which runs the
// distiller. Without the guard this recurses without bound.
func TestDistillBailsOutWhenGuardIsSet(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, memDir))
	write(t, filepath.Join(root, memDir, projectFile), "# memory")
	t.Setenv(guardEnvVar, "1")

	runDistill(strings.NewReader(`{"transcript_path":"/nope","cwd":"` + root + `"}`))

	if fileExists(filepath.Join(root, memDir, proposedFile)) {
		t.Error("distill ran while the recursion guard was set")
	}
}

// A broken distiller must never break the user's session, and must never touch
// project.md. Every input here must also bail out before runDistill reaches
// detach: under `go test` the re-exec target is the test binary itself, so
// moving detach above these checks makes the suite fork copies of itself.
func TestDistillSurvivesBadInput(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, memDir))
	const original = "# memory\n\n## Stack\n"
	projectPath := filepath.Join(root, memDir, projectFile)
	write(t, projectPath, original)

	for _, in := range []string{
		``,
		`not json`,
		`{}`,
		`{"transcript_path":""}`,
		`{"transcript_path":"/does/not/exist","cwd":"` + root + `"}`,
	} {
		runDistill(strings.NewReader(in)) // must not panic
	}

	if got := mustRead(t, projectPath); got != original {
		t.Errorf("distill modified project.md:\n%s", got)
	}
}

func TestDistillIgnoresReposThatNeverRanInit(t *testing.T) {
	root := t.TempDir() // no .memory/project.md
	runDistill(strings.NewReader(`{"transcript_path":"/nope","cwd":"` + root + `"}`))
	if fileExists(filepath.Join(root, memDir, proposedFile)) {
		t.Error("wrote proposals into a repo that never opted in")
	}
}

// Handing the payload off and returning is the whole reason SessionEnd stopped
// reporting `Hook cancelled`. If detach silently stops starting the child, the
// distiller just never runs again and nothing anywhere says so.
func TestDetachStartsAChildWithTheEnvAndTheWholePayload(t *testing.T) {
	probe := filepath.Join(t.TempDir(), "probe")
	t.Setenv(detachProbeVar, probe)

	// Longer than a pipe buffer, so a stdin that gets copied instead of handed
	// over straight shows up as a short read.
	payload := `{"transcript_path":"/x","cwd":"/y","pad":"` + strings.Repeat("p", 80000) + `"}`
	detach([]byte(payload))

	var got string
	for i := 0; got == "" && i < 500; i++ {
		if b, err := os.ReadFile(probe); err == nil {
			got = string(b)
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got == "" {
		t.Fatal("detach never started a child that could be reached")
	}

	env, arrived, _ := strings.Cut(got, "\n")
	if env != "1" {
		t.Errorf("child has %s=%q, so it would detach again instead of distilling", detachEnvVar, env)
	}
	if arrived != payload {
		t.Errorf("child read %d bytes of payload, want %d", len(arrived), len(payload))
	}
}
