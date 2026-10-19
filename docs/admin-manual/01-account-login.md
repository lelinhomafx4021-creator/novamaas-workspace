# 账号与钱包

先接通 Gateway 账号、登录会话和个人钱包，再进入套餐、密钥与模型调用。全册接口字段按当前代码核对；请求示例使用占位值，真实两平台联调尚未完成。

## 版本、状态与接入顺序

版本：2.0 收口稿，2026-10-19。六章及 article ID 保持不变。本册同时描述现有 API 和必须建设的目标契约；“现有”只表示源码存在，“拟新增”表示尚未实现，“待联调”表示没有真实两端验收证据。

| 对接范围 | 当前状态 | 实施时的完成条件 |
| --- | --- | --- |
| 管理员阅读、root 开关、内容隔离 | 本地实现；本次独立整理 | 权限矩阵、撤权清缓存、构建与公开内容隔离通过 |
| 登录、钱包、只读套餐 | Gateway 现有；星途有接入代码 | 当前版本完成登录/刷新/撤销/换号、单位和失败展示的真实复验 |
| Key、模型、文本、图片、视频与素材 | Gateway 有对应接口；客户端适配需逐项核对 | 按各章协议跑通结果保存、恢复查询和消费回查 |
| 套餐/个人插件权限与同步 | 待开发 | 后台保存、本人快照、显示和执行校验一致 |
| 统一版本、强制更新与回滚 | 待开发 | 同一目标版本、固定包、安装再授权、健康检查及恢复 |

先复验星途现有对话与功能，再通过独立公司适配接现有接口，随后实施权限服务/管理后台和客户端守卫，最后完成统一发布及分平台验收。[第三章实施规划](/admin-manual?article=api-keys#实施规划与交付门槛)给出阶段和通过条件；仓库详细交付计划为 `docs/design/STARPATH_INTEGRATION_DELIVERY_PLAN.zh_CN.md`。

Gateway 已有购买/充值 API 不代表星途当前产品已启用购买。星途现阶段套餐只读，不因手册列出接口就恢复旧购买入口、改变官方对话/工具/插件/技能/MCP 核心流程。本次仅升级手册并收口阅读功能，不实施拟新增业务能力。

## 对接约定

Gateway 保存账号、登录会话、钱包账本、订阅和模型密钥。星途负责登录界面、个人中心、套餐界面、插件界面和调用这些接口。星途保存 Gateway 返回的 `user.id` 作为账号关联，不用昵称或邮箱推测两个账号是同一个人。

运营管理集中在 Gateway：套餐定义与详情、功能开放、用户例外、插件目录和统一版本策略均由这里配置；星途只接收本人可见的结果并落实。星途安装包不持有管理员凭证，用户无需再注册另一套独立账号才能获得这些权益。本次交付只补对接文档，第三章和第二章标为“拟新增”的业务接口未实现。

全册保留六章：本章账号/钱包；第二章套餐定义、购买、发放、重置和商品详情；第三章密钥、插件显示与执行、版本发布/统一更新/回滚；后面三章分别覆盖文本、图片和视频，素材及媒体回调集中在视频章。

| 凭证 | 使用场景 | 从哪里取得 |
| --- | --- | --- |
| 用户登录 Access Token | 个人资料、钱包、订阅、管理自己的密钥 | 登录或刷新响应的 `data.access_token` |
| Refresh Cookie | 刷新该用户的登录会话 | 登录响应的 `Set-Cookie`，不在 JSON 中返回 |
| 管理员 Access Token / 面板 PAT | Gateway 运营后台；可选受信任服务端管理接入 | 管理员登录；PAT 由 `GET /api/user/token` 生成 |
| 用户模型 API Key | 文本、图片、视频生成 | 第三部分的密钥接口 |
| 素材 AK/SK | 素材 Action RPC 的签名 | 视频章的素材 access-keys 接口；不是上游火山账号凭证 |

示例以 `https://gateway.example.com` 为 Gateway 地址。带 `<...>` 的字符串需替换；ID 示例 `123`、`7`、`42` 也都需替换成接口实际返回的 ID。多数管理接口返回 `{"success":true,"message":"","data":...}`；业务失败也可能是 HTTP 200 + `success:false`，同时检查 HTTP 状态和业务字段。模型接口、支付跳转接口有各自的响应格式。

### 两平台接入时序

| 时机 | 星途请求/动作 | Gateway 返回/处理 |
| --- | --- | --- |
| 未登录启动 | status，渲染当前部署支持的登录方式 | 公开站点能力，不返回个人权益 |
| 登录完成 | 保存当前会话；并行读 user/self、subscription/self | 本人资料、钱包、订阅 |
| 初始化调用 | 选择本人模型 Key、读 v1/models | 密钥级可用模型 |
| 初始化插件（拟新增） | entitlements、plugins、plugin-updates/check | 本人最终功能、可见目录、更新要求 |
| 用户购买套餐 | 向 Gateway 创建支付；返回后回查 | 回调确认后才发放订阅和权益 |
| 后台调规则（拟新增） | 下次刷新读取新版本 | Gateway 计算并失效缓存，星途不自行推导授权 |
| 新执行 | 先权限/版本检查，再模型或插件操作 | 云端再校验并按本人账本计费 |
| 退出/换号 | 清空当前账号会话、Key、权限、目录、更新决策 | 下个账号重新取本人结果 |

拟新增路径的完整前缀为 `/api/apps/starpath/`，具体请求见第二、三章；现在调用会找不到接口。现有媒体接口无需等待插件管理开发就能按各自协议对接。

## 注册与登录

```http
GET /api/status
```

使用响应 `data` 中的 `register_enabled`、`password_register_enabled`、`password_login_enabled`、`email_verification`、`turnstile_check`、`turnstile_site_key` 决定显示哪些入口。不要假定每个部署都启用了相同的登录方式。

若 `turnstile_check=true`，先取得验证码结果，并把 `?turnstile=<TURNSTILE_RESULT>` 加到要求 Turnstile 的请求 URL；不是放进 JSON。注册、密码登录和发送注册邮件验证码均经过该检查。

### 注册

开启邮箱验证时先发送验证码：

```http
GET /api/verification?email=reader%40example.com
```

再提交注册：

```http
POST /api/user/register
Content-Type: application/json

{
  "username": "<USERNAME>",
  "password": "<PASSWORD>",
  "email": "reader@example.com",
  "verification_code": "<EMAIL_CODE>"
}
```

用户名最长 20 字符，密码为 8–20 字符。未开启邮箱验证时省略 `email`、`verification_code`；当前注册逻辑只在启用邮箱验证时保存该邮箱。成功响应没有登录凭证，也没有新用户 ID，继续调用登录获取账号信息。注册接口只创建普通用户。

### 登录与二次验证

```http
POST /api/user/login
Content-Type: application/json

{"username":"<USERNAME>","password":"<PASSWORD>"}
```

正常成功时，响应核心结构如下（仅展示接入所需字段）：

```json
{
  "success": true,
  "message": "",
  "data": {
    "access_token": "<USER_ACCESS_TOKEN>",
    "token_type": "Bearer",
    "access_expires_at": 1900000900,
    "session": {"sid":"<SESSION_ID>","expires_at":1902592000},
    "user": {"id":123,"username":"<USERNAME>","display_name":"<DISPLAY_NAME>","role":1,"group":"<USER_GROUP>","quota":50000}
  }
}
```

`access_expires_at` 和其他时间戳均为 Unix 秒。保存 `access_token`、过期时间、`session.sid`、`user.id`；之后的用户接口发送 `Authorization: Bearer <USER_ACCESS_TOKEN>`。不需要旧版 `New-Api-User` 请求头。

若响应是 `success:true` + `data.require_2fa=true`，此时还没有登录成功。使用返回的 `flow_token` 提交动态码或备份码：

```http
POST /api/user/login/2fa
Content-Type: application/json

{"flow_token":"<FLOW_TOKEN>","code":"<TOTP_OR_BACKUP_CODE>"}
```

完成后才会取得同样的登录凭证。挑战过期则重新密码登录，不把 `flow_token` 当作 Access Token。

其他登录方式按对应流程接入：短信 `/api/user/login/sms/code`、`/api/user/login/sms`；Passkey `/api/user/passkey/login/begin`、`/finish`；OAuth `/api/oauth/state`、`/api/oauth/:provider`。这些入口不等于 Gateway 已向星途提供通用 SSO 授权服务。

## 刷新与退出

登录返回的 `new_api_refresh` Cookie 为 HttpOnly、SameSite=Strict、路径 `/api/user/auth`。浏览器请求需携带 Cookie；仅保存 JSON 中的 Access Token 无法长时间维持登录。

```http
POST /api/user/auth/refresh
X-Auth-Session: <SESSION_ID>
Cookie: new_api_refresh=<REFRESH_COOKIE_VALUE>
Origin: https://gateway.example.com
```

Cookie 应由浏览器或服务端 Cookie 容器自动维护，示例只是说明协议。刷新成功后整体更新返回的 Access Token、过期时间、用户资料和会话；保留新的 Set-Cookie。刷新会轮换 Cookie，同一会话同时只发一次刷新，其他请求等待它完成。

星途与 Gateway 若跨站，单加 `credentials:include` 不能绕过 SameSite=Strict。可通过星途服务端代理维护每个用户独立的 Gateway Cookie 会话，或使用双方评审后的同站代理方案；Cookie 路径、Origin、受信任来源和浏览器 Cookie 规则都必须一起匹配。桌面客户端使用独立 Cookie 容器时也要保留轮换结果和正确 Origin。当前没有专供任意第三方客户端交换 Refresh Token 的通用接口。

退出时调用：

```http
POST /api/user/auth/logout
Authorization: Bearer <USER_ACCESS_TOKEN>
X-Auth-Session: <SESSION_ID>
Origin: https://gateway.example.com
```

同时携带该会话 Cookie。之后清掉星途当前账号的会话、订阅、钱包和密钥缓存。Access Token 过期时可刷新后重试一次；会话撤销、账号禁用或刷新失败则返回登录界面。不要把管理员的登录状态替代用户会话。

## 个人资料、钱包与积分

```http
GET /api/user/self
Authorization: Bearer <USER_ACCESS_TOKEN>
```

| `data` 字段 | 星途如何使用 |
| --- | --- |
| `id`、`username`、`display_name`、`email`、`phone` | 个人资料；账号关联使用 `id` |
| `role`、`status`、`group` | 账号状态和用户组；不能据此推导插件清单 |
| `quota` | 钱包剩余额度，单位是原始 quota |
| `used_quota`、`request_count` | 账户累计使用指标；不是订阅剩余额度 |
| `setting` | JSON 字符串形式的用户设置，按需解析 |
| `sidebar_modules`、`permissions` | Gateway 自身导航与后台能力，不是星途插件授权 |

钱包、订阅额度、密钥上限分别展示。钱包用 `quota`；订阅用第二部分的 `amount_total` / `amount_used`；密钥的 `remain_quota` 只是调用上限，不能再加到钱包余额中。

从 `/api/status` 读取 `quota_per_unit`、`quota_display_type`、`usd_exchange_rate`、`custom_currency_symbol`、`custom_currency_exchange_rate` 后换算：

| 展示类型 | 展示值 |
| --- | --- |
| `USD` | quota ÷ quota_per_unit |
| `CNY` | quota ÷ quota_per_unit × usd_exchange_rate |
| `CUSTOM` | quota ÷ quota_per_unit × custom_currency_exchange_rate |
| `TOKENS` | 原始 quota；不等于模型实际 token 数 |

例如仅当 `quota_per_unit=500000`、自定义汇率为 100 时，50000 quota 显示为 10 积分。星途不要写死这个比例，也不要在接口里回传已换算的积分作为 quota。

资料修改用 `PUT /api/user/self`；找回密码用 `GET /api/reset_password`、`POST /api/user/reset`；设备管理用 `/api/user/sessions` 系列。按需接入，修改安全资料后重新确认当前会话。

### 账号相关接口索引与会话管理

| 方法 | 现有地址 | 身份 | 用途 |
| --- | --- | --- | --- |
| GET | `/api/status` | 匿名 | 登录方式和单位配置 |
| GET | `/api/verification` | 匿名 | 注册邮件验证码，email Query |
| POST | `/api/user/register` | 匿名 | 普通用户注册，不自动登录 |
| POST | `/api/user/login` | 匿名 | 密码登录或取得 MFA flow_token |
| POST | `/api/user/login/2fa` | 挑战凭证 | 完成 TOTP/备份码登录 |
| POST | `/api/user/login/sms/code` | 匿名 | 发送短信登录码 |
| POST | `/api/user/login/sms` | 匿名 | 手机号/验证码登录 |
| POST | `/api/user/login/sms/mfa/code` | 挑战凭证 | 短信二次认证发码 |
| POST | `/api/user/login/sms/mfa` | 挑战凭证 | 完成短信 MFA |
| POST | `/api/user/passkey/login/begin`、`/finish` | WebAuthn 挑战 | 浏览器/桌面安全上下文流程 |
| POST | `/api/user/auth/refresh` | Refresh Cookie | 轮换会话和 Access Token |
| POST | `/api/user/auth/logout` | 当前会话 | 撤销当前登录 |
| GET / PUT | `/api/user/self` | 用户 | 读取/修改本人资料 |
| GET | `/api/user/sessions` | 用户 | 当前账号登录设备列表 |
| DELETE | `/api/user/sessions/:sid` | 用户 | 撤销指定会话 |
| POST | `/api/user/sessions/revoke-others` | 用户 | 撤销其他会话，保留当前 |
| GET | `/api/reset_password` | 匿名 | email Query，发送密码重置链接 |
| POST | `/api/user/reset` | 重置凭证 | 执行重置，按服务端流程重新登录 |

短信、Passkey 和 OAuth 是可选接入流程，索引不代替各自挑战格式；星途应先完成一种登录闭环，不能把这些路径当作与 username/password 相同的请求体。客户端版本更新、插件权限也不能替代登录身份校验。

`GET /api/user/sessions` 的 data 为会话数组，每项含 sid、current、login_method、ip、user_agent、created_at、last_active_at、expires_at，时间均为 Unix 秒。会话管理要求当前浏览器登录会话的 Access Token，面板 PAT 不满足该控制器的会话身份要求。sid 是会话标识，不能用 user.id 代替。撤销当前会话后清掉本机权限和模型调用配置；“撤销其他设备”不删除用户密钥，模型 Key 的轮换/停用走第三章。

### 钱包充值

先读 `GET /api/user/topup/info`，按 `enable_*_topup`、`payment_compliance_confirmed`、`pay_methods`、`amount_options` 和最低数量显示入口；不要把钱包充值与购买订阅共用同一个提交地址。

易支付示例：先报价，再用相同 amount 创建支付。10 仅示意，需替换成当前部署允许的数量；全部请求使用当前用户 Access Token。

```http
POST /api/user/amount
Authorization: Bearer <USER_ACCESS_TOKEN>
Content-Type: application/json

{"amount":10}
```

成功为 `message:"success"`，`data` 是收款金额字符串。确认后提交：

```http
POST /api/user/pay
Authorization: Bearer <USER_ACCESS_TOKEN>
Content-Type: application/json

{"amount":10,"payment_method":"<EPAY_METHOD>"}
```

`<EPAY_METHOD>` 使用已开放的易支付渠道值。成功响应顶层 `url` 是提交地址，`data` 是表单参数。Stripe 另走 `/api/user/stripe/amount`、`/api/user/stripe/pay`，请求含 `amount` 和 `payment_method:"stripe"`，支付地址读 `data.pay_link`；其他支付渠道按各自字段接入，不能统一套用易支付结构。

这里的 amount 是充值入口的数量字段，不是钱包 raw quota 或任意展示积分。易支付在 TOKENS 模式按 quota 输入并按整单位入账，其他模式按 amount × quota_per_unit 入账；CUSTOM 显示汇率不参与该数量反算。星途需明确充值档位与到账积分，实际收款金额以报价为准。

支付返回后读 `GET /api/user/topup/self?p=1&page_size=20`，从 `data.items` 读取 `trade_no`、`status`、`money`、`complete_time`；可用 `keyword=<TRADE_NO>` 筛选当前用户记录。状态包括 pending/success/failed/expired。确认订单 success 后重新拉取 `/api/user/self`；跳转成功不代表到账，也不能只比较余额差（期间可能有消费）。超时先查记录，不直接再次创建付款。

`POST /api/user/topup` 是兑换码入口，请求 `{"key":"<REDEMPTION_CODE>"}`，不是在线支付或模型 Key 充值。订阅支付后的权益回查与订单查询缺口见[套餐与订阅](/admin-manual?article=subscriptions#操作完成后的回查)，不能把充值记录当成完整订阅订单列表。

## 消费回查与请求结果

生成调用结束后读取 `/api/user/self`、`/api/subscription/self`，更新钱包和订阅余额。消费明细用：

```http
GET /api/log/self?p=1&page_size=20&request_id=<GATEWAY_REQUEST_ID>
Authorization: Bearer <USER_ACCESS_TOKEN>
```

响应为 `data.items/total/page/page_size`，还可按 `model_name`、`start_timestamp`、`end_timestamp` 筛选。每条 quota 按本章规则展示；other 是 JSON 字符串，存在 billing_source 时可区分钱包和订阅，缺少时不自行推断扣费来源。汇总用 `/api/log/self/stat`。不要用列表展示序号 id 作为跨页或跨刷新唯一标识。

Gateway 响应头 `X-Oneapi-Request-Id` 对应明细的 request_id，任务 ID 是另一种标识。桌面客户端或同站代理可保存该头；当前 CORS 没有配置 Expose-Headers，浏览器跨域直连可能无法读取，不能把“响应头存在”当成“浏览器脚本一定可读”。登录 Cookie 的同站限制仍按本章会话规则处理。

### 账号与结果关联的交接要求

主进程为当前登录维护 origin、user.id、session.sid 和本地登录代次；只有仍属于同一次登录的响应才能更新界面。换号/退出取消旧读取并丢弃迟到结果，不能只清一个头像或一个 localStorage 项。

模型 Key 属于单独的调用身份：现有登录不能证明手工填入 Key 的所有者与当前账号相同。自动关联完成前，明确记录所选 Key 的来源和状态，不按密钥名猜测归属，不将登录 Access Token 填入模型配置。专用应用 Key 的待开发契约见第三章。

一次生成至少保存账号关联、模型、提交协议、结果状态和可取得的请求编号；视频另存公开任务 ID，文件另存项目与本地路径。请求编号、模型响应 ID、公开任务 ID、支付订单号互不替代。查到消费不保证成果已保存，未查到即时日志也不证明可以安全重复提交。

本章联调依次验证正常登录、二次验证、过期刷新、主动撤销、同设备换号、读取失败保留旧数据标记、一次真实调用后的用量回查。自动测试、实际调用和支付到账分别登记，不能以登录成功勾选全部通过。

| 结果 | 星途的处理 |
| --- | --- |
| 登录接口的 Access Token 过期 | 按会话流程刷新一次；模型 Key 被拒绝不能靠刷新登录凭证解决 |
| 生成接口限流或暂时失败 | 显示可重试状态；先确认是否已受理，不自动重发付费请求 |
| 已拿到视频任务 ID，但轮询中断 | 保存 ID，恢复后继续查询原任务，不重新生成 |
| 提交超时且没有任务 ID | 状态记为待确认；先查本人任务和消费记录，不能把“超时”直接显示成“未扣费” |

本人任务列表 `/api/task/self?p=1&page_size=20` 返回 `data.items`，用 task_id 继续查任务；列表含 status、progress、result_url、model_name。列表内部状态与 v1/v3 详情状态不同，不能共用一套枚举。该列表覆盖异步任务，不等于完整图片/文本生成历史。项目、对话、提示词和文件历史仍由星途维护。

源码依据：`router/api-router.go`；`controller/{user,auth_session,twofa,misc,topup,topup_stripe,log,task}.go`；`model/{topup,log}.go`；`service/{auth_session,log_info_generate}.go`；`middleware/{auth_origin,turnstile-check,request-id,cors}.go`。以上按源码核对，真实充值和两平台消费回查尚未联调。
