# dbx-taf-mysql-plugin

dbx 插件：在 dbx 里以自建 `taf-mysql` 连接类型查询 TAF 环境 MySQL（Go Sidecar 基于开源 [TarsGo](https://github.com/TarsCloud/TarsGo) 直连 TgDataAsync）。
决策背景见 `CONTEXT.md` 与 `docs/adr/`。

## 构建与打包

```bash
cd backend && go test ./...                  # 策略/执行核/handler 单测（不触网）
go build -o ../bin/dbx-plugin-taf-mysql .
node ../tools/pack.mjs darwin-x64            # 产出 dist/*.dbxp（支持 darwin|linux|win32 × x64|arm64 交叉打包）
```

CodeMirror 编辑器只在升级依赖时重建（产物 `ui/vendor/editor.js` 随包提交，dbx 禁止 CDN）：

```bash
cd editor && npm install && node build.mjs
```

协议桩源码在 `backend/ETG/`，基于 tarsgo 手写最小实现（只含 select 接口）。
TARS 网关与 TAF 网关的包字段 tag 布局不同，`tafprotocol.go` 以 tarsgo 的 `model.Protocol`
注入 TAF BasePacket 布局，字节级与 TAF 官方客户端一致。

## 多平台发布（GitHub Actions）

`.github/workflows/release.yml` 两个 job：

- `build`：matrix 并行（darwin-x64 / darwin-arm64 / linux-x64 / win32-x64，均在 ubuntu runner 上交叉编译），
  跑单测后 `tools/pack.mjs <target>` 产出 `.dbxp` 并上传 artifact；
- `release`：汇总全部 artifact，一次性发布——main 分支每次推送更新 `nightly`（显示为 Pre-release），
  推 `v*` tag 则出对应正式版并附自动生成的 release notes。

产物命名 `com.bao.taf-mysql-<版本>-<target>.dbxp`；加新平台只需扩 matrix。

## 本地联调

```bash
npx @dbx-app/plugin-cli dev --path . --port 8731
```

## 安装使用

从本仓库 [Releases](https://github.com/Bobjoy/dbx-taf-mysql-plugin/releases) 下载对应平台的
`.dbxp`（main 分支每次推送自动出 `nightly` 预发布版，打 `v*` tag 出正式版），
在 dbx 里安装后新建 "TAF MySQL" 连接，
Servant（格式 `应用名.服务名.Object名@tcp -h <host> -t 60000 -p <port>`，向你们环境的 TAF 运维要），
双击连接后在 "TAF MySQL 查询" 工作台执行 SQL。默认只读，勾选 Allow write 放开 DML，DDL 永远拒绝。
