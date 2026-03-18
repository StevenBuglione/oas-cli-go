#!/usr/bin/env node

"use strict";

const os = require("os");
const fs = require("fs");
const path = require("path");
const https = require("https");
const { execSync } = require("child_process");

const REPO = "StevenBuglione/open-cli";
const BINARIES = ["ocli", "oclird"];

const PLATFORM_MAP = {
  darwin: "darwin",
  linux: "linux",
  win32: "windows",
};

const ARCH_MAP = {
  x64: "amd64",
  arm64: "arm64",
};

function getVersion() {
  const pkg = require("./package.json");
  return pkg.version;
}

function getPlatformArch() {
  const platform = PLATFORM_MAP[os.platform()];
  const arch = ARCH_MAP[os.arch()];

  if (!platform) {
    console.error(
      `Unsupported platform: ${os.platform()}. Supported: ${Object.keys(PLATFORM_MAP).join(", ")}`
    );
    process.exit(1);
  }
  if (!arch) {
    console.error(
      `Unsupported architecture: ${os.arch()}. Supported: ${Object.keys(ARCH_MAP).join(", ")}`
    );
    process.exit(1);
  }

  return { platform, arch };
}

function getArchiveName(version, platform, arch) {
  const ext = platform === "windows" ? "zip" : "tar.gz";
  return `open-cli_${version}_${platform}_${arch}.${ext}`;
}

function getDownloadUrl(version, archiveName) {
  return `https://github.com/${REPO}/releases/download/v${version}/${archiveName}`;
}

function followRedirects(url, maxRedirects = 5) {
  return new Promise((resolve, reject) => {
    if (maxRedirects <= 0) {
      return reject(new Error("Too many redirects"));
    }

    const proto = url.startsWith("https") ? https : require("http");
    proto
      .get(url, { headers: { "User-Agent": "open-cli-npm-installer" } }, (res) => {
        if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          return resolve(followRedirects(res.headers.location, maxRedirects - 1));
        }
        if (res.statusCode !== 200) {
          return reject(
            new Error(`Download failed: HTTP ${res.statusCode} for ${url}`)
          );
        }
        const chunks = [];
        res.on("data", (chunk) => chunks.push(chunk));
        res.on("end", () => resolve(Buffer.concat(chunks)));
        res.on("error", reject);
      })
      .on("error", reject);
  });
}

async function extractArchive(buffer, platform, binDir) {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "open-cli-"));

  if (platform === "windows") {
    // Write zip and extract with PowerShell or tar
    const zipPath = path.join(tmpDir, "archive.zip");
    fs.writeFileSync(zipPath, buffer);
    try {
      execSync(`tar -xf "${zipPath}" -C "${tmpDir}"`, { stdio: "pipe" });
    } catch {
      // Fallback for systems without tar for zip
      execSync(
        `powershell -Command "Expand-Archive -Path '${zipPath}' -DestinationPath '${tmpDir}'"`,
        { stdio: "pipe" }
      );
    }
  } else {
    // Write tar.gz and extract
    const tarPath = path.join(tmpDir, "archive.tar.gz");
    fs.writeFileSync(tarPath, buffer);
    execSync(`tar -xzf "${tarPath}" -C "${tmpDir}"`, { stdio: "pipe" });
  }

  // Move binaries to bin dir
  for (const binary of BINARIES) {
    const ext = platform === "windows" ? ".exe" : "";
    const srcName = binary + ext;
    const src = path.join(tmpDir, srcName);

    if (fs.existsSync(src)) {
      const dest = path.join(binDir, srcName);
      fs.copyFileSync(src, dest);
      if (platform !== "windows") {
        fs.chmodSync(dest, 0o755);
      }
    }
  }

  // Clean up
  fs.rmSync(tmpDir, { recursive: true, force: true });
}

async function main() {
  const version = getVersion();
  const { platform, arch } = getPlatformArch();
  const archiveName = getArchiveName(version, platform, arch);
  const url = getDownloadUrl(version, archiveName);
  const binDir = path.join(__dirname, "bin");

  // Skip download if binaries already present (e.g. from CI)
  const ext = platform === "windows" ? ".exe" : "";
  const allPresent = BINARIES.every((b) =>
    fs.existsSync(path.join(binDir, b + ext))
  );
  if (allPresent) {
    console.log("open-cli: binaries already present, skipping download");
    return;
  }

  console.log(`open-cli: downloading v${version} for ${platform}/${arch}...`);
  console.log(`open-cli: ${url}`);

  try {
    const buffer = await followRedirects(url);
    await extractArchive(buffer, platform, binDir);
    console.log("open-cli: installation complete ✓");
  } catch (err) {
    console.error(`open-cli: failed to install — ${err.message}`);
    console.error(
      `open-cli: you can download manually from https://github.com/${REPO}/releases`
    );
    process.exit(1);
  }
}

main();
