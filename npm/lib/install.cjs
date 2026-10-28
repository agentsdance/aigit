"use strict";

const { createHash } = require("node:crypto");
const fs = require("node:fs/promises");
const path = require("node:path");

const packageRoot = path.resolve(__dirname, "..");
const releaseBase = "https://github.com/agentsdance/aigit/releases/download";
const platforms = Object.freeze({
  "darwin-x64": "aigit_darwin_amd64",
  "darwin-arm64": "aigit_darwin_arm64",
  "linux-x64": "aigit_linux_amd64",
  "win32-x64": "aigit_windows_amd64.exe",
});

function assetFor(platform = process.platform, arch = process.arch) {
  const asset = platforms[`${platform}-${arch}`];
  if (!asset) {
    throw new Error(
      `No aigit binary is available for ${platform}-${arch}. ` +
      `Supported platforms: ${Object.keys(platforms).join(", ")}. ` +
      "Build from source: https://github.com/agentsdance/aigit#installation",
    );
  }
  return asset;
}

async function download(url) {
  let response;
  try {
    response = await fetch(url, { signal: AbortSignal.timeout(60_000) });
  } catch (error) {
    throw new Error(`Could not download ${url}: ${error.message}`, { cause: error });
  }
  if (!response.ok) {
    throw new Error(`Could not download ${url}: HTTP ${response.status}`);
  }
  return Buffer.from(await response.arrayBuffer());
}

async function installBinary({
  root = packageRoot,
  platform = process.platform,
  arch = process.arch,
  fetchBinary = download,
} = {}) {
  const asset = assetFor(platform, arch);
  const pkg = JSON.parse(await fs.readFile(path.join(root, "package.json"), "utf8"));
  const manifest = JSON.parse(await fs.readFile(path.join(root, "checksums.json"), "utf8"));
  const checksum = manifest.assets?.[asset];
  if (manifest.version !== pkg.version || !/^[a-f0-9]{64}$/.test(checksum || "")) {
    throw new Error("The aigit package has an invalid release checksum manifest. Reinstall @agentsdance/aigit.");
  }
  const vendor = path.join(root, "vendor");
  const binary = path.join(vendor, platform === "win32" ? "aigit.exe" : "aigit");
  const url = `${releaseBase}/v${pkg.version}/${asset}`;
  const data = await fetchBinary(url);
  if (createHash("sha256").update(data).digest("hex") !== checksum) {
    throw new Error(`SHA256 checksum mismatch for ${asset}; the downloaded binary was not installed.`);
  }

  await fs.mkdir(vendor, { recursive: true });
  const staging = await fs.mkdtemp(path.join(vendor, ".install-"));
  try {
    const staged = path.join(staging, path.basename(binary));
    await fs.writeFile(staged, data, { mode: 0o755 });
    await fs.rename(staged, binary);
  } finally {
    await fs.rm(staging, { recursive: true, force: true });
  }
  return binary;
}

async function ensureBinary({ root = packageRoot, ...options } = {}) {
  const platform = options.platform || process.platform;
  assetFor(platform, options.arch || process.arch);
  const binary = path.join(root, "vendor", platform === "win32" ? "aigit.exe" : "aigit");
  try {
    await fs.access(binary, platform === "win32" ? fs.constants.F_OK : fs.constants.X_OK);
    return binary;
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
    return installBinary({ root, ...options });
  }
}

module.exports = { assetFor, download, ensureBinary, installBinary, platforms, releaseBase };
