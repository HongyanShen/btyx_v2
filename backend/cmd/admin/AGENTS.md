# 管理后台

继承仓库根规则和 `backend/AGENTS.md`。Go 模板在 `templates/`，浏览器 JavaScript、CSS 和固定版本的第三方资源在 `static/`。两者按业务模块组织并嵌入 Go 二进制。局部表单通过 htmx 提交，Axios 同源 API 层处理 JSON 请求的 CSRF、响应、错误和幂等键。页面覆盖加载、空数据、错误、提交中、无权限和并发冲突。

输入金额按元字符串转换成分字符串。金额计算禁止使用 Number 或 parseFloat。权限由后端强制执行。敏感操作显示影响及确认，并避免暴露实现细节。

`tests/` 存放 Vitest、Testing Library 和 Edge 回归脚本及其依赖。优先测试实际运行的模板、路由和浏览器脚本。通过 `go run ./cmd/contracts` 从 `backend/` 生成 `tests/generated/` 中的 TypeScript 契约。
