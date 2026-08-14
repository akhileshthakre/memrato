package main

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Shared memory rots faster than personal memory because nobody owns it, so
// anything older than this gets flagged. Not configurable on purpose.
const staleAfter = 6 * 30 * 24 * time.Hour

type entry struct {
	section   string
	text      string
	author    string
	when      time.Time
	committed bool
}

func (e entry) stale() bool { return e.committed && time.Since(e.when) > staleAfter }

// blameEntries answers "who added this, and when" without storing either in the
// file: project.md is line-oriented, so git already knows. That is the whole
// reason the format is one fact per line.
func blameEntries(root string) ([]entry, error) {
	cmd := exec.Command("git", "blame", "--line-porcelain", "--", filepath.Join(memDir, projectFile))
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git blame failed — is this a git repo with %s committed?", filepath.Join(memDir, projectFile))
	}
	return parseBlame(string(out)), nil
}

// parseBlame reads git's line-porcelain format: a header line per commit,
// then key/value lines, then the file line itself prefixed with a tab.
func parseBlame(out string) []entry {
	var entries []entry
	var author, section string
	var when time.Time
	committed := true

	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "author "):
			author = strings.TrimSpace(line[len("author "):])
		case strings.HasPrefix(line, "author-time "):
			if sec, err := strconv.ParseInt(strings.TrimSpace(line[len("author-time "):]), 10, 64); err == nil {
				when = time.Unix(sec, 0)
			}
		case strings.HasPrefix(line, "\t"):
			content := strings.TrimRight(line[1:], "\r")
			trimmed := strings.TrimSpace(content)
			if strings.HasPrefix(trimmed, "## ") {
				section = strings.TrimSpace(trimmed[3:])
				continue
			}
			if strings.HasPrefix(trimmed, "- ") {
				entries = append(entries, entry{
					section:   section,
					text:      strings.TrimSpace(trimmed[2:]),
					author:    author,
					when:      when,
					committed: committed,
				})
			}
		default:
			// A 40-char all-zero hash heads an uncommitted (working tree) line.
			if len(line) >= 40 && strings.Trim(line[:40], "0") == "" {
				committed = false
			} else if len(line) >= 40 && isHex(line[:40]) {
				committed = true
			}
		}
	}
	return entries
}

func isHex(s string) bool {
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

func runBlame(root string, stdout io.Writer) error {
	entries, err := blameEntries(root)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return fmt.Errorf("no entries found in %s", filepath.Join(memDir, projectFile))
	}

	stale := 0
	section := ""
	for _, e := range entries {
		if e.section != section {
			section = e.section
			fmt.Fprintf(stdout, "\n%s\n", section)
		}
		mark, date := "     ", e.when.Format("2006-01-02")
		if !e.committed {
			mark, date = "  new", "uncommitted"
		} else if e.stale() {
			mark = "STALE"
			stale++
		}
		fmt.Fprintf(stdout, "  %s  %-11s  %-18s  %s\n", mark, date, truncate(e.author, 18), e.text)
	}

	fmt.Fprintf(stdout, "\n%d entries, %d older than 6 months.\n", len(entries), stale)
	if stale > 0 {
		fmt.Fprintln(stdout, "Stale entries are not automatically wrong — they are unreviewed. Check them or delete them.")
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
