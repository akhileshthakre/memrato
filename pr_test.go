package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newRepo builds a throwaway git repo with a committed memory file, plus a bare
// repo as its remote so pushes are real without touching anything of the user's.
func newRepo(t *testing.T, remoteName string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	bare := t.TempDir()

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run(bare, "init", "--bare", "-q")
	run(root, "init", "-q")
	run(root, "config", "user.email", "test@example.com")
	run(root, "config", "user.name", "Test User")
	run(root, "config", "commit.gpgsign", "false")
	run(root, "checkout", "-q", "-b", "trunk")

	mkdir(t, filepath.Join(root, memDir))
	write(t, filepath.Join(root, memDir, projectFile), "# M\n\n## Stack\n- Go\n")
	run(root, "add", ".")
	run(root, "commit", "-q", "-m", "initial")
	run(root, "remote", "add", remoteName, bare)
	return root
}

func gitOut(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := git(root, args...)
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return out
}

func TestOpenPRCommitsAndPushes(t *testing.T) {
	// Deliberately not named "origin": plenty of repos are not.
	root := newRepo(t, "upstream")
	write(t, filepath.Join(root, memDir, projectFile), "# M\n\n## Stack\n- Go\n- Redis\n")

	var out bytes.Buffer
	if err := openPR(root, 1, &out); err != nil {
		t.Fatal(err)
	}

	branch := gitOut(t, root, "rev-parse", "--abbrev-ref", "HEAD")
	if !strings.HasPrefix(branch, "memrato/review-") {
		t.Errorf("on branch %q, want a memrato/review- branch", branch)
	}
	if body := gitOut(t, root, "show", "--stat", "--oneline", "HEAD"); !strings.Contains(body, projectFile) {
		t.Errorf("commit does not touch %s:\n%s", projectFile, body)
	}
	if remote := gitOut(t, root, "ls-remote", "--heads", "upstream", branch); !strings.Contains(remote, branch) {
		t.Errorf("branch was not pushed to the remote")
	}
	if s := out.String(); !strings.Contains(s, "git checkout trunk") {
		t.Errorf("did not tell the user how to get back to their branch:\n%s", s)
	}
}

// Only the memory file goes into the PR — unrelated work in progress must not
// be swept into it.
func TestOpenPRCommitsOnlyTheMemoryFile(t *testing.T) {
	root := newRepo(t, "origin")
	write(t, filepath.Join(root, memDir, projectFile), "# M\n\n## Stack\n- Go\n- Redis\n")
	write(t, filepath.Join(root, "unrelated.txt"), "work in progress")

	var out bytes.Buffer
	if err := openPR(root, 1, &out); err != nil {
		t.Fatal(err)
	}

	files := gitOut(t, root, "show", "--name-only", "--format=", "HEAD")
	if strings.Contains(files, "unrelated.txt") {
		t.Errorf("swept an unrelated file into the commit:\n%s", files)
	}
	if status := gitOut(t, root, "status", "--porcelain"); !strings.Contains(status, "unrelated.txt") {
		t.Errorf("unrelated work should still be uncommitted, got:\n%s", status)
	}
}

func TestOpenPROutsideGitRepo(t *testing.T) {
	var out bytes.Buffer
	if err := openPR(t.TempDir(), 1, &out); err == nil {
		t.Error("expected an error outside a git repo")
	}
}

func TestCompareURL(t *testing.T) {
	root := newRepo(t, "origin")
	for _, tt := range []struct{ remote, want string }{
		{"https://github.com/u/r.git", "https://github.com/u/r/compare/b?expand=1"},
		{"https://github.com/u/r", "https://github.com/u/r/compare/b?expand=1"},
		{"git@github.com:u/r.git", "https://github.com/u/r/compare/b?expand=1"},
	} {
		if _, err := git(root, "remote", "set-url", "origin", tt.remote); err != nil {
			t.Fatal(err)
		}
		got, err := compareURL(root, "origin", "b")
		if err != nil {
			t.Fatalf("%s: %v", tt.remote, err)
		}
		if got != tt.want {
			t.Errorf("%s -> %s, want %s", tt.remote, got, tt.want)
		}
	}
}
