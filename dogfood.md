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

<!-- example, delete when the first real one lands
- 2026-08-14 — knew the seed script must run before integration tests, so it ran it
  unprompted instead of me debugging three failing tests and explaining why.
-->

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

## Weekly checkpoint

Count the saves. If week 2 ends with zero, that is information, not a reason to wait.

- [ ] Week 1 — 2026-08-19
- [ ] Week 2 — 2026-08-26
- [ ] Week 3 — 2026-09-02
- [ ] Week 4 — 2026-09-09
- [ ] Decision — 2026-09-11
