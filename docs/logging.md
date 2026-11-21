# 后端日志与请求追踪

所有应用日志为逐行 JSON，包含 `time`、`level`、`process`、`request_id` 和 `message`。业务代码必须使用 `logging.From(ctx)`；异常使用 `logging.Error(ctx, err)`。禁止直接使用全局 Zerolog 或标准库日志，生产导入检查会拒绝这些用法。

```go
logging.From(ctx).Info().
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

应用分别写入 `adminapi.log`、`miniapi.log`、`worker.log`；迁移和示例数据命令写入 `migrate.log`、`seed.log`。文件由 [lumberjack v2.2.1](https://github.com/natefinch/lumberjack/tree/v2.2.1) 按大小自动轮换，旧文件带 UTC 时间戳并可压缩为 `.log.gz`。启动或轮换时按备份数量和保留天数清理；这些设置不是每日定时分割。不同实例应使用各自日志目录，避免多个进程实例同时写入同一文件。

目录不可创建、目标不是普通文件、文件不可写或保留策略无效时，进程启动失败，不假装日志已经落盘。配置尚未读入或日志文件无法打开时，启动错误仅输出到标准错误；本地启动脚本会保存为对应 `.err.log`。正常应用日志直接写入轮换文件，不再复制到无限增长的 stdout 重定向文件。

## 后台任务

`jobs.request_id` 保存来源请求 ID。提现及通知 outbox 在创建时写入该值，worker 领取和重试时恢复它，日志额外记录 `job_id`、任务类型、尝试次数和状态。已有任务若没有来源 ID，则使用其任务 KSUID 作为稳定追踪 ID。

该字段通过 Ent schema 和独立 `cmd/migrate` 添加，API 不在启动时迁移数据库。启动新版本前先执行迁移。

## 敏感数据

不记录请求/响应正文、Authorization、Cookie、口令、token、身份证、签名地址或第三方原始错误正文。错误仅记录类型、可用的 SQLSTATE、取消/超时分类，以及明确的业务错误码。必要诊断字段应显式选择内部实体 ID、状态等信息，不能把完整 DTO 或配置对象直接写入日志。
