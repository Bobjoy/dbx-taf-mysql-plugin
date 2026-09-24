# 依赖从内网 tafgo 切换为开源 tarsgo + 自定义 TAF 线协议

状态：Accepted（2026-09-24，用户拍板"采用开源的 tarsgo"；部分取代 ADR-0004 的 SDK 选型，纯 Go 直连决策不变）

背景：插件原依赖内网 gitlab 的 tafgo（jce2go 生成协议桩），意味着公开仓库不能含生成码、构建机需 GOPRIVATE、vendor 目录不可公开。用户要求改用开源 TarsCloud/TarsGo（v1.4.6）。

决策与实现：
- `backend/ETG/` 删除 3462 行 jce2go 生成码，手写最小协议桩（只含 select 接口）。
- TAF 与 TARS 的包字段 tag 布局差一位（TAF BasePacket：servant=6/func=7/buffer=8/…；TARS RequestPacket 从 5 起），tarsgo 默认发包会被 TAF 网关**静默丢弃**。经本地假端点抓包逐字节比对，实现 `model.Protocol` 注入 TAF BasePacket 布局（`tafprotocol.go`），请求与 tafgo 字节级一致。
- 响应解析按真实网关字节修正：sBuffer 前需消费 SimpleList 字段头与 BYTE 元素类型头，再读 int32 长度。
- head 编码为 `(tag<<4)|type`，tarsgo 类型常量 BYTE=0…SimpleList=13（与常见 JCE 文档的 type<<4 记法相反，易踩坑）。

线上端到端验证（生产网关）：`select 1` 与 information_schema 多行查询均正常返回。

后果：vendor 全部为开源代码，仓库可公开，构建不再依赖内网；代价是线协议兼容性由自研 `tafprotocol.go` 承担，若网关升级布局需自行跟随。tafgo 回退路径保留在 git 历史与 ADR-0004。
