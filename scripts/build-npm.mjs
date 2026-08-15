#!/usr/bin/env node
// Cross-compiles the Go binary for every supported platform and lays out the
// npm packages under npm-dist/.
//
// One root package (memrato) declares the six platform packages as
// optionalDependencies with os/cpu fields, so npm downloads only the ~6 MB that
// matches the machine instead of all 35 MB.
//
// Usage: node scripts/build-npm.mjs [version]
import { execFileSync } from "node:child_process";
import { mkdirSync, copyFileSync, writeFileSync, rmSync, chmodSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(dirname(fileURLToPath(import.meta.url)));
const outDir = join(root, "npm-dist");

// node's process.platform/process.arch on the left, Go's GOOS/GOARCH on the
// right. They disagree on two names, and mixing them up is the classic bug in
// this pattern: node says win32/x64, Go says windows/amd64.
const targets = [
  { platform: "darwin", arch: "arm64", goos: "darwin", goarch: "arm64" },
  { platform: "darwin", arch: "x64", goos: "darwin", goarch: "amd64" },
  { platform: "linux", arch: "arm64", goos: "linux", goarch: "arm64" },
  { platform: "linux", arch: "x64", goos: "linux", goarch: "amd64" },
  { platform: "win32", arch: "x64", goos: "windows", goarch: "amd64" },
  { platform: "win32", arch: "arm64", goos: "windows", goarch: "arm64" },
];

const version = (process.argv[2] || "0.0.0-dev").replace(/^v/, "");

// npm refuses to create the unscoped name `memrato-win32-arm64` in the global
// namespace — E403 "Package name triggered spam detection", its typosquatting
// heuristic. The darwin and linux names went through; win32 did not. Scoped
// names live in the publisher's own namespace and are not subject to it, which
// is why esbuild, swc and rollup all ship their platform binaries this way.
//
// The root package stays unscoped: it is the name people type.
const scope = "@akhileshthakre";

// The root package depends on its platform packages by range rather than exact
// version, so a docs-only release can republish the root alone and still resolve
// binaries from the previous one instead of shipping six identical rebuilds.
//
// Floored at <major>.<minor>.0 and derived from the version being built — never
// hardcoded. `^0.2.0` frozen in the source would still be asking for 0.2.x
// binaries at v0.3.0, which is a stale binary rather than a failed install, so
// nothing would catch it. Flooring at the exact version instead would exclude
// the very packages the range exists to reuse.
//
// Prereleases pin exactly: a semver range does not match a prerelease, so
// ^0.3.0 would fail to resolve 0.3.0-rc.1.
const [major, minor] = version.split(".");
const depRange = version.includes("-") ? version : `^${major}.${minor}.0`;

const shared = {
  version,
  license: "MIT",
  homepage: "https://github.com/akhileshthakre/memrato",
  repository: { type: "git", url: "git+https://github.com/akhileshthakre/memrato.git" },
  bugs: { url: "https://github.com/akhileshthakre/memrato/issues" },
  author: "Akhilesh Thakre",
};

rmSync(outDir, { recursive: true, force: true });

const optionalDependencies = {};
const dirs = [];
for (const t of targets) {
  // The directory stays unscoped and flat — only the package name is scoped.
  const dir = `memrato-${t.platform}-${t.arch}`;
  const name = `${scope}/${dir}`;
  const exe = t.goos === "windows" ? "memrato.exe" : "memrato";
  const pkgDir = join(outDir, dir);
  dirs.push(dir);
  mkdirSync(join(pkgDir, "bin"), { recursive: true });

  // CGO_ENABLED=0 is what makes one linux binary work on both glibc and musl,
  // so these packages need no libc field.
  execFileSync("go", ["build", "-ldflags", `-s -w -X main.version=${version}`, "-o", join(pkgDir, "bin", exe), "."], {
    cwd: root,
    stdio: "inherit",
    env: { ...process.env, GOOS: t.goos, GOARCH: t.goarch, CGO_ENABLED: "0" },
  });
  chmodSync(join(pkgDir, "bin", exe), 0o755);

  writeFileSync(
    join(pkgDir, "package.json"),
    JSON.stringify(
      {
        name,
        ...shared,
        description: `memrato binary for ${t.platform} ${t.arch}`,
        os: [t.platform],
        cpu: [t.arch],
        // No "bin" here on purpose: only the root package owns the memrato
        // command, otherwise the six of them collide on install.
        files: ["bin"],
      },
      null,
      2
    ) + "\n"
  );
  copyFileSync(join(root, "LICENSE"), join(pkgDir, "LICENSE"));
  optionalDependencies[name] = depRange;
}

// Root package: the shim plus the platform table.
const rootPkg = join(outDir, "memrato");
mkdirSync(join(rootPkg, "bin"), { recursive: true });
copyFileSync(join(root, "npm", "bin", "memrato.js"), join(rootPkg, "bin", "memrato.js"));
chmodSync(join(rootPkg, "bin", "memrato.js"), 0o755);
copyFileSync(join(root, "README.md"), join(rootPkg, "README.md"));
copyFileSync(join(root, "LICENSE"), join(rootPkg, "LICENSE"));
writeFileSync(
  join(rootPkg, "package.json"),
  JSON.stringify(
    {
      name: "memrato",
      ...shared,
      description: "Auto-maintained project memory for Claude Code. Injects it at session start, proposes edits at session end.",
      keywords: ["claude", "claude-code", "memory", "context", "hooks", "ai", "cli"],
      bin: { memrato: "bin/memrato.js" },
      files: ["bin"],
      optionalDependencies,
      engines: { node: ">=16" },
    },
    null,
    2
  ) + "\n"
);

// The ./ prefix is load-bearing. `npm publish npm-dist/memrato-darwin-arm64`
// matches npm's <user>/<repo> GitHub shorthand, so npm tries to clone
// github.com/npm-dist/memrato-darwin-arm64 instead of reading the directory.
// And npm refuses to publish a prerelease version without an explicit --tag,
// which the default 0.0.0-dev is.
const tag = version.includes("-") ? " --tag next" : "";

console.log(`\nBuilt ${targets.length + 1} packages at version ${version} in npm-dist/`);
console.log("Publish platform packages first, then the root:");
for (const dir of dirs) console.log(`  npm publish ./npm-dist/${dir} --access public${tag}`);
console.log(`  npm publish ./npm-dist/memrato --access public${tag}`);
