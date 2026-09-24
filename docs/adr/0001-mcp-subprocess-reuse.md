# 复用 taf-mysql-mcp 走"子进程 + MCP 客户端"，不内嵌 TAF 代码

状态：Superseded by ADR-0004（2026-09-23，用户改为表单手填 servant、taf-mysql-mcp 仅作参考实现）。本文保留作为 tafgo 不可用时的回退方案。


背景：dbx Sidecar 的 `backend.executable` 必须是包内原生二进制（Rust/Go 编译产物），不接受 Node 脚本，而 TAF RPC 能力（@taf/taf-rpc、JCE 代理、SQL 策略）只存在于 Node 生态的 taf-mysql-mcp 里。

决策：插件后端写一个很薄的 Go 二进制，收到请求后用 os/exec 拉起本机 `node <mcp目录>/index.js --config <路径>`，作为它的 stdio MCP 客户端调 `query` 工具。只读/DML 判定、超时、行数截断等策略代码零复制，与 CLI 使用行为永远一致。

否决的备选：
- Sidecar 内嵌 TAF 代码——被平台约束挡死（不能跑 Node）；即便用 bun --compile 打二进制，@taf/taf-rpc 的动态 require/native addon 兼容性未验证，构建链和体积都重。
- Go/Rust 移植 tars/JCE 协议——自研协议移植成本不现实。

后果：运行机器上必须有 node 和 taf-mysql-mcp 安装目录（本机自用成立）；node 可执行文件由 Go 壳按 PATH → /usr/local/bin → /opt/homebrew/bin → ~/.nvm 等常见位置探测，dbx 作为 GUI 应用 PATH 可能不含 node（已知坑）。
