package main

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func plural(n int) string {
	if n == 1 {
		return "1 entry"
	}
	return fmt.Sprintf("%d entries", n)
}

func git(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// openPR puts the accepted entries on their own branch and opens a pull
// request, so team memory is reviewed like code instead of landing on main
// because one person pressed y.
//
// It commits only .memory/project.md — anything else in the working tree is
// left alone, and the branch stays checked out so nothing looks like it
// vanished.
func openPR(root string, count int, stdout io.Writer) error {
	memoryPath := filepath.Join(memDir, projectFile)

	original, err := git(root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("not a git repository — drop --pr and commit %s yourself", memoryPath)
	}

	branch := "memrato/review-" + time.Now().Format("20060102-150405")
	if _, err := git(root, "checkout", "-b", branch); err != nil {
		return err
	}
	// Pathspec commit: stages and commits this file only, whatever else is
	// dirty or already staged.
	if _, err := git(root, "commit", "-m",
		fmt.Sprintf("memory: %s from review", plural(count)), "--", memoryPath); err != nil {
		_, _ = git(root, "checkout", original)
		_, _ = git(root, "branch", "-D", branch)
		return fmt.Errorf("commit failed (is git user.name/user.email set?) — returned to %s", original)
	}

	remote, err := defaultRemote(root)
	if err != nil {
		fmt.Fprintf(stdout, "\nCommitted %s to %s. No git remote, so nothing was pushed.\n", plural(count), branch)
		return nil
	}
	if _, err := git(root, "push", "-u", remote, branch); err != nil {
		fmt.Fprintf(stdout, "\nCommitted to %s, but the push failed. Push it yourself:\n  git push -u %s %s\n", branch, remote, branch)
		return nil
	}

	fmt.Fprintf(stdout, "\nPushed %s to %s.\n", plural(count), branch)
	if out, err := gh(root, branch, count); err == nil {
		fmt.Fprintln(stdout, out)
	} else if url, err := compareURL(root, remote, branch); err == nil {
		fmt.Fprintln(stdout, "Open the PR:\n  "+url)
	}
	fmt.Fprintf(stdout, "You are on %s. Back to your work with:\n  git checkout %s\n", branch, original)
	return nil
}

// defaultRemote does not assume "origin" — plenty of repos name it something else.
func defaultRemote(root string) (string, error) {
	out, err := git(root, "remote")
	if err != nil || out == "" {
		return "", fmt.Errorf("no git remote configured")
	}
	remotes := strings.Fields(out)
	for _, r := range remotes {
		if r == "origin" {
			return r, nil
		}
	}
	return remotes[0], nil
}

func gh(root, branch string, count int) (string, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return "", err
	}
	cmd := exec.Command("gh", "pr", "create",
		"--title", fmt.Sprintf("memory: %s from review", plural(count)),
		"--body", "Proposed by `memrato distill`, accepted in `memrato review`.\n\nEach line is one durable fact. Reject anything that is not true, not durable, or not worth a future reader's attention.",
		"--head", branch)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return "Opened " + strings.TrimSpace(string(out)), nil
}

// compareURL turns a git remote into the browser URL for opening a PR, for the
// common case where gh is not installed.
func compareURL(root, remote, branch string) (string, error) {
	raw, err := git(root, "remote", "get-url", remote)
	if err != nil {
		return "", err
	}
	url := strings.TrimSuffix(raw, ".git")
	url = strings.TrimPrefix(url, "ssh://")
	if strings.HasPrefix(url, "git@") { // git@github.com:user/repo
		url = "https://" + strings.Replace(strings.TrimPrefix(url, "git@"), ":", "/", 1)
	}
	if !strings.HasPrefix(url, "http") {
		return "", fmt.Errorf("unrecognised remote %q", raw)
	}
	return url + "/compare/" + branch + "?expand=1", nil
}
