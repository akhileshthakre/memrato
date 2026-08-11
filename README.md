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

Verify with `memrato --version` in a new terminal. If that prints nothing, the hooks will
not find it either.

## 30-second demo

```bash
cd your-project
memrato init
```

That creates `.memory/project.md` and wires two hooks into `.claude/settings.json`.
Now start Claude Code and ask it what it knows about the project — it answers from your file.

Work for a while, quit, then:

```bash
memrato review
```

```
[1/3] Gotchas (confidence 0.90)
  The staging deploy needs CGO_ENABLED=0 or the container will not start.
  [y]es / [n]o / [e]dit / [q]uit: y
```

Accepted entries land in `.memory/project.md`. Diff it, commit it, and your whole team
gets the context on their next session.

## Commands

| Command | What it does |
|---|---|
| `memrato init` | Creates `.memory/`, wires the hooks, updates `.gitignore`. Idempotent. |
| `memrato inject` | Prints the memory files. Runs automatically on `SessionStart`. |
| `memrato distill` | Proposes additions from a transcript. Runs automatically on `SessionEnd`. |
| `memrato review` | Steps through proposals and applies the ones you accept. |
| `memrato status` | Shows what would be injected, from where, and how many tokens. |

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
- **No staleness tracking or attribution yet.** Entries are line-oriented so `git blame`
  answers "who added this and when" today. Anything richer is not built.
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
