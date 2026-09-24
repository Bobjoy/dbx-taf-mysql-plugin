import { build } from "esbuild";

// 一次性构建：产物 ui/vendor/editor.js 随 .dbxp 分发（dbx 禁止 CDN/在线脚本）
await build({
  entryPoints: ["src/editor.mjs"],
  bundle: true,
  minify: true,
  format: "iife",
  globalName: "TafEditor",
  target: "es2020",
  outfile: "../ui/vendor/editor.js",
});
