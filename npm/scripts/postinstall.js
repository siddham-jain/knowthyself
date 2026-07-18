#!/usr/bin/env node
"use strict";

// Downloads the prebuilt `reflect` binary for this OS/arch from the matching
// GitHub Release (produced by GoReleaser) and drops it next to bin/run.js.
//
// Skip the download with KNOWTHYSELF_SKIP_DOWNLOAD=1 (e.g. CI, or building from
// source locally).

const fs = require("fs");
const os = require("os");
const path = require("path");
const { execFileSync } = require("child_process");

const REPO = "siddham-jain/reflect";
const { version } = require("../package.json");
const binDir = path.join(__dirname, "..", "bin");

const PLATFORMS = { darwin: "darwin", linux: "linux", win32: "windows" };
const ARCHS = { x64: "amd64", arm64: "arm64" };

function fail(msg) {
  console.error(`\nreflect install failed: ${msg}\n`);
  process.exit(1);
}

async function main() {
  if (process.env.KNOWTHYSELF_SKIP_DOWNLOAD) {
    console.log("reflect: KNOWTHYSELF_SKIP_DOWNLOAD set — skipping binary download.");
    return;
  }

  const goos = PLATFORMS[process.platform];
  const goarch = ARCHS[process.arch];
  if (!goos || !goarch) {
    fail(
      `unsupported platform ${process.platform}/${process.arch}. ` +
        `Prebuilt binaries: darwin|linux|windows × amd64|arm64.`
    );
  }

  const ext = goos === "windows" ? "zip" : "tar.gz";
  const archive = `reflect_${version}_${goos}_${goarch}.${ext}`;
  const url = `https://github.com/${REPO}/releases/download/v${version}/${archive}`;

  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "reflect-"));
  const archivePath = path.join(tmpDir, archive);

  console.log(`reflect: downloading ${archive} …`);
  const res = await fetch(url, { redirect: "follow" });
  if (!res.ok) {
    fail(
      `could not download ${url} (HTTP ${res.status}). ` +
        `Check that release v${version} exists at https://github.com/${REPO}/releases`
    );
  }
  fs.writeFileSync(archivePath, Buffer.from(await res.arrayBuffer()));

  // Extract the binary into bin/. `tar` handles .tar.gz everywhere and .zip on
  // Windows 10+ (bsdtar); fall back to PowerShell for older Windows.
  fs.mkdirSync(binDir, { recursive: true });
  const binName = goos === "windows" ? "reflect.exe" : "reflect";
  try {
    if (ext === "tar.gz") {
      execFileSync("tar", ["-xzf", archivePath, "-C", tmpDir, binName], { stdio: "inherit" });
    } else {
      try {
        execFileSync("tar", ["-xf", archivePath, "-C", tmpDir, binName], { stdio: "inherit" });
      } catch {
        execFileSync(
          "powershell",
          ["-NoProfile", "-Command", `Expand-Archive -Force -LiteralPath '${archivePath}' -DestinationPath '${tmpDir}'`],
          { stdio: "inherit" }
        );
      }
    }
  } catch (err) {
    fail(`could not extract ${archive}: ${err.message}`);
  }

  const src = path.join(tmpDir, binName);
  if (!fs.existsSync(src)) fail(`archive did not contain ${binName}.`);
  const dest = path.join(binDir, binName);
  fs.copyFileSync(src, dest);
  if (goos !== "windows") fs.chmodSync(dest, 0o755);

  fs.rmSync(tmpDir, { recursive: true, force: true });
  console.log(`reflect: installed ${binName} (v${version}). Run \`reflect\` to start.`);
}

main().catch((err) => fail(err.message));
