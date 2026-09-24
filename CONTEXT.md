# CONTEXT — dbx-taf-mysql-plugin

dbx 插件：在 dbx 里以自建连接类型查询 TAF 环境的 MySQL（经 TgDataAsync 服务）。纯术语表，不含实现细节。

## 术语

| 术语 | 定义 |
|------|------|
| **dbx** | 桌面数据库客户端（dbxio.com），支持 `.dbxp` 插件扩展。 |
| **.dbxp 包** | dbx 插件的发布形态；manifest v1 声明入口与贡献点，按 OS/CPU 架构打包。本机自用，不进商店。 |
| **Sidecar** | 插件的原生后端进程；dbx 只接受包内编译好的二进制（本插件为 Go，darwin-x64），stdio JSON-RPC 通信。 |
| **connection-provider** | dbx 贡献点：注册自定义数据库类型 `taf-mysql`，宿主只管表单与连接持久化，**不会向插件连接下发 SQL 查询**。 |
| **workbench 查询页** | 插件自带 iframe 页面：SQL 输入框 + 执行 + 结果表格 + 错误条（极简；无表树、无语法高亮）。查询经 `connection/action` 进 Sidecar。 |
| **TgDataAsync / servant** | TAF 数据访问服务对象；servant 是完整寻址串 `应用名.TgDataAsyncServer.TgDataAsyncObj@tcp -h <host> -t 60000 -p <port>`，连接表单手填。 |
| **tafgo** | TAF 官方 Go SDK（公司内网 gitlab，路径 tafgo/taf）。曾用作 Sidecar 依赖，现已被开源 tarsgo 替代（见 ADR-0005），仅其 **BasePacket 线协议布局**仍是 `ETG/tafprotocol.go` 的对齐目标。 |
| **tarsgo** | 开源 TARS Go SDK（github.com/TarsCloud/TarsGo）。`NewCommunicator + StringToProxy` 直连 servant；因 TAF/TARS 包 tag 布局差一位，注入自定义 Protocol 按 TAF 布局组包/解包，协议桩手写于 `backend/ETG/`。 |
| **taf-mysql-mcp** | 已有的本机 Node MCP 服务。**仅作行为参考**，不是插件运行依赖：SQL 首词分类、只读/DML 放行、maxRows 截断、timeoutMs 兜底等语义在 Go 侧重写。 |
| **只读/写模式** | 连接表单 allow_write 勾选框：不勾仅放行 SELECT 族；勾选放开 INSERT/UPDATE/DELETE/REPLACE；DDL 任何情况拒绝。 |
| **对象树** | workbench 页左侧自建树（宿主侧边栏无法挂插件节点）：表/视图 → 字段/索引，数据来自 information_schema 经 `taf-mysql/meta` RPC；单击表名插入编辑器，双击预览前 100 行。 |
| **线上 TAF 环境** | 用户持有的线上 TgDataAsync servant 地址所指向的环境；端到端验收用它。插件无"环境"概念，环境=servant 值。 |
| **元数据查询** | TgDataAsync 只执行 SELECT 族：SHOW/DESC/EXPLAIN 无效；表清单/字段查 `information_schema.tables/columns`（配 `database()`），跨库用全限定表名。 |

## 已定参数（非表单旋钮）

- maxRows 截断与 timeoutMs 超时在 Go 侧硬编码，默认对齐 CLI 版（200 行 / 30s）；放大靠 SQL 自带 LIMIT 或改码，不加旋钮。
- `connection/test` 语义 = 对 servant 执行 `select 1`。
