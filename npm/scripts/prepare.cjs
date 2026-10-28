"use strict";

const fs = require("node:fs/promises");
const path = require("node:path");
const { download, platforms, releaseBase } = require("../lib/install.cjs");

function parseChecksums(text) {
  const assets = {};
  for (const line of text.trim().split(/\r?\n/)) {
    const match = /^([a-f0-9]{64})\s+\*?(aigit_[a-z0-9_.]+)$/.exec(line);
    if (!match || assets[match[2]]) throw new Error(`Invalid checksum entry: ${line}`);
    assets[match[2]] = match[1];
  }
  for (const asset of Object.values(platforms)) {
    if (!assets[asset]) throw new Error(`Release checksums are missing ${asset}`);
  }
  return assets;
}

async function prepare(tag, root = path.resolve(__dirname, ".."), fetchFile = download) {
  const pkgPath = path.join(root, "package.json");
  const pkg = JSON.parse(await fs.readFile(pkgPath, "utf8"));
  tag ||= `v${pkg.version}`;
  if (!/^v\d+\.\d+\.\d+$/.test(tag)) throw new Error(`Invalid stable release tag: ${tag}`);
  const assets = parseChecksums((await fetchFile(`${releaseBase}/${tag}/checksums.txt`)).toString("utf8"));
  pkg.version = tag.slice(1);
  await fs.writeFile(path.join(root, "checksums.json"), JSON.stringify({ version: pkg.version, assets }, null, 2) + "\n");
  await fs.writeFile(pkgPath, JSON.stringify(pkg, null, 2) + "\n");
  return pkg.version;
}

if (require.main === module) {
  prepare(process.argv[2]).then((version) => {
    console.log(`Prepared @agentsdance/aigit@${version} from its published release checksums.`);
  }).catch((error) => {
    console.error(error.message);
    process.exitCode = 1;
  });
}

module.exports = { parseChecksums, prepare };
