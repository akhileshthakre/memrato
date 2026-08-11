# memrato

A CLI that keeps `.memory/project.md` accurate: injects it at session start, proposes
edits at session end that a human reviews and commits.

## Hard constraints

These are decisions, not preferences. Do not relitigate them in a PR.

- **No proxy.** Claude Code's `SessionStart` (stdout is injected as context) and
  `SessionEnd` (`transcript_path` on stdin) hooks cover the entire read and write path.
  No traffic interception, no credential custody.
- **No vector DB, no embeddings, no semantic search.** The memory is one markdown file a
  human is willing to read. If it outgrows that, the answer is to write less, not to add
  retrieval.
- **`distill` never writes to `project.md`.** It only ever writes `proposed.md`. The only
  thing that touches `project.md` is `review`, driven by a human keystroke.
- **`distill` never fails loudly.** It runs on `SessionEnd`; a broken distiller must not
  break the user's session. Every error path returns and exits 0.
- **Zero third-party dependencies.** stdlib covers JSON, HTTP, file IO and subprocesses.
  A new dependency needs a real argument, not a convenience one.
- **No config file format, no plugin system, no daemon, no logging library.** Environment
  variables where a knob is genuinely needed (`MEMR_BUDGET_TOKENS`).

## Layout

One file per command, all `package main` at the repo root.

| File | Contains |
|---|---|
| `main.go` | Routing, the memory sources, `inject`, `status`, budget trimming |
| `init.go` | Scaffolding and the `.claude/settings.json` merge |
| `distill.go` | Transcript parsing, the distiller prompt, the model call |
| `review.go` | Proposal parsing and applying entries to `project.md` |

npm distribution lives outside the Go code: `npm/bin/memrato.js` is the launcher shim and
`scripts/build-npm.mjs` generates all seven packages into `npm-dist/`. Nothing under
`npm-dist/` is committed.

## Gotchas

- **`distill` shells out to `claude`, which fires `SessionEnd`, which runs `distill`.**
  The `MEMR_DISTILLING` env guard is the only thing stopping an unbounded fork bomb.
  Do not remove it, and set it on any new subprocess that could re-enter Claude Code.
- **Go 1.25+ is required.** The Go 1.22 linker emits binaries that will not run on current
  macOS (`missing LC_UUID load command`).
- **`settings.json` is parsed before anything is written.** If it is malformed, `init`
  must fail having changed nothing on disk — the ordering in `runInit` is load-bearing.
  It is parsed into `map[string]any` so unknown keys survive the round trip.
- **node and Go disagree on platform names.** node says `win32`/`x64`, Go says
  `windows`/`amd64`. The mapping table in `scripts/build-npm.mjs` is the only place that
  knows both; package names use the *node* spelling because that is what
  `process.platform`/`process.arch` produce at runtime in the shim.
- **Publish platform packages before the root package.** The root lists them as
  `optionalDependencies`; publishing it first leaves a window where `npm i -g memrato`
  installs a shim with no binary behind it.
- **The shim must use `stdio: "inherit"`.** `inject` writes to stdout for Claude Code to
  capture, `distill` reads the hook payload on stdin, and `review` is interactive. Piping
  instead of inheriting breaks all three.
- **Never commit a real transcript to `testdata/`.** They contain absolute paths,
  environment variables and, in at least one observed case, live API keys. Fixtures are
  written by hand.

## The distiller prompt

The `distillPrompt` constant in `distill.go` decides whether this tool is useful or
annoying. It is the highest-leverage string in the repo. Change it deliberately, and check
the change against `testdata/` before shipping.

## Testing

`make test`. The `settings.json` merge has the most thorough table in the repo, because
mangling a user's Claude Code config is the worst bug this tool can have.
