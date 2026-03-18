#!/usr/bin/env node

"use strict";

const { execFileSync } = require("child_process");
const path = require("path");

const ext = process.platform === "win32" ? ".exe" : "";
const bin = path.join(__dirname, "ocli" + ext);

try {
  const result = execFileSync(bin, process.argv.slice(2), {
    stdio: "inherit",
    windowsHide: true,
  });
} catch (e) {
  if (e.status !== null) {
    process.exit(e.status);
  }
  console.error("open-cli: failed to run ocli —", e.message);
  console.error("open-cli: try reinstalling with: npm install -g @sbuglione/open-cli");
  process.exit(1);
}
