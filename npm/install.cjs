"use strict";

const { installBinary } = require("./lib/install.cjs");

installBinary().catch((error) => {
  console.error(`aigit: ${error.message}`);
  process.exitCode = 1;
});
