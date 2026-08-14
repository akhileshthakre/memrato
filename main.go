// Command memrato keeps a project's memory file accurate over time: it injects
// .memory/*.md at session start and proposes edits at session end.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	memDir       = ".memory"
	projectFile  = "project.md"
	localFile    = "local.md"
	proposedFile = "proposed.md"
	globalFile   = "global.md"

	// Rough estimate. Good enough to keep injection off the rails; we are not
	// shipping a tokenizer for a budget check.
	charsPerToken = 4
	defaultBudget = 4000 // tokens
	budgetEnvVar  = "MEMR_BUDGET_TOKENS"
	guardEnvVar   = "MEMR_DISTILLING"
	distillModel  = "claude-haiku-4-5-20251001"
	maxTranscript = 60000 // chars of transcript tail fed to the distiller
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "init":
		err = runInit(".")
	case "inject":
		err = runInject(".", os.Stdout, os.Stderr)
	case "distill":
		// Never returns an error: a broken distiller must not break the session.
		runDistill(os.Stdin)
	case "review":
		err = runReview(".", os.Stdin, os.Stdout, hasFlag("--pr"))
	case "status":
		if hasFlag("--blame") {
			err = runBlame(".", os.Stdout)
		} else {
			err = runStatus(".", os.Stdout)
		}
	case "-h", "--help", "help":
		usage()
	case "-v", "--version", "version":
		fmt.Println("memrato", version)
	default:
		fmt.Fprintf(os.Stderr, "memrato: unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "memrato:", err)
		os.Exit(1)
	}
}

var version = "dev" // overridden at release time via -ldflags

// hasFlag keeps the whole CLI at one switch statement. Two boolean flags do not
// justify a flag.FlagSet per subcommand, let alone a routing library.
func hasFlag(name string) bool {
	for _, a := range os.Args[2:] {
		if a == name {
			return true
		}
	}
	return false
}

func usage() {
	fmt.Fprint(os.Stderr, `memrato — auto-maintained project memory

  memrato init      scaffold .memory/ and wire the Claude Code hooks
  memrato inject    print the memory files (used by the SessionStart hook)
  memrato distill   propose additions from a transcript (SessionEnd hook)
  memrato review    step through proposals and apply the good ones
  memrato status    show what would be injected, from where, and how big

  --pr      (review) open a pull request instead of writing to your branch
  --blame   (status) show who added each entry, when, and what has gone stale
`)
}

// source is one memory file, in injection order: most stable first.
type source struct {
	label string
	path  string
}

func sources(root string) []source {
	home, err := os.UserHomeDir()
	global := ""
	if err == nil {
		global = filepath.Join(home, memDir, globalFile)
	}
	return []source{
		{"global", global},
		{"project", filepath.Join(root, memDir, projectFile)},
		{"local", filepath.Join(root, memDir, localFile)},
	}
}

// read returns the file's contents, or "" if it is missing. A missing memory
// file is not an error.
func read(path string) string {
	if path == "" {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func budgetChars() int {
	tokens := defaultBudget
	if v := os.Getenv(budgetEnvVar); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			tokens = n
		}
	}
	return tokens * charsPerToken
}

func runInject(root string, stdout, stderr io.Writer) error {
	srcs := sources(root)
	bodies := make([]string, len(srcs))
	for i, s := range srcs {
		bodies[i] = read(s.path)
	}

	if trimToBudget(bodies, budgetChars()) {
		fmt.Fprintf(stderr, "memrato: memory exceeded %d-token budget; trimmed lowest-priority sections\n", budgetChars()/charsPerToken)
	}

	out := render(srcs, bodies)
	if out == "" {
		return nil // nothing to inject; still a success
	}
	fmt.Fprint(stdout, out)
	return nil
}

func render(srcs []source, bodies []string) string {
	var b strings.Builder
	for i, s := range srcs {
		if bodies[i] == "" {
			continue
		}
		if b.Len() == 0 {
			b.WriteString("# Project memory (managed by memrato)\n\n")
		}
		fmt.Fprintf(&b, "<!-- %s: %s -->\n%s\n\n", s.label, s.path, bodies[i])
	}
	return b.String()
}

// trimToBudget drops content until it fits, lowest-priority file first and
// last-section-first within that file. Reports whether anything was dropped.
func trimToBudget(bodies []string, budget int) bool {
	trimmed := false
	for total(bodies) > budget {
		i := lastNonEmpty(bodies)
		if i < 0 {
			break // everything is empty; nothing left to give
		}
		bodies[i] = dropLastSection(bodies[i])
		trimmed = true
	}
	return trimmed
}

func total(bodies []string) int {
	n := 0
	for _, b := range bodies {
		n += len(b)
	}
	return n
}

func lastNonEmpty(bodies []string) int {
	for i := len(bodies) - 1; i >= 0; i-- {
		if bodies[i] != "" {
			return i
		}
	}
	return -1
}

// dropLastSection removes the final "## " section, or the whole body if the
// preamble is all that is left. Always shrinks, so callers terminate.
func dropLastSection(body string) string {
	if i := strings.LastIndex(body, "\n## "); i >= 0 {
		return strings.TrimSpace(body[:i])
	}
	return ""
}

func runStatus(root string, stdout io.Writer) error {
	srcs := sources(root)
	bodies := make([]string, len(srcs))
	sum := 0
	for i, s := range srcs {
		bodies[i] = read(s.path)
		sum += len(bodies[i])
	}

	budget := budgetChars()
	fmt.Fprintln(stdout, "Memory sources (injection order):")
	for i, s := range srcs {
		state := "missing"
		if bodies[i] != "" {
			state = fmt.Sprintf("%d tokens", len(bodies[i])/charsPerToken)
		} else if s.path != "" && fileExists(s.path) {
			state = "empty"
		}
		fmt.Fprintf(stdout, "  %-8s %-11s %s\n", s.label, state, s.path)
	}
	fmt.Fprintf(stdout, "\nTotal: ~%d of %d tokens", sum/charsPerToken, budget/charsPerToken)
	if sum > budget {
		fmt.Fprint(stdout, "  (OVER BUDGET — inject will trim)")
	}
	fmt.Fprintln(stdout)

	if p := filepath.Join(root, memDir, proposedFile); fileExists(p) {
		if n := len(parseProposals(read(p))); n > 0 {
			fmt.Fprintf(stdout, "\n%d proposal(s) pending. Run `memrato review`.\n", n)
		}
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
