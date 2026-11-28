# 后端

继承根规则。miniapi、admin 和 worker 共享业务服务。migrate 为独立一次性命令，常驻进程启动时不变更数据库。

进程特有的启动装配、handler、路由、调度 task 放在各自 `cmd/<进程>/` 下；`internal/` 仅保存业务逻辑及确实复用的基础能力和接口。生产代码禁止从 `internal/` 导入 `cmd/`，不同进程也不互相导入。HTTP handler 统一返回 `(data, error)`，在各进程集中注册，由公共适配器处理响应及错误；文件和支付回调使用专用返回类型保持协议。

HTTP handler 使用具体请求与响应 DTO，不返回 `any`。JSON 输入通过 `httpx.JSON` 泛型适配器统一绑定，无请求体的 handler 使用 `httpx.Adapt`；不在每个 handler 中重复解码和处理绑定错误。无数据操作使用 `*httpx.NoData` 并返回 nil，保持响应 `data: null`。

HTTP DTO 与 Ent 实体隔离，业务层不得依赖 gin.Context。事务由业务服务管理，禁止一次业务分步提交。微信通知通过同一 Ent 事务写入 River，提现沿用现有 outbox。外部调用在事务提交后异步处理。

业务日志统一使用 `logging.Info(ctx)` 或 `logging.Warn(ctx)`，异常使用 `logging.Error(ctx, err)`，继承请求 ID；禁止全局 logger 或标准库日志。不得记录请求正文、凭据、身份证、签名地址或任意原始错误正文。异步任务保存并恢复来源 `request_id`，重试保持一致。日志写入按进程区分的文件，由轮换组件控制大小及保留策略。

本地使用 postgres 数据库内独立 btyx_v2 schema，不修改 public 表。测试创建独立 btyx_test_ 前缀 schema，仅允许清理本轮创建的测试 schema。

验证：go test ./...，go vet ./...，go build ./cmd/...；PG 集成测试不得用 SQLite 代替。生成 Ent 代码后验证模型与数据库一致。
