# 后端日志与请求追踪

所有应用日志为逐行 JSON，包含 `time`、`level`、`process`、`request_id` 和 `message`。业务代码必须使用 `logging.Info(ctx)` 或 `logging.Warn(ctx)`；异常使用 `logging.Error(ctx, err)`。禁止直接使用全局 Zerolog 或标准库日志，生产导入检查会拒绝这些用法。

```go
logging.Info(ctx).
    Str("task_id", taskID).
    Msg("task published")

logging.Error(ctx, err).
    Str("job_id", jobID).
    Msg("background job failed")
```

## HTTP 请求

每次请求由服务端生成新的 KSUID，通过 `context.Context` 传给 handler、业务服务和事务，并在响应头 `X-Request-ID` 返回。不会直接采用客户端传入的 ID。已允许的后台 Origin 同时收到 `Access-Control-Expose-Headers: X-Request-ID`，浏览器可读取此头。

请求开始与结束各写一条日志，使用相同的 `request_id` 关联：

- `http request started`：进入请求中间件后立即写入，包含 `method`、`route`；业务 handler 执行前即可查到。
- `http request completed`：请求处理结束后写入，包含 `method`、`route`、最终 HTTP `status` 和字符串 `duration`。业务日志和错误日志位于两条日志之间。

两条日志均包含带时区的 `time`；未匹配路由时 `route` 为空，不以原始 URL 代替。进程异常退出或请求仍在处理时可能只有开始记录，可用它排查未完成请求。

业务拒绝、内部错误、404/405、OPTIONS 和 panic 都保留请求 ID。panic 恢复记录类型和调用栈，不输出 panic 原文或请求转储；访问日志使用注册路由，不记录查询参数。事务成功日志只在提交成功后写入。

## 文件与自动轮换

配置在本地 `.env` 中；相对 `LOG_DIR` 以该配置文件所在目录为基准，不依赖启动目录。

```dotenv
LOG_DIR=artifacts/logs
LOG_MAX_SIZE_MB=20
LOG_MAX_BACKUPS=7
LOG_MAX_AGE_DAYS=14
LOG_COMPRESS=true
```

应用分别写入 `admin.log`、`miniapi.log`、`worker.log`；迁移和示例数据命令写入 `migrate.log`、`seed.log`。文件由 [lumberjack v2.2.1](https://github.com/natefinch/lumberjack/tree/v2.2.1) 按大小自动轮换，旧文件带 UTC 时间戳并可压缩为 `.log.gz`。启动或轮换时按备份数量和保留天数清理；这些设置不是每日定时分割。不同实例应使用各自日志目录，避免多个进程实例同时写入同一文件。

目录不可创建、目标不是普通文件、文件不可写或保留策略无效时，进程启动失败，不假装日志已经落盘。配置尚未读入或日志文件无法打开时，启动错误仅输出到标准错误；本地启动脚本会保存为对应 `.err.log`。正常应用日志直接写入轮换文件，不再复制到无限增长的 stdout 重定向文件。

## 后台任务

提现任务在 `jobs` 中保存 `request_id` 和 `trace_parent`。微信通知在业务 Ent 事务中调用 River 的 `InsertTx`，任务参数保存通知 ID、请求 ID 和 traceparent。提交后 PostgreSQL 唤醒监听的 worker。River 每分钟检查一次就绪任务，覆盖断线期间错过的唤醒。提现查单和任务过期检查继续按原周期运行。

升级时先停止旧写入进程。新 worker 启动时分批把旧通知 outbox 中待处理的记录移交 River，每批入队和标记移交完成在同一事务提交。保留未来的重试时间。没有请求 ID 的旧记录沿用原任务 ID，没有 traceparent 的记录从 worker 开始新链路。

## OpenTelemetry 链路

本机在 [按请求 ID 查询](http://127.0.0.1:16687/) 粘贴响应头里的 `X-Request-ID`，即可打开对应的 Jaeger 瀑布图。[Jaeger](http://127.0.0.1:16686/) 的菜单也提供同一入口。普通请求有 HTTP 和 SQL 记录，触发微信通知的请求还会出现 River 入队、worker 消费及实际发生的微信调用。展开 SQL span 的 Attributes，可以在 `db.query.text` 查看带占位符的 SQL。微信订阅接口的 span 名为 `wechat.subscribe.send`。

在 Windows 项目根目录运行 `./scripts/start-jaeger.ps1` 启动本地 Jaeger 和查询入口。三个应用进程从本地 `.env` 读取 `TRACE_OTLP_ENDPOINT=http://127.0.0.1:4318/v1/traces`，修改后重新启动应用。Jaeger 仅监听本机，链路在 `artifacts/jaeger/data` 保存 7 天。查询覆盖接入后导出的数据，刚完成的请求或异步任务需等批量导出后再查。

用响应头 `X-Request-ID` 找到访问日志，再按其中的 `trace_id` 查询同一业务请求的 SQL、外部 HTTP 和异步任务。三个进程分别批量写入 `LOG_DIR/<进程>.trace.jsonl`，沿用日志轮换策略。配置 `TRACE_OTLP_ENDPOINT`（如 `http://127.0.0.1:4318/v1/traces`）可同时导出到 OTLP HTTP 收集器。`TRACE_SAMPLE_RATIO` 范围为 0～1，默认 1，worker 继承来源的采样决定。批量导出在进程强制退出或导出器长期阻塞时可能缺少 span，业务审计和通知状态以数据库记录为准。

HTTP span 记录路由模板或已知的服务操作、状态及耗时。数据库 span 记录操作类型和耗时。对于单条业务查询或写入，`db.query.text` 保留表名、列名和 `$1` 等绑定占位符，字面量显示为 `?`，注释被移除。驱动参数不会写入链路。多语句、管理命令、超过 64 KiB 或无法安全分词的语句只记录操作和耗时。该字段同时写入本地 trace 文件和 Jaeger，仅对更新后采集的请求生效。空闲轮询没有业务父 span 时不导出 SQL。微信通知的每次执行有独立消费 span，记录尝试次数和排队时长。外部 HTTP 调用保留进程内的父子关系，不向微信、腾讯云和 TOS 转发内部追踪标识。

上线前用独立的 `cmd/migrate` 增加 `jobs.trace_parent` 字段，再启动新版 API 和 worker。

### 按用户查链路

在 Jaeger Search 选择服务，在 Attributes 输入 `user.id=内部账号ID`。展开命中的 span，Attributes 中的 `user.roles` 区分 `user` 和 `admin`。这些标签从更新后采集的链路开始生效。

登录成功、会话验证成功后，HTTP 入口 span 记录当前账号。运营人员触发通知时，HTTP span 记录操作员，通知入队和消费 span 各自记录收件人。提现查单和已核验的微信回调记录提现人。任务到期批处理在单条任务的检查 span 上记录所属用户。匿名请求和没有特定用户的批处理入口省略身份标签。SQL 和外部 HTTP 子 span 可以通过同一 trace ID 关联。

### 给链路添加标签、事件和业务日志

业务函数使用调用方传入的 `ctx`。用 `trace.SpanFromContext(ctx)` 取得当前 span，`SetAttributes` 写可检索的标签，`AddEvent` 写带时间戳的事件。整条请求共用的标签可以在 HTTP 根 span 上设置，某个步骤的标签就写在该步骤的 span 上。属性按 span 保存，子 span 有自己的属性。

```go
// 在业务函数中，notificationID 是数据库中的内部通知 ID。
span := trace.SpanFromContext(ctx)
span.SetAttributes(attribute.String("notification.id", notificationID))
span.AddEvent("notification.delivery.started",
	trace.WithAttributes(attribute.Int("job.attempt", attempt)),
)

// 写入进程日志，自动关联当前 trace_id 和 span_id。
logging.Info(ctx).Str("notification_id", notificationID).Msg("通知开始投递")
```

上例的 Go 包为 `go.opentelemetry.io/otel/trace`、`go.opentelemetry.io/otel/attribute` 和项目的 `internal/platform/logging`。标签在 Jaeger 的 Attributes 中查看，事件在对应 span 的 Events 中查看。需要在 Jaeger 展示的时间点使用 `AddEvent`，常规业务日志写入对应进程的日志文件。

## 敏感数据

不记录请求/响应正文、Authorization、Cookie、口令、token、身份证、签名地址或第三方原始错误正文。错误仅记录类型、可用的 SQLSTATE、取消/超时分类，以及明确的业务错误码。必要诊断字段应显式选择内部实体 ID、状态等信息，不能把完整 DTO 或配置对象直接写入日志。

## 操作审计参数

产生业务审计时，服务使用现有服务端 AES-GCM 密钥，将请求参数写入同一事务的 `audit_events.parameters`。新密文以 `enc:v1:` 开头。解密后的 JSON 包含原始查询串 `query`、路由路径参数 `path` 以及原样保留的非文件请求正文 `body`。非 UTF-8 文本用 `body_base64` 保存原始字节。multipart 只保存 `multipart_fields` 中的普通字段和重复值，文件名和文件内容不入库。请求头及 Cookie 不在快照内。提交在查询串或正文中的值随原文加密。

系统操作没有 HTTP 请求时，加密保存业务提供的操作说明。HTTP 审计同时记录请求来自小程序还是管理后台。历史记录保持原样，查询时可按当时两端独有的接口识别来源，无法确定时显示「未记录」。审计页可按操作人名字、平台、已记录的接口和时间组合查询。审计表格及列表 API 不返回逐条接口名、请求正文或密文。

接口结果取实际 HTTP 状态码。2xx 和 3xx 显示成功，4xx 和 5xx 显示失败；旧记录或系统任务没有对应响应时显示「未记录」。每个产生审计的 HTTP 请求使用服务端请求 ID，将响应状态、内容类型及加密的非文件正文写入独立的不可变记录。同一请求产生多条审计时共用一份响应。文件下载及预览只保存响应元数据。已鉴权的写请求返回错误且业务事务没有留下审计时，独立追加请求失败记录。审计列表只读取状态码。
