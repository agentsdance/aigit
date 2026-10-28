#!/usr/bin/env node
"use strict";

const { spawn } = require("node:child_process");
const { ensureBinary } = require("../lib/install.cjs");

async function run(args = process.argv.slice(2)) {
  const binary = await ensureBinary();
  const child = spawn(binary, args, {
    stdio: "inherit",
    // npm owns this installation; the Go self-upgrader builds from source.
    env: { ...process.env, AIGIT_NO_UPDATE_CHECK: "1" },
  });
  const forwarders = new Map();
  for (const signal of ["SIGINT", "SIGTERM"]) {
    const forward = () => child.kill(signal);
    forwarders.set(signal, forward);
    process.on(signal, forward);
  }
  const cleanup = () => {
    for (const [signal, forward] of forwarders) process.off(signal, forward);
  };
  child.once("error", (error) => {
    cleanup();
    console.error(`aigit: could not start the CLI: ${error.message}`);
    process.exitCode = 1;
  });
  child.once("exit", (code, signal) => {
    cleanup();
    if (signal) {
      process.kill(process.pid, signal);
    } else {
      process.exitCode = code ?? 1;
    }
  });
}

run().catch((error) => {
  console.error(`aigit: ${error.message}`);
  process.exitCode = 1;
});
