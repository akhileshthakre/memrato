package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// The single highest-leverage string in this repo. Iterate on it more than on
// any code here; testdata/ holds real transcripts to check changes against.
const distillPrompt = `You maintain a long-lived memory file for a software project.

Below is a transcript of one coding session, and the project's current memory file.
Output ONLY durable facts learned in this session that are NOT already in the memory file.

Include: architecture decisions and their reasons, conventions the team follows,
non-obvious constraints, gotchas that cost time, tooling and stack choices.

Exclude, without exception:
- transient debugging details and anything specific to one bug
- one-off file paths, line numbers, function names, or command invocations
- anything already stated in the memory file, even in different words
- anything phrased as a question, a guess, a plan, or an uncertainty
- restatements of what the code obviously says

Prefer proposing nothing over proposing noise. An empty array is a good answer.

Reply with a JSON array and nothing else. Each element:
{"section": one of [Stack, Conventions, Decisions, Gotchas], "text": "one sentence, present tense, no preamble", "confidence": 0.0-1.0}

=== CURRENT MEMORY FILE ===
%s

=== SESSION TRANSCRIPT ===
%s
`

type proposal struct {
	Section    string  `json:"section"`
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
}

// runDistill never returns an error and never exits non-zero: it runs on
// SessionEnd, and a broken distiller must not break the user's session. It
// also never writes to project.md — only proposed.md.
func runDistill(stdin io.Reader) {
	defer func() { _ = recover() }()

	// We shell out to `claude`, which fires SessionEnd again. Without this
	// guard that is an unbounded fork bomb.
	if os.Getenv(guardEnvVar) != "" {
		return
	}

	var payload struct {
		TranscriptPath string `json:"transcript_path"`
		Cwd            string `json:"cwd"`
	}
	if json.NewDecoder(stdin).Decode(&payload) != nil || payload.TranscriptPath == "" {
		return
	}
	root := payload.Cwd
	if root == "" {
		root = "."
	}
	// Only distill for repos that opted in.
	if !fileExists(filepath.Join(root, memDir, projectFile)) {
		return
	}

	transcript, err := readTranscript(payload.TranscriptPath)
	if err != nil || len(transcript) < 200 {
		return // too short to have learned anything
	}

	raw, err := askModel(fmt.Sprintf(distillPrompt, read(filepath.Join(root, memDir, projectFile)), transcript))
	if err != nil {
		return
	}
	proposals := parseModelJSON(raw)
	if len(proposals) == 0 {
		return
	}
	_ = appendProposals(filepath.Join(root, memDir, proposedFile), proposals)
}

// readTranscript pulls the human-readable text out of a Claude Code .jsonl
// transcript: user prompts and assistant prose only. Thinking blocks, tool
// calls and tool results are the noise the distiller is told to ignore, so
// they never reach the model. Returns the tail, which is where conclusions land.
func readTranscript(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var b strings.Builder
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024) // transcript lines get big
	for sc.Scan() {
		var e struct {
			Type    string `json:"type"`
			Message struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		if e.Type != "user" && e.Type != "assistant" {
			continue
		}
		if text := contentText(e.Message.Content); text != "" {
			fmt.Fprintf(&b, "%s: %s\n\n", e.Message.Role, text)
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}

	out := b.String()
	if len(out) > maxTranscript {
		out = "[...earlier turns omitted...]\n" + out[len(out)-maxTranscript:]
	}
	return out, nil
}

// contentText handles both content shapes: a bare string, or an array of
// blocks of which we keep only the text ones.
func contentText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) != nil {
		return ""
	}
	var parts []string
	for _, blk := range blocks {
		if blk.Type == "text" && strings.TrimSpace(blk.Text) != "" {
			parts = append(parts, strings.TrimSpace(blk.Text))
		}
	}
	return strings.Join(parts, "\n")
}

// askModel prefers the user's existing Claude Code auth and falls back to the
// API when the binary is not on PATH.
func askModel(prompt string) (string, error) {
	if bin, err := exec.LookPath("claude"); err == nil {
		cmd := exec.Command(bin, "-p", "--model", distillModel, "--allowed-tools", "")
		cmd.Stdin = strings.NewReader(prompt)
		cmd.Env = append(os.Environ(), guardEnvVar+"=1")
		out, err := cmd.Output()
		if err != nil {
			return "", err
		}
		return string(out), nil
	}
	return callAPI(prompt)
}

func callAPI(prompt string) (string, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return "", fmt.Errorf("no claude binary on PATH and ANTHROPIC_API_KEY unset")
	}
	body, _ := json.Marshal(map[string]any{
		"model":      distillModel,
		"max_tokens": 2048,
		"messages":   []any{map[string]any{"role": "user", "content": prompt}},
	})
	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic api: %s", resp.Status)
	}
	var out struct {
		Content []struct{ Text string } `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	var b strings.Builder
	for _, c := range out.Content {
		b.WriteString(c.Text)
	}
	return b.String(), nil
}

// parseModelJSON digs the array out of whatever the model wrapped it in —
// code fences, a preamble, a sign-off. Cheaper than fighting the model.
func parseModelJSON(raw string) []proposal {
	start, end := strings.Index(raw, "["), strings.LastIndex(raw, "]")
	if start < 0 || end <= start {
		return nil
	}
	var proposals []proposal
	if json.Unmarshal([]byte(raw[start:end+1]), &proposals) != nil {
		return nil
	}
	var keep []proposal
	for _, p := range proposals {
		p.Text = strings.TrimSpace(p.Text)
		p.Section = strings.TrimSpace(p.Section)
		if p.Text != "" && p.Section != "" {
			keep = append(keep, p)
		}
	}
	return keep
}

const proposedHeader = "# Proposed memory additions\n\nRun `memr review` to accept or discard. Nothing here is in project.md yet.\n"

// appendProposals adds to the pending list rather than replacing it, so a run
// of unreviewed sessions does not silently drop earlier findings. Deduped on
// exact text.
func appendProposals(path string, proposals []proposal) error {
	existing := parseProposals(read(path))
	seen := map[string]bool{}
	for _, p := range existing {
		seen[strings.ToLower(p.Text)] = true
	}

	var b strings.Builder
	b.WriteString(proposedHeader)
	for _, p := range existing {
		b.WriteString(formatProposal(p))
	}
	added := 0
	for _, p := range proposals {
		if seen[strings.ToLower(p.Text)] {
			continue
		}
		seen[strings.ToLower(p.Text)] = true
		b.WriteString(formatProposal(p))
		added++
	}
	if added == 0 {
		return nil
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func formatProposal(p proposal) string {
	return fmt.Sprintf("- [ ] **%s** — %s (%.2f)\n", p.Section, p.Text, p.Confidence)
}
