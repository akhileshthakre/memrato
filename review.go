package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var proposalLine = regexp.MustCompile(`^- \[ \] \*\*(.+?)\*\* — (.+?) \(([0-9.]+)\)\s*$`)

func parseProposals(doc string) []proposal {
	var out []proposal
	for _, line := range strings.Split(doc, "\n") {
		m := proposalLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		conf, _ := strconv.ParseFloat(m[3], 64)
		out = append(out, proposal{Section: m[1], Text: m[2], Confidence: conf})
	}
	return out
}

// Words too common to signal that two entries are about the same thing.
var stopwords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true, "be": true,
	"for": true, "in": true, "is": true, "it": true, "of": true, "on": true, "or": true,
	"that": true, "the": true, "this": true, "to": true, "we": true, "with": true,
}

func words(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !('a' <= r && r <= 'z' || '0' <= r && r <= '9' || r == '.' || r == '-')
	}) {
		if !stopwords[w] {
			out[w] = true
		}
	}
	return out
}

// overlap is the overlap coefficient — shared words over the smaller set. It
// beats Jaccard here because a terse existing entry and a wordy proposal about
// the same fact should still register as a conflict.
//
// ponytail: word overlap, not embeddings. It only has to be good enough to make
// a human look; it never decides anything.
func overlap(a, b string) float64 {
	wa, wb := words(a), words(b)
	if len(wa) == 0 || len(wb) == 0 {
		return 0
	}
	shared := 0
	for w := range wa {
		if wb[w] {
			shared++
		}
	}
	if shared < 2 { // one word in common is a coincidence, not a conflict
		return 0
	}
	return float64(shared) / float64(min(len(wa), len(wb)))
}

const conflictThreshold = 0.6

// sectionEntries maps each "## Section" to its "- entry" lines.
func sectionEntries(doc string) map[string][]string {
	out := map[string][]string{}
	section := ""
	for _, line := range strings.Split(doc, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "## ") {
			section = strings.TrimSpace(t[3:])
		} else if strings.HasPrefix(t, "- ") && section != "" {
			out[section] = append(out[section], strings.TrimSpace(t[2:]))
		}
	}
	return out
}

// conflicts returns existing entries in the same section that look like they
// state the same fact. Surfaced to the human, never resolved automatically:
// two people distilling contradicting facts is exactly the case where guessing
// is worse than asking.
func conflicts(doc string, p proposal) []string {
	var out []string
	for _, existing := range sectionEntries(doc)[p.Section] {
		if overlap(existing, p.Text) >= conflictThreshold {
			out = append(out, existing)
		}
	}
	return out
}

func runReview(root string, stdin io.Reader, stdout io.Writer, asPR bool) error {
	proposedPath := filepath.Join(root, memDir, proposedFile)
	pending := parseProposals(read(proposedPath))
	if len(pending) == 0 {
		fmt.Fprintln(stdout, "Nothing to review.")
		return nil
	}

	projectPath := filepath.Join(root, memDir, projectFile)
	doc := read(projectPath)
	if doc == "" {
		return fmt.Errorf("%s is missing — run `memrato init` first", projectPath)
	}

	in := bufio.NewScanner(stdin)
	var accepted, remaining []proposal
loop:
	for i, p := range pending {
		fmt.Fprintf(stdout, "\n[%d/%d] %s (confidence %.2f)\n  %s\n", i+1, len(pending), p.Section, p.Confidence, p.Text)
		for _, c := range conflicts(doc, p) {
			fmt.Fprintf(stdout, "  ! may conflict with an existing entry:\n      %s\n", c)
		}
		fmt.Fprint(stdout, "  [y]es / [n]o / [e]dit / [q]uit: ")

		if !in.Scan() {
			remaining = append(remaining, pending[i:]...)
			break loop
		}
		switch strings.ToLower(strings.TrimSpace(in.Text())) {
		case "y", "":
			accepted = append(accepted, p)
		case "e":
			// ponytail: inline retype, not $EDITOR. Add an editor spawn if
			// people start editing multi-sentence entries.
			fmt.Fprint(stdout, "  new text: ")
			if in.Scan() {
				if t := strings.TrimSpace(in.Text()); t != "" {
					p.Text = t
					accepted = append(accepted, p)
				}
			}
		case "q":
			remaining = append(remaining, pending[i:]...)
			break loop
		default: // "n" and anything else: drop it
		}
	}

	for _, p := range accepted {
		doc = appendToSection(doc, p.Section, p.Text)
	}
	if len(accepted) > 0 {
		if err := os.WriteFile(projectPath, []byte(doc+"\n"), 0o644); err != nil {
			return err
		}
	}
	if err := rewriteProposed(proposedPath, remaining); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "\nApplied %d, discarded %d, left %d pending.\n",
		len(accepted), len(pending)-len(accepted)-len(remaining), len(remaining))
	if len(accepted) == 0 {
		return nil
	}
	if asPR {
		return openPR(root, len(accepted), stdout)
	}
	fmt.Fprintln(stdout, "Review the diff and commit", projectPath)
	return nil
}

// appendToSection inserts a line-oriented entry at the end of its section,
// creating the section if it is missing. Line-oriented on purpose: it makes
// `git blame` the attribution feature.
func appendToSection(doc, section, text string) string {
	lines := strings.Split(doc, "\n")
	heading := "## " + section

	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == heading {
			start = i
			break
		}
	}
	if start < 0 {
		return strings.TrimRight(doc, "\n") + "\n\n" + heading + "\n\n- " + text
	}

	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			end = i
			break
		}
	}
	// Back up over trailing blank lines so the entry lands inside the section.
	insert := end
	for insert > start+1 && strings.TrimSpace(lines[insert-1]) == "" {
		insert--
	}

	out := append([]string{}, lines[:insert]...)
	out = append(out, "- "+text)
	out = append(out, lines[insert:]...)
	return strings.Join(out, "\n")
}

func rewriteProposed(path string, remaining []proposal) error {
	if len(remaining) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	var b strings.Builder
	b.WriteString(proposedHeader)
	for _, p := range remaining {
		b.WriteString(formatProposal(p))
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
