# Dogfood log

Phase 3. Started **2026-08-12**. Decision due **2026-09-11**.

**Kill criterion:** if by 2026-09-11 this file does not contain three specific saves,
stop building. Don't rationalise, don't add features to fix the feeling.

**Build nothing during this phase.** Bugs that break the loop are fair game. Features are
not, however obvious they seem — write them in Parked instead.

---

## Saves

A save is: **Claude used a fact from `project.md` that you would otherwise have typed
out.** Not "the memory looked useful" — you have to be able to name the sentence you
didn't have to write.

Format: date, what it knew, what you'd have had to explain.

- 2026-08-16 — knew that every command takes its `io.Writer` as an argument so it stays
  testable, and wrote the new `init` PATH check as `checkPath(stdout io.Writer)` on that
  basis. The surrounding code in `init.go` prints with bare `fmt.Println`; matching it
  would have produced an untestable function, and I'd have had to explain the convention
  and ask for the rewrite. `TestCheckPath` exists because of one line in `project.md`.

## Noise

Every proposal you rejected in `memrato review`, and why. This is the raw material for
improving `distillPrompt` — it is the only way to find out what the distiller gets wrong.

Format: date, what it proposed, why it was wrong.

<!-- example, delete when the first real one lands
- 2026-08-14 — proposed "the auth middleware is in middleware/auth.go" — a file path,
  which the prompt already excludes. Prompt is not being followed, or the rule is too
  buried to land.
-->

## Parked

Features you wanted mid-phase. Do not build them. Revisit on 2026-09-11 — most will look
much less urgent by then, and that is the point.

- Relax the root npm package's exact-version pin on its platform packages to `^0.2.0`, so
  a docs-only release stops republishing six binaries that differ only in the version
  string they report. Cost today is bandwidth, not correctness.

**Freeze was not held.** `status --blame`, `review --pr`, conflict detection and the
`init` PATH check all shipped between 2026-08-12 and 2026-08-16, in a phase whose first
rule was to build nothing. Noted so the 2026-09-11 decision is made knowing the tool was
moving underneath its own trial.

## Shipped

Not a changelog — only what changes how the loop gets dogfooded.

- 2026-08-16 — v0.2.2 on npm, and `npm i -g memrato` works for the first time. Every
  earlier release published some platform packages but never the root one, so the install
  line at the top of the README had never worked for anyone, including me. Dogfooding no
  longer needs a local `go build` and a `PATH` fixup.

## Weekly checkpoint

Count the saves. If week 2 ends with zero, that is information, not a reason to wait.

- [ ] Week 1 — 2026-08-19
- [ ] Week 2 — 2026-08-26
- [ ] Week 3 — 2026-09-02
- [ ] Week 4 — 2026-09-09
- [ ] Decision — 2026-09-11
