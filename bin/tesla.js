#!/usr/bin/env node
// tesla-cli-cn entrypoint
//
// 由 npm 安装(package.json bin.tesla)注册为 `tesla` 命令。
// 真正的命令逻辑在同目录的 Go 二进制(tesla 或 tesla.exe);
// 本 JS shim 只负责把 stdio / signal / exit code 透传过去。
//
// 二进制由 scripts/install.js 在 postinstall 阶段从 GitHub Releases 下载。
"use strict";

const path = require("path");
const { spawn } = require("child_process");

const ext = process.platform === "win32" ? ".exe" : "";
const binary = path.join(__dirname, "tesla" + ext);

const child = spawn(binary, process.argv.slice(2), {
  stdio: "inherit",
  windowsHide: true,
});

child.on("exit", (code, signal) => {
  if (signal) {
    // 把信号语义透传出去,跟原生子进程行为一致
    process.kill(process.pid, signal);
  } else {
    process.exit(code === null ? 1 : code);
  }
});

child.on("error", (err) => {
  if (err.code === "ENOENT") {
    process.stderr.write(
      "tesla-cli-cn: binary not found at " + binary + "\n" +
        "  Try reinstalling: npm install -g tesla-cli-cn\n" +
        "  Or download manually:\n" +
        "    https://github.com/MiniBotFactory/tesla-cli/releases\n"
    );
    process.exit(127);
  }
  process.stderr.write("tesla-cli-cn: " + err.message + "\n");
  process.exit(1);
});
