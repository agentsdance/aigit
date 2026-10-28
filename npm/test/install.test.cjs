"use strict";

const assert = require("node:assert/strict");
const { createHash } = require("node:crypto");
const fs = require("node:fs/promises");
const os = require("node:os");
const path = require("node:path");
const { spawn, spawnSync } = require("node:child_process");
const { once } = require("node:events");
const { test } = require("node:test");
const { assetFor, download, ensureBinary, installBinary, platforms } = require("../lib/install.cjs");
const { parseChecksums, prepare } = require("../scripts/prepare.cjs");

const payload = Buffer.from("verified release binary\n");
const checksum = createHash("sha256").update(payload).digest("hex");

async function fixture(t) {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), "aigit npm test "));
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  await fs.writeFile(path.join(root, "package.json"), JSON.stringify({ version: "0.2.0" }));
  await fs.writeFile(path.join(root, "checksums.json"), JSON.stringify({
    version: "0.2.0",
    assets: Object.fromEntries(Object.values(platforms).map((asset) => [asset, checksum])),
  }));
  return root;
}

test("maps Node platforms to the published Go assets and rejects unsupported pairs", () => {
  assert.equal(assetFor("darwin", "arm64"), "aigit_darwin_arm64");
  assert.equal(assetFor("darwin", "x64"), "aigit_darwin_amd64");
  assert.equal(assetFor("linux", "x64"), "aigit_linux_amd64");
  assert.equal(assetFor("win32", "x64"), "aigit_windows_amd64.exe");
  for (const [platform, arch] of [["linux", "arm64"], ["win32", "arm64"], ["freebsd", "x64"]]) {
    assert.throws(() => assetFor(platform, arch), /No aigit binary is available/);
  }
});

test("installs only a checksum-verified binary from the exact package release", async (t) => {
  const root = await fixture(t);
  const binary = await installBinary({ root, platform: "darwin", arch: "arm64", fetchBinary: async (url) => {
    assert.equal(url, "https://github.com/agentsdance/aigit/releases/download/v0.2.0/aigit_darwin_arm64");
    return payload;
  } });
  assert.equal(binary, path.join(root, "vendor", "aigit"));
  assert.deepEqual(await fs.readFile(binary), payload);
  assert.deepEqual(await fs.readdir(path.join(root, "vendor")), ["aigit"]);
  if (process.platform !== "win32") assert.ok((await fs.stat(binary)).mode & 0o111);
});

test("uses an exe filename for Windows", async (t) => {
  const root = await fixture(t);
  const binary = await installBinary({ root, platform: "win32", arch: "x64", fetchBinary: async () => payload });
  assert.equal(path.basename(binary), "aigit.exe");
});

test("a checksum mismatch preserves the previous executable", async (t) => {
  const root = await fixture(t);
  await fs.mkdir(path.join(root, "vendor"));
  const binary = path.join(root, "vendor", "aigit");
  await fs.writeFile(binary, "previous binary");
  await assert.rejects(installBinary({ root, platform: "linux", arch: "x64", fetchBinary: async () => Buffer.from("corrupt") }), /checksum mismatch/);
  assert.equal(await fs.readFile(binary, "utf8"), "previous binary");
  assert.deepEqual(await fs.readdir(path.join(root, "vendor")), ["aigit"]);
});

test("rejects a version mismatch before attempting a download", async (t) => {
  const root = await fixture(t);
  await fs.writeFile(path.join(root, "package.json"), JSON.stringify({ version: "0.3.0" }));
  await assert.rejects(installBinary({ root, platform: "linux", arch: "x64", fetchBinary: () => {
    assert.fail("must not download with mismatched release metadata");
  } }), /invalid release checksum manifest/);
});

test("download failures never create an executable", async (t) => {
  const root = await fixture(t);
  await assert.rejects(installBinary({ root, platform: "linux", arch: "x64", fetchBinary: async () => {
    throw new Error("network unavailable");
  } }), /network unavailable/);
  await assert.rejects(fs.stat(path.join(root, "vendor")), { code: "ENOENT" });
});

test("first use installs a missing binary and subsequent uses work offline", async (t) => {
  const root = await fixture(t);
  const platform = process.platform === "win32" ? "win32" : "linux";
  const binary = await ensureBinary({ root, platform, arch: "x64", fetchBinary: async () => payload });
  assert.equal(await ensureBinary({ root, platform, arch: "x64", fetchBinary: () => {
    assert.fail("installed binaries must not need a network connection to start");
  } }), binary);
});

test("HTTP and network errors report the failed download", async (t) => {
  const mocked = t.mock.method(globalThis, "fetch", async () => new Response("missing", { status: 404 }));
  await assert.rejects(download("https://example.invalid/binary"), /HTTP 404/);
  mocked.mock.mockImplementation(async () => { throw new Error("offline"); });
  await assert.rejects(download("https://example.invalid/binary"), /Could not download .*offline/);
});

test("release preparation validates all checksums before updating package metadata", async (t) => {
  const root = await fixture(t);
  const valid = Object.values(platforms).map((asset) => `${checksum}  ${asset}`).join("\n");
  await assert.rejects(prepare("v0.3.0", root, async () => Buffer.from(valid.split("\n")[0])), /missing/);
  assert.equal(JSON.parse(await fs.readFile(path.join(root, "package.json"), "utf8")).version, "0.2.0");
  assert.equal(await prepare("v0.3.0", root, async (url) => {
    assert.match(url, /\/v0\.3\.0\/checksums\.txt$/);
    return Buffer.from(valid);
  }), "0.3.0");
  assert.equal(JSON.parse(await fs.readFile(path.join(root, "checksums.json"), "utf8")).version, "0.3.0");
  await assert.rejects(prepare("../../untrusted", root), /Invalid stable release tag/);
  assert.throws(() => parseChecksums(valid + "\n" + valid.split("\n")[0]), /Invalid checksum entry/);
});

async function launcherFixture(t, script) {
  const root = await fixture(t);
  for (const dir of ["bin", "lib", "vendor"]) await fs.mkdir(path.join(root, dir));
  await fs.copyFile(path.resolve(__dirname, "../bin/aigit.cjs"), path.join(root, "bin", "aigit.cjs"));
  await fs.copyFile(path.resolve(__dirname, "../lib/install.cjs"), path.join(root, "lib", "install.cjs"));
  await fs.writeFile(path.join(root, "vendor", "aigit"), `#!/usr/bin/env node\n${script}\n`, { mode: 0o755 });
  return path.join(root, "bin", "aigit.cjs");
}

test("launcher preserves arguments, stdin, output and the native exit code", { skip: process.platform === "win32" }, async (t) => {
  const launcher = await launcherFixture(t, `
    const fs = require('node:fs');
    console.log(JSON.stringify({args: process.argv.slice(2), input: fs.readFileSync(0, 'utf8'), updates: process.env.AIGIT_NO_UPDATE_CHECK}));
    console.error('native stderr');
    process.exitCode = 7;
  `);
  const result = spawnSync(process.execPath, [launcher, "commit", "--from-agent", "path with spaces.json", "literal;$value"], {
    input: "commit response on stdin", encoding: "utf8", timeout: 10_000,
  });
  assert.ifError(result.error);
  assert.equal(result.status, 7);
  assert.deepEqual(JSON.parse(result.stdout), {
    args: ["commit", "--from-agent", "path with spaces.json", "literal;$value"],
    input: "commit response on stdin", updates: "1",
  });
  assert.equal(result.stderr.trim(), "native stderr");
});

test("launcher preserves a native termination signal", { skip: process.platform === "win32" }, async (t) => {
  const launcher = await launcherFixture(t, "process.kill(process.pid, 'SIGTERM');");
  const result = spawnSync(process.execPath, [launcher], { encoding: "utf8", timeout: 10_000 });
  assert.ifError(result.error);
  assert.equal(result.signal, "SIGTERM");
});

test("launcher forwards termination to a running native process", { skip: process.platform === "win32", timeout: 10_000 }, async (t) => {
  const launcher = await launcherFixture(t, `
    process.on('SIGTERM', () => { console.log('terminated'); process.exit(0); });
    console.log('ready');
    setInterval(() => {}, 1000);
  `);
  const child = spawn(process.execPath, [launcher], { stdio: ["ignore", "pipe", "pipe"] });
  t.after(() => { if (child.exitCode === null) child.kill("SIGKILL"); });
  const exited = once(child, "exit");
  let output = "";
  child.stdout.on("data", (chunk) => { output += chunk; });
  await once(child.stdout, "data");
  child.kill("SIGTERM");
  const [code] = await exited;
  assert.equal(code, 0);
  assert.match(output, /terminated/);
});
