#!/usr/bin/env node
"use strict";

// Thin launcher: exec the platform binary that postinstall placed next to this
// file, forwarding args, stdio, and exit code.
const path = require("path");
const { spawnSync } = require("child_process");
const { existsSync } = require("fs");

const binName = process.platform === "win32" ? "reflect.exe" : "reflect";
const binPath = path.join(__dirname, binName);

if (!existsSync(binPath)) {
  console.error(
    "reflect: binary not found. The install step may have failed.\n" +
      "Try reinstalling: npm install -g knowthyself\n" +
      "Or download a build from https://github.com/siddham-jain/reflect/releases"
  );
  process.exit(1);
}

const result = spawnSync(binPath, process.argv.slice(2), { stdio: "inherit" });

if (result.error) {
  console.error(`reflect: ${result.error.message}`);
  process.exit(1);
}
process.exit(result.status === null ? 1 : result.status);
