package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const projectTemplate = `# Project memory

What Claude Code should know about this repo before you say a word.
Keep entries short and durable. One fact per line.

## Stack

## Conventions

## Decisions

## Gotchas
`

const localTemplate = `# Local memory

Personal notes about this repo. Gitignored — never shared with the team.
`

// hookWiring is the set of hooks memr installs. Matcher is omitted so the hook
// fires for every source/reason (startup, resume, clear, compact, fork).
var hookWiring = []struct{ event, command string }{
	{"SessionStart", "memr inject"},
	{"SessionEnd", "memr distill"},
}

func runInit(root string) error {
	// Parse settings.json BEFORE writing anything. If it is malformed we must
	// fail having changed nothing at all.
	settingsPath := filepath.Join(root, ".claude", "settings.json")
	settings, err := loadSettings(settingsPath)
	if err != nil {
		return err
	}
	changed, err := wireHooks(settings)
	if err != nil {
		return err
	}

	dir := filepath.Join(root, memDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := seed(filepath.Join(dir, projectFile), projectTemplate); err != nil {
		return err
	}
	if err := seed(filepath.Join(dir, localFile), localTemplate); err != nil {
		return err
	}
	if err := addGitignore(root, memDir+"/"+localFile, memDir+"/"+proposedFile); err != nil {
		return err
	}

	if changed {
		if err := writeSettings(settingsPath, settings); err != nil {
			return err
		}
		fmt.Println("wired SessionStart + SessionEnd hooks in", settingsPath)
	} else {
		fmt.Println("hooks already wired in", settingsPath)
	}
	fmt.Println("memory initialised in", dir)
	fmt.Println("\nNext: make sure `memr` is on your PATH, then start Claude Code and ask what it knows about this project.")
	return nil
}

// seed writes content only if the file is absent or empty. Never clobbers.
func seed(path, content string) error {
	if b, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(b))) > 0 {
		return nil
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// loadSettings reads settings.json into a generic map so every key we do not
// understand survives the round trip. A missing or blank file is an empty
// config; anything else that will not parse is a hard error.
func loadSettings(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return map[string]any{}, nil
	}
	var settings map[string]any
	if err := json.Unmarshal(b, &settings); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON (%v) — fix it by hand; nothing was changed", path, err)
	}
	return settings, nil
}

// wireHooks adds our hook entries if absent. Returns whether it changed
// anything. Existing hooks and unrelated keys are left untouched.
func wireHooks(settings map[string]any) (bool, error) {
	hooks, err := childMap(settings, "hooks")
	if err != nil {
		return false, err
	}

	changed := false
	for _, w := range hookWiring {
		entries, ok := hooks[w.event].([]any)
		if !ok && hooks[w.event] != nil {
			return false, fmt.Errorf("settings.json: hooks.%s is not a list — fix it by hand; nothing was changed", w.event)
		}
		if hasCommand(entries, w.command) {
			continue
		}
		hooks[w.event] = append(entries, map[string]any{
			"hooks": []any{map[string]any{"type": "command", "command": w.command}},
		})
		changed = true
	}
	if changed {
		settings["hooks"] = hooks
	}
	return changed, nil
}

func childMap(parent map[string]any, key string) (map[string]any, error) {
	switch v := parent[key].(type) {
	case nil:
		return map[string]any{}, nil
	case map[string]any:
		return v, nil
	default:
		return nil, fmt.Errorf("settings.json: %q is not an object — fix it by hand; nothing was changed", key)
	}
}

// hasCommand reports whether any hook under these entries already runs cmd.
func hasCommand(entries []any, cmd string) bool {
	for _, e := range entries {
		entry, ok := e.(map[string]any)
		if !ok {
			continue
		}
		inner, ok := entry["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range inner {
			hook, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if s, ok := hook["command"].(string); ok && strings.Contains(s, cmd) {
				return true
			}
		}
	}
	return false
}

func writeSettings(path string, settings map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// addGitignore appends any missing entries, idempotently.
func addGitignore(root string, entries ...string) error {
	path := filepath.Join(root, ".gitignore")
	b, _ := os.ReadFile(path)
	existing := map[string]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		existing[strings.TrimSpace(line)] = true
	}

	var add []string
	for _, e := range entries {
		if !existing[e] {
			add = append(add, e)
		}
	}
	if len(add) == 0 {
		return nil
	}

	out := string(b)
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return os.WriteFile(path, []byte(out+strings.Join(add, "\n")+"\n"), 0o644)
}
