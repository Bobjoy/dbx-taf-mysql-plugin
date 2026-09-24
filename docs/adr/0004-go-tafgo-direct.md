# 后端用 tafgo 直连 TgDataAsync，彻底去掉 Node 子进程链

> 更新（2026-09-24）：纯 Go 直连决策不变，但 SDK 已由 tafgo 切换为开源 tarsgo，见 ADR-0005。

背景：原方案（ADR-0001）是 Go 薄壳拉起本机 node + taf-mysql-mcp 子进程。用户拍板改为：连接表单手填 servant，不依赖 taf-mysql-mcp 的运行时与配置文件机制。公司内部存在官方 Go SDK `tafgo`（内网 gitlab，含 tars/JCE 与 servant/ObjectProxy 能力），使纯 Go 直连成为可能。

决策：插件 Sidecar（Go）用 tafgo 客户端 + TgDataAsync 的 JCE 代理（用 taf-tools-go 生成或参照 proxy 定义手写）直接执行 SQL；`node`、taf-mysql-mcp 安装目录、config.json 均不再是插件运行依赖。taf-mysql-mcp 仅作为**行为参考**：SQL 首词分类拒绝 DDL、只读/DML 策略、maxRows 截断、timeoutMs 兜底（servant 配错时 TAF 客户端自身超时会长时间挂起）等语义在 Go 侧翻译重写（逻辑量小）。

后果与前置验证：
- 协议桩：已有 TgDataAsync 的 .taf 定义，用 taf-tools-go 生成 Go 代理（与 TestApp.Hello 示例同路径）；客户端模式已确认可行——`taf.NewCommunicator()` + `comm.StringToProxy("<app>.TgDataAsyncServer.TgDataAsyncObj@tcp -h <host> -p <port> -t 60000", proxy)`。
- 第一个任务仍是技术验证（spike）：生成桩后对线上 servant 完成一次真实 SELECT（含 skipSqlCheck option、SelectReq 组包、response 拆包 stRsp），确认返回 JSON 形状与 Node 版一致。
- 若 tafgo 不可用/不兼容，回退 ADR-0001 的 node 子进程方案（该决策记录保留）。
- 依赖内网 gitlab/私有源构建二进制；行为与 CLI 版不再保证逐字节一致（接受漂移，换取运行依赖归零）。
- SQL 策略由插件自担后，写权限采用表单 allowWrite 勾选（照搬 CLI 语义：勾后放开 DML、DDL 永拒），未选"永久只读"与"requestUserInput 弹窗确认"，后者留作未来选项。
