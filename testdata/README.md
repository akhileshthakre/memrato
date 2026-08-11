# testdata

Fixtures for iterating on the distiller prompt.

`session.jsonl` is a hand-written transcript in Claude Code's format.
`session.expected.txt` is exactly what `readTranscript` should hand the model —
prose only, with thinking blocks, tool calls and tool results stripped out.

**Never commit a real transcript here.** They contain absolute paths, environment
variables and sometimes live API keys. Write fixtures by hand.

Adding a case: append entries to `session.jsonl`, run `go test ./...`, and update
`session.expected.txt` if the new output is what you intended.
