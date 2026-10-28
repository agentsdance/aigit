"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const { spawnSync } = require("node:child_process");

const root = path.resolve(__dirname, "..");
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "aigit npm smoke "));
const pkg = require("../package.json");

function command(executable, args, options = {}) {
  const result = spawnSync(executable, args, {
    cwd: root,
    encoding: "utf8",
    timeout: 180_000,
    env: { ...process.env, AIGIT_NO_UPDATE_CHECK: "1" },
    ...options,
  });
  if (result.error) throw result.error;
  assert.equal(result.status, 0, `${executable} ${args.join(" ")}\n${result.stdout}\n${result.stderr}`);
  return result.stdout;
}

function npm(args) {
  assert.ok(process.env.npm_execpath, "Run this check with npm run test:install");
  return command(process.execPath, [process.env.npm_execpath, ...args]);
}

try {
  const [pack] = JSON.parse(npm(["pack", "--json", "--pack-destination", tmp]));
  const files = pack.files.map((entry) => entry.path);
  for (const name of ["bin/aigit.cjs", "lib/install.cjs", "install.cjs", "checksums.json", "LICENSE", "README.md"]) {
    assert.ok(files.includes(name), `Missing from tarball: ${name}`);
  }
  assert.ok(files.every((name) => !name.startsWith("vendor/") && !name.startsWith("test/") && !name.startsWith("scripts/")));
  const tarball = path.join(tmp, pack.filename);
  const prefix = path.join(tmp, "prefix");
  for (const ignoreScripts of [false, true]) {
    const installPrefix = `${prefix}-${ignoreScripts ? "lazy" : "postinstall"}`;
    npm(["install", "--global", "--prefix", installPrefix, "--cache", path.join(tmp, "cache"),
      "--no-audit", "--no-fund", ...(ignoreScripts ? ["--ignore-scripts"] : []), tarball]);
    const modules = process.platform === "win32" ? "node_modules" : "lib/node_modules";
    const installed = path.join(installPrefix, modules, "@agentsdance", "aigit");
    const shim = process.platform === "win32" ? path.join(installPrefix, "aigit.cmd") : path.join(installPrefix, "bin", "aigit");
    assert.ok(fs.existsSync(shim), "npm did not create the aigit command");
    const launcher = path.join(installed, "bin", "aigit.cjs");
    const executable = process.platform === "win32" ? process.execPath : shim;
    const args = process.platform === "win32" ? [launcher] : [];
    assert.equal(command(executable, [...args, "version"]).trim(), `v${pkg.version}`);
    assert.match(command(executable, [...args, "prepare", "--help"]), /base_tree/);
    assert.match(command(executable, [...args, "commit", "--help"]), /--from-agent/);
    console.log(`Global install ${ignoreScripts ? "with scripts disabled" : "with postinstall"}: v${pkg.version} OK`);
  }
} finally {
  fs.rmSync(tmp, { recursive: true, force: true });
}
