# HTTP 接口

小程序和管理后台的 JSON 接口路径前缀均为 `/api/v1`，会话按角色隔离。小程序使用 `Authorization: Bearer <token>`。后台页面与管理接口同源，使用 HttpOnly Cookie，JSON 写请求还需携带页面提供的 `X-CSRF-Token`。

成功：`{"code":0,"msg":"","data":...}`；失败：HTTP 4xx/5xx，`code` 为对应非零 HTTP 状态，`msg` 是安全业务提示，`data` 为 null。微信通知与文件下载使用其协议要求的响应格式。

金额全部是“分”的字符串；示例 `"750"` 表示 7.50 元。ID、金额不得转换成 JS Number。日期为北京时间 `YYYY-MM-DD`。时间点统一使用带时区的 RFC 3339 字符串，例如 `"2026-11-17T10:30:45.123+08:00"` 或表示同一时间点的 `"2026-11-17T02:30:45.123Z"`；不得传秒/毫秒时间戳或无时区时间。浮点数、无符号整数不得通过 JSON 数值传输。当前业务 DTO 不包含浮点数或无符号整数字段。

后端使用 `time.Time`，数据库使用 `timestamptz`，保留时间点，不保留输入的原始时区名称或偏移。JSON 小数秒位数可变；前端按时间字符串解析，不能假设返回值总是北京时间或固定字符串长度。业务日期（如 deadline、through、business_date）仍为日期字符串，不随客户端时区换日。可缺省的时间返回 `null`，不使用 `0` 或零值日期；例如已完成上传凭证、历史本地文件下载没有 `expires_at` 时返回 `null`。布尔软删标记 `deleted` 仅用于数据库内部。

管理后台回归使用的 Go DTO 和枚举生成类型分别在 `backend/cmd/admin/tests/generated/models.ts`、`enums.ts`。小程序使用同源生成的 JS 枚举。枚举外部输入严格校验，未知字段、未知枚举值返回 400。

### 小程序微信订阅通知

登录用户通过 `POST /api/v1/notifications/subscriptions` 上报一次原生订阅操作。请求含操作标识 `attempt`、业务场景 `scene`、带时区的 `observed_at`、可缺省的总开关 `main_switch`、错误码 `error_code` 及最多三个 `choices`。每个选项含 `template_id`、本次 `result` 和微信设置中可见的 `remembered`。空记忆设置表示未观测到。服务端从当前会话取用户 ID，按用户、操作标识和模板去重。设置快照不增加发送次数。

`GET /api/v1/notifications` 的 `delivery_state` 和 `delivery_reason` 供本人消息中心回显。客服排查可按用户、模板和时间查看 `subscription_choices`，按 `notifications.choice_id` 或 `subscription_choices.used_by` 追溯授权使用情况。微信接口受理记录为 `sent`，没有收件人阅读回执。微信明确拒绝时保存数值错误码，网络发送结果不明确时记录 `unknown`。

业务事务同时保存站内消息、微信模板快照，并通过 River InsertTx 写入对应的微信发送任务。PostgreSQL 在提交后唤醒 worker，发送前重查授权并预占一次机会。首次链接截止前 15 分钟提醒仍无链接的任务，截图截止前 2 小时提醒已通过或照常结算、尚未交齐截图的任务。时间依据现有营销设置，北京时间展示。

## 身份与小程序

| 方法 / 路径 | 输入 / 行为 |
| --- | --- |
| POST /auth/wechat | `{code}`；真实微信 code2session，缺配置不模拟成功 |
| POST /auth/local | `{device}`；仅 DEMO_ENABLED 时注册，显式本地演示登录 |
| POST /auth/logout | 注销当前用户会话 |
| GET /me | 本人昵称、头像文件 ID、脱敏后的微信验证手机号 `verified_phone` 和实名联系手机号 `contact_phone`、微信号、`identity_verified`、当前报名编号 `enrollment_id`、报名状态及当前协议同意状态。当前实名有效时 `identity_document` 返回脱敏证件号及留存照片的文件 ID。本人当前报名已驳回且证件已做自动核验时，未实名用户可通过 `enrollment_document` 获取该报名的留存照片 ID。曾核验通过但当前实名档案缺失时返回 `identity_resubmission_required: true`，授权状态以 `identity_verified` 为准 |
| PUT /me | 按需提交 `{nickname,avatar_file_id}` 中的字段。头像必须是本人已核验的 TOS 图片 |
| POST /me/phone | `{code}` 来自微信手机号组件。后端向微信换取手机号，加密保存并返回 `{verified_phone}`。传 `use_for_contact: true` 可同时采用该号码作为实名联系手机号，另返回本人可在本次实名提交中使用的 `contact_phone_receipt`，有效期 30 分钟 |
| POST /me/identity/recognize | `{file_id}` 为本人已确认的人像面图片。返回 `{name,identity}` |
| POST /me/identity | 提交 `{name,identity,front_file_id,back_file_id}`。联系方式可选，按本次实际填写传 `wechat` 和 `contact_phone`，或用 `contact_phone_receipt` 传本次微信取号结果。校验年龄并通过腾讯云核验后保存实名状态。更正时传 `correct_identity: true`，沿用当前已认证人像面时可省略 `identity`，由服务端读取原号码。更正核验通过后更新当前档案，被替换的照片从当前档案退役，历史核验保留各自的照片引用。已实名可单独更新微信号和联系手机号，手填号码仅作联系使用 |
| POST /enrollment | 提交 `{douyin,xhs,agreement_id}`，其中 `xhs` 填写小红书号，`agreement_id` 填写报名人刚确认的协议版本。报名与协议确认一并保存，版本已更新或尚未发布时返回 409。未实名时额外提交识别的 `name,identity` 及照片 `front_file_id,back_file_id`。驳回后重新报名须传当前已驳回报名的 `previous_enrollment_id`，服务端创建新的待审核记录。沿用这次驳回报名的人像面时可以只传两张照片的 ID。提交时自动核验实名资料，审核通过后完成实名认证 |
| GET /agreement | 当前协议 `{id,title,body,body_html?}`。富文本版本同时返回可供旧客户端显示的纯文本；历史纯文本原样展示。尚未发布时返回 409 |
| POST /agreement/consent | `{agreement_id}`；版本变更返回 409 |
| GET /tasks/today | `{has_content,price_fen,settlement_label}`，表示当前可领取状态、每平台参考单价和结算时间说明 |
| POST /tasks/claim | 空对象，要求 Idempotency-Key。后端按当前分发员标签选一条，返回任务详情快照 |
| GET /assignments | 本人任务，含五步进度及平台记录。`published_on` 为平台首次交链接的北京时间日期。`playback_deadline` 和 `playback_settlement_at` 分别为截图截止及当前补交预计开放入账的时间。已提交截图的平台可从 `settlement_eligible_at` 读取保存的开放入账时间。 |
| GET /assignments/:id | 本人任务详情及领取时的要求/价格快照 |
| POST /assignments/:id/steps | `{step}`；仅 content、publish 可手工确认 |
| POST /assignments/:id/submissions | `{platform,url,screenshot}`，截图为本人文件 ID，可先提交链接再补图。同一作品链接可以用于不同任务 |
| GET /wallet | `{available_fen,frozen_fen,withdrawn_fen,earned_fen,month_earned_fen,pending_fen,min_withdrawal_fen,max_withdrawal_fen}`。累计与本月为营销结算入账额，本月按北京时间划分。待结算为已审核通过或标记照常结算、尚未入账的平台奖励 |
| POST /withdrawals | `{amount_fen}`，要求 Idempotency-Key，冻结后异步转账 |
| GET /withdrawals | 本人提现记录，含状态、原因、商户号及确认收款 package_info |
| GET /ledger | 本人余额流水 |
| GET /notifications | 本人站内通知 |
| POST /notifications/:id/read | 空对象，按所有者检查并幂等标记已读 |
| GET /feedback | 本人反馈与回复 |
| POST /feedback | `{body}`，最多 2000 字 |

## 运营后台

运营人员通过 `GET /login` 打开登录页，提交 `POST /login` 表单建立 Cookie 会话，提交 `POST /logout` 注销。已登录的运营人员通过 `GET /api/v1/auth/me` 获取当前账号及权限。业务接口按 HTTP 方法与路由模板分别授权，例如 `POST /tasks/:id/assign`。服务端按页面权限控制菜单和直接访问。

超级管理员拥有全部业务权限，通过 `GET /operators` 查询运营账号，通过 `GET /operators/permissions` 查询可授权的页面及接口。创建账号使用 `POST /operators`，输入 `{username,password,pages,endpoints}`。更新使用 `POST /operators/:id`，输入 `{pages,endpoints,disabled,password?}`，空列表表示无权限。更新授权后，后端按新权限校验后续请求。停用及重置密码会撤销该账号的现有会话。

创建和更新可分别传入 `all_pages`、`all_endpoints`。某组为 `true` 时授权该组当前及以后新增的可分配项，此时可传空数组。省略或传 `false` 时按提交的明细授权。账号列表和当前账号接口返回两个全选标记及当前生效的明细。运营账号管理始终只对超级管理员开放。

登录后台后，运营账号可以从侧栏用户名进入个人信息页（`GET /profile`）。页面通过带 CSRF 的表单向 `POST /profile/password` 提交当前密码、新密码及确认密码。改密成功后撤销本账号的所有后台会话。发布人和运营操作人显示对应账号的登录用户名。

| 方法 / 路径 | 输入 / 行为 |
| --- | --- |
| GET /users | 分发员资料、实名与报名状态。已报名用户返回 `enrollment_updated_at` |
| GET /users?view=enrollments | 按报名返回已提交的记录，包括报名编号 `enrollment_id`、提交时间、证件上传状态和人工复核项。报名人的 `name` 为该次留存的脱敏姓名，`nickname` 为提交时的昵称。历史记录缺少相应快照时返回空值。传 `page` 启用页码分页，每页 30 条，响应头 `X-Total-Count` 返回筛选后总条数。可同时传 `status`（`pending`、`approved` 或 `rejected`）和 `tag`（标签键），两项取交集。省略 `page` 时可用 `before` 传上一页最后一条的报名编号 |
| GET /users?view=enrollments&page=1&photos=1 | 需同时具备 `GET /users` 和 `GET /users/:id/enrollment-documents` 权限。对本页报名人返回 `enrollment_photo_previews`，含两分钟有效的 `front_url` 和 `back_url`。缺少的一侧返回空串。点击缩略图后通过单个报名资料接口获取新的大图签名 |
| GET /identity-verifications | 只读实名认证记录，包含成功和失败原因。可用上页的 `next_before` 作为 `before` 加载更早的记录 |
| GET /identity-verifications/documents | 同一分页，返回每条记录留存的脱敏姓名、脱敏证件号和正反面的预览签名，以及当次填写的脱敏手机号和微信号。记录缺少的字段返回空串。需要单独的运营权限。照片签名有效期两分钟，缺少可预览照片时返回空串，签发失败时设置 `photos_unavailable` |
| GET /users/:id/enrollment-documents | 需独立权限。传 `enrollment_id` 查看指定报名。返回报名关联的脱敏实名资料、联系方式及证件照片的两分钟预览签名。省略报名编号时查看当前报名 |
| POST /users/:id/review（编辑标签） | `{tags_only:true,enrollment_id,enrollment_updated_at,tags}`，单独修改指定报名的标签，保留审核状态和已保存的画像。传空列表可解除全部标签绑定 |
| POST /users/:id/review | `{enrollment_id,approved,reason,tags,enrollment_updated_at,recent_post,new_account,age_group,portrait_only}`。仅待审核的当前报名可作出审核决定，驳回时必填原因并保存选中的标签。已驳回是终态，`portrait_only` 只更新已通过报名的画像。修改使用过期版本时返回 409 |
| GET /users?view=distributors | 当前报名已通过的分发员。带 `id` 时返回该分发员的资料、脱敏证件号和北京当天任务。完整手机号需要报名实名资料权限 |
| POST /users/:id/block | `{blocked,reason,updated_at}`。拉黑需填写原因，取消拉黑可省略原因。`updated_at` 可传详情版本校验并发操作 |
| GET /marketing-tags | 查询标签库。返回 `{id,key,name,updated_at}`，引用标签时使用 `key`，展示时使用 `name` |
| POST /marketing-tags | `{name}`，添加标签。已删除名称可再次使用，新标签使用新的 ID 和匹配键 |
| POST /marketing-tags/:id/rename | `{name,previous_name}`，修改展示名称，标签匹配键保持稳定 |
| POST /marketing-tags/:id/delete | `{updated_at}`，软删除无绑定的标签。分发员报名、内容池素材或当前任务版本仍引用标签时返回 409。任务历史版本保留原有标签键 |
| GET /contents | 素材库 |
| POST /contents | `{title,body,project,files,tags}`，文件须为已确认的运营素材 |
| POST /contents/:id/tags | `{tags,previous_tags}`，替换素材标签。提交空列表可解除全部绑定，`previous_tags` 用于防止覆盖并发编辑 |
| GET /tasks | 营销内容，含当前版本、上传时间 `uploaded_at`、素材类型 `media_types`、当前分配及进度 `allocation`、队首标记 `next_in_queue` 和最近失效记录 `last_expiry` |
| GET /marketing-settings | `{price_fen,expiry_mode,expiry_minutes,online,price_changes}`，含最近 5 条单价修改记录 |
| POST /marketing-settings | 部分更新 `price_fen`、`online` 或失效规则。固定时长使用 `expiry_mode: "fixed_duration"` 和 `expiry_minutes`（30～1440 分钟，以 30 分钟为单位）。当天截止只提交 `expiry_mode: "end_of_day"`，保留上次的固定时长。旧客户端单独提交 `expiry_minutes` 时按固定时长处理。价格和截止规则用于后续分配 |
| POST /tasks | `{title,body,project,files,price_fen,platforms,tags,deadline}`，同时保存素材及上架的任务 V1。省略单价使用营销设置。兼容传入已有素材的 `content_id` |
| GET /tasks/:id/versions | 不可变版本记录及创建人 |
| POST /tasks/:id/versions | 同新任务输入；只影响之后的领取/派发 |
| POST /tasks/:id/visibility | `{online}` |
| POST /tasks/:id/assign | `{user_id}`，要求 Idempotency-Key |
| GET /assignments、GET /assignments/:id | 跨用户执行、审核与结算状态 |
| POST /assignments/:id/review | 单平台审核传 `{platform,revision,approved,reason}`。批量通过传 `{approved:true,submissions:[{platform,revision}]}`，包含任务的全部待审核平台。多平台驳回传 `{submissions:[{platform,revision}],reason}`。每个平台的 revision 必须与审核时看到的版本一致，批量审核在同一事务中提交 |
| POST /assignments/:id/reissue | `{platform,reason,keep_pay}`；keep_pay=true 表示照常结算，false 为重派 |
| POST /assignments/:id/revoke | `{reason}`；未提交链接的工作人员指派任务 |
| GET /settlements/preview?through=YYYY-MM-DD | 结算预览及平台明细 |
| GET /settlements/weeks?month=YYYY-MM | 按北京时间结算日归月，返回平台记录是否到期及已保存的开放时间。结算预览与批次读取权限分别控制待核对与已入账明细。分发员资料、截图引用及已到账提现汇总沿用对应读取权限。 |
| GET /distribution/settings | 查看首次链接时限、结算频率、开放时间、截图截止与补交顺延设置。 |
| POST /distribution/settings | 保存分发时间设置并记录审计。需「修改分发设置」权限。 |
| GET /settlements/export?through=YYYY-MM-DD | 使用结算批次读取权限下载指定结算日的已入账 Excel 汇总。 |
| POST /settlements | `{through, review_token?}`，要求 Idempotency-Key。结算日不得晚于当前北京时间日期，各平台记录须已达到保存的开放时间。提供概览的 `review_token` 时只核对该期已到期记录，记录变化返回冲突。 |
| GET /settlements | 不可变入账批次 |
| GET /withdrawals、GET /ledger | 跨用户资金记录，含 user_id |
| POST /ledger/:id/adjust | `{delta_fen,reason}`，要求 Idempotency-Key；关联原流水调整 |
| GET /agreements | 协议历史 |
| POST /agreements | 发布新协议版本。纯文本传 `{title,body}`，富文本传 `{title,body_html}`。服务端校验段落、标题、粗体、斜体、下划线、列表及换行，并生成纯文本正文 |
| GET /feedback | 全部分发员反馈 |
| POST /feedback/:id/reply | `{reply}`，同步创建站内通知 |
| GET /audits | 返回操作人、来源平台、动作、接口返回的 HTTP 状态码（`http_status`，未记录时省略）、关联编号、原因及时间。可组合传入 `actor_name`（运营账号或小程序昵称，部分匹配）、`platform`（`miniapp` 或 `admin`）、`endpoint`（完整方法及路由模板）、`start`、`before` 和 `page`。每页 15 条，总数见 `X-Total-Count`。时间使用 RFC 3339，含开始、不含结束。接口传 `unrecorded` 可查无接口快照的记录。无法确认来源时省略平台字段。无查询条件时沿用最近 300 条。 |

## 文件与回调

运营账号需取得相应接口权限。分发员和具有上传权限的运营人员通过 `POST /files/uploads` 传入 JSON `{name,media_type,size}` 和稳定的 `Idempotency-Key`，获取单对象、私有 ACL、类型和字节数受限的 10 分钟表单凭证。前端直接 POST 到 TOS，随后 `POST /files/:id/complete` 由服务端核验并冻结正式对象。完成后才返回可引用的 `{id,name,media_type,size}`。文件最大 25MB。

`GET /files/:id/download` 使用当前客户端的会话认证，检查文件所有者、管理员或领取该任务版本的权限，返回 5 分钟 GET 签名地址。前端直接下载 TOS 文件，不转发业务凭据。响应禁止缓存。`GET /files/:id` 只兼容历史本地文件的认证下载；新上传不经过业务 API。详见 `file-storage.md`。

小程序进程额外提供 `POST /callbacks/wechatpay/transfer`，路径不含 `/api/v1`；SDK 验签解密并完成记账后返回 204。

## 幂等与并发

领取、指派、周结算、提现、资金调整必须带稳定 `Idempotency-Key`。同一业务重试沿用原键，已成功后发起新业务才使用新键；金额或目标与原键不一致返回冲突。撤回、同版本审核、同平台重派、确认协议、终态通知使用业务记录和条件校验保证重复请求不重复记账。

当前列表是有明确上限的数组（通常最近 200 条，账本/审计 300 条，协议 100 条、批次 200 条），后台表格当前为本地分页。大规模使用前需要补充服务端分页与筛选；本版不应把列表结果当作全部历史导出。结算另有独立的最多 1000 条预览检查，超限会拒绝入账。


## 1126 营销创作任务

小程序 GET /api/v1/creative/catalog 返回可领取任务、自己的作品、当天领取数量和独立名额。POST /creative/tasks/:id/claim 使用幂等键，POST /creative/works/:id/submit 携带作品文件 ID、文案与修订号，/links 提交指定平台链接，/evidence 提交对应平台的已确认截图。以上相对路径均位于 /api/v1。

后台在相同前缀下以独立的运营账号权限访问 GET/POST /creative/tasks、POST /creative/tasks/:id/visibility、GET /creative/works 和 POST /creative/works/:id/review、/return、/award。核定点赞须逐平台传入已核对的截图 ID 和整数点赞数。确认入账沿用现有结算预览、期间核对、账本及可提现余额接口。
