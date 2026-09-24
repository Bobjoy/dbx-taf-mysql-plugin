#!/usr/bin/env node
// 交叉组装 .dbxp：任意 runner 上产出任意 target 的包。
// 官方 plugin-cli 的 package 强制 target==host（Intel Mac 的 darwin-x64 在 GitHub 托管 runner 上无法原生产出），
// 而 .dbxp 只是 zip(manifest.json + assets/ + ui/ + bin/<target>/<binary> + checksums.json)，故自行组装。
import { execFileSync, spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdirSync, readFileSync, readdirSync, rmSync, statSync, writeFileSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const target = process.argv[2];
if (!/^(darwin|linux|win32)-(x64|arm64)$/.test(target ?? "")) {
  console.error("usage: node tools/pack.mjs <darwin|linux|win32>-<x64|arm64>");
  process.exit(2);
}
const [targetOs, targetArch] = target.split("-");
const goos = targetOs === "win32" ? "windows" : targetOs; // dbx 的 win32 == Go 的 windows
const goarch = targetArch === "x64" ? "amd64" : targetArch;
const manifest = JSON.parse(readFileSync(join(root, "manifest.json"), "utf8"));
const binaryName = "dbx-plugin-taf-mysql" + (targetOs === "win32" ? ".exe" : "");

const stage = join(root, ".pack-stage");
rmSync(stage, { recursive: true, force: true });
mkdirSync(join(stage, "bin", target), { recursive: true });

execFileSync(
  process.env.GO ?? "go",
  ["build", "-trimpath", "-ldflags", "-s -w", "-o", join(stage, "bin", target, binaryName), "."],
  {
    cwd: join(root, "backend"),
    env: { ...process.env, GOOS: goos, GOARCH: goarch, CGO_ENABLED: "0" },
    stdio: "inherit",
  },
);

// 宿主按 manifest 的 executable 路径校验二进制存在；官方 package 会重写成 bin/<target>/，这里对齐
const packed = { ...manifest, entrypoints: { ...manifest.entrypoints, backend: { executable: `bin/${target}/${binaryName}` } } };
writeFileSync(join(stage, "manifest.json"), JSON.stringify(packed, null, 2) + "\n");
for (const dir of ["assets", "ui"]) {
  copyTree(join(root, dir), join(stage, dir));
}

const files = listFiles(stage).sort();
const checksums = {
  algorithm: "sha256",
  files: Object.fromEntries(files.map((f) => [f, sha256(readFileSync(join(stage, f)))])),
};
writeFileSync(join(stage, "checksums.json"), JSON.stringify(checksums, null, 2) + "\n");

const dist = join(root, "dist");
mkdirSync(dist, { recursive: true });
const out = join(dist, `${manifest.id}-${manifest.version}-${target}.dbxp`);
rmSync(out, { force: true });
const zip = spawnSync("zip", ["-X", "-q", out, ...files, "checksums.json"], { cwd: stage, stdio: "inherit" });
if (zip.status !== 0) throw new Error("zip failed (需要 zip 命令)");
rmSync(stage, { recursive: true, force: true });
console.log(`Success: Built ${out}`);

function copyTree(from, to) {
  for (const entry of readdirSync(from)) {
    const src = join(from, entry);
    const dest = join(to, entry);
    if (statSync(src).isDirectory()) copyTree(src, dest);
    else {
      mkdirSync(dirname(dest), { recursive: true });
      writeFileSync(dest, readFileSync(src));
    }
  }
}

function listFiles(dir, base = dir) {
  const out = [];
  for (const entry of readdirSync(dir)) {
    const p = join(dir, entry);
    if (statSync(p).isDirectory()) out.push(...listFiles(p, base));
    else out.push(relative(base, p));
  }
  return out;
}

function sha256(buf) {
  return createHash("sha256").update(buf).digest("hex");
}
