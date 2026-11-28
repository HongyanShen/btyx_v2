# 微信支付接入

使用微信支付官方开源 Go SDK：[wechatpay-apiv3/wechatpay-go](https://github.com/wechatpay-apiv3/wechatpay-go)，当前固定 `v0.2.21`。使用商家转账单接口，不使用旧版批次转账协议。

该 SDK 当前版本未提供新版 `transfer-bills` 的专用服务包，所以按官方 README 的未封装接口用法，通过 `core.Client` 发起 POST/GET。适配层只定义字段与业务状态映射；请求签名、响应验签、私钥/公钥加载使用 SDK，通知验签和 AES 解密使用 `core/notify`。

官方接口依据：[发起转账](https://pay.wechatpay.cn/doc/v3/merchant/4012716434)、[商户单号查单](https://pay.wechatpay.cn/doc/v3/merchant/4012716437)。查单目前仅支持最近 30 天；超窗不能把“记录不存在”当作未支付后重新打款。

## 待提供的配置

均填入本地 `.env` 或由部署环境注入，示例只在 `.env.example` 声明字段，不放真实值。

| 字段 | 含义 |
| --- | --- |
| WECHAT_APP_ID / WECHAT_APP_SECRET | 小程序身份 |
| WECHAT_PAY_ENABLED | 默认 false，完整配置后才开启真实转账 |
| WECHAT_PAY_MERCHANT_ID | 商户号 |
| WECHAT_PAY_CERTIFICATE_ID | 商户 API 证书序列号 |
| WECHAT_PAY_PRIVATE_KEY_PATH | 商户私钥 PEM 的绝对路径 |
| WECHAT_PAY_PUBLIC_KEY_ID / WECHAT_PAY_PUBLIC_KEY_PATH | 微信支付公钥 ID 与公钥 PEM 路径 |
| WECHAT_PAY_API_V3_KEY | 32 字节 API v3 密钥，用于通知解密 |
| WECHAT_PAY_NOTIFY_URL | HTTPS 地址，路径 `/callbacks/wechatpay/transfer`，不带查询参数 |
| WECHAT_PAY_SCENE_ID | 商户已获批的转账场景 ID |
| WECHAT_PAY_SCENE_REPORTS | 对应该场景的 JSON 数组，元素包含 `info_type`、`info_content` |

场景 ID 和报备内容需使用商户实际获批值，不预填假定的“佣金”场景。当前采用微信支付公钥模式，不依赖自行下载平台证书的定时逻辑。

## 实现与验收边界

- 适配代码：`backend/internal/integrations/wechatpay/`。
- 资金与状态：`internal/modules/wallet/`；异步处理：`cmd/worker/task/`；通知入口：`cmd/miniapi/handler/payment_callback.go`；共享转账接口：`internal/platform/transfer/`。
- 回调：仅小程序 API 进程暴露，验证成功并完成事务后返回 HTTP 204；配置缺失返回 503。
- 缺配置的任务保留冻结并显示“等待开通转账通道”；配置完成、重启 worker 后恢复原提现。演示身份 `local:` 在适配层禁止真实转账。
- 本地测试已覆盖未知状态、无签名通知拒绝、超时查原单、重复终态、回调和 worker 响应竞态、配置恢复。没有向真实微信商户发起请求。
- 小程序从「收益 → 提现记录」或「提现 → 查看全部」查看本人记录。等待处理、转账中、结果待确认、待确认收款及通道待开通期间显示金额冻结。到账或退回使用服务端确认结果。客户端丢失申请响应时，重新进入提现页仍以原金额和幂等键重试。

## 真机联调

1. 由商户管理员开通「商家转账」，确认获批场景和报备字段，将商户号和实际预览的小程序 AppID 绑定，把 worker 的公网出口 IP 加入接口安全白名单，并确认运营账户有足够资金及适用的单笔限额。[微信支付开发指引](https://pay.wechatpay.cn/doc/v3/merchant/4012715211) · [接口安全 IP](https://pay.wechatpay.cn/doc/v3/merchant/4013751010)
2. 在联调环境填写上表参数，私钥 PEM 放到服务端受限路径。AppSecret、API v3 密钥和商户私钥通过环境配置或授权的密钥渠道注入。配置外网可达的 HTTPS 回调 `/callbacks/wechatpay/transfer`，并配置小程序请求合法域名。
3. 确认联调环境、实际收款的测试微信号及测试金额。启用真实转账前核对待处理单，因为通道恢复后 worker 会继续处理已有申请。准备体验版权限和测试账号的已入账余额。
4. 第一笔小额测试在真机确认收款，核对冻结前后余额、提现单、微信侧到账和后台验签或查单结果。第二笔暂不收款，在微信确认关闭后核对原金额释放。真机回到小程序只触发刷新，最终状态以微信通知或查单为准。[小程序确认收款](https://pay.wechatpay.cn/doc/v3/merchant/4012716430) · [商户单号查询](https://pay.wechatpay.cn/doc/v3/merchant/4012716437)
