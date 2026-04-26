#!/usr/bin/env node
// tesla-cli-cn postinstall:
//   1. 解析当前平台(darwin/linux/win32 × x64/arm64)
//   2. 拼 GitHub Releases 下载 URL(命名对齐 .goreleaser.yaml)
//   3. 下载 archive → 解压到 ./bin/
//   4. 设可执行权限(Unix)
//
// 跳过条件:bin/tesla(.exe) 已存在(开发者本地 npm link 安装)
// 私仓库:export GH_TOKEN=<token> 后再 npm install
// 自定义 release 仓:export TESLA_CLI_REPO=owner/repo
//
// 失败兜底:打印手动下载链接,exit(1)
"use strict";

const fs = require("fs");
const path = require("path");
const https = require("https");
const { spawnSync } = require("child_process");

const pkg = require("../package.json");
const VERSION = pkg.version;
const REPO = process.env.TESLA_CLI_REPO || "MiniBotFactory/tesla-cli";

// 把 Node 平台名映射到 GoReleaser 命名
function platformPair() {
  const platMap = { darwin: "darwin", linux: "linux", win32: "windows" };
  const archMap = { x64: "amd64", arm64: "arm64" };
  const os = platMap[process.platform];
  const arch = archMap[process.arch];
  if (!os || !arch) {
    throw new Error(
      "unsupported platform: " + process.platform + "-" + process.arch
    );
  }
  if (os === "windows" && arch === "arm64") {
    throw new Error("windows/arm64 build not yet released");
  }
  return { os: os, arch: arch, ext: os === "windows" ? "zip" : "tar.gz" };
}

function archiveURL(version, os, arch, ext) {
  return (
    "https://github.com/" +
    REPO +
    "/releases/download/v" +
    version +
    "/tesla_v" +
    version +
    "_" +
    os +
    "_" +
    arch +
    "." +
    ext
  );
}

function fetchBuffer(url, redirects) {
  if (redirects === undefined) redirects = 5;
  return new Promise(function (resolve, reject) {
    const opts = new URL(url);
    opts.headers = {
      "User-Agent": "tesla-cli-cn-installer/" + VERSION,
      Accept: "application/octet-stream",
    };
    if (process.env.GH_TOKEN) {
      opts.headers["Authorization"] = "token " + process.env.GH_TOKEN;
    }
    const req = https
      .get(opts, function (res) {
        if (
          [301, 302, 307, 308].indexOf(res.statusCode) !== -1 &&
          redirects > 0
        ) {
          res.resume();
          return resolve(fetchBuffer(res.headers.location, redirects - 1));
        }
        if (res.statusCode !== 200) {
          return reject(
            new Error(
              "HTTP " +
                res.statusCode +
                " " +
                (res.statusMessage || "") +
                " for " +
                url
            )
          );
        }
        const chunks = [];
        res.on("data", function (c) {
          chunks.push(c);
        });
        res.on("end", function () {
          resolve(Buffer.concat(chunks));
        });
        res.on("error", reject);
      })
      .on("error", reject);
    // 60s 网络超时(GitHub Releases 在国内偶发慢连接)
    req.setTimeout(60000, function () {
      req.destroy(new Error("fetch timeout (60s) for " + url));
    });
  });
}

// fetchBufferWithRetry 在 ETIMEDOUT / 网络瞬态错误时重试 3 次,
// 指数退避 2s / 4s / 8s。HTTP 4xx 不可重试(URL/认证错重试无意义)。
async function fetchBufferWithRetry(url, attempts) {
  if (attempts === undefined) attempts = 3;
  let lastErr;
  for (let i = 0; i < attempts; i++) {
    try {
      return await fetchBuffer(url);
    } catch (err) {
      lastErr = err;
      if (i === attempts - 1) break;
      if (/HTTP 4\d\d/.test(err.message)) break;
      const wait = 2000 * Math.pow(2, i);
      process.stderr.write(
        "tesla-cli-cn: fetch failed (" +
          err.message +
          "); retry " +
          (i + 1) +
          "/" +
          (attempts - 1) +
          " in " +
          wait +
          "ms\n"
      );
      await new Promise(function (r) {
        setTimeout(r, wait);
      });
    }
  }
  throw lastErr;
}

// effectiveURL 把 https://github.com 前缀替换为 TESLA_CLI_MIRROR 环境变量
// 指定的镜像域。国内常用:ghfast.top / mirror.ghproxy.com / gh-proxy.com 等。
//
//   export TESLA_CLI_MIRROR=https://ghfast.top
//   npm install tesla-cli-cn
function effectiveURL(url) {
  const mirror = (process.env.TESLA_CLI_MIRROR || "").replace(/\/+$/, "");
  if (!mirror) return url;
  return url.replace(/^https:\/\/github\.com/, mirror);
}

function extractArchive(archivePath, destDir, ext) {
  let cmd, args;
  if (ext === "tar.gz") {
    cmd = "tar";
    args = ["-xzf", archivePath, "-C", destDir];
  } else {
    cmd = "unzip";
    args = ["-o", archivePath, "-d", destDir];
  }
  const r = spawnSync(cmd, args, { stdio: "inherit" });
  if (r.error) {
    throw new Error(cmd + " not found on PATH; please install it");
  }
  if (r.status !== 0) {
    throw new Error(cmd + " failed with status " + r.status);
  }
}

async function main() {
  const binDir = path.join(__dirname, "..", "bin");
  const winExt = process.platform === "win32" ? ".exe" : "";
  const binPath = path.join(binDir, "tesla" + winExt);

  // dev shortcut:本地 npm link 时已有 bin/tesla,skip 下载
  if (fs.existsSync(binPath)) {
    console.log("tesla-cli-cn: binary already at " + binPath + " — skip");
    return;
  }

  const pair = platformPair();
  const baseURL = archiveURL(VERSION, pair.os, pair.arch, pair.ext);
  const url = effectiveURL(baseURL);
  if (url !== baseURL) {
    console.log(
      "tesla-cli-cn: using mirror " + (process.env.TESLA_CLI_MIRROR || "")
    );
  }
  console.log("tesla-cli-cn: downloading " + url);

  const data = await fetchBufferWithRetry(url);
  console.log("tesla-cli-cn: downloaded " + data.length + " bytes");

  fs.mkdirSync(binDir, { recursive: true });
  const tmpArchive = path.join(binDir, "archive." + pair.ext);
  fs.writeFileSync(tmpArchive, data);

  try {
    extractArchive(tmpArchive, binDir, pair.ext);
  } finally {
    try {
      fs.unlinkSync(tmpArchive);
    } catch (_) {}
  }

  if (!fs.existsSync(binPath)) {
    throw new Error("expected " + binPath + " after extract, not found");
  }
  if (process.platform !== "win32") {
    fs.chmodSync(binPath, 0o755);
  }
  console.log("tesla-cli-cn: installed at " + binPath);
}

main().catch(function (err) {
  process.stderr.write("tesla-cli-cn install failed: " + err.message + "\n");
  process.stderr.write("\n");
  process.stderr.write("Troubleshooting:\n");
  process.stderr.write("  1. 国内网络差时设镜像(任选其一):\n");
  process.stderr.write(
    "       TESLA_CLI_MIRROR=https://ghfast.top npm install tesla-cli-cn\n"
  );
  process.stderr.write(
    "       TESLA_CLI_MIRROR=https://gh-proxy.com  npm install tesla-cli-cn\n"
  );
  process.stderr.write("  2. 或手动下载二进制:\n");
  process.stderr.write("       https://github.com/" + REPO + "/releases\n");
  process.stderr.write("  3. 跳过 postinstall(你得自己把 tesla 二进制放到 PATH):\n");
  process.stderr.write("       npm install --ignore-scripts tesla-cli-cn\n");
  process.exit(1);
});
