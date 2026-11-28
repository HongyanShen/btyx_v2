# 白兔严选 v2

第一阶段的本地实现：营销分发、运营后台、原生微信小程序及必要公共能力。其他业务只显示“即将上线”。

## 本地运行

本地运行需要 Go 1.27、PostgreSQL 18 和 Windows PowerShell。管理后台使用 Go 模板、htmx 和浏览器原生 JavaScript。Go 将静态资源嵌入 `admin` 二进制，启动和部署无需 npm 安装或前端构建。前端回归测试使用 Node.js 24。

IDE 可直接打开项目根目录；根目录的 `go.work` 引用 `backend` 模块，后端依赖仍由 `backend/go.mod` 管理。新增工作区配置后若仍显示导入错误，重新加载 IDE 的 Go 工作区或重启 Go 语言服务。从根目录检查后端使用 `go list ./backend/...`，在 `backend` 内使用 `go list ./...`。

本机连接及口令存放在被忽略的 `.env`，示例字段见 `.env.example`。不要把 `.env`、商户密钥、上传目录或日志提交到 Git。

```powershell
# 首次在本地 .env 设置 BOOTSTRAP_ADMIN_USERNAME 和 BOOTSTRAP_ADMIN_PASSWORD。
# 执行 Ent 迁移并创建首个超级管理员。
./scripts/start.ps1 -Initialize -BootstrapAdmin

# 本地演示：额外创建示例内容与任务；要求 DEMO_ENABLED=true 且支付关闭。
# ./scripts/start.ps1 -Initialize -BootstrapAdmin -Seed
```

- 运营后台位于 http://127.0.0.1:8083，使用运营账号的用户名和密码登录。超级管理员在「通用管理 → 运营账号」创建普通账号、配置页面与接口权限、重置密码或停用账号。首次初始化成功后可清除 `.env` 中的两个初始化字段。旧共享口令及其会话已退役。
- 小程序 API：`127.0.0.1:8081`；后台任务独立进程。
- 日志：`artifacts/logs/admin.log`、`miniapi.log`、`worker.log`，逐行 JSON 并按大小自动轮换、压缩及清理。配置及请求追踪见 [日志说明](docs/logging.md)。停止本项目进程：`./scripts/stop.ps1`。
- 再次启动可使用 `./scripts/start.ps1 -SkipBuild` 复用已有二进制。修改管理后台模板、脚本或样式后，重新编译并启动 `admin`。

微信开发者工具导入 `apps/miniapp/`。目前采用游客 AppID 与本机地址；提供真实配置后替换项目 AppID、合法域名与 `src/services/config.js`。本地演示登录仅在开发者工具显式开启，不会在真实微信登录失败后自动降级。发布前关闭 `DEMO_ENABLED` 和 `allowLocalDemo`，开启项目域名校验。

### htmx 管理后台

在已有 PostgreSQL schema 和运营账号的本地环境中，从仓库根目录运行。

```powershell
. ./scripts/environment.ps1
Push-Location backend
go run -buildvcs=false ./cmd/admin
Pop-Location
```

打开 [htmx 管理后台](http://127.0.0.1:8083)，使用现有运营账号登录。按 Ctrl+C 停止该进程。页面、静态资源和同源管理接口由 `cmd/admin` 提供。Go 模板位于 `backend/cmd/admin/templates/`，按功能模块组织。htmx 在同源请求中接收页面片段。

JavaScript、CSS 和第三方浏览器库位于 `backend/cmd/admin/static/`，通过 `/assets/` 由 `admin` 同源提供。独立部署时静态资源随二进制发布。设置 `ADMIN_WEB_ADDR` 可以修改监听地址。正式站点使用 HTTPS，并设置 `ADMIN_WEB_SECURE_COOKIE=true`。该进程独立写入 `artifacts/logs/admin.log`。

## 目录

```text
backend/
  cmd/
    miniapi/               小程序 API 启动、handler、集中路由注册、支付通知入口
    admin/                 htmx 管理后台，含模板、静态资源、会话和管理业务接口
      tests/               组件与 Edge 回归测试，含 generated/ 前端契约
    worker/task/           outbox、租约、异步转账及查单恢复
    migrate/               独立 Ent 迁移入口
    adminbootstrap/        一次性初始化超级管理员
    seed/                  本地示例数据
    contracts/             前端契约生成
  ent/schema/              Ent 模型、Mixin、枚举数据库约束
  internal/
    domain/enum/           Go 枚举唯一来源
    modules/               identity、enrollment、agreement、content、task、submission、
                           settlement、wallet、files、notification、feedback、audit
    integrations/          微信支付 / 火山 TOS 官方 SDK 适配
    transport/httpapi/     两个 API 复用的鉴权、文件及查询接口
    platform/              配置、数据库、事务、金额、转账接口及 HTTP 基础能力
    business/              模块组装与跨模块 PG 集成测试
apps/
  miniapp/src/pages/       任务、工作台、收益、我的
  miniapp/src/subpackages/ 任务详情、报名、协议、通知、反馈
scripts/                   本地启动、停止、验证、Go 可读性检查
docs/                      业务规则、接口说明、微信支付接入与验收记录
analysis/                  已部署原型的只读分析快照
```

模块内部按 DTO、查询、业务动作拆分。生成的 Ent 文件不手工修改。项目规则见 `AGENTS.md` 和各应用目录中的同名文件。

进程专有代码只放在对应 `cmd/<进程>/`，共享业务服务和基础能力放在 `internal/`。生产代码不能从 `internal/` 反向导入 `cmd/`，不同进程之间也不互相导入。小程序路由集中在 `miniapi/handler/router.go`，管理端操作路由集中在 `admin/operations`，两者均使用 `httpx.Register` 注册 JSON 接口。JSON 请求使用 `httpx.JSON(handler)`，泛型自动推导请求和响应类型，handler 直接接收明确类型的输入并返回 `(具体DTO, error)`；无请求体的接口使用 `httpx.Adapt(handler)`。无数据的操作返回 `(*httpx.NoData, error)`，成功时仍输出 `data: null`。响应封套为 `Envelope[T]`，`any` 仅保留在泛型约束、框架回调及最终协议分派边界。普通接口统一输出 `{code,msg,data}`，下载和支付回调分别返回专用文件及状态响应，保持既有协议。

## 验证与生成

```powershell
npm --prefix backend/cmd/admin/tests ci
./scripts/check.ps1

# 已启动本地后台并提供测试账号配置时，使用无头 Edge 验证登录、菜单与退出。
npm --prefix backend/cmd/admin/tests run test:browser
```

从仓库根目录执行 htmx 后台浏览器回归。测试启动独立 HTTP 服务，连接临时 PostgreSQL schema，使用真实浏览器操作测试数据。

```powershell
. ./scripts/environment.ps1
$env:BTYX_ADMIN_BROWSER = '1'
go test -v ./backend/cmd/admin -run '^TestAdminBrowser$' -count=1
Remove-Item Env:BTYX_ADMIN_BROWSER
```

全量检查包括 Go 可读性、真实 PostgreSQL 集成测试、Go vet、进程入口编译、管理后台格式和回归测试及小程序 JS 测试。数据库测试只创建和清理本次的 `btyx_test_` schema。小程序尚未进行开发者工具和真机联调，真实微信登录、转账与通知需要用户后续提供配置。

```powershell
. ./scripts/environment.ps1
cd backend
go generate ./ent
go run ./cmd/contracts
cd cmd/admin/tests
npm run format
```

数据库变更通过独立 `cmd/migrate` 执行，API 启动不自动迁移。已知的旧字段和索引通过对应的迁移明确替换。生产变更应先审查迁移差异并备份。

时间点使用 Go `time.Time`、PostgreSQL `timestamptz`，前后端以带时区的 RFC 3339 字符串传输。业务日期保留 `YYYY-MM-DD`。迁移命令中的 Ent hook 会在规划差异之前，将指定 schema 内旧的 bigint 时间列按毫秒转换为 `timestamptz`。可缺省时间原有的 `0` 转为 `NULL`，其他整数列保持原值。时间转换在一个事务中完成且可重复执行，后续 Ent 迁移单独执行。时间类型升级需要停下 API 和 worker 并备份，迁移完成后统一部署符合新时间契约的前后端。

软删使用布尔字段 `deleted`。迁移先根据旧 `deleted_at` 写入布尔标记，再由 Ent 替换标签的部分唯一索引。标签的原删除时间回填到 `marketing_tag_deletions.created_at`，随后移除旧标记列。升级已有环境时，先备份并停止 API 和 worker，完成迁移后用新版进程启动。迁移失败后可修复并重跑。

详见 [业务规则](docs/business-rules.md)、[接口说明](docs/api.md)、[微信支付接入](docs/wechat-pay.md)、[文件直传配置](docs/file-storage.md)、[验收与限制](docs/acceptance.md)。
