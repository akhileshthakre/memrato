#!/usr/bin/env node
// Launcher for the platform-specific binary. npm installed exactly one of the
// optionalDependencies — the one matching this machine's os/cpu — and this
// hands off to it.
//
// stdio is inherited rather than piped, and that is load-bearing:
//   inject   writes the memory file to stdout, which Claude Code captures
//   distill  reads the hook payload as JSON on stdin
//   review   is interactive on both
"use strict";

const { spawnSync } = require("child_process");
const path = require("path");

const pkg = `memrato-${process.platform}-${process.arch}`;
const exe = process.platform === "win32" ? "memrato.exe" : "memrato";

let binary;
try {
  // Resolve package.json rather than the binary itself: it is the one path an
  // "exports" field can never block.
  binary = path.join(path.dirname(require.resolve(`${pkg}/package.json`)), "bin", exe);
} catch {
  console.error(
    `memrato: no prebuilt binary for ${process.platform}-${process.arch}.\n` +
      `Install it with Go instead:\n` +
      `  go install github.com/akhileshthakre/memrato@latest\n` +
      `or open an issue at https://github.com/akhileshthakre/memrato/issues`
  );
  process.exit(1);
}

const result = spawnSync(binary, process.argv.slice(2), { stdio: "inherit" });

if (result.error) {
  console.error(`memrato: could not run ${binary}: ${result.error.message}`);
  process.exit(1);
}
// status is null when the child was killed by a signal.
process.exit(result.status === null ? 1 : result.status);
