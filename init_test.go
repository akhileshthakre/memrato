package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Mangling a user's Claude Code config is the worst bug this tool can have,
// so this is the most thorough table in the repo.
func TestInitSettings(t *testing.T) {
	tests := []struct {
		name     string
		settings string // "" means no .claude/settings.json at all
		wantErr  bool
		check    func(t *testing.T, root string, before string)
	}{
		{
			name: "fresh repo",
			check: func(t *testing.T, root, _ string) {
				s := readSettings(t, root)
				assertHook(t, s, "SessionStart", "memr inject")
				assertHook(t, s, "SessionEnd", "memr distill")
			},
		},
		{
			name:     "blank settings file",
			settings: "   \n",
			check: func(t *testing.T, root, _ string) {
				assertHook(t, readSettings(t, root), "SessionStart", "memr inject")
			},
		},
		{
			name: "preserves unrelated keys and other hooks",
			settings: `{
			  "model": "opus",
			  "permissions": {"allow": ["Bash(ls)"]},
			  "hooks": {
			    "PreToolUse": [{"matcher":"Bash","hooks":[{"type":"command","command":"rtk hook claude"}]}],
			    "SessionStart": [{"hooks":[{"type":"command","command":"echo hi"}]}]
			  }
			}`,
			check: func(t *testing.T, root, _ string) {
				s := readSettings(t, root)
				if s["model"] != "opus" {
					t.Error("dropped the model key")
				}
				if _, ok := s["permissions"]; !ok {
					t.Error("dropped the permissions key")
				}
				hooks := s["hooks"].(map[string]any)
				if _, ok := hooks["PreToolUse"]; !ok {
					t.Error("dropped the PreToolUse hook")
				}
				if n := len(hooks["SessionStart"].([]any)); n != 2 {
					t.Errorf("SessionStart entries = %d, want 2 (theirs + ours)", n)
				}
				assertHook(t, s, "SessionStart", "echo hi")
				assertHook(t, s, "SessionStart", "memr inject")
			},
		},
		{
			name:     "malformed settings changes nothing",
			settings: `{"hooks": {`,
			wantErr:  true,
			check: func(t *testing.T, root, before string) {
				if got := mustRead(t, filepath.Join(root, ".claude", "settings.json")); got != before {
					t.Errorf("settings.json was modified:\n%s", got)
				}
				if _, err := os.Stat(filepath.Join(root, memDir)); err == nil {
					t.Error(".memory/ was created despite the hard failure")
				}
			},
		},
		{
			name:     "hooks key of the wrong type is a hard failure",
			settings: `{"hooks": "nope"}`,
			wantErr:  true,
			check: func(t *testing.T, root, before string) {
				if got := mustRead(t, filepath.Join(root, ".claude", "settings.json")); got != before {
					t.Error("settings.json was modified")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if tt.settings != "" {
				mkdir(t, filepath.Join(root, ".claude"))
				write(t, filepath.Join(root, ".claude", "settings.json"), tt.settings)
			}

			err := runInit(root)
			if (err != nil) != tt.wantErr {
				t.Fatalf("runInit err = %v, wantErr %v", err, tt.wantErr)
			}
			tt.check(t, root, tt.settings)
		})
	}
}

func TestInitIsIdempotent(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 3; i++ {
		if err := runInit(root); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}

	hooks := readSettings(t, root)["hooks"].(map[string]any)
	for _, event := range []string{"SessionStart", "SessionEnd"} {
		if n := len(hooks[event].([]any)); n != 1 {
			t.Errorf("%s entries = %d after 3 inits, want 1", event, n)
		}
	}
	if got := strings.Count(mustRead(t, filepath.Join(root, ".gitignore")), memDir+"/"+localFile); got != 1 {
		t.Errorf(".gitignore has %d copies of the local.md entry, want 1", got)
	}
}

func TestInitNeverClobbersExistingMemory(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, memDir))
	write(t, filepath.Join(root, memDir, projectFile), "# mine\n\n## Stack\n\n- Go\n")

	if err := runInit(root); err != nil {
		t.Fatal(err)
	}
	if got := mustRead(t, filepath.Join(root, memDir, projectFile)); !strings.Contains(got, "- Go") {
		t.Errorf("existing project.md was overwritten:\n%s", got)
	}
}

func TestGitignoreAppendsToExistingFile(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".gitignore"), "node_modules") // no trailing newline

	if err := addGitignore(root, "a.md", "b.md"); err != nil {
		t.Fatal(err)
	}
	want := "node_modules\na.md\nb.md\n"
	if got := mustRead(t, filepath.Join(root, ".gitignore")); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// helpers

func readSettings(t *testing.T, root string) map[string]any {
	t.Helper()
	var s map[string]any
	if err := json.Unmarshal([]byte(mustRead(t, filepath.Join(root, ".claude", "settings.json"))), &s); err != nil {
		t.Fatalf("settings.json is not valid JSON after init: %v", err)
	}
	return s
}

func assertHook(t *testing.T, settings map[string]any, event, cmd string) {
	t.Helper()
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("no hooks object in settings")
	}
	entries, _ := hooks[event].([]any)
	if !hasCommand(entries, cmd) {
		t.Errorf("%s is missing the %q hook", event, cmd)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
