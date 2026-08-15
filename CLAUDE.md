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
| `review.go` | Proposal parsing, conflict detection, applying entries to `project.md` |
| `blame.go` | `status --blame`: attribution and staleness, parsed out of `git blame` |
| `pr.go` | `review --pr`: branch, commit, push, open the PR |

npm distribution lives outside the Go code: `npm/bin/memrato.js` is the launcher shim and
`scripts/build-npm.mjs` generates all seven packages into `npm-dist/`. Nothing under
`npm-dist/` is committed.

Releasing is `git tag vX.Y.Z && git push --tags`; the workflow builds and publishes.
To do it by hand, `make npm VERSION=vX.Y.Z` and run the commands it prints, in order.

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
- **The platform packages are scoped, the root package is not.** npm's typosquatting
  heuristic refuses to create unscoped `memrato-win32-arm64` — E403 "Package name
  triggered spam detection". The darwin and linux names were accepted; win32 was not.
  Scoped names are exempt, so they ship as `@akhileshthakre/memrato-<platform>-<arch>`.
  The `scope` constant in `build-npm.mjs` and the `pkg` string in `npm/bin/memrato.js`
  must stay in lockstep, or the shim resolves a package that was never published.
- **Attribution and staleness are not stored in the file.** `project.md` is one fact per
  line precisely so `git blame` can answer both. Do not add timestamp or author syntax to
  entries — it would duplicate what git already knows and immediately drift.
- **`--pr` commits with a pathspec** (`git commit -- .memory/project.md`) so a user's
  unrelated work in progress never gets swept into the PR. It also must not assume the
  remote is called `origin`.
- **Conflict detection surfaces, never resolves.** Word overlap, no embeddings. Two people
  distilling contradicting facts is the case where guessing is worse than asking.
- **`npm publish` needs a `./` prefix on the directory.** `npm publish npm-dist/foo-bar`
  matches npm's `<user>/<repo>` GitHub shorthand and npm tries to clone
  `github.com/npm-dist/foo-bar` instead of reading the folder. `./npm-dist/foo-bar` works.
- **`.gitattributes` forces LF.** Without it, Windows checkouts get CRLF and the
  `testdata/` golden comparison in `distill_test.go` fails there and nowhere else.
- **Publish platform packages before the root package.** The root lists them as
  `optionalDependencies`; publishing it first leaves a window where `npm i -g memrato`
  installs a shim with no binary behind it.
- **A prerelease version needs `--tag`.** npm refuses to publish anything with a `-` in
  the version (`0.0.0-dev`, `v1.0.0-rc.1`) unless a dist-tag is given, or it would move
  `latest` to a prerelease. Both `build-npm.mjs` and the release workflow derive it.
- **Hooks run in a non-interactive shell**, so `memrato` being on an interactive `PATH`
  proves nothing. `checkPath` in `init.go` is what stops a user wiring hooks that
  silently never fire — the failure that reads as "I installed it and nothing happened".
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
