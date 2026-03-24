#!/usr/bin/env node
const { execSync } = require("child_process");
const os = require("os");
const fs = require("fs");
const path = require("path");

const REPO = "oSEAItic/tidal";
const BIN_NAME = "tidal";

function getPlatform() {
  const platform = os.platform();
  const arch = os.arch();
  const goos = platform === "win32" ? "windows" : platform;
  const goarch = arch === "x64" ? "amd64" : arch === "arm64" ? "arm64" : arch;
  const ext = platform === "win32" ? ".exe" : "";
  return { goos, goarch, ext };
}

function install() {
  const { goos, goarch, ext } = getPlatform();
  const binDir = path.join(__dirname, "bin");
  const binPath = path.join(binDir, BIN_NAME + ext);

  if (fs.existsSync(binPath)) return;

  fs.mkdirSync(binDir, { recursive: true });

  // Try downloading pre-built binary from GitHub releases
  const asset = `tidal-${goos}-${goarch}${ext}`;
  const url = `https://github.com/${REPO}/releases/latest/download/${asset}`;

  try {
    console.log(`Downloading tidal from ${url}...`);
    execSync(`curl -sfL "${url}" -o "${binPath}" && chmod +x "${binPath}"`, {
      stdio: "inherit",
    });
    return;
  } catch {
    // fallback: build from source if Go is available
  }

  try {
    console.log("No pre-built binary. Building from source...");
    execSync(
      `go install github.com/${REPO}/cmd/tidal@latest`,
      { stdio: "inherit" }
    );
    const gobin =
      execSync("go env GOPATH").toString().trim() + "/bin/tidal" + ext;
    if (fs.existsSync(gobin)) {
      fs.copyFileSync(gobin, binPath);
      fs.chmodSync(binPath, 0o755);
      return;
    }
  } catch {
    // go not available
  }

  console.error(
    "Could not install tidal. Install Go and run: go install github.com/oSEAItic/tidal/cmd/tidal@latest"
  );
  process.exit(1);
}

install();
