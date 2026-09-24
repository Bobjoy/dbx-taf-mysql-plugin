# 查询体验由插件自带 workbench 页承担，不指望 dbx 原生 SQL 通道

背景：反编译官方 dev-runtime（@dbx-app/plugin-cli 0.1.9）证实，插件连接协议只有 `connection/test | connect | disconnect`（+ action / requestUserInput），宿主 dbx 的 SQL 编辑器、表树、数据网格不会向插件注册的 database_type 下发查询。最初设想"注册连接类型后可像普通 MySQL 一样用 dbx 全家桶"不成立。

决策：采用 connection-provider（负责连接持久化、表单、生命周期）+ 插件自带 workbench 查询页（SQL 输入框 + 结果表格，极简范围：不做表树面板、不做 CodeMirror 语法高亮）的组合。元数据查询引导用户跑 information_schema（taf-mysql-mcp 本就不支持 SHOW/DESC）。

否决的备选：
- 本地 MySQL 协议 shim（Go 壳在 127.0.0.1 起假 MySQL 服务，dbx 用标准 MySQL 连接直连）——能拿到全套原生体验，但认证握手/协议兼容/information_schema 伪造的实现复杂度高一个量级，且插件本身沦为进程管理器。
- 纯 workbench 插件（无 connection-provider）——少一层，但多环境连接管理要在插件页里重造，不如直接用 dbx 连接列表。

后果：用户在 dbx 里获得的是"插件页查询"而非原生网格体验；result-view 等更深的原生集成留作未来选项。
