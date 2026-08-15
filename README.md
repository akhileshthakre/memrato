# memrato

Your project's memory file, maintained for you instead of by you.

`memrato` injects `.memory/project.md` into every Claude Code session, and at the end of a
session proposes what should be added to it. You review the proposals, accept the good
ones, and commit the result. Your team's AI context gets reviewed like code, because it
is code.

No proxy, no vector database, no embeddings, no hosted service, no API key. Two Claude
Code hooks and a 300 KB binary.

## Install

```bash
npm install -g memrato
```

That's it — no Go toolchain, no `PATH` setup. npm ships a prebuilt binary for your
platform (~6 MB, and only yours) and puts `memrato` on your `PATH` where the hooks can
find it.

<details>
<summary>What that actually pulls in</summary>

`memrato` itself is a small Node shim. The binary lives in a per-platform package, and npm
downloads only the one matching your machine:

| Package | Platform |
|---|---|
| `@akhileshthakre/memrato-darwin-arm64` | macOS, Apple silicon |
| `@akhileshthakre/memrato-darwin-x64` | macOS, Intel |
| `@akhileshthakre/memrato-linux-arm64` | Linux, arm64 |
| `@akhileshthakre/memrato-linux-x64` | Linux, x86-64 |
| `@akhileshthakre/memrato-win32-arm64` | Windows, arm64 |
| `@akhileshthakre/memrato-win32-x64` | Windows, x86-64 |

They are `optionalDependencies` carrying `os` and `cpu` fields, so one ~6 MB package lands
instead of all six. **Behind an npm allowlist or proxy, allow `memrato` plus the single row
matching your platform** — the others are never requested.

The names use node's spelling (`win32`, `x64`) rather than Go's, because that is what
`process.platform` and `process.arch` return at runtime. They are scoped because npm's
spam heuristic refuses to create the unscoped `memrato-win32-arm64`.

</details>

Try it without installing anything:

```bash
npx memrato init
```

> Use `npx` to *try* it, never in a hook. `npx` re-resolves the package on every
> invocation, which would add seconds to every session start. Hooks need the global
> install.

<details>
<summary>Other ways to install</summary>

**Go:**

```bash
go install github.com/akhileshthakre/memrato@latest
```

This writes to `$(go env GOPATH)/bin`, which is **not** on `PATH` by default — and hooks
run in a non-interactive shell, so `~/.zshrc` is not enough. Put it somewhere every shell
sees:

```bash
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.zshenv
```

**Binaries:** [Releases](https://github.com/akhileshthakre/memrato/releases) — darwin,
linux and windows on amd64 and arm64.

</details>

However you install it, `memrato` has to be on your `PATH` — hooks run in a
non-interactive shell, and one that cannot find `memrato` fails silently. `memrato init`
checks this and tells you if it is wrong.

## Quick start

### 1. Wire it into a project

```bash
cd your-project
memrato init
```

```
wired SessionStart + SessionEnd hooks in .claude/settings.json
memory initialised in .memory

Next: start Claude Code and ask what it knows about this project.
```

That creates `.memory/project.md`, adds a `SessionStart` and a `SessionEnd` hook to
`.claude/settings.json`, and gitignores the two files that are personal. It never
overwrites a memory file that already has content, and running it twice is safe.

If `memrato` is not on your `PATH`, `init` prints a warning in place of that last line.
Fix that before going further — everything downstream depends on the hooks being able to
run it.

### 2. Fill in a fact or two

Open `.memory/project.md` and write something under a heading. One fact per line, no file
paths, nothing that will be false next month:

```markdown
## Gotchas

- The seed script must run before the integration tests or three of them fail.
```

You can leave it empty and let the distiller propose everything, but one real entry makes
step 3 much easier to believe.

### 3. Verify it, without starting Claude Code

```bash
memrato status
```

```
Memory sources (injection order):
  global   missing     /Users/you/.memory/global.md
  project  87 tokens   .memory/project.md
  local    22 tokens   .memory/local.md

Total: ~109 of 4000 tokens
```

`missing` for `global` is normal — that file is optional and personal. What matters is
that `project` reports a token count rather than `missing`.

```bash
memrato inject
```

This prints the exact text Claude Code receives at session start, because the
`SessionStart` hook runs this same command and captures its stdout:

```
# Project memory (managed by memrato)

<!-- project: .memory/project.md -->
# Project memory
...
## Gotchas

- The seed script must run before the integration tests or three of them fail.
```

If `inject` prints your memory here, the hook will inject it too.

### 4. Verify it inside Claude Code

Start Claude Code in the project and ask it something only your memory file knows:

> What do you know about running the integration tests here?

It should answer from your entry without reading a single file. If it does not, see
[Troubleshooting](#troubleshooting).

### 5. Review what it proposes

When the session ends, the `SessionEnd` hook distills the transcript into
`.memory/proposed.md`. Nothing is written to `project.md` automatically — ever. Step
through the proposals yourself:

```bash
memrato review
```

```
[1/2] Gotchas (confidence 0.90)
  The staging deploy needs CGO_ENABLED=0 or the container will not start
  [y]es / [n]o / [e]dit / [q]uit: y

[2/2] Stack (confidence 0.75)
  Postgres 16 is the datastore, accessed with hand-written SQL
  ! may conflict with an existing entry:
      Postgres 16 is the datastore; there is no ORM, queries are hand-written SQL.
  [y]es / [n]o / [e]dit / [q]uit: n

Applied 1, discarded 1, left 0 pending.
Review the diff and commit .memory/project.md
```

`y` — or just Enter — accepts. `n` discards, `e` lets you retype the entry before
accepting, `q` stops and leaves everything from there on pending for next time. Accepted
entries land in `.memory/project.md`:

```bash
git diff .memory/project.md
git add .memory/project.md && git commit -m "memory: staging deploy needs CGO_ENABLED=0"
```

Commit it and your whole team gets the context on their next session.

## Commands

| Command | What it does |
|---|---|
| `memrato init` | Creates `.memory/`, wires the hooks, updates `.gitignore`. Idempotent. |
| `memrato inject` | Prints the memory files. Runs automatically on `SessionStart`. |
| `memrato distill` | Proposes additions from a transcript. Runs automatically on `SessionEnd`. |
| `memrato review` | Steps through proposals and applies the ones you accept. |
| `memrato status` | Shows what would be injected, from where, and how many tokens. |
| `memrato status --blame` | Who added each entry, when, and what has gone stale. |
| `memrato review --pr` | Puts accepted entries on a branch and opens a PR instead of committing to yours. |

## Troubleshooting

**Claude Code does not know anything from the file.** Check the three things in order:

```bash
memrato --version              # 1. is the binary reachable at all?
memrato inject                 # 2. does it produce the memory?
cat .claude/settings.json      # 3. are the hooks actually wired?
```

You should see a `SessionStart` entry running `memrato inject` and a `SessionEnd` entry
running `memrato distill`. If `--version` works in your terminal but the hooks still do
nothing, `memrato` is on your interactive `PATH` only — hooks do not read `~/.zshrc`. Put
it in `~/.zshenv` or install globally with npm. Restart Claude Code after any change to
`settings.json`.

**`.memory/proposed.md` never appears.** The distiller needs a session with enough content
to be worth distilling, and it exits quietly on every error path by design — a broken
distiller must never break your session. Transcripts are also written asynchronously, so
the last turn of a session sometimes misses it. Try a longer session before assuming it is
broken.

**`memrato review` says "Nothing to review."** That means `proposed.md` is absent or has no
unreviewed entries. See above.

## For teams

Shared memory rots faster than personal memory, because nobody owns it. Three things
help, and none of them need a database.

**See who wrote what, and what has gone stale.** Entries are one fact per line, so `git`
already knows who added each one and when — nothing is stored twice and nothing can drift
out of sync:

```bash
memrato status --blame
```

```
Gotchas
         2026-08-12   Grace Hopper        CI needs CGO_ENABLED=0 or the container will not start
  STALE  2025-01-03   Ada Lovelace        Deploys go out through Heroku
```

Stale entries are not automatically wrong — they are *unreviewed*. Six months is the
threshold. Check them or delete them.

**Review memory like code.** `--pr` puts accepted entries on their own branch and opens a
pull request, so a fact reaches everyone's context after a review rather than because one
person pressed `y`:

```bash
memrato review --pr
```

It commits `.memory/project.md` and nothing else, so unrelated work in progress is never
swept in. If `gh` is installed it opens the PR; otherwise it prints the URL.

**Conflicts get surfaced, never resolved.** When two people distill contradicting facts,
`review` shows you the existing entry next to the new one before you answer:

```
[1/2] Stack (confidence 0.80)
  Deploys run on Fly.io, not Vercel
  ! may conflict with an existing entry:
      Deploys run on Vercel
  [y]es / [n]o / [e]dit / [q]uit:
```

Detection is word overlap — no embeddings, no model call. It only has to be good enough to
make a human look.

## Files

```
your-project/
  .memory/
    project.md    # committed. team-shared. the thing being maintained.
    local.md      # gitignored. your personal notes about this repo.
    proposed.md   # gitignored. pending proposals awaiting review.
  .claude/
    settings.json # committed. wires the hooks for everyone.

~/.memory/
  global.md       # personal, cross-project. "I prefer Go. Be terse."
```

Injection order is `global.md`, `project.md`, `local.md` — most stable content first.

## Limitations

Read these before you install. They are real, not hypothetical.

- **Claude Code only.** It works because Claude Code has a hook system. Cursor, Codex CLI
  and everything else are not supported yet. `memrato inject | pbcopy` is the fallback.
- **Distillation costs one model call per session.** It runs on Haiku via your existing
  Claude Code auth (or `ANTHROPIC_API_KEY` if the `claude` binary is not on your PATH).
  Cheap, but not free, and it happens on every session end.
- **The distiller proposes noise sometimes.** That is why nothing is ever written to
  `project.md` automatically. Every entry passes through `memrato review` and then through
  your normal commit review. Both gates are deliberate.
- **The npm install adds ~70 ms to session start.** `memrato` is a Node shim that execs
  the real Go binary, and Node's startup floor is ~60 ms. Measured, not guessed. Once per
  session, so you will not notice it — but if you want the ~0 ms path, install with Go and
  skip the shim.
- **The token budget is estimated, not tokenized.** 4 characters per token. Set
  `MEMR_BUDGET_TOKENS` to change the default of 4000. Over budget, the lowest-priority
  file is trimmed last-section-first.
- **Attribution is `git blame`, with git's semantics.** `status --blame` needs
  `.memory/project.md` committed, and rewording a line reassigns it to whoever touched it
  last. The six-month staleness threshold is not configurable.
- **Transcripts are written asynchronously**, so the final turn of a session occasionally
  misses the distiller.

## Contributing

The distiller prompt in [`distill.go`](distill.go) is the highest-leverage thing in this
repo and the best place to start — it decides whether the whole tool is useful or
annoying. Issues labelled `good-first-issue` are self-contained and do not require
understanding the whole codebase.

```bash
make test
```

## Prior art

[mem0](https://github.com/mem0ai/mem0) and MemPalace solve a broader problem — durable
memory for agents in general, with vector search and cross-session recall. If you want
semantic retrieval over a large memory corpus, use those; they are much more capable than
this is.

`memrato` deliberately does one narrow thing instead: keep one committed markdown file
accurate, with a human in the loop on every change. If your memory does not fit in a file
a person is willing to read, this is the wrong tool.

## License

MIT.
