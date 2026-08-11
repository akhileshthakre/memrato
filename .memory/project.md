# Project memory

What Claude Code should know about this repo before you say a word.
Keep entries short and durable. One fact per line.

## Stack

- Go 1.25+, standard library only. No third-party dependencies.
- Single `package main` at the repo root, one file per command.

## Conventions

- Every command takes its root directory and its io.Reader/Writer as arguments, so it is testable without touching the real filesystem or stdin.
- Table-driven tests, heaviest coverage on the `.claude/settings.json` merge.
- Fixtures in `testdata/` are hand-written; real transcripts are never committed.

## Decisions

- No proxy: Claude Code's SessionStart and SessionEnd hooks cover the whole read and write path.
- No vector DB and no embeddings: the memory is one markdown file a human is willing to read.
- `distill` writes only to `proposed.md`; only `review` may touch `project.md`.
- `distill` exits 0 on every error path, because it runs on SessionEnd.

## Gotchas

- `distill` shells out to `claude`, which fires SessionEnd again; the `MEMR_DISTILLING` env guard is what stops the recursion.
- Go 1.22's linker emits binaries that will not run on current macOS (`missing LC_UUID load command`).
- `init` parses `settings.json` before writing anything, so a malformed config fails having changed nothing.
