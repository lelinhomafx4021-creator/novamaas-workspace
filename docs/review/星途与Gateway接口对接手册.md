# 星途与 Gateway 接口对接手册

版本：2.0 收口稿。更新日期：2026-10-19。

聚合网关（Gateway）管理账号、钱包、套餐与模型调用；功能权限、插件目录、配置生效回报及统一发布是本册明确标记的待开发能力。星途负责公司接口适配、本人状态展示与后续受控执行。

本审阅稿由六章正文生成；保留既有 article ID。现有 API 已按源码核对，不等于真实两端联调通过；所有 `/api/apps/starpath/...` 均未实现。统一版本由后台指定，所有获授权用户同一目标，平台/架构只选择同版本文件。

星途当前先复验现有功能，套餐只读；不因接口存在恢复购买或改造官方对话、工具、技能、MCP 核心。原参考 PDF 的差异核对保留于视频章，本次未取得原路径 PDF 源文件，未新增逐页复核结果。

开发交付、源码接点、阶段责任、兼容迁移和验收场景见[实施与交付计划](../design/STARPATH_INTEGRATION_DELIVERY_PLAN.zh_CN.md)。手册状态与阅读隔离说明见[设计说明](../design/ADMIN_MANUAL.zh_CN.md)。编辑 `docs/admin-manual/` 后在 `web/` 运行 `bun run docs:admin-manual`，不要直接维护第二份正文。

## 阅读目录

1. [账号与钱包](#account-login)
2. [套餐与订阅](#subscriptions)
3. [密钥与插件权益](#api-keys)
4. [文本接口对接](#text-models)
5. [图片接口对接](#image-models)
6. [视频接口对接](#video-models)

## 详细索引

- [1 账号与钱包](#account-login)
  - [版本、状态与接入顺序](#account-login--版本-状态与接入顺序)
  - [对接约定](#account-login--对接约定)
  - [注册与登录](#account-login--注册与登录)
  - [刷新与退出](#account-login--刷新与退出)
  - [个人资料、钱包与积分](#account-login--个人资料-钱包与积分)
  - [消费回查与请求结果](#account-login--消费回查与请求结果)
- [2 套餐与订阅](#subscriptions)
  - [业务对象和完整接入顺序](#subscriptions--业务对象和完整接入顺序)
  - [本章现有接口总表](#subscriptions--本章现有接口总表)
  - [用户端与管理端分工](#subscriptions--用户端与管理端分工)
  - [查询套餐与订阅](#subscriptions--查询套餐与订阅)
  - [用户购买与扣费来源](#subscriptions--用户购买与扣费来源)
  - [管理端操作](#subscriptions--管理端操作)
  - [套餐详情和插件权益如何配置（拟新增）](#subscriptions--套餐详情和插件权益如何配置-拟新增)
  - [操作完成后的回查](#subscriptions--操作完成后的回查)
  - [套餐对接任务与验收](#subscriptions--套餐对接任务与验收)
- [3 密钥与插件权益](#api-keys)
  - [密钥和模型目录接口总表（现有）](#api-keys--密钥和模型目录接口总表-现有)
  - [获取星途使用的用户密钥](#api-keys--获取星途使用的用户密钥)
  - [密钥配置与修改](#api-keys--密钥配置与修改)
  - [模型与插件可见性](#api-keys--模型与插件可见性)
  - [必须补齐的用户功能与插件权限](#api-keys--必须补齐的用户功能与插件权限)
  - [Gateway 后台具体控制哪些内容（拟新增）](#api-keys--gateway-后台具体控制哪些内容-拟新增)
  - [真实插件如何接入：源码调研与落地细则](#api-keys--真实插件如何接入-源码调研与落地细则)
  - [插件列表、详情与显示控制（拟新增）](#api-keys--插件列表-详情与显示控制-拟新增)
  - [插件版本、统一发布、强制更新和回滚（拟新增）](#api-keys--插件版本-统一发布-强制更新和回滚-拟新增)
  - [实施规划与交付门槛](#api-keys--实施规划与交付门槛)
  - [两个平台逐项验收（待实施）](#api-keys--两个平台逐项验收-待实施)
  - [权益变化后的同步](#api-keys--权益变化后的同步)
- [4 文本接口对接](#text-models)
  - [接口和凭证矩阵](#text-models--接口和凭证矩阵)
  - [Chat 请求字段](#text-models--chat-请求字段)
  - [同步对话](#text-models--同步对话)
  - [视觉输入与工具调用](#text-models--视觉输入与工具调用)
  - [流式与其他文本协议](#text-models--流式与其他文本协议)
  - [模型协议能力表与接入验收](#text-models--模型协议能力表与接入验收)
  - [相关模型能力入口与错误处理](#text-models--相关模型能力入口与错误处理)
- [5 图片接口对接](#image-models)
  - [接口选择和字段约定](#image-models--接口选择和字段约定)
  - [文生图与图片结果](#image-models--文生图与图片结果)
  - [图片编辑与流式图片](#image-models--图片编辑与流式图片)
  - [火山原生图片请求](#image-models--火山原生图片请求)
  - [旧媒体工具的适配与交付](#image-models--旧媒体工具的适配与交付)
  - [结果保存、权限变化和错误](#image-models--结果保存-权限变化和错误)
- [6 视频接口对接](#video-models)
  - [现有接口总表](#video-models--现有接口总表)
  - [协议选择](#video-models--协议选择)
  - [Wan：v1](#video-models--wan-v1)
  - [Seedance：v3 原生与 v1 兼容](#video-models--seedance-v3-原生与-v1-兼容)
  - [查询、取消与下载](#video-models--查询-取消与下载)
  - [参考素材上传与使用](#video-models--参考素材上传与使用)
  - [素材 AK/SK 取得与签名（现有）](#video-models--素材-ak-sk-取得与签名-现有)
  - [素材组与素材 Action 全生命周期（现有）](#video-models--素材组与素材-action-全生命周期-现有)
  - [素材与任务 Webhook（现有 REST，非权益推送）](#video-models--素材与任务-webhook-现有-rest-非权益推送)
  - [AK/SK 到视频结果的 Python 示例（现有协议）](#video-models--ak-sk-到视频结果的-python-示例-现有协议)
  - [旧媒体链路替换、任务恢复与验收](#video-models--旧媒体链路替换-任务恢复与验收)
  - [参考 PDF 与当前实现差异](#video-models--参考-pdf-与当前实现差异)

<a id="account-login"></a>

## 1 账号与钱包

先接通 Gateway 账号、登录会话和个人钱包，再进入套餐、密钥与模型调用。全册接口字段按当前代码核对；请求示例使用占位值，真实两平台联调尚未完成。

<a id="account-login--版本-状态与接入顺序"></a>

### 版本、状态与接入顺序



版本：2.0 收口稿，2026-10-19。六章及 article ID 保持不变。本册同时描述现有 API 和必须建设的目标契约；“现有”只表示源码存在，“拟新增”表示尚未实现，“待联调”表示没有真实两端验收证据。

| 对接范围 | 当前状态 | 实施时的完成条件 |
| --- | --- | --- |
| 管理员阅读、root 开关、内容隔离 | 本地实现；本次独立整理 | 权限矩阵、撤权清缓存、构建与公开内容隔离通过 |
| 登录、钱包、只读套餐 | Gateway 现有；星途有接入代码 | 当前版本完成登录/刷新/撤销/换号、单位和失败展示的真实复验 |
| Key、模型、文本、图片、视频与素材 | Gateway 有对应接口；客户端适配需逐项核对 | 按各章协议跑通结果保存、恢复查询和消费回查 |
| 套餐/个人插件权限与同步 | 待开发 | 后台保存、本人快照、显示和执行校验一致 |
| 统一版本、强制更新与回滚 | 待开发 | 同一目标版本、固定包、安装再授权、健康检查及恢复 |

先复验星途现有对话与功能，再通过独立公司适配接现有接口，随后实施权限服务/管理后台和客户端守卫，最后完成统一发布及分平台验收。[第三章实施规划](#api-keys--实施规划与交付门槛)给出阶段和通过条件；仓库详细交付计划为 `docs/design/STARPATH_INTEGRATION_DELIVERY_PLAN.zh_CN.md`。

Gateway 已有购买/充值 API 不代表星途当前产品已启用购买。星途现阶段套餐只读，不因手册列出接口就恢复旧购买入口、改变官方对话/工具/插件/技能/MCP 核心流程。本次仅升级手册并收口阅读功能，不实施拟新增业务能力。

<a id="account-login--对接约定"></a>

### 对接约定



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

<a id="account-login--两平台接入时序"></a>

#### 两平台接入时序



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

<a id="account-login--注册与登录"></a>

### 注册与登录



```http
GET /api/status
```

使用响应 `data` 中的 `register_enabled`、`password_register_enabled`、`password_login_enabled`、`email_verification`、`turnstile_check`、`turnstile_site_key` 决定显示哪些入口。不要假定每个部署都启用了相同的登录方式。

若 `turnstile_check=true`，先取得验证码结果，并把 `?turnstile=<TURNSTILE_RESULT>` 加到要求 Turnstile 的请求 URL；不是放进 JSON。注册、密码登录和发送注册邮件验证码均经过该检查。

<a id="account-login--注册"></a>

#### 注册



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

<a id="account-login--登录与二次验证"></a>

#### 登录与二次验证



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

<a id="account-login--刷新与退出"></a>

### 刷新与退出



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

<a id="account-login--个人资料-钱包与积分"></a>

### 个人资料、钱包与积分



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

<a id="account-login--账号相关接口索引与会话管理"></a>

#### 账号相关接口索引与会话管理



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

<a id="account-login--钱包充值"></a>

#### 钱包充值



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

`POST /api/user/topup` 是兑换码入口，请求 `{"key":"<REDEMPTION_CODE>"}`，不是在线支付或模型 Key 充值。订阅支付后的权益回查与订单查询缺口见[套餐与订阅](#subscriptions--操作完成后的回查)，不能把充值记录当成完整订阅订单列表。

<a id="account-login--消费回查与请求结果"></a>

### 消费回查与请求结果



生成调用结束后读取 `/api/user/self`、`/api/subscription/self`，更新钱包和订阅余额。消费明细用：

```http
GET /api/log/self?p=1&page_size=20&request_id=<GATEWAY_REQUEST_ID>
Authorization: Bearer <USER_ACCESS_TOKEN>
```

响应为 `data.items/total/page/page_size`，还可按 `model_name`、`start_timestamp`、`end_timestamp` 筛选。每条 quota 按本章规则展示；other 是 JSON 字符串，存在 billing_source 时可区分钱包和订阅，缺少时不自行推断扣费来源。汇总用 `/api/log/self/stat`。不要用列表展示序号 id 作为跨页或跨刷新唯一标识。

Gateway 响应头 `X-Oneapi-Request-Id` 对应明细的 request_id，任务 ID 是另一种标识。桌面客户端或同站代理可保存该头；当前 CORS 没有配置 Expose-Headers，浏览器跨域直连可能无法读取，不能把“响应头存在”当成“浏览器脚本一定可读”。登录 Cookie 的同站限制仍按本章会话规则处理。

<a id="account-login--账号与结果关联的交接要求"></a>

#### 账号与结果关联的交接要求



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

<a id="subscriptions"></a>

## 2 套餐与订阅

版本：2.0 收口稿，2026-10-19。Gateway 现有用户购买和管理接口按下文说明；星途当前阶段仅展示套餐与订阅，购买、支付和偏好写入不属于已启用的客户端流程。商品详情与插件权益 API 仍为待开发契约。

聚合网关（Gateway）是套餐、订阅和额度账本的管理中心。运营人员在 Gateway 后台配置套餐及插件权益；星途只展示允许购买的内容、提交用户购买请求、读取最终权益并落实界面和执行限制。

本章标为“现有”的接口已经存在于当前分支；“拟新增”是待开发契约，不能作为已上线接口调用。套餐的价格/额度定义、展示详情、功能授权是三个关联到同一 `plan_id` 的对象。现有套餐接口只实现其中的价格/额度定义，插件权益与详细介绍需按下文补齐。

<a id="subscriptions--业务对象和完整接入顺序"></a>

### 业务对象和完整接入顺序



| 对象 | 唯一标识 | 负责什么 | 生效方式 |
| --- | --- | --- | --- |
| 套餐定义 | `plan.id` | 价格、期限、额度、重置、支付方式 | 管理员维护，用户读取可购项 |
| 用户订阅 | `subscription.id` | 某用户实际持有的额度和有效期 | 支付回调或管理员发放后创建 |
| 套餐详情（拟新增） | `plan_id` | 亮点、详细说明、展示顺序与角标 | 只影响商品展示，不授予权限 |
| 套餐功能（拟新增） | `plan_id + feature_id` | 授予模型功能、插件管理和指定插件 | 服务端按有效订阅计算最终权限 |
| 用户覆盖（拟新增） | `user_id + feature_id` | 单人试用、禁用、隐藏、到期恢复 | 优先于套餐，仍不能越过全局关闭 |

运营流程：创建套餐定义 → 配置详情 → 配置套餐功能 → 预览指定用户结果 → 上架。现有接口只能完成定义和上架；后面三个配置/预览能力要开发完成才能形成完整商品。

用户流程：登录 → 查询套餐与本人订阅 → 展示价格和权益 → 用户选择支付方式 → Gateway 收款/扣钱包并发放订阅 → 重新读取订阅、钱包、最终功能和插件目录 → 星途立即更新可见项。客户端不能凭支付页返回参数自行发放权益。

<a id="subscriptions--本章现有接口总表"></a>

### 本章现有接口总表



| 方法 | 地址 | 身份 | 用途 |
| --- | --- | --- | --- |
| GET | `/api/subscription/plans` | 用户 | 全部可购套餐，非分页 |
| GET | `/api/subscription/self` | 用户 | 当前及历史订阅、扣费偏好 |
| PUT | `/api/subscription/self/preference` | 用户 | 修改扣费偏好 |
| POST | `/api/subscription/balance/pay` | 用户 | 钱包购买 |
| POST | `/api/subscription/epay/pay` | 用户 | 易支付购买 |
| POST | `/api/subscription/stripe/pay` | 用户 | Stripe 购买 |
| POST | `/api/subscription/creem/pay` | 用户 | Creem 购买 |
| POST | `/api/subscription/waffo-pancake/pay` | 用户 | Waffo Pancake 购买 |
| GET | `/api/subscription/admin/plans` | 管理员 | 全部套餐，含下架，非分页 |
| POST | `/api/subscription/admin/plans` | 管理员 | 创建定义 |
| PUT | `/api/subscription/admin/plans/:id` | 管理员 | 覆盖更新定义 |
| PATCH | `/api/subscription/admin/plans/:id` | 管理员 | 只修改上下架 |
| POST | `/api/subscription/admin/bind` | 管理员 | 兼容发放入口 |
| GET | `/api/subscription/admin/users/:id/subscriptions` | 管理员 | 指定用户的全部订阅，非分页 |
| POST | `/api/subscription/admin/users/:id/subscriptions` | 管理员 | 无支付发放 |
| POST | `/api/subscription/admin/users/:id/subscriptions/reset` | 管理员 | 重置该用户持有的指定套餐额度 |
| POST | `/api/subscription/admin/plans/:id/subscriptions/reset` | 管理员 | 重置该套餐全部有效订阅额度 |
| POST | `/api/subscription/admin/user_subscriptions/:id/invalidate` | 管理员 | 立即失效 |
| DELETE | `/api/subscription/admin/user_subscriptions/:id` | 管理员 | 删除订阅记录 |

支付通知 `/api/subscription/epay/notify`（GET/POST）、返回 `/api/subscription/epay/return`（GET/POST），以及共享 Stripe/Creem/Waffo Pancake webhook 是 Gateway 与支付平台的服务端链路；星途不调用它们确认支付，也不伪造通知。

<a id="subscriptions--用户端与管理端分工"></a>

### 用户端与管理端分工



用户登录后并行拉取 `/api/subscription/plans`、`/api/subscription/self`、`/api/user/self`，分别得到可购套餐、持有订阅、钱包余额。不要把套餐定义当成用户已经购买的订阅，也不要让客户端传入任意用户 ID 查询其他人的权益。

普通用户请求使用自己的 Access Token。下面的管理端请求由 Gateway 管理后台使用管理员身份发送。若以后接独立运营服务，管理员凭证只能保存在该服务端；星途安装包只持有用户凭证，不提供第二套可独立授权的运营后台。

<a id="subscriptions--查询套餐与订阅"></a>

### 查询套餐与订阅



```http
GET /api/subscription/plans
Authorization: Bearer <USER_ACCESS_TOKEN>
```

成功的 `data` 是数组，每项结构为 `{"plan":{...}}`，不是直接的套餐对象。仅返回启用套餐；支付合规未确认时会返回空数组。

| `plan` 字段 | 星途的用途 |
| --- | --- |
| `id`、`title`、`subtitle` | 购买使用 `id`，标题仅展示 |
| `price_amount`、`currency` | 套餐价格；当前管理接口按 USD 保存 |
| `duration_unit`、`duration_value`、`custom_seconds` | 有效期；单位支持 year/month/day/hour/custom |
| `total_amount` | 新建订阅的总 quota，0 表示不限额 |
| `quota_reset_period`、`quota_reset_custom_seconds` | 周期额度重置规则；never/daily/weekly/monthly/custom |
| `allow_balance_pay` | 是否支持钱包余额购买 |
| `allow_wallet_overflow` | 新建订阅用尽时是否允许回退钱包 |
| `max_purchase_per_user` | 同套餐购买次数限制，0 表示不限 |
| `upgrade_group`、`downgrade_group` | 套餐关联的 Gateway 用户分组，不是插件 ID |
| `stripe_price_id`、`creem_product_id`、`waffo_pancake_product_id` | 相应支付渠道产品关联；不代表渠道当前已可用 |

字段类型与保存规则：`id`、`duration_value`、`sort_order`、`max_purchase_per_user` 为整数；`custom_seconds`、`total_amount`、`quota_reset_custom_seconds` 为整数；`price_amount` 为数值；两个 `allow_*` 为布尔值；其他表列配置为字符串。`enabled` 是布尔值，`created_at/updated_at` 是服务端 Unix 秒，客户端不设置。列表按 `sort_order` 降序、`id` 降序返回。

创建时标题去空白后必须非空，价格 0–9999；空 duration_unit 归为 month，非 custom 的 duration_value 小于等于 0 时归为 1；空 allow_balance_pay/allow_wallet_overflow 归为 true；max_purchase_per_user、total_amount 不能为负。duration_unit 应只提交 year/month/day/hour/custom，custom 需 custom_seconds > 0；管理保存不完整校验全部期限组合，不能将“保存成功”当成“一定能购买”。

重置周期只提交 never/daily/weekly/monthly/custom；未知值会归为 never，custom 需 quota_reset_custom_seconds > 0。daily 对齐服务端下一天零点，weekly 对齐下周一零点，monthly 对齐下月一日零点，custom 从计算基点加秒数；以接口返回的 next_reset_time 为准，星途不按本地时区重算。

<a id="subscriptions--套餐列表成功响应"></a>

#### 套餐列表成功响应



以下为字段节选，其他字段见创建请求。不能用商品名作为关联键。

```json
{
  "success": true,
  "message": "",
  "data": [
    {"plan": {"id": 7, "title": "示例月度套餐", "subtitle": "示例说明", "price_amount": 10, "currency": "USD", "duration_unit": "month", "duration_value": 1, "total_amount": 500000, "enabled": true, "sort_order": 10, "allow_balance_pay": true, "allow_wallet_overflow": true, "quota_reset_period": "daily", "quota_reset_custom_seconds": 0, "max_purchase_per_user": 1}}
  ]
}
```

```http
GET /api/subscription/self
Authorization: Bearer <USER_ACCESS_TOKEN>
```

```json
{
  "success": true,
  "data": {
    "billing_preference": "subscription_first",
    "subscriptions": [{"subscription":{"id":42,"user_id":123,"plan_id":7,"amount_total":500000,"amount_used":100000,"start_time":1900000000,"end_time":1902592000,"status":"active","next_reset_time":1900086400,"allow_wallet_overflow":true}}],
    "all_subscriptions": []
  }
}
```

以上仅示意字段。`subscriptions` 为当前有效项；`all_subscriptions` 是全部记录，真实响应会包含上述有效项及历史项。每项都包一层 `subscription`，用 `plan_id` 对应套餐。已下架套餐可能不在可购列表中，界面仍应保留这条订阅，不把它丢弃；用户侧当前没有“按任意套餐 ID 查询完整下架套餐”的专门接口。

显示订阅剩余量时，对有限额度显示 `max(0, amount_total - amount_used)`，按第一部分换算展示；`amount_total=0` 显示“不限额”。时间为 Unix 秒，`next_reset_time=0` 表示没有下一次重置。当前有效列表只筛选 active 和未到结束时间，额度耗尽的订阅仍可能在内，不能把“有效”直接显示成“还有额度”。

<a id="subscriptions--用户订阅完整字段说明"></a>

#### 用户订阅完整字段说明



| 字段 | 类型 | 含义和展示规则 |
| --- | --- | --- |
| id / user_id / plan_id | integer | 订阅、持有人、套餐三个不同 ID |
| amount_total / amount_used | integer | 此订阅周期的额度上限/已用 raw quota；不能当金额 |
| start_time / end_time | integer | Unix 秒；结束时间到达不再授予功能 |
| status | string | active、expired、cancelled；保持原值，不能与支付订单状态混用 |
| source | string | order 或 admin，表示发放来源 |
| last_reset_time / next_reset_time | integer | 最近/下次重置；0 表示无相应周期时间 |
| upgrade_group / prev_user_group / downgrade_group | string | 发放时的分组快照和回退信息 |
| allow_wallet_overflow | boolean | 该订阅是否允许钱包回退的快照 |
| created_at / updated_at | integer | 记录时间，不是付款确认时间 |

当前 `/self` 遇到订阅读取错误时会返回空列表，没有独立“账本读取失败”标记。因此未来插件权限服务必须自己可靠读取账本，失败返回不可用，不能依赖这个接口的空数组撤销或授予权限。

<a id="subscriptions--用户购买与扣费来源"></a>

### 用户购买与扣费来源



```http
PUT /api/subscription/self/preference
Authorization: Bearer <USER_ACCESS_TOKEN>
Content-Type: application/json

{"billing_preference":"subscription_first"}
```

| 值 | 含义 |
| --- | --- |
| `subscription_first` | 优先订阅，条件允许时回退钱包 |
| `wallet_first` | 优先钱包，不足时尝试订阅 |
| `subscription_only` | 只用订阅 |
| `wallet_only` | 只用钱包 |

非法值会规范化为 `subscription_first`，界面应使用保存后返回的实际值。回退钱包还受当前有效订阅的 `allow_wallet_overflow` 约束；任一有效订阅禁止回退时，订阅优先路径不能因钱包有余额就继续使用。多份订阅不能当成单次请求可任意拼接的额度池。

用钱包购买套餐：

```http
POST /api/subscription/balance/pay
Authorization: Bearer <USER_ACCESS_TOKEN>
Content-Type: application/json

{"plan_id":7}
```

钱包按 `ceil(price_amount × quota_per_unit)` 扣 quota，并创建新的订阅。成功是 `success:true`，没有可依赖的新订阅 ID；重新读取订阅和钱包确认最新数据。请求超时不要自动重复购买，先回查；该入口没有客户端幂等键。

在线支付入口如下；所有 `plan_id` 均使用实际套餐 ID，易支付的 `payment_method` 从 `/api/user/topup/info` 的配置取值：

| 接口 | JSON 请求体 | 成功时如何跳转 |
| --- | --- | --- |
| `POST /api/subscription/epay/pay` | `{"plan_id":7,"payment_method":"<PAYMENT_METHOD>"}` | 顶层 `url` 为提交目标，`data` 为表单参数 |
| `POST /api/subscription/stripe/pay` | `{"plan_id":7}` | `data.pay_link` |
| `POST /api/subscription/creem/pay` | `{"plan_id":7}` | `data.checkout_url` |
| `POST /api/subscription/waffo-pancake/pay` | `{"plan_id":7}` | `data.checkout_url` |

这些跳转接口成功使用 `message:"success"`，不能只按统一 `success` 字段判断。错误结构依渠道和失败分支而异，需要同时处理 `success:false`、`message:"error"` 和非 2xx。支付订单由 Gateway 接收渠道回调后发放订阅；星途跳转回来只触发重新查询，不自行发放，也不重复扣钱包。

现有支付响应没有统一的订单编号字段：易支付表单参数包含 data.out_trade_no；Creem、Waffo Pancake 返回 data.order_id；Stripe 当前只返回 data.pay_link。保留实际响应中已有的编号，但不能据此推导一个未提供的订单查询地址。

购买另一个套餐会新建订阅，不会自动替换现有项。当前没有用户自行编辑、取消、按差价升级或降级现有订阅的通用接口；“切换扣费来源”也不是“切换套餐”。

<a id="subscriptions--管理端操作"></a>

### 管理端操作



```http
GET /api/subscription/admin/plans
Authorization: Bearer <ADMIN_ACCESS_TOKEN_OR_PAT>
```

返回全部套餐，含未启用项，结构同样是 `data[].plan`。新建示例（数值仅示意）：

```http
POST /api/subscription/admin/plans
Authorization: Bearer <ADMIN_ACCESS_TOKEN_OR_PAT>
Content-Type: application/json

{
  "plan": {
    "title": "<PLAN_TITLE>",
    "subtitle": "<PLAN_DESCRIPTION>",
    "price_amount": 10,
    "currency": "USD",
    "duration_unit": "month",
    "duration_value": 1,
    "custom_seconds": 0,
    "enabled": true,
    "sort_order": 0,
    "allow_balance_pay": true,
    "allow_wallet_overflow": true,
    "max_purchase_per_user": 0,
    "total_amount": 500000,
    "upgrade_group": "",
    "downgrade_group": "",
    "quota_reset_period": "never",
    "quota_reset_custom_seconds": 0,
    "stripe_price_id": "",
    "creem_product_id": "",
    "waffo_pancake_product_id": ""
  }
}
```

成功的 `data` 是创建的套餐对象，保存它的 `id`。修改使用 `PUT /api/subscription/admin/plans/7` 和同样的 `{"plan":{...}}`。先从列表取原对象，再构造保留其他可编辑字段的完整请求；这是覆盖更新，不能只传一个标题。标题不能为空，价格范围为 0–9999，当前货币固定 USD；指定升级/降级组时必须是 Gateway 已存在的组。

只上下架用：

```http
PATCH /api/subscription/admin/plans/7
Authorization: Bearer <ADMIN_ACCESS_TOKEN_OR_PAT>
Content-Type: application/json

{"enabled":false}
```

当前没有删除套餐接口。套餐更新不等于修改所有已发出的订阅：额度、有效期、升降级组和钱包回退等是在创建订阅时写入的快照；周期重置相关逻辑会读取套餐定义，不能假定所有字段都永久快照化。星途收到套餐编辑成功后分别刷新套餐列表和用户订阅。

PUT 成功与 PATCH 成功都返回 `{"success":true,"message":"","data":null}`，不返回更新后的完整套餐，需重读管理员列表。PUT 未传的两个 allow_* 指针字段保留原值，其余可编辑字段应完整提交。现有管理写入不提供 expected_revision；多管理员同时编辑可能覆盖，运营界面需先刷新再保存，不能宣称已有乐观锁。

<a id="subscriptions--管理用户订阅"></a>

#### 管理用户订阅



| 操作 | 接口与请求 | 结果 |
| --- | --- | --- |
| 查询 | `GET /api/subscription/admin/users/123/subscriptions` | `data[].subscription`，包括历史记录 |
| 新增 | `POST /api/subscription/admin/users/123/subscriptions`，`{"plan_id":7}` | 从套餐创建订阅，不收款；成功后回查列表 |
| 兼容绑定入口 | `POST /api/subscription/admin/bind`，`{"user_id":123,"plan_id":7}` | 与上面的新增操作二选一，不能都调用 |
| 立即失效 | `POST /api/subscription/admin/user_subscriptions/42/invalidate`，无请求体 | 该订阅停止生效，保留记录 |
| 删除记录 | `DELETE /api/subscription/admin/user_subscriptions/42` | 硬删除，后续历史列表也不再返回 |

上述请求全部携带管理员凭证。`123` 是用户 ID、`7` 是套餐 ID、`42` 是用户订阅 ID，不能互换。新增与付费购买共用套餐次数限制。失效不是退款，也不是支付渠道侧的自动续费取消；删除更不能替代退款。当前没有任意 `PUT /user_subscriptions/:id` 修改结束时间或额度的接口，星途不要构造不存在的“订阅编辑”请求。

管理端创建/修改/上下架套餐、发放订阅，以及用户购买，均要求 Gateway 已确认支付合规；星途不能通过换成管理员身份绕开该前提。手册仅说明这一接口条件，不要求客户端修改平台合规配置。

<a id="subscriptions--重置指定用户的指定套餐额度-现有"></a>

#### 重置指定用户的指定套餐额度（现有）



```http
POST /api/subscription/admin/users/123/subscriptions/reset
Authorization: Bearer <ADMIN_ACCESS_TOKEN_OR_PAT>
Content-Type: application/json

{"plan_id":7,"advance_reset_time":false}
```

| 字段 | 类型 | 必填 / 默认 | 作用 |
| --- | --- | --- | --- |
| 路径 id | integer | 必填，> 0 | 用户 ID |
| plan_id | integer | 必填，> 0 | 只匹配这个套餐 |
| advance_reset_time | boolean | 可选，默认 true | false 仅清空已用；true 还从本次时间重新计算重置日程 |

匹配该用户此套餐全部 status=active 且 end_time > now 的订阅，不是只选第一份；没有有效匹配时业务失败。每条 amount_used 归零，amount_total、end_time、钱包不变。

```json
{"success":true,"message":"","data":{"plan_id":7,"matched_count":2,"reset_count":2,"user_count":1,"advance_reset_time":false}}
```

true 不是“有效期延长”：它更新 next_reset_time/last_reset_time，不能把 end_time 延后。false 保留原定重置时间；如果马上到达原重置点，还会发生正常周期重置。重置后重读订阅，显示实际 next_reset_time。

<a id="subscriptions--重置一个套餐的全部有效订阅-现有"></a>

#### 重置一个套餐的全部有效订阅（现有）



```http
POST /api/subscription/admin/plans/7/subscriptions/reset
Authorization: Bearer <ADMIN_ACCESS_TOKEN_OR_PAT>
Content-Type: application/json

{"advance_reset_time":true}
```

路径 7 为套餐 ID；请求体必须是 JSON 对象。该接口不使用 body.plan_id。行为与上项相同，但跨该套餐全部持有人；没有匹配时成功且计数为 0。返回相同计数结构，user_count 是去重用户数。此接口有全体影响，Gateway 后台应明确显示目标套餐与操作范围。

重置、失效和删除控制器当前没有与套餐编辑相同的 requirePaymentCompliance 检查；鉴权仍要求管理员。两项重置会记录管理审计。它们都不是付款、退款、补发新订阅或变更插件授权的入口；没有客户端幂等键，超时先回查结果再决定是否操作。

<a id="subscriptions--套餐详情和插件权益如何配置-拟新增"></a>

### 套餐详情和插件权益如何配置（拟新增）



**以下 `/api/apps/starpath/...` 接口当前未实现。** Gateway 后台套餐编辑应分为“计费定义、展示详情、功能权益”三个区块，保留同一 plan_id；星途只读取聚合后的可购详情和本人最终权益。

| 方法 | 拟定地址 | 身份 | 作用 |
| --- | --- | --- | --- |
| GET | `/api/apps/starpath/plans` | 用户 | 返回启用套餐及其允许公开的详情、权益说明 |
| GET | `/api/apps/starpath/plans/:plan_id` | 用户 | 可购套餐完整详情；下架项返回 404 |
| GET | `/api/apps/starpath/admin/plans/:plan_id/presentation` | 管理员 | 编辑前读取详情和 revision |
| PUT | `/api/apps/starpath/admin/plans/:plan_id/presentation` | 管理员 | 完整替换展示信息 |
| GET / PUT | `/api/apps/starpath/admin/plans/:plan_id/features` | 管理员 | 读取/替换授予的稳定功能 ID |

展示字段契约：

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| expected_revision | integer | PUT 必填，首次 0；冲突 409，不覆盖 |
| badge | string | 0–32 字符，空串去掉角标 |
| highlights | string[] | 0–20 条，每条 1–200 字符；纯文本 |
| description_markdown | string | 最长 20000 字符；渲染需净化，禁任意脚本和嵌入执行 |
| terms_markdown | string | 最长 10000 字符；使用范围、限制、退款说明，不能代替支付规则实现 |
| recommended | boolean | 仅显示推荐样式，不影响排序或权限 |
| reason | string | 1–500 字符，管理审计原因，不下发用户 |

```http
PUT /api/apps/starpath/admin/plans/7/presentation
Authorization: Bearer <ADMIN_ACCESS_TOKEN_OR_PAT>
Content-Type: application/json

{"expected_revision":0,"badge":"推荐","highlights":["图片创作","已授权业务插件"],"description_markdown":"适用于日常创作。具体可用模型由账号和密钥共同决定。","terms_markdown":"订阅有效期和额度以 Gateway 返回值为准。","recommended":true,"reason":"完善套餐介绍"}
```

拟定成功：`{"success":true,"message":"","data":{"plan_id":7,"revision":1}}`。GET 返回相同展示字段和 revision，不含 expected_revision/reason。所有拟新增管理写入均先验证完整对象再事务保存。

拟定用户 `GET /api/apps/starpath/plans/7` 响应：

```json
{
  "success":true,
  "message":"",
  "data":{
    "plan":{"id":7,"title":"示例月度套餐","price_amount":10,"currency":"USD","duration_unit":"month","duration_value":1,"total_amount":500000},
    "presentation":{"revision":1,"badge":"推荐","highlights":["图片创作","已授权业务插件"],"description_markdown":"适用于日常创作。","terms_markdown":"订阅有效期和额度以 Gateway 返回值为准。","recommended":true},
    "features":[{"id":"image.generate","name":"图片创作"},{"id":"plugin:example.storyboard","name":"示例分镜插件"}],
    "features_revision":2
  }
}
```

`example.storyboard` 仅为本文契约示例，不代表现有已登记插件。公开 feature 说明只来自全局 available=true 的目录项；不泄露全局隐藏项。商品介绍用于解释购买内容，最终能否使用只能读本人 entitlements。`GET /plans` 的 data 为同结构数组，非分页；支付合规未确认时为空，详情 404；隐藏/下架套餐不能靠 ID 绕过购买限制。历史已购套餐展示使用订阅数据和本地留存标题，不能开放任意下架详情。

配置套餐功能请求示例：

```http
PUT /api/apps/starpath/admin/plans/7/features
Authorization: Bearer <ADMIN_ACCESS_TOKEN_OR_PAT>
Content-Type: application/json

{"expected_revision":1,"feature_ids":["image.generate","plugins.manage","plugin:example.storyboard"],"reason":"配置创作套餐"}
```

该列表原子替换；`[]` 清空本套餐授权，其他套餐和个人覆盖仍参与计算。修改影响已持有该套餐的有效用户，不能只更新购买页；要保持旧用户原权益，应新建另一套餐。费用定义保存和权益保存当前/拟定为不同请求，不能伪装跨接口原子成功；Gateway 后台需分别显示保存结果并回查。

<a id="subscriptions--常见运营操作与两端结果"></a>

#### 常见运营操作与两端结果



| Gateway 后台动作 | Gateway 账本/权限结果 | 星途应该发生什么 |
| --- | --- | --- |
| 上架/下架套餐 | 只改可购状态，原有效订阅保留 | 更新购买页；不能隐藏既有已授权插件 |
| 修改价格/额度 | 价格用于后续购买；既有额度快照不自动改 | 刷新商品，不把本地订阅额度改成新数值 |
| 修改套餐功能（拟新增） | 所有当前有效持有人重新计算 | 下一次刷新更新入口和新执行权限 |
| 给用户发放套餐 | 新建有效订阅，不扣款 | 刷新钱包/订阅/功能/插件目录 |
| 失效一份订阅 | 该份停止参与；其他有效套餐仍参与 | 移除失去的权限；保留仍由其他来源授予的功能 |
| 额度耗尽/周期重置 | 只影响计费可用额，不直接改变功能授权 | 保留入口，调用前显示额度状态 |
| 用户单独隐藏插件（拟新增） | 覆盖套餐授予，直到撤销或到期 | 隐藏各入口并阻断该插件新执行 |
| 全局关闭插件（拟新增） | 所有用户隐藏；个人例外无效 | 停止新执行，保留配置与历史 |

<a id="subscriptions--错误与支付确认边界"></a>

#### 错误与支付确认边界



现有订阅接口的参数、业务与数据库失败主要通过 `success:false/message` 返回，不能假定全部为 400 或存在稳定 error_code。常见失败包括支付合规未确认、套餐不存在/未启用、购买次数超限、钱包不足、钱包购买关闭、组不存在。鉴权失败另由中间件决定 HTTP 状态；UI 同时处理网络、HTTP 和业务三层结果。

未来展示/授权接口采用第三章的稳定错误码：400 INVALID_ARGUMENT、401 AUTH_REQUIRED、403 FORBIDDEN、404 RESOURCE_NOT_FOUND、409 POLICY_REVISION_CONFLICT、503 ENTITLEMENTS_UNAVAILABLE。支付创建超时仍是“结果待确认”，不能按这些设计错误码推断已退款。

<a id="subscriptions--操作完成后的回查"></a>

### 操作完成后的回查



购买、发放、失效或删除成功后，按[密钥与插件权益](#api-keys--权益变化后的同步)统一回查。到期由 Gateway 的维护任务处理，星途按返回状态显示，不自行延长订阅。

当前没有统一的用户订阅订单列表/详情接口；还缺客户端操作幂等键、订单与具体订阅的持久化关联，以及订阅/钱包权益通知。超时后先回查，不自动重复购买或发放；已有支付回调去重不等于客户端重发请求会去重。升级差价、下周期切换、取消续费和退款也没有通用接口，需要另定业务规则。

<a id="subscriptions--套餐对接任务与验收"></a>

### 套餐对接任务与验收



| 任务 | 责任与依赖 | 验收条件 |
| --- | --- | --- |
| 现有只读展示 | 星途公司适配；使用现有登录 | 可购套餐与本人订阅分别解析；换号/刷新失败不串数据，不显示虚假的零余额 |
| 展示详情 | Gateway 后端/前端；拟新增 presentation | 计费定义、展示详情、功能授权分开保存和回查；不能一项成功就提示全部保存 |
| 套餐关联功能 | Gateway 权限服务；拟新增 features | 以 plan_id 关联，当前有效订阅取并集；变更影响当前持有人，运营能预览影响 |
| 本人同步 | 星途权限快照；依赖 entitlements | 下架不撤销有效订阅；失效一份仍保留其他来源；额度重置不误撤权 |
| 客户端购买 | 后续单独确认产品范围与支付闭环 | 统一未知结果处理、订单到订阅关联及幂等方案明确后再接 UI，不借旧总开关恢复购买 |

管理员发放用于明确测试账号的隔离联调，不能将“发放订阅成功”写成“真实支付通过”。一个真实支付渠道的到账也不能替代其他渠道、退款或续费验收。每次记录实际 plan_id、用户订阅 ID、前后状态和脱敏证据，不在手册中预填真实账号/价格。

套餐授权修改与插件发包是两条链路：改 feature_ids 只重新计算权限；需要代码变化才设置新的发布物及统一目标。新权限接口出现前，模型列表、本机企业 UI 配置和套餐标题都不能作为插件授权结果。

源码依据：`controller/subscription.go`；`model/subscription.go`；`controller/subscription_payment_*.go`；`common/str.go`；`service/billing_session.go`；`service/subscription_reset_task.go`。以上为源码契约，实际支付渠道、到期和两平台同步尚未完成联调。

<a id="api-keys"></a>

## 3 密钥与插件权益

登录后用当前用户的会话管理其模型密钥，再把可用模型和套餐权益同步到星途。模型 Key、钱包额度、订阅权益和插件开关各有职责，统一在本章完成配置与回查。

版本：2.0 收口稿，2026-10-19。现有密钥接口、待开发的权限/插件/统一发布契约，以及两端实施规划分节说明；规划和接口样例不代表业务功能已经上线。

Gateway 负责功能目录、插件商品资料、套餐授予、个人例外、版本包和发布策略；星途负责读取、显示、安装和执行。星途本地配置只能进一步限制，不能成为另一份能扩大服务端授权的规则源。本章 `/api/token/...` 等为现有能力，全部 `/api/apps/starpath/...` 为拟新增、尚未实现。

<a id="api-keys--密钥和模型目录接口总表-现有"></a>

### 密钥和模型目录接口总表（现有）



| 方法 | 地址 | 身份 | 请求与返回要点 |
| --- | --- | --- | --- |
| GET | `/api/token/` | 用户 | p/page_size 分页，返回 data.items |
| GET | `/api/token/search` | 用户 | 按 keyword/token 查询本人密钥；返回分页对象 |
| GET | `/api/token/:id` | 用户 | 本人单条配置，key 脱敏 |
| POST | `/api/token/:id/key` | 用户 | 取得明文 data.key |
| POST | `/api/token/` | 用户 | 创建；仅 success/message，无 ID |
| PUT | `/api/token/` | 用户 | 完整修改；status_only=1 只改状态 |
| DELETE | `/api/token/:id` | 用户 | 删除一条 |
| POST | `/api/token/batch` | 用户 | `{"ids":[42,43]}` 批量删除 |
| POST | `/api/token/batch/keys` | 用户 | `{"ids":[42,43]}` 批量取明文，谨慎按需使用 |
| GET | `/api/token/auto-groups` | 用户 | 自动组候选和最大数量 |
| GET | `/api/user/models` | 用户 | 可选模型名数组，可按 group 筛选 |
| GET | `/api/user/self/groups` | 用户 | 可选分组 |
| GET | `/v1/models` | 模型 Key | 该密钥实际模型目录，data[].id |
| GET | `/v1/models/:model` | 模型 Key | 单模型目录项，不是完整参数规格 |
| GET | `/api/pricing` | 由导航策略决定 | 价格和部分协议元数据，不是逐请求报价 |

<a id="api-keys--获取星途使用的用户密钥"></a>

### 获取星途使用的用户密钥



`/api/token/` 只管理当前身份自己的密钥，管理员调用也不会自动变成替其他用户操作。管理账号用于星途服务端的运营操作；文本、图片、视频调用使用用户自己的模型 Key，费用归该用户。

```http
GET /api/token/?p=1&page_size=20
Authorization: Bearer <USER_ACCESS_TOKEN>
```

响应 `data.items` 为密钥列表，含 page/page_size/total。先让用户选择已有密钥，保存其 `id`；需要时再创建专用密钥，避免每次登录重复创建。名称不是唯一标识。

```http
POST /api/token/
Authorization: Bearer <USER_ACCESS_TOKEN>
Content-Type: application/json

{
  "name": "<XINGTU_KEY_NAME>",
  "expired_time": -1,
  "remain_quota": 0,
  "unlimited_quota": true,
  "model_limits_enabled": false,
  "model_limits": "",
  "allow_ips": "",
  "group": "",
  "cross_group_retry": false
}
```

示例让密钥跟随用户组、不另设密钥预算；钱包与订阅仍正常扣费。若要限制模型或预算，明确设置相应字段。

创建成功只返回 success:true 和 message，没有新密钥 ID 或明文。重新查询列表，结合创建前的 ID 确认新增项；名称不是唯一标识，不能只按名称取第一条。同一账号串行处理创建，结果不明先回查，不自动重发。当前没有原子“创建并返回 ID”或“按应用幂等初始化密钥”的接口，这两项属于待补能力。

拿到 ID 后取完整 Key：

```http
POST /api/token/42/key
Authorization: Bearer <USER_ACCESS_TOKEN>
```

```json
{"success":true,"data":{"key":"sk-<USER_MODEL_KEY>"}}
```

直接使用 `data.key`，不再拼前缀。列表和 `GET /api/token/42` 的 key 已脱敏，不能用于模型调用。密钥归属与缓存按用户隔离，退出或换号时清理当前调用配置。

<a id="api-keys--密钥配置与修改"></a>

### 密钥配置与修改



| 字段 | 含义 |
| --- | --- |
| id / name | ID 用于关联；名称仅展示 |
| status | 1 启用、2 禁用、3 过期、4 额度耗尽 |
| expired_time | Unix 秒；-1 不过期，0 已过期 |
| unlimited_quota / remain_quota | 密钥调用上限，不是另一份钱包余额 |
| model_limits_enabled / model_limits | 模型白名单开关；名称以英文逗号分隔 |
| group | 空字符串跟随用户组；具体组固定选择；auto 使用自动组 |
| auto_groups / cross_group_retry / allow_ips | 自动组候选、跨组重试及来源限制 |

普通修改用 `PUT /api/token/`：先读取原对象，保留全部可编辑字段再修改目标字段，不能只传 ID 和 group。修改不轮换密钥字符串。

仅停用用以下请求；启用将 status 改为 1，并先解决过期或额度耗尽条件：

```http
PUT /api/token/?status_only=1
Authorization: Bearer <USER_ACCESS_TOKEN>
Content-Type: application/json

{"id":42,"status":2}
```

删除用 `DELETE /api/token/42`。换 Key 采用创建新密钥、切换调用端、删除旧密钥的顺序，没有原地重置字符串入口。

<a id="api-keys--模型与插件可见性"></a>

### 模型与插件可见性



| 星途需要的内容 | 当前 Gateway 能力 |
| --- | --- |
| 用户可用模型 | `GET /api/user/models`，可用 group 查询参数筛选 |
| 用户可选组 | `GET /api/user/self/groups` |
| auto 候选组 | `GET /api/token/auto-groups`，返回 groups/max_count |
| 实际选中密钥的模型目录 | `GET /v1/models`，使用该用户的模型 Key，读取 `data[].id` |
| 价格及部分协议元数据 | `GET /api/pricing`，受 pricing 导航开放策略控制 |
| 插件、技能、MCP 的最终权益 | 当前没有统一接口 |

用户组、渠道和密钥白名单约束实际模型调用。Gateway 的 sidebar_modules、HeaderNavModules 等只控制自身导航，不是星途插件授权。

`/api/user/models` 使用登录 Access Token，data 是模型名字符串数组，范围是用户可选组；未传 group 时可能合并多个组。调用配置应再用选中的模型 Key 请求：

```http
GET /v1/models
Authorization: Bearer <FULL_USER_MODEL_API_KEY>
```

默认 OpenAI 风格返回 `object:"list"` 与 `data[].id`。该目录结合密钥组、模型白名单及计费配置筛选，更适合星途实际调用的模型选择器；不是把用户全部可选模型直接绑定到任意密钥。目录存在仍不保证本次有可用额度或上游实时可用。

星途已有本机企业配置，可用 `ui["settings.plugins"] = "hide"` 隐藏插件设置导航，但这不是 Gateway 按用户下发的授权，也不会自动阻止已安装插件执行。源码保留的“团队管理后台”是外部网页入口，当前产品配置的后台地址为空，旧企业身份也不同于 Gateway 用户，不能作为已经可用的星途管理端。

模型名称列表和 pricing 还不能提供完整的图片/视频参数表单；尺寸、时长、媒体角色和协议版本等能力规格需要固定映射或后续能力目录。不要把模型 ID、套餐 ID、插件 ID 混成一个标识。

<a id="api-keys--必须补齐的用户功能与插件权限"></a>

### 必须补齐的用户功能与插件权限



**以下是必须建设的对接方案，尚未实现；本轮只整理文档。** 运营人员在 Gateway 管理侧栏的“星途功能权限”统一配置目录、套餐授权和用户覆盖；星途不另建账号或订阅账本，也不把管理员凭证放进下载客户端。

同一安装包按登录用户显示不同功能。文本、图片、Wan 视频、Seedance 视频、素材库、插件管理页和具体业务插件分别登记稳定功能 ID。`plugins.manage` 只控制插件管理，`plugin:<REGISTERED_PLUGIN_ID>` 控制对应插件的可见与运行；隐藏管理页不等于禁用全部插件。内置核心插件保持正常工作。

`plugin:<REGISTERED_PLUGIN_ID>` 使用 Gateway 登记的 plugin_id，再通过已核验映射定位 manifest.runtime_id；两者恰好相同也不能省略映射核对。package_id、runtime_id、channel_id 的差异见下文源码调研。

| 最终状态 | 界面 | 新操作 |
| --- | --- | --- |
| hidden | 不显示入口或搜索项 | 拒绝 |
| disabled | 显示但不可用，说明原因 | 拒绝 |
| enabled | 正常显示 | 仍需通过 Key、模型、额度等检查 |

规则由服务端计算：全局关闭优先；否则从默认状态开始，合并全部有效订阅的套餐授权，再应用未到期的用户单独覆盖。本机企业配置只能进一步限制，不能扩大权限。取消覆盖恢复继承；套餐下架不撤销已有有效订阅，额度重置/耗尽不等于订阅失效。admin/root 身份也不自动获得全部业务功能。

权限基础接口如下，**当前均未提供，不可直接调用**；插件目录、版本和发布接口见本章后续各节：

| 接口 | 身份和用途 |
| --- | --- |
| `GET /api/apps/starpath/entitlements` | 用户读取自己的最终功能权限，不可代查其他用户 |
| `POST /api/apps/starpath/model-key` | 用户幂等取得/初始化服务端绑定星途的模型 Key，返回 token_id |
| `POST /api/apps/starpath/config-reports` | 用户回报本机已应用的权限/目录版本；仅作为观察记录 |
| `GET /api/apps/starpath/admin/features` | 管理员查看功能目录 |
| `PUT /api/apps/starpath/admin/features/:feature_id` | root 控制全局开放及默认状态 |
| `GET/PUT /api/apps/starpath/admin/plans/:plan_id/features` | 管理员查询/替换套餐授权 |
| `GET/PUT /api/apps/starpath/admin/users/:user_id/feature-overrides` | 管理员查询/替换用户覆盖；空列表恢复继承 |
| `GET /api/apps/starpath/admin/users/:user_id/entitlements` | 管理员预览最终结果及来源 |
| `GET /api/apps/starpath/admin/users/:user_id/config-reports` | 管理员查看目标用户各客户端最近的配置回报 |

未来用户读取示例：

```http
GET /api/apps/starpath/entitlements
Authorization: Bearer <USER_ACCESS_TOKEN>
```

拟定 `data` 示意：

```json
{
  "schema_version": 1,
  "application_id": "starpath",
  "user_id": 123,
  "effective_version": "<OPAQUE_EFFECTIVE_VERSION>",
  "server_time": 1800000000,
  "valid_until": 1800000060,
  "refresh_after_seconds": 30,
  "features": [
    {"id": "image.generate", "state": "enabled"},
    {"id": "video.seedance", "state": "disabled", "reason_code": "PLAN_REQUIRED"}
  ]
}
```

hidden 项不返回；已知功能未出现即隐藏并拒绝使用，未知 ID 不默认开放。时间单位为 Unix 秒，示例值需替换；普通用户不读取完整管理目录。完整管理请求、字段、版本冲突和验收条件见仓库 `docs/design/STARPATH_FEATURE_ACCESS.zh_CN.md`。

星途主进程复用 Gateway 会话读取快照，按地址、用户和登录代次隔离；退出/换号立即清理，丢弃旧响应。登录、订阅变化、唤醒/回到前台及网络恢复时刷新，前台正常联网约每 30 秒回查；快照最长有效 60 秒且不跨已知到期点。首次读取失败或快照过期时，不放开新的受控操作，也不删除本地成果。

界面入口、快捷键、页面直达、搜索、插件管理 IPC、业务插件加载和工具执行都要使用同一权限。云能力还需在 Gateway 按认证用户和专用 Key 的可信应用绑定检查，先鉴权再预扣费；当前 Key 尚无此应用绑定，不能靠名称或自报请求头替代。纯本地插件由星途执行端落实，不能承诺修改客户端后仍有服务端同等强制控制。

已有任务查询/取消、成果回收和账务查询继续按归属检查；撤销新生成功能不等于删除历史。管理变更沿用写操作审计，并处理多节点缓存失效；专用 Key 初始化、运行校验和权限同步都必须实现后才能称为两平台权限联动。

<a id="api-keys--本人权限与专用模型-key-的完整契约"></a>

#### 本人权限与专用模型 Key 的完整契约



`GET /api/apps/starpath/entitlements` 无请求体、无 user_id 参数，仅接受当前用户登录身份。data 的 schema_version 为整数，目前拟定 1；application_id 固定 starpath；user_id 为认证用户 ID；effective_version 是比较变化的不透明字符串；server_time/valid_until 是 Unix 秒；refresh_after_seconds 为建议回查秒数；features 是可见功能数组，每项 id/state 必填、reason_code 可选。有效期最长 60 秒，不能晚于最近的订阅/覆盖到期；空数组不等于全部开放。与服务端目录不一致或未知 ID 都拒绝默认开放。

`POST /api/apps/starpath/model-key` 使用用户 Access Token，body 为 `{}`，不允许指定 user_id/application_id/group/权限。按当前用户和 starpath 原子取得或初始化唯一当前绑定，重复和并发请求返回同一 token_id，不需要靠密钥名查重：

```json
{"success":true,"message":"","data":{"application_id":"starpath","token_id":42,"created":false,"status":1}}
```

首次创建 created=true；有效绑定已存在则 false。status 沿用现有 Key 的 1/2/3/4，停用/过期/耗尽照实返回，不自动新建绕过；绑定的 Key 被删除时返回 409 APP_KEY_REBIND_REQUIRED，需用户显式确认新绑定，不能默默把另一个普通 Key 认作专用 Key。首期不新增自动轮换接口，主动轮换需另定明确流程。

此接口不返回明文；有 token_id 后由现有 `POST /api/token/42/key` 取明文。应用绑定必须由服务端保护，普通 Key 编辑不得取消；专用 Key 在星途外调用也受应用功能约束。接口幂等不等于所有现有 token 创建请求都幂等，后者仍按本章现有流程处理。

<a id="api-keys--可信应用身份与插件来源的边界"></a>

#### 可信应用身份与插件来源的边界



专用 Key 只能证明服务端保存的应用归属，不能证明一次通用模型请求实际来自哪个插件。Gateway 应通过认证身份、Key 绑定、路由及已验证的模型/业务动作判断可检查的功能；不得信任客户端自报 plugin_id、X-App-ID、feature_id 或安装版本作为授权依据。

例如两个插件都请求 `/v1/chat/completions`，仅看 Key 或请求头无法区分调用来源。通用文本路由只能落实它可确定的 text.chat 等能力；如果某插件要求服务端强制限制其专属云业务，需先建设可验证的独立业务服务边界。没有这条边界时，该插件只能验收受支持客户端的本机执行限制，不能宣称已做到防修改客户端的远程强制许可。

图片/视频等云能力同样要覆盖别名协议、模型映射及素材入口，在鉴权和参数验证之后、预扣费与上游副作用之前检查。历史查询、取消和成果回收继续按资源归属放行；关闭“新生成”不应阻止用户取回已有成果。纯本地工具、办公技能及 MCP 的授权仍按各自既有规则，本次不重写其核心执行链。

<a id="api-keys--配置生效回报与管理端观察-拟新增"></a>

#### 配置生效回报与管理端观察（拟新增）



“后台修改 → 星途获取 → 本机应用 → 回报结果”需要独立于安装结果的配置回报。以下两个接口也未实现；`plugin-updates/report` 只描述包安装，不能替代权限或目录同步的回报。

```http
POST /api/apps/starpath/config-reports
Authorization: Bearer <USER_ACCESS_TOKEN>
Content-Type: application/json

{"report_id":"c496b43e-1a56-4ce3-9e1c-02bf4c7e58a1","client_instance_id":"4d53e068-4c10-4bd3-b798-f3616aa3a0f4","client_version":"2026.9.23","effective_version":"<OPAQUE_EFFECTIVE_VERSION>","catalog_revision":3,"result":"partial","failed_items":[{"feature_id":"image.generate","reason_code":"LOCAL_APPLY_FAILED"}]}
```

| 字段 | 类型与约束 | 作用 |
| --- | --- | --- |
| report_id | UUID | 当前账号/客户端的一次回报标识，重试保持不变 |
| client_instance_id | UUID | 本机首次启动生成的随机标识，不采集硬件序列号；换号仍按服务端真实用户隔离 |
| client_version | string，1–64 字符 | 实际主应用版本；不赋予版本兼容或权益 |
| effective_version | string，1–256 字符 | 本机处理的权限快照版本；报告过期版本仍可记录，但标为落后 |
| catalog_revision | integer，可省略，>=0 | 本机已应用的目录版本；未读目录时省略，不伪填为最新 |
| result | applied / partial / failed | 本机显示与守卫配置的应用结果，不表示安装成功或实际执行已验证 |
| failed_items | array，最多 100 项 | feature_id 不重复且为已登记 ID；reason_code 仅 LOCAL_APPLY_FAILED / CLIENT_UNSUPPORTED / ACCOUNT_CHANGED |

result=applied 时 failed_items 必须为空；partial/failed 时至少一项。body 不接受 user_id、application_id、密钥、配置明文、文件路径或任意日志。身份来自当前用户 Access Token，application_id 固定 starpath。换号后丢弃旧账号待发回报，不能借新会话替旧账号回报。

拟定成功：`{"success":true,"message":"","data":{"report_id":"c496b43e-1a56-4ce3-9e1c-02bf4c7e58a1","received_at":1800000000}}`。received_at 由服务端产生；同账号、client_instance_id、report_id 在保留期内相同 body 重放原结果，不同 body 返回 409 CONFIG_REPORT_CONFLICT。记录保留 30 天；过期重试创建新的 report_id，不承诺永久去重。

管理员 `GET /api/apps/starpath/admin/users/123/config-reports?p=1&page_size=20` 返回 `data.items/total/page/page_size`，page_size 默认 20、最大 100。items 包含以上回报字段、服务端 received_at 和查询时计算的 sync_state。sync_state=current 表示 effective_version 及已上报的目录版本与当前值一致；behind 表示已确定不同、需要刷新；暂不能求得当前版本则 unknown，不把读取故障标成同步完成。不透明版本只比较相等，不比较大小。

无回报时 items=[]、total=0；首期每个 client_instance_id 展示最近一条，按 received_at 降序分页，30 天无记录显示未回报，不按客户端自报时间排序。客户端在首次应用、配置变化、前台恢复后重新应用或应用失败时回报；重复失败按退避重试，不为每个卡片建立定时器，回报时间也不作为设备实时在线证明。

管理界面显示“客户端已回报”“部分应用失败”“版本落后”或“未回报”。自报 applied 不能标成服务端验证成功，不能延长快照、恢复停用 Key、批准安装或授予权限；实时云鉴权与本机执行守卫仍独立进行。多设备分别观察，当前没有稳定的设备推送或指令执行通道，按快照刷新与回报收敛。

<a id="api-keys--gateway-后台具体控制哪些内容-拟新增"></a>

### Gateway 后台具体控制哪些内容（拟新增）



拟在 Gateway 的“星途管理”下设置功能权限、插件目录、版本发布三个页面，套餐和用户页可直达对应授权配置。下列操作均在 Gateway 完成，星途没有独立发布权。

| 运营意图 | Gateway 配置 | 星途消费位置 | 不应发生的副作用 |
| --- | --- | --- | --- |
| 所有人看不到插件 X | X 对应功能 available=false | 导航、插件列表、搜索、快捷键、执行入口 | 不卸载插件，不删配置 |
| 未购买可见但用不了 | 默认 disabled，reason_code=PLAN_REQUIRED | 插件卡片显示购买引导 | 不靠卡片文字授予权限 |
| 只给套餐 7 使用 | 套餐 feature_ids 包含 X | 本人权限变为 enabled | 不改变模型白名单或给钱包加钱 |
| 单个用户临时试用 | 用户覆盖 enabled，设置 expires_at | 到期点前可用，到期自动恢复继承 | 不生成虚假付费订阅 |
| 隐藏插件管理页但允许 X 运行 | plugins.manage=hidden，X=enabled | 设置页隐藏，已授权 X 按原入口运行 | 不把父菜单隐藏当作全部插件禁用 |
| 修改插件名称、图标、说明、位置 | 插件目录元数据 | 卡片/详情刷新 | 不改 runtime_id，不触发安装包更新 |
| 发布新版本提示 | 发布 release 并建立 optional 策略 | 版本提示、更新按钮 | 不直接安装任意 latest |
| 强制升级后才能运行 | required 策略、最低版本、截止时间 | 到期未升级仅阻断这个插件新执行 | 不阻断登录、历史查看或其他插件 |
| 全体统一使用指定版本 | 唯一当前版本策略，后台设目标与期限 | 所有获授权用户执行同一版本要求 | 不提供用户自主选版入口 |
| 版本出故障回退 | 暂停故障发布，将当前统一策略切为 rollback | 精确回到指定包或提示需人工处理 | 不覆盖旧发布包，不回退审计 revision |

显示状态和版本要求要分别计算。例如 X 的权限为 enabled，但当前安装版被撤销，则卡片仍可展示“需要更新”，执行被版本门槛阻断；未获 X 权限则不能因拿到新版包而运行。权限全局关闭优先于任何版本策略。

<a id="api-keys--功能目录字段和读取"></a>

#### 功能目录字段和读取



`GET /api/apps/starpath/admin/features`：管理员读取，data 为 `{revision,items}`，不分页；items 含全部登记项，包括全局关闭项。每项字段如下：

| 字段 | 类型 | 含义 |
| --- | --- | --- |
| id | string | 稳定功能 ID，最多 128 个 ASCII 字符 |
| kind | string | capability 或 plugin；登记后不可换类型 |
| name / description | string | 管理展示，1–100 / 0–2000 字符 |
| available | boolean | false 全局隐藏且拒绝新使用 |
| default_state | string | hidden、disabled、enabled，首次必须显式配置 |
| plugin_id | string 或 null | plugin 类型关联的不可变插件 ID |
| revision | integer | 这一条功能的编辑版本；新项首次 0 |
| updated_at | integer | 服务端 Unix 秒 |

```json
{"success":true,"message":"","data":{"revision":9,"items":[{"id":"plugin:example.storyboard","kind":"plugin","name":"示例分镜插件","description":"本文契约示例","available":true,"default_state":"disabled","plugin_id":"example.storyboard","revision":3,"updated_at":1800000000}]}}
```

`PUT /api/apps/starpath/admin/features/:feature_id`：root 写入单条功能。`:feature_id` 按 URL 路径段编码。新项只能登记当前客户端支持的 capability 或已经建档的业务 plugin；不能传任意执行地址。PUT 完整提交可编辑字段，写入时同时增加该项 revision 和目录 revision。

```http
PUT /api/apps/starpath/admin/features/plugin%3Aexample.storyboard
Authorization: Bearer <ROOT_ACCESS_TOKEN>
Content-Type: application/json

{"expected_revision":3,"kind":"plugin","name":"示例分镜插件","description":"按套餐开放","plugin_id":"example.storyboard","available":true,"default_state":"disabled","reason":"开放套餐购买入口"}
```

拟定成功：`{"success":true,"message":"","data":{"id":"plugin:example.storyboard","revision":4,"catalog_revision":10}}`。隐藏用同一 PUT 的 available=false，不删除 ID；恢复时必须再经过原授权计算，不全体自动启用。

<a id="api-keys--套餐授权的读取-替换和生效"></a>

#### 套餐授权的读取、替换和生效



`GET /api/apps/starpath/admin/plans/7/features`：管理员；套餐必须存在，下架套餐允许管理。

```json
{"success":true,"message":"","data":{"plan_id":7,"revision":2,"feature_ids":["image.generate","plugins.manage","plugin:example.storyboard"]}}
```

`PUT` 同地址，必填 expected_revision（>=0）、feature_ids（数组，最多 500 个，去重且只能包含已登记 ID）、reason（1–500 字符）。空数组表示清空此套餐全部授予，不能表示“不修改”。保存返回 `{plan_id,revision}`；订阅账本不变，功能计算立即使用新规则，相关缓存失效。详细请求见第二章。

功能 available=false 时可以保留套餐关联，服务端仍不给用户授予；后台应显示全局关闭原因。解除全局关闭后恢复按原关联计算。

<a id="api-keys--用户单独覆盖和恢复继承"></a>

#### 用户单独覆盖和恢复继承



`GET /api/apps/starpath/admin/users/123/feature-overrides`：管理员，返回 `{user_id,revision,overrides}`。`PUT` 同地址完整替换：

```http
PUT /api/apps/starpath/admin/users/123/feature-overrides
Authorization: Bearer <ADMIN_ACCESS_TOKEN_OR_PAT>
Content-Type: application/json

{"expected_revision":5,"overrides":[{"feature_id":"plugins.manage","state":"hidden","expires_at":0},{"feature_id":"plugin:example.storyboard","state":"enabled","expires_at":1800086400}],"reason":"单用户试用一天"}
```

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| expected_revision | integer | 必填，读取该用户覆盖后的版本 |
| overrides | array | 必填，最多 500 项，feature_id 不重复 |
| overrides[].feature_id | string | 必须已登记 |
| overrides[].state | string | hidden / disabled / enabled；inherit 通过移除项表达 |
| overrides[].expires_at | integer | Unix 秒；0 永久；非 0 必须晚于保存时服务端时间 |
| reason | string | 必填，1–500 字符 |

成功返回 `{user_id:123,revision:6}`。恢复一项继承，从数组移除该项；全部恢复用 overrides=[]。过期条目可以保留在管理读结果中供解释，但不参与最终授权。保存时整批验证，不能只保存有效的前几条。

管理预览：`GET /api/apps/starpath/admin/users/123/entitlements`。返回所有已知功能，包括 hidden、最终状态、来源、原始覆盖和下一个到期点，仅管理员可读：

```json
{"success":true,"message":"","data":{"user_id":123,"server_time":1800000000,"effective_version":"preview-17","next_change_at":1800086400,"features":[{"id":"plugins.manage","state":"hidden","source":{"type":"user_override","expires_at":0}},{"id":"plugin:example.storyboard","state":"enabled","source":{"type":"user_override","expires_at":1800086400},"inherited_sources":[{"type":"plan","plan_id":7,"subscription_id":42}]}]}}
```

同一计算服务提供管理预览和本人结果；预览只解释，不替其他用户发起操作。该拟定接口需补目标用户层级校验，不假定所有现有管理员接口都已经有这项限制。

<a id="api-keys--三个业务例子"></a>

#### 三个业务例子



| 用户情况 | 计算过程 | 结果 |
| --- | --- | --- |
| 无套餐，无个人覆盖 | X 默认 disabled，全局可用 | 可见购买入口，不能运行 |
| 有套餐 7，又有个人 hidden | 套餐先授予，再被个人覆盖 | 各入口隐藏，不能通过 IPC 或计划任务运行 |
| 套餐 7 到期，套餐 8 仍授予 X | 只合并当前有效订阅，套餐 8 仍命中 | X 继续使用，不因一份到期全部关闭 |

全局关闭会使以上全部变为 hidden。订阅有效判断必须包含 start_time <= now < end_time；额度耗尽不改变授权，只影响实际计费是否通过。

<a id="api-keys--真实插件如何接入-源码调研与落地细则"></a>

### 真实插件如何接入：源码调研与落地细则



本节按星途 2026.9.23、固定 OpenClaw v2026.8.1 与当前 Gateway 路由核对，包含已确认行为及明确标注的待开发接缝。这里只补文档，未改星途/业务接口、未安装插件或进行真实联调。Gateway 管规则和发布决策，星途沿现有接口适配。

<a id="api-keys--真实插件清单与接入分类"></a>

#### 真实插件清单与接入分类



**18 项随包声明**

版本为本地 package.json 固定值。package_id 是构建目录/声明标识，runtime_id 才是 manifest.id 和 plugins.entries 的键；channel_id 又是独立标识。

| package_id | runtime_id | 固定版本 | 类型 / channel_id | 接入方式 |
| --- | --- | --- | --- | --- |
| qwen | qwen | 2026.8.1 | 模型 provider | 保留宿主适配；模型权限走 Gateway Key/模型目录 |
| deepseek | deepseek | 2026.8.1 | 模型 provider | 同上 |
| moonshot | moonshot | 2026.8.1 | 模型 provider，另有 webSearch 配置 | 不因套餐关闭通用 provider；搜索能力单独确认边界 |
| qianfan | qianfan | 2026.8.1 | 模型 provider | 保留宿主适配 |
| stepfun | stepfun | 2026.8.1 | 模型 provider | 保留宿主适配 |
| zai | zai | 2026.8.1 | 模型 provider | 保留宿主适配 |
| xiaomi | xiaomi | 2026.8.1 | 模型 provider | 保留宿主适配 |
| volcengine | volcengine | 2026.8.1 | 模型 provider | 保留适配；不能当作 Gateway 的火山渠道凭证 |
| dingtalk-connector | dingtalk-connector | 0.8.26 | 消息渠道 / dingtalk-connector | 现有钉钉配置和连接链路 |
| openclaw-lark | openclaw-lark | 2026.7.16 | 消息渠道 / feishu | 现有飞书配置；不能把 entries 键写成 feishu |
| qqbot | openclaw-qqbot | 2.0.1 | 消息渠道 / qqbot | 保留隐藏底层能力；当前产品不恢复 QQ 设置入口 |
| discord | discord | 2026.8.1 | 消息渠道 / discord | runtimeBundled=true；不走普通第三方覆盖安装 |
| wecom-openclaw-plugin | wecom-openclaw-plugin | 2026.5.25 | 消息渠道 / wecom | 现有企业微信配置和连接链路 |
| openclaw-weixin | openclaw-weixin | 2.4.3 | 消息渠道 / openclaw-weixin | 现有扫码/账号状态，保留登录资料 |
| moltbot-popo | 待核对 | 2.1.13 | 可选消息渠道 | installByDefault=false，本地 vendor 无对应 manifest，不宣称可用 |
| openclaw-nim-channel | nimsuite-openclaw-nim-channel | 1.1.1 | 可选消息渠道 / nim | 固定 Git 来源；不恢复已隐藏产品入口 |
| openclaw-netease-bee | openclaw-netease-bee | 0.1.3 | 可选消息渠道 / netease-bee | 可选且默认不安装 |
| clawemail-email | email | 0.9.16 | 消息渠道 / email | 与本机邮箱账号配置关联，不等于邮件技能本身 |

8 个 provider 加 10 项消息渠道声明共 18 项。当前产品远程连接只公开微信、钉钉、飞书、企业微信，邮箱另有恢复后的设置；保留代码不构成恢复其他产品入口的要求。

**5 个本地扩展**

| runtime_id | 注册能力 | 当前接点 | 如何纳入管理 |
| --- | --- | --- | --- |
| ask-user-question | AskUserQuestion | 本机 /askuser 回调 | 核心确认能力保留；不能因 plugins.manage 隐藏而关闭 |
| lobster-media-generation | lobsterai_image_generate、lobsterai_video_generate、lobsterai_skin_manage | 本机 /media-generation/tool 回调 | 保留扩展加载，在业务动作处分别检查图片/Wan/Seedance；不能整包当一个付费开关 |
| lobsterai-model-compat | 显式模型/思考档位适配 | modelProfiles / thinkingProfiles | 宿主兼容基础设施，随客户端/运行时维护 |
| mcp-bridge | 按配置暴露 MCP 工具 | 旧桥接协议 | 现有主链路使用原生 mcp.servers；不能为做权限而重新强制切回旧桥 |
| lobster-decision | decision_evaluate | 本机 /decision/tool | 实验能力默认关闭；本次不自动恢复或售卖 |

插件、技能和 MCP 分别维护。DOCX/XLSX/PPTX/PDF 来自技能和文档运行时，不是本表中的四个独立原生插件；禁止把它们改登记成可随意卸载的付费插件以绕过既有保留要求。

**Gateway 商品目录首期准入范围**

首期 `/api/apps/starpath/admin/plugins` 只接经核对的独立业务工具插件。核心/provider/随包渠道不允许通过该目录创建另一份同 runtime_id 的包，也不套用普通用户卸载、强制更新或回滚流程。渠道未来可有远程功能授权，但安装和凭据仍沿用现有专用链路。

尚未取得真实业务插件商品清单；手册中的 example.storyboard 为对接样例，不是本次发现的已安装商品。nsp-clawguard 只有明确版本的兼容处理源码，不能据此断言用户已经安装或需要将其上架售卖。

<a id="api-keys--现有链路-从按钮到运行时"></a>

#### 现有链路：从按钮到运行时



**插件管理的真实调用顺序**

1. PluginsSettings/PluginConfigPage 调用 window.electron.plugins。
2. preload 将请求转到 plugins:* IPC，所有本地文件和进程操作由主进程执行。
3. handlers 调用 PluginManager，读写 user_plugins，并在安装/卸载/启停/配置后调用 syncOpenClawConfig。
4. 安装器把 npm 包打成 tgz、把 Git 源检出后打包，或使用 ClawHub/本地来源，再交给固定 OpenClaw CLI。
5. OpenClaw 在独立 staging 中安装；星途识别 manifest.id，切换到用户 third-party-extensions 目录，记录版本。
6. 配置同步生成 plugins.load.paths、allow、entries，再按影响分类请求重启本地 OpenClaw。
7. 运行时发现/校验/加载插件，注册工具、消息渠道等能力；模型调用工具后才到对应 execute 或本机回调。

这里的“本地 OpenClaw gateway”是客户端运行时，不是远程聚合网关 Gateway。远程管理响应不会自动修改本地加载器，必须经星途主进程适配。

**现有 IPC 完整表**

这些名称全部是本地 IPC，不是 Gateway 的 HTTP 路由。响应外壳并不统一，接入时保留现有调用者期待的格式。

| IPC | 参数 | 现有返回 | 拟补检查 |
| --- | --- | --- | --- |
| plugins:list | 无 | success/plugins | 合并本人可见目录；不把内置隐藏项自动变商品 |
| plugins:detect | 无 | plugins/error | 发现不等于授权；不自动导入为受控商品 |
| plugins:sync | 无 | synced/error | 核对登记身份，再同步本地记录 |
| plugins:install | source/spec/registry/version | ok/pluginId/version 或 error | 受控 ID 必须转受控发布链路；不能自由 latest 绕过 |
| plugins:uninstall | pluginId | ok/error | 保留停用/卸载清理通道，不影响其他账号记录 |
| plugins:set-enabled | pluginId/enabled | ok/error | 本人授权、版本和本机选择共同决定有效状态 |
| plugins:get-config-schema | pluginId | success/schema/config | 校验归属；返回当前安装包 schema，敏感值独立处理 |
| plugins:save-config | pluginId/config | ok/error | 保存前按安装包 schema 校验，不能只写入任意对象 |
| plugins:batch-save | toggles/configs | ok/error | 整批预校验；现有逐项循环不是事务式整批保存 |
| plugins:check-updates | 可选 pluginIds | success/updates | 受控插件读 Gateway，普通用户源保留原边界 |
| plugins:update | pluginId | ok/pluginId/version/error | 受控插件固定 release，不重装未固定版本的源 |

安装日志另通过 plugins:install-log 事件发送，只用于进度，不是成功凭证。最终成功须结合安装、配置同步和健康检查；当前 handler 没有把这些阶段做成可回查的远程安装操作。

**三层目录与元数据**

| 位置 | 当前用途 | 接入要求 |
| --- | --- | --- |
| 应用 resources/cfmind 或开发 vendor/openclaw-runtime/current | 固定运行时及随包资源 | 不在活动客户端目录重建/覆盖；随宿主版本维护 |
| runtime 的 third-party-extensions | 构建时同步的随包渠道/本地扩展 | 不与用户安装目标混用，不随意覆盖 SDK 依赖 |
| userData/third-party-extensions | 用户安装的插件 | 现有 PluginManager 主目标，保存跨应用升级的包 |
| userData/openclaw/state/extensions | OpenClaw 其他安装来源 | 当前也参与发现；受控 ID 必须排除影子副本绕过 |
| userData/plugin-install-staging | 安装中间和切换备份 | 在扫描目录之外，保留同卷 rename 语义 |
| user_plugins | plugin_id/source/spec/version/enabled/config | 当前按插件 ID 全局存储，没有 Gateway user_id/origin 隔离 |

不能把某个目录存在当成插件已经可运行。配置 enabled=true 仍需 load.paths 能发现、allow 允许、schema 合法、宿主兼容且实际注册成功。

<a id="api-keys--gateway-与星途的新增接缝"></a>

#### Gateway 与星途的新增接缝



**身份、绑定和本地状态**

Gateway 保存稳定 plugin_id、可信 runtime_id、可见目录、功能授权、不可变 release 和每插件唯一的当前版本策略。星途保存本机安装状态及用户本地选择，不能反向把自报“已安装/已启用”当作授予权益。

拟新增的本地关联至少包含：

| 字段 | 保存位置/作用 |
| --- | --- |
| gateway_origin / gateway_user_id | 账号范围；来自已认证会话 |
| plugin_id / runtime_id | 服务端登记和校验后的固定映射 |
| desired_enabled | 用户自己的本机启停意愿；不被服务端失权永久覆盖 |
| config_scope / config_schema_version | 区分该账号配置与旧全局配置，记录版本兼容 |
| installed_release_id / package_sha256 / package_version | 共享本机包的实际身份，不能只存显示版本 |
| effective_version / valid_until / account_generation | 本次执行时的授权快照；不能作为永久许可 |
| operation_id / phase / previous_release_id | 安装与恢复的本机操作日志 |

字段为设计，当前没有这张关联表。新增本地 metadata/config 以 origin+user_id+plugin_id 为范围，旧 user_plugins 保留兼容，不直接对现有数据全表改名或清空。共享包文件可以复用，但新账号必须重新授权并使用本人的配置；旧全局配置的迁移要明确选择归属，不能自动分发给每个登录账号。

有效启用状态应由“本地希望启用 ∧ 服务端准许 ∧ 快照有效 ∧ 安装版本可运行 ∧ 所需配置完整”计算。用户切回获授权账号后恢复其原选择，而非沿用另一个账号写下的 false 或敏感配置。

**配置同步的准确插入点**

在 openclawConfigSync 现有托管 entries/allow 合并处计算受控插件的 effective_enabled：先保留核心/provider/现有渠道的原规则，再对已登记业务插件收紧。

- entries 使用 manifest runtime_id，不用 npm 名或构建目录名。
- 从原 existingAllow 中去掉受控业务插件的旧许可，再加入当前有效可运行的受控项，防止残留 allow 扩大授权。
- 对受控插件设置最终 entries.enabled；不能因为 raw user_plugins.enabled=true 又覆盖前面的拒绝。
- 不覆盖其他插件的原有配置、memory slots、原生 mcp.servers；不以清空全部 allow/deny 或删除目录来撤权。
- 当前全部插件变更均分类为 Restart。权限刷新内容没变时不重写配置/重启；确有需要时合并一次操作，并在安全点处理运行中的对话。

仅改配置还不够：已加载工具、在途回调和后台任务要在真实执行边界再次检查。首次登录未取到快照不闪现受控入口，换号清掉旧决策并丢弃迟到响应。

**受控安装的 IPC 输入**

建议保留原普通用户插件 IPC，同时增加或等价实现两个受控入口：

| 拟定 IPC | renderer 提交 | 主进程自行取得 |
| --- | --- | --- |
| plugins:install-managed | pluginId | 当前账号、check 决策、download/install 授权、固定包和摘要 |
| plugins:update-managed | pluginId | 同上，并验证实际当前 release 与回滚方向 |

renderer 不传可任意替换的 URL、registry、文件路径或 SHA256；不能以“我知道 release_id”为授权。现有泛用 install/local/git、检测导入、旧目录发现和改名安装也要识别受控 runtime_id，避免从另一个入口加载影子包绕过策略。

<a id="api-keys--插件作者具体交付什么"></a>

#### 插件作者具体交付什么



**最小包结构**

```text
starpath-storyboard/
  package.json
  openclaw.plugin.json
  dist/index.js
  LICENSE
```

下面是独立业务工具插件的契约示例，未创建实际商品；manifest.id 与代码 id、Gateway runtime_id 保持一致，npm 包名可以不同。

package.json 示例：

```json
{
  "name":"@example/starpath-storyboard",
  "version":"1.0.0",
  "type":"module",
  "main":"./dist/index.js",
  "files":["dist","openclaw.plugin.json","LICENSE"],
  "openclaw":{"extensions":["./dist/index.js"]},
  "peerDependencies":{"openclaw":"2026.8.1"},
  "peerDependenciesMeta":{"openclaw":{"optional":true}},
  "dependencies":{"@sinclair/typebox":"0.34.49"}
}
```

OpenClaw 是宿主 peer，不把另一份完整运行时装进插件 dependencies。构建与安装阶段需使用项目现有宿主 peer 处理，不能手工从另一设备复制绝对 symlink/junction。精确 peer 版本只覆盖已验证宿主，其他版本需重新测试。

openclaw.plugin.json 示例：

```json
{
  "id":"example.storyboard",
  "name":"示例分镜插件",
  "version":"1.0.0",
  "contracts":{"tools":["starpath_storyboard_create"]},
  "configSchema":{
    "type":"object",
    "additionalProperties":false,
    "properties":{
      "callbackUrl":{"type":"string","minLength":1},
      "secret":{"type":"string","minLength":1}
    },
    "required":["callbackUrl","secret"]
  },
  "uiHints":{"secret":{"label":"内部桥接凭据","sensitive":true}}
}
```

callbackUrl/secret 属于星途主进程注入的内部配置，不显示成让最终用户填写的套餐字段，不由 Gateway 返回任意执行 URL。sensitive 只是 UI 提示，不等于加密存储；现有 get-config-schema 会连同 config 返回，应在公司托管字段上补隔离。

**业务工具入口示例**

以下使用项目已经采用的 OpenClawPluginApi/registerTool 风格。新增本机 `/app-plugins/execute` 桥接和 `x-app-plugin-secret` 均为拟定，当前并不存在，不能把这段代码当作已可联调插件。注册不联网拉取权益；执行时由星途主进程按可信目录、当前账号和 session 统一检查。

```typescript
import { Type } from '@sinclair/typebox'
import type { OpenClawPluginApi } from 'openclaw/plugin-sdk'

export default {
  id: 'example.storyboard',
  name: '示例分镜插件',
  register(api: OpenClawPluginApi) {
    const config = api.pluginConfig ?? {}
    if (typeof config.callbackUrl !== 'string' || typeof config.secret !== 'string') {
      return
    }
    const target = new URL(config.callbackUrl)
    if (target.protocol !== 'http:' || target.hostname !== '127.0.0.1'
        || target.pathname !== '/app-plugins/execute' || target.username
        || target.password || target.search || target.hash || !config.secret) {
      throw new Error('Invalid managed plugin bridge')
    }
    const secret = config.secret
    api.registerTool((context) => ({
      name: 'starpath_storyboard_create',
      description: '根据当前用户提供的创作提示生成分镜。',
      parameters: Type.Object({
        prompt: Type.String({ minLength: 1, maxLength: 20000 }),
      }, { additionalProperties: false }),
      async execute(toolCallId: string, input: unknown, signal?: AbortSignal) {
        const args = input as { prompt?: unknown } | null
        if (typeof args?.prompt !== 'string' || !args.prompt.trim()
            || args.prompt.length > 20000) {
          return { content: [{ type: 'text', text: '提示词不合法' }], isError: true }
        }
        const timeout = AbortSignal.timeout(120000)
        const response = await fetch(target, {
          method: 'POST',
          redirect: 'error',
          signal: signal ? AbortSignal.any([signal, timeout]) : timeout,
          headers: { 'Content-Type': 'application/json', 'x-app-plugin-secret': secret },
          body: JSON.stringify({
            tool: 'starpath_storyboard_create',
            args: { prompt: args.prompt },
            context: { sessionKey: context.sessionKey ?? '', toolCallId },
          }),
        })
        if (!response.ok) {
          return { content: [{ type: 'text', text: '操作未完成，请查看当前权限或稍后重试' }], isError: true }
        }
        return await response.json() as {
          content: Array<{ type: 'text'; text: string }>
          isError?: boolean
          details?: Record<string, unknown>
        }
      },
    }))
  },
}
```

桥接接收端必须根据登记工具映射到 runtime_id/feature_id，不能相信调用体自报插件身份或 user_id。sessionKey/toolCallId 用于本机归属和关联，不是 Gateway 身份凭据；通过原有会话解析取得持有人。所有插件代码仍在受信任宿主环境内，loopback secret 不构成对恶意同进程代码的独立沙箱。

**两种业务插件的计费路径**

| 类型 | 实际执行 | Gateway 责任 |
| --- | --- | --- |
| 调用模型的公司插件 | 工具 → 本机桥接 → 用户模型 Key → 现有模型 API | 功能/Key/模型/额度校验后预扣费，按实际调用结算 |
| 纯本地插件 | 工具 → 主进程许可检查 → 本地逻辑 | 只提供远程权益，当前不因此新增云端调用费用 |

如要按“插件执行一次”另外收费，需要独立服务边界、预扣/结算/失败退款和幂等业务设计；当前没有这一计费接口。不能用调用插件 API 的次数、客户端上报或下载次数直接扣钱包，也不能在现有模型费用外暗加一笔。

<a id="api-keys--现有媒体插件怎么接-gateway"></a>

#### 现有媒体插件怎么接 Gateway



**保持工具协议，替换主进程适配**

已确认：媒体扩展注册的是用户对话工具，执行时发送 `{tool,args,context:{sessionKey,toolCallId}}` 到本机 `/media-generation/tool`，由 main.ts 的 handleMediaGenerationCallback 处理。现有账号代次和任务归属保护已经存在，应复用，不能因接 Gateway 删除。

主进程当前仍调用 `/api/media/images|videos/models`、`generate`、`tasks/:id`、`videos/tasks/:id/cancel` 并按 code=0 解析；这些不是当前 Gateway 的业务路由。适配工作放在主进程公司服务层，不修改工具名/参数，也不在 Gateway 复制整套旧企业媒体 API。

| 工具动作 | 旧依赖 | Gateway 现有入口 | 客户端需补的转换 |
| --- | --- | --- | --- |
| image/video list | /api/media/.../models，含 parameterSpec | 模型 Key 的 GET /v1/models；部分 pricing 元数据 | 按已验证模型映射生成能力表；完整参数目录目前缺失 |
| image generate | /api/media/images/generate 异步任务 | POST /v1/images/generations、/images/edits 或 /api/v3/images/generations | 同步结果转现有工具完成结构；不要创造不存在的图片远程任务 |
| image status | /api/media/images/tasks/:id | 没有通用图片任务详情 | 同步生成已完成；本地作业可查本机记录，不编造 Gateway 图片 task_id |
| Wan generate/status | 旧 videos/generate/tasks | POST /v1/videos、GET /v1/videos/:id | 规范化 Wan 参数、公开任务 ID、状态和 metadata.url |
| Seedance v3 generate/status | 同上 | POST/GET /api/v3/contents/generations/tasks | content/duration，succeeded 和 content.video_url |
| Seedance v3 cancel | POST 旧 cancel | DELETE /api/v3/contents/generations/tasks/:id | 读取真实 cancelled/终态，不先假报退款 |
| Wan cancel | POST 旧 cancel | 无通用 v1 取消路由 | 明确暂不支持远程取消，保留任务继续查询 |

**工具字段到网关字段**

| 工具字段 | 图片 | Wan v1 | Seedance v3 |
| --- | --- | --- | --- |
| model | 公开图片模型 | 公开 Wan 模型 | 公开 Volc Native 模型 |
| prompt | prompt | input.prompt 或适配器平铺 prompt | content 中 text 项 |
| image/images/firstFrame/lastFrame | edits 的参考输入；按模型 | input.media 或旧版支持字段 | image_url 项及 first_frame/last_frame |
| imageRoles/referenceImages | 按所选图片模型 | 按所选 Wan 模型 | reference_image 等角色，与图片一一对应 |
| durationSeconds | 不使用 | parameters.duration | duration；Seedance v1 则 seconds 字符串 |
| aspectRatio | 仅模型支持时 | ratio | ratio |
| audio | 不使用 | parameters.audio | generate_audio |
| n/count | 通常取一个明确值，当前工具限制 1–10 | 不作为视频张数 | 不作为任务批量字段 |
| providerOptions | 只映射已允许字段 | 防止覆盖已校验的时长/数量 | 不允许绕过 content、duration、权限和 URL 规则 |

当前工具 n/count 上限 10，video durationSeconds 上限 60；Gateway 通用安全上限不应反过来放宽工具语义。参数落到具体模型还要更窄校验，例如 Wan 3.0 为 2–30 秒。保留 false/0，不用简单真假判断丢掉 audio=false。

模型列表不能告诉客户端每个 ID 是 Wan 还是 Seedance。首期在受维护的模型映射里保存协议、模型家族、参数和媒体角色；未来能力目录需要单独定义接口。未知模型不按名字猜测并尝试付费生成。

**工具结果、权限与历史**

星途继续返回 `content[]/isError/details`，不把 Gateway 原生 JSON 直接丢给模型当作旧 code/data。媒体插件原有 status 动作会自适应轮询，保持这一能力，避免模型重复发起多轮忙轮询。

查询时保留原 Gateway 状态，同时映射既有本地媒体状态：v1 completed 与 v3 succeeded 均为完成，queued/running/in_progress 为处理，失败/取消/过期分别保留原因。先保存 `{origin,user_id,account_generation,protocol,model_id,gateway_task_id}` 再开始查询；不把上游 ID 或临时 UI 作业 ID当 Gateway ID。

image.generate、video.wan、video.seedance 分别在新生成动作检查；同一媒体插件不能只配一个 plugin:lobster-media-generation 开关。status、本人旧任务下载/取消及历史查看按原所有权保留。皮肤工具复用媒体扩展，但不能因本次接入恢复或扩大当前产品未开放的皮肤功能。

现有工具说明包含“必须有订阅”和“只有排队可取消”等旧企业假设，主进程还有固定 100 积分/秒估算。切换公司适配时同步校正这些说明：Gateway 支持钱包/订阅扣费偏好，v3 取消按真实状态判断，准确报价当前缺失；不能继续向用户展示固定估价或承诺不可取消。对话工具参数和返回协议保持原约定。

<a id="api-keys--消息渠道-模型适配-技能和-mcp-的边界"></a>

#### 消息渠道、模型适配、技能和 MCP 的边界



**消息渠道**

微信/钉钉/飞书/企业微信的连接、扫码和平台凭据继续走星途现有 IM 控制器与 channels 配置，Gateway 不代理它们全部消息流，也不将用户的微信/邮箱凭据当成模型 Key。

将来需要按套餐限制某个渠道，可登记独立功能（例如 im.weixin，当前尚未登记/实现），在设置入口、配置保存、建立连接和入站业务触发处接同一授权。仅隐藏导航不会断开已有连接；停用要安全停止接收新任务并保留凭据，不能卸载随包渠道。

远端 IM 发件人 ID 不是 Gateway user_id。若多个远端成员共用一个客户端，必须明确调用归属是当前安装的登录账号还是经过绑定的成员账号；没有映射时不能自动让任意发件人消耗当前用户钱包。该业务规则尚未确认，不能将“一个插件连接成功”验收为多用户计费接通。

**模型 provider**

平台配置当前锁到 Gateway /v1 的公司 provider，Key 和模型仍需正确关联。随包 provider 的注册/兼容代码保留，不因模型未对某用户开放就卸载 provider。云端能力以模型 Key、用户组、模型白名单和 Gateway 实际路由为准，客户端菜单不能扩大范围。

**技能与 MCP**

技能授权若以后纳入范围，应在技能发现/启用及其真正调用工具的边界设计；SKILL.md 本身不是可执行软件包的授权证据。MCP 当前经原生 mcp.servers 配置和运行管理，逐工具权限不能只改旧 mcp-bridge manifest。两者首期只保留现状和接缝，不因插件方案改写加载与工具协议。

<a id="api-keys--发布包-升级和回滚如何落实"></a>

#### 发布包、升级和回滚如何落实



**现有能力与缺口**

| 项目 | 已确认的现有行为 | 必须补齐 |
| --- | --- | --- |
| 版本来源 | npm/clawhub 检查远端最新；update 不传固定版本 | 受控插件只接受 Gateway 决策和 immutable release |
| staging | 与目标同卷，CLI 安装、manifest ID 检查、rename | 发布前检查期望 runtime_id/version/hash 与当前账号决策 |
| 切换失败恢复 | rename 失败恢复有效旧目录 | 不能把它称为运行健康失败后的持久回滚 |
| 切换成功 | publisher 立即删除旧备份 | 保留到配置同步和运行健康验证完成，再按保留策略清理 |
| 本机存储 | addUserPlugin 更新时 enabled=true，config 未被 upsert 覆盖 | 托管流程保留原启停意愿/配置，不能靠安装默认扩大授权 |
| 兼容处理 | nsp-clawguard 有精确版本的专用适配 | 新包不得默认套同补丁；按包/版本核对，不伪报“适用所有插件” |
| 原生依赖 | Electron/Node、平台架构、host peer/junction 参与 | 发布前验证目标组合，不能把 macOS 包标成跨平台 |

**发布产物契约补充（拟定）**

首期受控业务工具优先使用固定 npm-pack tgz，经现有 CLI 的 local 安装路径进入 staging。通用 managed_archive 是远程分发类型，不代表现有星途已经有“任意 ZIP 解压即运行”的安装器。

在手册发布登记字段上，补充以下兼容性信息并在发布前、安装前都检查：

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| runtime_id | string | 必须与插件登记及 manifest.id 相同 |
| package_name | string | 必须与 package.json.name 相同，不拿目录名替代 |
| package.format | string | 首期 npm_tgz；其他格式安装器支持后再开放 |
| runtime_version | string | 首期固定 2026.8.1，对应 v2026.8.1 宿主；不能只按客户端版本判断 |
| node_major | integer | 当前宿主要求 24，需按实际 Electron Node 环境验证 |
| config_schema_version | integer | >0，配置读写/升级恢复使用 |
| compatibility_profile | string 或 null | 可选已登记兼容规则 ID，不接收远程任意补丁脚本 |
| declared_capabilities | string[] | 首期 tool:加真实工具名；必须与包实际注册能力一致，未知 hooks/service/HTTP 注册不通过商品准入 |

SHA256 固定了下载 tarball，不一定固定安装后拉取的传递依赖。发布流水线需锁定依赖、记录可复现安装产物，验证宿主 peer 链接；客户端不得在最后一步重新解析未固定 latest。若某个包无法满足这些条件，先不开放远程托管更新，不能只验证 tgz 摘要就宣称完整供应链可复现。

package.version 是安装版本事实来源，manifest.version 可用于检查但不盲信二者永远相等；现有 nsp-clawguard 2.4.13 包的 manifest.version=2.4.12 已有明确兼容记录。新公司插件要求二者一致；第三方例外必须登记具体包/版本对应关系，不放宽所有包。

**安装到运行成功的阶段**

download authorize → 下载固定包 → hash/大小/身份/格式检查 → staging 安装和依赖校验 → install authorize → 等待安全点并切换 → 记录本机版本 → 合并配置 → 必要的运行时重启 → 健康检查 → report succeeded → 清理旧代备份。

同一插件安装/升级串行，避免两次操作交错；每次 await 后检查账号代次和取消状态，特别是发布目录与写本机元数据前。中途换号保留可恢复操作记录，不把 A 的安装结果认作 B 的权益。

健康检查至少确认插件被发现、schema 通过、工具实际注册、桥接接通；纯注册成功不证明真实业务成功。模型计费测试应在明确测试请求下进行，不能把每次健康检查实现成付费生成。

**回滚的两种情况**

- 安装中失败：恢复当前操作保留的旧目录及对应本地元数据/配置版本，重新核对当前权限；不要只回退文件而保留数据库新版本号。
- 已运行版本出问题：Gateway 把唯一当前版本策略切换为 rollback，星途按授权取得指定安全包，确认配置/数据可逆后安装。现有 publisher 没有保留全部历史版本，不能假定本机一定有旧包。

旧文件中的绝对 symlink/junction 不跨机器复用，不覆盖当前运行时目录。运行健康失败时保留诊断和可恢复路径，先恢复可用版本；不可逆数据迁移必须另有恢复方案，不能以降级代码冒充数据恢复。

<a id="api-keys--执行校验覆盖范围"></a>

#### 执行校验覆盖范围



| 执行类型 | 接入检查位置 | 边界 |
| --- | --- | --- |
| 用户点插件入口 | renderer + 主进程 IPC | UI 只反映状态，主进程再次拒绝越权 |
| 业务工具 execute | 公司插件桥接/受支持工具策略扩展点 | 每次调用检查，不能只在注册时查一次 |
| 媒体工具 | 现有媒体回调及 Gateway 计费前 | 按业务动作/模型家族，不关闭整个核心扩展 |
| 计划任务触发 | 任务执行时解析实际账号，进入相同工具守卫 | 不沿用创建任务时永久有效的权限 |
| 消息渠道入站 | 账号/渠道业务适配层 | 需明确远端发件人与 Gateway 归属 |
| 插件自带 HTTP/service/hook | 对应真实接收/执行边界 | before_tool_call 不覆盖这些入口，未落实则不纳入受控商品 |
| 插件直连第三方服务 | 可信客户端或第三方服务自己的授权 | Gateway 无法凭本地 plugin_id 验证远端实际动作 |

固定运行时存在 before_tool_call 扩展点，但不能据此宣称可以一处拦截全部插件。先确认工具归属、支持的会话上下文和行为，再通过公司适配或受支持扩展点接入；本次不改上游代理循环、核心工具签名、流式/历史合并或技能/MCP 执行协议。

完整调研来源、18 项随包版本、源码接点和分阶段验收保存在仓库 `docs/design/STARPATH_PLUGIN_INTEGRATION_RESEARCH.zh_CN.md`。[固定版本 manifest](https://github.com/openclaw/openclaw/blob/v2026.8.1/docs/plugins/manifest.md)与[固定版本工具插件](https://github.com/openclaw/openclaw/blob/v2026.8.1/docs/plugins/tool-plugins.md)作为外部对照；当前官网的新接口不自动适用于项目固定宿主。

<a id="api-keys--插件列表-详情与显示控制-拟新增"></a>

### 插件列表、详情与显示控制（拟新增）



当前星途 `plugins:list/install/uninstall/set-enabled/save-config/check-updates/update` 是本地 IPC；安装器已支持 npm/clawhub/git/local，当前 update 对 npm/clawhub 重新安装未固定版本的源。它们还没有消费 Gateway 最终授权或发布策略。`plugins:*` 不是可从网络调用的 Gateway HTTP API。

星途的主应用另有 `appUpdateGrayClient.ts`，调用 `/api/client-updates/check` 并期待旧 `code:0` 信封；当前 Gateway 路由没有这个地址。该实现也属于主应用更新，不能当作已经具备 Gateway 插件发布能力。本章拟定插件 API 统一使用 success/data，接入时需要单独适配。

<a id="api-keys--插件目录管理接口"></a>

#### 插件目录管理接口



首期只允许登记已经完成准入核对的独立业务工具插件；拒绝核心/provider/随包渠道的受保护 runtime_id，避免将它们通过通用商品包覆盖安装。更多能力类型落实执行边界后再扩展，不把现有包声明全部直接售卖。

| 方法 | 拟定地址 | 身份 | 作用 |
| --- | --- | --- | --- |
| GET | `/api/apps/starpath/admin/plugins` | 管理员 | p/page_size、keyword、status 查询，返回 data.items/total/page/page_size |
| POST | `/api/apps/starpath/admin/plugins` | root | 建档，固定 plugin_id/runtime_id，初始 draft |
| GET | `/api/apps/starpath/admin/plugins/:plugin_id` | 管理员 | 编辑详情和 revision |
| PUT | `/api/apps/starpath/admin/plugins/:plugin_id` | root | 修改完整元数据、目录状态 |
| GET | `/api/apps/starpath/plugins` | 用户 | 本人可见目录，p/page_size；hidden 项不返回 |
| GET | `/api/apps/starpath/plugins/:plugin_id` | 用户 | 本人可见详情；hidden 或不存在均 404 |

分页 p 从 1 开始，page_size 默认 20、最大 100。插件 ID 最长 128，允许字母、数字、点、下划线和连字符；`feature_id` 固定为 plugin: 加 plugin_id。`example.storyboard` 是示例 ID，实际落地必须与现有 runtime manifest ID 对上。

| 元数据字段 | 类型 | 约束与含义 |
| --- | --- | --- |
| plugin_id | string | POST 必填，之后不可改；不是 npm 包名或套餐名 |
| runtime_id | string | POST 必填，不可改；映射运行时加载/工具/定时执行 |
| name | string | 1–100 字符 |
| summary | string | 0–500 字符 |
| description_markdown | string | 0–20000 字符，净化后展示 |
| icon_url | string | 为空或允许的 HTTPS 静态资源 URL，不允许脚本/data URL |
| category | string | 0–64 字符，筛选展示 |
| sort_order | integer | 降序排序，ID 为稳定次序 |
| status | string | draft / listed / unlisted；draft/unlisted 用户接口不展示且不能新安装 |
| min_client_version | string | 数字段版本，如 1.2.0；不支持的星途版本只显示升级提示 |
| expected_revision / reason | integer / string | PUT 必填；POST 不传 expected_revision，reason 必填 |

目录上下架和运行全局关闭不同：unlisted 停止发现和新安装，已授权且已安装的用户仍可运行；要全体停用，必须把对应功能 available=false。draft 创建时同步创建 hidden 默认的功能目录项，避免半建档项被意外开放。

```http
POST /api/apps/starpath/admin/plugins
Authorization: Bearer <ROOT_ACCESS_TOKEN>
Content-Type: application/json

{"plugin_id":"example.storyboard","runtime_id":"example.storyboard","name":"示例分镜插件","summary":"根据提示词整理分镜","description_markdown":"## 使用方法\n填写提示词后创建分镜。","icon_url":"https://cdn.example.com/plugins/storyboard.png","category":"创作","sort_order":10,"status":"draft","min_client_version":"1.2.0","reason":"建立插件目录"}
```

成功 data 为 `{plugin_id,feature_id,revision:1}`。PUT 使用同样可编辑字段加 expected_revision，不能提交新的 plugin_id/runtime_id/feature_id。服务端禁止更改受保护的内置核心插件，不能接受任意 registry 地址或命令行。

<a id="api-keys--用户列表和详情响应"></a>

#### 用户列表和详情响应



```json
{"success":true,"message":"","data":{"page":1,"page_size":20,"total":1,"catalog_revision":12,"effective_version":"policy-17","items":[{"plugin_id":"example.storyboard","runtime_id":"example.storyboard","feature_id":"plugin:example.storyboard","name":"示例分镜插件","summary":"根据提示词整理分镜","icon_url":"https://cdn.example.com/plugins/storyboard.png","category":"创作","sort_order":10,"state":"disabled","reason_code":"PLAN_REQUIRED","purchase_plan_ids":[7],"min_client_version":"1.2.0"}]}}
```

详情 data 为同一项加 `description_markdown` 和 `revision`。disabled 可返回允许公开的介绍和可购套餐 ID，不能返回包下载授权；hidden 不得从列表、详情、搜索或更新检查泄露元数据。purchase_plan_ids 只包含当前可购、确实授予该功能的套餐，空数组不编造“购买后可用”。

列表没有安装状态：安装版本、本地启停和配置由星途本机提供，按 plugin_id 合并。用户本地 disabled 不会改写 Gateway 套餐规则；Gateway enabled 也不会强行把用户手动关闭的插件打开。元数据刷新以 catalog_revision 判断变化，授权有效期仍由本人 entitlements 决定，不能用目录缓存延长授权。

<a id="api-keys--星途每个入口的执行约束"></a>

#### 星途每个入口的执行约束



| 位置 | 读取与处理 |
| --- | --- |
| 设置/侧栏/商店/搜索/快捷键 | 同一份权限和目录快照；hidden 无入口，disabled 显示原因 |
| 插件安装/升级 | plugins.manage=enabled 且插件功能 enabled，目录可安装、版本可用；运维强制更新可在不开放管理页的情况下单独提示获授权用户 |
| 插件配置/启用 | 主进程 IPC 校验，不能只由 renderer 禁用按钮 |
| 插件加载/工具调用/计划任务 | 根据可信 runtime_id 再校验插件功能和版本门槛；旧缓存不得绕过 |
| 模型/云服务调用 | Gateway 在计费前按真实身份/专用 Key、模型与功能边界检查 |
| 历史和已受理任务 | 按所有权继续查询、取消、下载；不因新执行被禁而删除成果 |

纯本地插件执行由受支持星途版本落实；任意修改客户端无法获得同等服务端强制保证。卸载、关闭本地插件和本地配置清理应保留安全收尾通道；失权不应让用户连停用或备份既有数据都无法完成。

<a id="api-keys--插件版本-统一发布-强制更新和回滚-拟新增"></a>

### 插件版本、统一发布、强制更新和回滚（拟新增）



用户已明确采用全权统一控制：每个受控插件只有一份当前版本策略，所有获授权用户执行同一目标版本。Gateway 指定版本、升级时限、暂停、撤销与回滚；星途执行，没有用户自主选版入口。套餐和个人覆盖继续决定能否使用插件，不参与版本分群。

不同操作系统/架构可以使用不同安装文件，但这些文件必须属于同一个插件目标版本。客户端过旧或平台不支持时明确返回兼容问题，不能自动给这类用户下发另一条旧版本策略。本节全部接口仍为拟定，当前未实现。

<a id="api-keys--发布对象与状态"></a>

#### 发布对象与状态



| 对象 | ID / 版本 | 生命周期 |
| --- | --- | --- |
| 插件元数据 | plugin_id / revision | draft → listed → unlisted，可重新上架 |
| 发布包 | release_id / version | draft → published → withdrawn；published 内容不可变 |
| 当前统一策略 | plugin_id / revision | 每插件唯一；active / paused，修改事务提交即更新当前规则 |
| 本人下载/安装决策 | decision_id | 按当前账号权限、统一策略和兼容性签发，短时有效 |
| 本机安装操作 | operation_id | downloaded → installing → succeeded / failed；失败不代表已回滚成功 |

版本为三段数字 major.minor.patch。当前目标以后台指定的精确版本为准，已安装更高版本也不自动视为合规；降低版本必须有后台明确 rollback 指令和兼容检查。发布包只登记文件；设置统一策略才决定全体用户接下来应使用哪个版本。

<a id="api-keys--发布管理接口总表"></a>

#### 发布管理接口总表



| 方法 | 拟定地址 | 身份 | 作用 |
| --- | --- | --- | --- |
| GET | `/api/apps/starpath/admin/plugins/:plugin_id/releases` | 管理员 | 分页版本列表，含草稿/撤销状态 |
| POST | `/api/apps/starpath/admin/plugins/:plugin_id/releases` | root | 登记发布草稿与固定包信息 |
| GET | `/api/apps/starpath/admin/plugin-releases/:release_id` | 管理员 | 单版本详情和 revision |
| PUT | `/api/apps/starpath/admin/plugin-releases/:release_id` | root | 仅编辑 draft，完整替换 |
| POST | `/api/apps/starpath/admin/plugin-releases/:release_id/publish` | root | 发布并锁定包信息 |
| POST | `/api/apps/starpath/admin/plugin-releases/:release_id/withdraw` | root | 撤销版本；不自动安装旧版 |
| GET | `/api/apps/starpath/admin/plugins/:plugin_id/release-policy` | 管理员 | 当前唯一统一策略和 revision |
| PUT | `/api/apps/starpath/admin/plugins/:plugin_id/release-policy` | root | 原子替换全体统一规则；首次 expected_revision=0 |
| POST | `/api/apps/starpath/admin/plugins/:plugin_id/release-policy/preview` | 管理员 | 预览平台/客户端兼容与升级要求，不划分人群 |
| POST | `/api/apps/starpath/plugin-updates/check` | 用户 | 批量读取本人有权使用的插件的统一更新要求 |
| POST | `/api/apps/starpath/plugin-updates/authorize` | 用户 | 下载/安装前重新校验，签发短期下载信息 |
| POST | `/api/apps/starpath/plugin-updates/report` | 用户 | 上报安装结果，仅用于观察 |

列表统一 p/page_size，默认 1/20，最大 100；data 为 items/total/page/page_size。PUT 与发布/撤销动作必填 expected_revision、reason。POST 建包草稿及用户 authorize/report 使用 Idempotency-Key：同账号、方法、资源和键在 24 小时内相同请求重放原结果，不同 body 返回 409 IDEMPOTENCY_CONFLICT；authorize 每次重放仍重查权限、策略和包状态，不延长原决策/链接有效期。

<a id="api-keys--登记-发布和撤销版本"></a>

#### 登记、发布和撤销版本



```http
POST /api/apps/starpath/admin/plugins/example.storyboard/releases
Authorization: Bearer <ROOT_ACCESS_TOKEN>
Idempotency-Key: <RELEASE_CREATE_KEY>
Content-Type: application/json

{"version":"1.4.0","runtime_id":"example.storyboard","package_name":"@example/starpath-storyboard","runtime_version":"2026.8.1","node_major":24,"config_schema_version":1,"compatibility_profile":null,"declared_capabilities":["tool:starpath_storyboard_create"],"platform":"darwin","arch":"arm64","min_client_version":"1.2.0","package":{"source":"managed_archive","format":"npm_tgz","url":"https://cdn.example.com/plugins/storyboard/1.4.0/darwin-arm64.tgz","sha256":"<64_LOWERCASE_HEX_CHARACTERS>","size_bytes":123456},"changelog_markdown":"修复分镜导出问题。","rollback_compatible":true,"reason":"登记 macOS ARM64 发布包"}
```

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| version | string | 三段数字版本，同插件/平台/架构/版本唯一 |
| platform | string | darwin / win32 / linux；未实现相应安装器的组合不得发布 |
| arch | string | arm64 / x64；平台无关包也需明确列出支持组合 |
| min_client_version | string | 三段数字版本；低版本不得安装 |
| runtime_id / package_name | string | 分别匹配登记和 manifest.id、package.json.name |
| runtime_version / node_major | string / integer | 首期固定宿主 2026.8.1、Node 24；目标 Electron 环境仍需验证 |
| config_schema_version | integer | >0；用于配置兼容与恢复 |
| compatibility_profile | string 或 null | 仅允许已登记精确兼容规则，不传任意补丁代码 |
| declared_capabilities | string[] | 首期已审阅工具，格式 tool:加真实工具名，必须与实际包注册能力一致 |
| package.source / package.format | string | 首期 managed_archive / npm_tgz，经固定 CLI 本地 archive 路径安装；其他格式待适配 |
| package.url | string | 固定内容的 HTTPS 发布包，最长 2048 字符，来源由服务端允许列表限制 |
| package.sha256 | string | 必填 64 位小写十六进制，必须与实际包一致 |
| package.size_bytes | integer | 必填，1–536870912；下载与解包另做资源限制 |
| changelog_markdown | string | 最长 20000 字符，净化后展示 |
| rollback_compatible | boolean | 仅标注是否已确认配置/数据可降级，客户端还需检查当前状态 |
| reason | string | 1–500 字符 |

成功 data 为 `{release_id:"rel-example-140-mac-arm64",revision:1,status:"draft"}`。服务端发布前验证包归属、摘要、大小、runtime manifest ID、客户端兼容及安装器支持；不能只校验 URL 字符串。包由发布流程上传到受控 OSS/CDN，再登记地址；此契约不提供任意代码执行或自动执行安装脚本接口。

`POST /admin/plugin-releases/:release_id/publish` body：`{"expected_revision":1,"reason":"验证后发布"}`；成功返回 `{release_id,revision:2,status:"published"}`。尚未设置 active 的统一策略时不会自动要求用户更新。

`POST /admin/plugin-releases/:release_id/withdraw` body 同结构；成功 status=withdrawn。撤销使后续 authorize 拒绝该包，安装前必须再次检查；已安装故障版本的新执行按版本状态拒绝并提示维护。已下载不等于永远允许安装。修复可以发布更高版本，也可把当前统一策略切换为下文的 rollback；不能直接改 published 包内容。

<a id="api-keys--指定全体统一版本与升级时限"></a>

#### 指定全体统一版本与升级时限



先 GET 读取当前对象和 revision，再 PUT 完整替换。示例只列出一个已支持的平台组合；部署支持多种组合时一并登记同版本对应包。

```http
PUT /api/apps/starpath/admin/plugins/example.storyboard/release-policy
Authorization: Bearer <ROOT_ACCESS_TOKEN>
Content-Type: application/json

{"expected_revision":2,"target_version":"1.4.0","packages":[{"platform":"darwin","arch":"arm64","release_id":"rel-example-140-mac-arm64"}],"direction":"upgrade","from_versions":[],"deadline_at":1800086400,"status":"active","reason":"统一更新到 1.4.0，截止后要求完成升级"}
```

| 字段 | 类型 | 约束与规则 |
| --- | --- | --- |
| expected_revision | integer | 必填，首次 0；不匹配返回 409 POLICY_REVISION_CONFLICT |
| target_version | string | 唯一目标版本；必须已有经过核验的 published 包 |
| packages | array | 必填且非空；每个受支持 platform/arch 唯一，全部包属于该插件及同一 target_version |
| packages[].platform / arch / release_id | string | 选择对应系统/架构的已发布固定文件，不选择不同功能版本 |
| direction | string | upgrade / rollback；后台统一决定 |
| from_versions | string[] | upgrade 为 []；rollback 必填，明确允许降级的原始版本 |
| deadline_at | integer | 0 表示策略生效时立即要求目标版本；否则为不早于保存时服务端时间的 Unix 秒 |
| status | string | active / paused；暂停停止新安装授权，已设定的版本要求仍保留 |
| reason | string | 1–500 字符，记录管理原因 |

拟定成功：`{"success":true,"message":"","data":{"plugin_id":"example.storyboard","revision":3,"target_version":"1.4.0","status":"active"}}`。GET 返回完整保存字段（不含 expected_revision/reason），加 plugin_id、revision、updated_at；首次无配置返回 revision=0、target_version=null、packages=[]、status=unconfigured，客户端不得据此自由选择版本。

套餐/个人权限只决定是否有权使用；有权用户全部按这一份目标执行。接口拒绝版本人群、比例或通道选择参数，不能把原来的分群字段保留为空值后继续实现一套隐藏分流。不同平台文件在同一事务内校验通过再保存，不允许部分平台先更新成另一语义版本。

`POST /admin/plugins/:plugin_id/release-policy/preview` body 为 `{"client_version":"1.2.0","platform":"darwin","arch":"arm64","installed_version":"1.3.0"}`。返回 target_version、匹配 release_id、兼容结果、required、blocked、reason_code；该预览没有用户人群参数，不签发下载 URL，也不改变当前规则。

截止前，已有未撤销、兼容的旧版可在原授权下完成升级过渡；截止后 installed_version 必须精确等于 target_version 才能新执行。首次安装、安装版已撤销或账号无权使用时不能借过渡期执行。管理员设 deadline_at=0 可立即要求更新；客户端没有“永久跳过”或自行切换版本的能力。

暂停不把旧危险版本重新放行；恢复发布或切换安全目标均需后台重新保存策略并增加 revision。系统安装权限、对话安全点和数据可逆性仍需满足，控制权不表示可以无视操作系统限制强行覆盖。客户端过旧返回 CLIENT_UPGRADE_REQUIRED，平台无匹配包返回 PLUGIN_PLATFORM_UNSUPPORTED，保留升级/停用和历史查看入口。

<a id="api-keys--星途检查更新-请求与完整响应"></a>

#### 星途检查更新：请求与完整响应



```http
POST /api/apps/starpath/plugin-updates/check
Authorization: Bearer <USER_ACCESS_TOKEN>
Content-Type: application/json

{"client_version":"1.2.0","platform":"darwin","arch":"arm64","installed":[{"plugin_id":"example.storyboard","version":"1.3.0"}]}
```

installed 最多 100 项且 plugin_id 不重复，version=null 表示首次安装。客户端只上报兼容信息和当前安装事实，服务端取得真实用户授权并选择统一目标。check 只读，不下载安装或修改本地启停。

```json
{
  "success":true,
  "message":"",
  "data":{
    "server_time":1800000100,
    "valid_until":1800000160,
    "policy_revision":18,
    "items":[{
      "plugin_id":"example.storyboard",
      "state":"enabled",
      "installed_version":"1.3.0",
      "update_available":true,
      "target_version":"1.4.0",
      "release_id":"rel-example-140-mac-arm64",
      "release_policy_revision":3,
      "required":true,
      "deadline_at":1800086400,
      "blocked":false,
      "reason_code":"UPDATE_REQUIRED_BEFORE_DEADLINE",
      "decision_id":"<OPAQUE_DECISION_ID>",
      "changelog_markdown":"修复分镜导出问题。"
    }]
  }
}
```

无更新仍返回当前 target_version、release_policy_revision、required/blocked，不能把 update_available=false 当作撤销要求。unknown/hidden 项仅返回无元数据的 `{plugin_id,update_available:false,blocked:true,reason_code:"PLUGIN_UNAVAILABLE"}`；disabled 返回公开原因，不签发决策。与目标相同且包未撤销、授权有效时 blocked=false；不匹配在截止后 blocked=true。更高但未经批准的版本也不能自动继续运行。

发布规则统一，decision_id 仍按真实账号和当前规则版本签发，用于保护本人下载与安装。用户换号或规则变化使旧决策失效；这种鉴权不产生按用户分版本。云服务按可验证服务边界校验，客户端自报版本不构成二进制完整性证明，纯本地执行由受支持客户端落实。

<a id="api-keys--下载和安装前重新授权"></a>

#### 下载和安装前重新授权



```http
POST /api/apps/starpath/plugin-updates/authorize
Authorization: Bearer <USER_ACCESS_TOKEN>
Idempotency-Key: <INSTALL_OPERATION_KEY>
Content-Type: application/json

{"decision_id":"<OPAQUE_DECISION_ID>","plugin_id":"example.storyboard","release_id":"rel-example-140-mac-arm64","installed_version":"1.3.0","client_version":"1.2.0","platform":"darwin","arch":"arm64","phase":"download","operation_id":null}
```

phase=download/install；download 的 operation_id 为 null，服务端创建并返回安装操作 ID；install 必须传先前的 operation_id，服务端检查归属、目标包和未完成状态，返回同一 ID。其他字段必填。install 阶段在切换目录之前再调用，使用新的幂等键；服务端重查当前账号、权限、版本是否撤销、策略和兼容性。决策最长 60 秒有效且不跨权益到期；下载较久导致过期时重新 check，再用新的决策和原 operation_id 请求同一 release 的 install 授权。过期返回 409 UPDATE_DECISION_EXPIRED；策略不再匹配返回 409 UPDATE_DECISION_STALE，版本撤销返回 409 RELEASE_WITHDRAWN，不自动放行旧包。

```json
{"success":true,"message":"","data":{"operation_id":"<OPERATION_ID>","decision_id":"<OPAQUE_DECISION_ID>","valid_until":1800000160,"release_id":"rel-example-140-mac-arm64","runtime_id":"example.storyboard","package_name":"@example/starpath-storyboard","runtime_version":"2026.8.1","node_major":24,"config_schema_version":1,"compatibility_profile":null,"declared_capabilities":["tool:starpath_storyboard_create"],"package":{"format":"npm_tgz","url":"https://cdn.example.com/plugins/storyboard/1.4.0/darwin-arm64.tgz","expires_at":1800000160,"sha256":"<64_LOWERCASE_HEX_CHARACTERS>","size_bytes":123456},"restart_required":true,"direction":"upgrade"}}
```

下载到受控临时目录，校验实际大小、SHA256、包路径和 runtime manifest，再安全解包；拒绝路径穿越和外部链接。下载 CDN 不携带 Gateway Bearer。安装前取 phase=install 新授权；所有验证失败均保留旧版本和原启停状态。复用现有 staging/原子目录切换能力，不能对运行目录先删后装。切换成功后按现有影响规则重启本地运行时，再健康检查；不能把本地运行时的“gateway”误当远程聚合网关。

现有 npm/clawhub 更新重新获取 latest，不能直接满足精确版本、哈希与回滚契约；需受控安装源适配后再开放此能力。首期文档不把这些待开发流程说成现成支持。

现有 publisher 在目录切换成功后即删除旧备份，仅能恢复切换失败，不具备健康检查失败后的持久回滚。受控安装需保留旧代到配置同步和健康检查完成，并同时恢复本机版本/配置元数据；tarball 摘要也不能替代传递依赖锁定。安装、重新授权都核对真实宿主运行时版本/平台架构，不能只看主应用版本和客户端自报兼容。

<a id="api-keys--安装结果上报"></a>

#### 安装结果上报



```http
POST /api/apps/starpath/plugin-updates/report
Authorization: Bearer <USER_ACCESS_TOKEN>
Idempotency-Key: <REPORT_KEY>
Content-Type: application/json

{"operation_id":"<OPERATION_ID>","plugin_id":"example.storyboard","release_id":"rel-example-140-mac-arm64","phase":"succeeded","installed_version":"1.4.0","error_code":null}
```

phase 为 downloaded/installing/succeeded/failed；failed 必须带稳定 error_code，不上传环境变量、密钥或完整本地路径。operation_id 只能由归属用户上报；按允许状态推进，不因乱序覆盖终态；同内容重复返回原结果，冲突终态返回 409 INSTALL_REPORT_CONFLICT。成功返回 `{accepted:true}`。结果上报只用于 Gateway 后台统计，不能凭“我装好了”获得额外功能，也不能当作远程证明客户端未被修改。

<a id="api-keys--全体统一回滚与故障恢复"></a>

#### 全体统一回滚与故障恢复



1. Gateway 暂停故障策略，必要时撤销故障包，阻断新安装；暂停本身不自动恢复旧包。
2. 选择仍为 published 且配置/数据可逆的安全版本，准备全部受支持平台的对应文件。
3. PUT 同一个 release-policy，把 target_version 统一改为安全版，direction=rollback，列出允许降级的 from_versions，并指定立即或限时完成。
4. 所有有权用户下一次 check 得到同一安全目标；星途重新授权、核验包，在安全点安装并恢复兼容配置。已有安全目标的用户无需重复安装；低于安全目标的合法旧版按正常升级到该目标。需要降级但来源不在 from_versions 的安装需明确处理，不能盲目覆盖未知数据格式。
5. 健康检查后如实 report；失败保留/恢复最近可用状态和诊断。后台统一决定暂停、继续或改目标，不让客户端各自挑旧版。

```http
PUT /api/apps/starpath/admin/plugins/example.storyboard/release-policy
Authorization: Bearer <ROOT_ACCESS_TOKEN>
Content-Type: application/json

{"expected_revision":3,"target_version":"1.3.0","packages":[{"platform":"darwin","arch":"arm64","release_id":"rel-example-130-mac-arm64"}],"direction":"rollback","from_versions":["1.4.0"],"deadline_at":0,"status":"active","reason":"统一回退到已验证的安全版本"}
```

回滚仍需检查包兼容、数据可逆性和操作系统安装条件；本轮只给出契约，没有执行发布或降级。

<a id="api-keys--刷新-断网-换号和错误契约"></a>

#### 刷新、断网、换号和错误契约



权限在前台约 30 秒回查、最长 60 秒有效；目录在登录/回前台/版本变化时刷新；更新在进入插件页、手动检查、恢复网络、安装前检查。客户端统一主进程调度，避免每个卡片轮询。元数据可以缓存，但过期权限不得沿用；更新检查失败不表示“已是最新”，显示暂不可检查。

未过期同账号快照可短时沿用；已知统一目标、截止时间和撤销不能因断网被清除。到截止仍不匹配目标版本，停止新执行；无法取得新安装授权时保留旧包及历史，但不继续运行被阻断版本。退出/换号立即清除授权决策、下载链接和用户范围缓存；共享磁盘包必须按 release_id+hash 校验，新账号重新授权才能用，不能凭前一用户已下载绕过。

拟新增接口错误信封统一如下；现有接口仍使用其原有契约：

```json
{"success":false,"message":"配置已变化，请刷新后重试","error":{"code":"POLICY_REVISION_CONFLICT","retryable":false},"request_id":"<REQUEST_ID>"}
```

| HTTP | error.code | 客户端处理 |
| --- | --- | --- |
| 400 | INVALID_ARGUMENT | 标出字段；不重试原请求 |
| 401 | AUTH_REQUIRED | 刷新会话一次，失败转登录 |
| 403 | FORBIDDEN / FEATURE_DISABLED | 拒绝操作并刷新本人权限 |
| 404 | RESOURCE_NOT_FOUND / PLUGIN_UNAVAILABLE | 不显示隐藏元数据；重读目录 |
| 409 | POLICY_REVISION_CONFLICT | 重读目标资源，展示冲突后再保存 |
| 409 | RELEASE_IMMUTABLE / IDEMPOTENCY_CONFLICT / PLUGIN_PLATFORM_UNSUPPORTED | 纠正管理动作，不盲重试 |
| 409 | UPDATE_DECISION_EXPIRED / UPDATE_DECISION_STALE | 重新 check 和 authorize |
| 409 | APP_KEY_REBIND_REQUIRED / RELEASE_WITHDRAWN | 明确处理绑定或选择新的获准版本，不自动绕过 |
| 409 | INSTALL_REPORT_CONFLICT | 保留实际本地状态，回查管理记录 |
| 409 | CONFIG_REPORT_CONFLICT | 新建一次实际应用结果的回报，不覆盖原记录 |
| 426 | CLIENT_UPGRADE_REQUIRED | 引导主应用升级；历史和退出仍可用 |
| 429 | RATE_LIMITED | 按 Retry-After 退避 |
| 503 | ENTITLEMENTS_UNAVAILABLE / UPDATE_SERVICE_UNAVAILABLE | 不默认授予，保留既有数据，允许后续重查 |

所有涉及账号权限、目录和决策的响应 private/no-store、Vary Authorization；管理操作记录操作者、目标、变更、理由及结果。权限撤销和版本策略生效都必须在多节点一致失效；文档中的 revision、TTL、幂等和包校验均为待实现契约。

版本字段分别作用于各自资源：功能单项 revision 防止功能编辑覆盖，套餐/用户覆盖 revision 防止授权编辑覆盖，插件 catalog_revision 只标记目录元数据变化，发布包/当前统一策略 revision 防止管理覆盖；用户 effective_version 随最终权限及有效来源变化，更新 policy_revision 标记当前发布规则版本。不得跨资源比较大小或把任一版本字段当作授权凭据；决策绑定真实账号、权限有效版本和发布策略，变更时重新校验。

<a id="api-keys--实施规划与交付门槛"></a>

### 实施规划与交付门槛



版本：2.0 收口稿，2026-10-19。本节列明后续工作的责任和依赖，全部业务阶段仍待开发/验收。阅读入口已实现不改变该状态；完整任务与场景索引在仓库 `docs/design/STARPATH_INTEGRATION_DELIVERY_PLAN.zh_CN.md`。

| 阶段 | Gateway 交付 | 星途交付 | 完成门槛 |
| --- | --- | --- | --- |
| D0 手册收口 | 六章、合并审阅稿、管理员阅读及 root 开关 | 只读核对接入源码 | 文档可重复生成；权限/前端/构建与公开隔离检查通过 |
| S0 现有功能复验 | 提供已有接口契约 | 先按当前稳定性计划复验对话、工具和现有功能 | 当前真实证据明确，核心无阻断；不凭历史测试跳过 |
| I1 现有接口对接 | 现有登录/账务/模型/媒体接口 | 独立公司协议适配、Key 关联、只读套餐与结果保存 | 各协议真实闭环，保留官方消息/工具及历史行为 |
| G1 权限服务 | 目录、套餐授权、个人覆盖、本人快照、应用 Key、审计/缓存失效 | 对齐快照类型与账号范围 | 管理预览与本人一致；三数据库与多节点关键边界通过 |
| G2/C1 管理与执行 | 套餐详情、授权后台、冲突预览、云业务鉴权 | 页面/快捷键/IPC/业务工具/计划任务统一检查 | 绕过被拒且不预扣费；同机换号/到期不串权，历史可用 |
| R1/R2 统一发布 | 固定包、发布撤销、唯一策略、检查/二次授权/上报 | 固定安装、包身份校验、健康检查、恢复旧代及元数据 | 同一目标和期限；下载后撤销、错包、不兼容、恢复失败可处理 |
| A1 联调与上线 | 账本/权限/策略与观察结果一致 | 分系统安装、升级、真实插件执行与恢复 | macOS/Windows/Linux 分别留证，缺失项标待验收 |

G2 管理页和 C1 客户端可在 G1 契约稳定后分别推进，但显示隐藏不替代执行限制。首个样例必须有真实业务插件、manifest/runtime_id、schema、固定产物和可验证业务动作；example.storyboard 仅说明接口，不是可直接上架的商品。

<a id="api-keys--后台配置如何落到客户端"></a>

#### 后台配置如何落到客户端



1. Gateway 管理端先 GET 资源及 revision，提交完整对象、expected_revision 和 reason；冲突后重读，不覆盖别人已保存的规则。
2. 服务端在同一资源事务中校验、写入与递增版本，记录管理审计，并让所有节点的相关权限/决策失效。多个资源的多个请求不伪装成跨接口原子成功。
3. 星途主进程按当前 origin/user.id/登录代次读取本人快照，统一驱动界面与执行入口，再发 config-reports。正常网络下目标 60 秒内生效，实际联调测量；云侧鉴权无需等待本机刷新，回报不作为授权凭证。
4. 安装与执行另查当前统一策略、包兼容与本机 desired_enabled；不把“有权益”变成强制开启用户手动关闭的插件。
5. 旧 Key 和 user_plugins 不按名称或当前登录账号批量认领。先保留原记录、核验归属，再做可回退关联；包可共享，配置、凭据和授权不可跨账号共享。

<a id="api-keys--必测场景与交接证据"></a>

#### 必测场景与交接证据



| 场景组 | 必测内容 |
| --- | --- |
| 身份/Key | 过期刷新、撤销、并发初始化、已删除绑定、伪造应用头、同机换号与迟到响应 |
| 权益 | 多套餐并集、个人覆盖/撤销/到期、全局关闭、下架与额度重置、多节点缓存 |
| 执行 | 页面直达、IPC、模型工具、计划任务和同协议替代路径；拒绝无新增扣费/上游副作用 |
| 媒体 | 文本/图片/Wan/Seedance 分别记录模型与协议；恢复任务、下载/保存、实际消费回查 |
| 发布 | 两账号同目标、全平台文件一致、截止/断网、下载后撤销、摘要/身份错误、注册健康失败、可逆回滚 |
| 发行 | 分系统安装、升级保留配置/成果、退出/重启恢复；构建与真实桌面验收分别登记 |

每条证据写明两端提交与本地差异、系统/架构、运行时、前置条件、步骤、预期/实际、日期和脱敏结果。源码存在、模拟测试、本地服务、CI 与真实验收分别记录，不预填“通过”。本地原始请求和临时脚本放忽略的 `.local-tests/`，禁止放入密钥、Cookie、密码或完整 DSN。

星途当前套餐只读；购买/支付 UI、主应用升级服务、完整云端对话历史和通用报价服务按各自产品范围另排。统一插件发布仍是必须完成的范围。客户端任务遵守其当前保护边界，本手册不授权改造官方对话、工具、技能、MCP 或从星途任务修改 Gateway。

<a id="api-keys--两个平台逐项验收-待实施"></a>

### 两个平台逐项验收（待实施）



<a id="api-keys--一个插件从建档到用户可用的完整对接"></a>

#### 一个插件从建档到用户可用的完整对接



1. Gateway root 建档插件（draft），对应功能初始 hidden；登记固定版本包、核验并 publish。
2. Gateway root 预览平台/客户端兼容，保存唯一 release-policy，统一指定目标版本和期限；这一步只决定版本，不授予使用权限。
3. Gateway 配置套餐详情和 feature_ids，包含该插件及需要的 plugins.manage；按需要设置功能 available=true、default_state=disabled，再将插件 listed。普通用户仍只看可购买说明。
4. Gateway 已存在用户有效订阅（由已授权的购买或管理员发放建立），星途回查本人订阅和 entitlements，确认该插件变为 enabled。星途当前仍为只读套餐；未来启用购买须另过支付闭环验收。
5. 星途读取本人 plugins 详情，check 得到获准 release；download 授权 → 下载核验 → install 再授权 → 原子安装/健康检查 → report。
6. 用户执行时，主进程按 runtime_id 检查功能、版本和本地启停；调用云模型时 Gateway 再校验模型 Key、功能与额度，然后预扣费/转发。
7. 后续改显示资料只刷新目录；改套餐授权刷新权限；发新版本走发布/统一策略；撤权和强制更新分别阻断相应的新执行，历史成果保留。

这条链路明确 Gateway 为主控端，星途没有第二套可以自行授予套餐、开放插件或越过版本策略的规则。

<a id="api-keys--验收矩阵"></a>

#### 验收矩阵



| 场景 | Gateway 负责 | 星途必须观察到 |
| --- | --- | --- |
| A、B 同设备换号 | 分别返回本人目录/权限/更新决策 | A 的插件不串到 B，迟到响应被丢弃 |
| 套餐购买并到账 | 真实账本发放，最终权限更新 | 购买页返回后查到有效订阅，插件正确显示并可执行 |
| 套餐或用户规则变更 | 写入新 revision，多节点失效 | 正常联网目标 60 秒内同步，IPC/工具/计划任务同样受控 |
| 套餐下架/额度重置 | 保留有效订阅与功能来源 | 不错误隐藏原已授权插件 |
| 修改名称和排序 | 只更新目录元数据 | 卡片变化，不下载安装包 |
| 两个获授权账号分别检查更新 | 同一插件返回相同目标版本及期限，平台只选择对应包文件 | 客户端不能自主选版，兼容性失败明确提示 |
| 统一升级过截止 | 返回 required/blocked/精确目标版本 | 仅该插件新执行被阻断，可升级和看历史 |
| 下载后撤销版本 | install 授权拒绝失效包 | 不安装，不删旧版，不清配置 |
| 错包/断网/磁盘不足 | 结果可上报、可观察 | 校验失败保留旧版，不假报成功 |
| 回滚有不可逆迁移 | 管理端禁止不兼容回退 | 显示需处理原因，不能强制覆盖数据 |
| 伪造管理员/套餐/插件头 | 按真实身份授权 | 拒绝且不预扣费、不触发上游任务 |

以上不是已通过测试记录。权限、插件目录和发布控制需在 Gateway 与星途分别实现并联调，不能只完成接口返回或隐藏按钮就验收。

<a id="api-keys--权益变化后的同步"></a>

### 权益变化后的同步



登录、换号、购买/发放完成、订阅失效及页面恢复时，统一执行：

1. 读取 `/api/user/self` 与 `/api/subscription/self`，确认用户、钱包、订阅和分组。
2. 读取 `/api/user/models`、`/api/user/self/groups`，刷新模型选择。
3. 读取选定的 `/api/token/42`，确认启停、过期、组和模型限制。
4. 获取当前账号选定 Key 后读取 `/v1/models`，更新星途实际调用配置；明文仅按调用需要获取和保存。
5. 上述拟新增权限接口实现后，再刷新最终功能快照；当前没有该接口，不能把本地开关或模型列表当成它的等价替代。

订阅变更可能改变用户组，但不会自动改写密钥。group 为空通常继续跟随用户；固定高档组的密钥在降级后可能被拒绝；模型白名单和密钥预算也不会因续订自动扩大。不要把订阅剩余量写进 remain_quota。

当前没有专用的订阅/钱包/插件变更推送，先回查同步。媒体 Webhook 不承担权益通知。星途现有 Gateway 登录与订阅读取已接入，但模型 Key/目录自动关联和旧媒体协议转换仍需客户端完成。

源码依据：`controller/{token,user,model,group,pricing}.go`、`middleware/auth.go`、`model/subscription.go`、`model/token.go`；星途 `maasAuth.ts`、`platformConfig.ts`、`enterpriseConfigSync.ts`、`Settings.tsx`、插件 IPC 和 `openclawConfigSync.ts`、企业账号菜单及 `product.json`。现有能力来自源码核对；新增权限契约仅为方案，自动配置与权益同步尚未完成两平台联调。

<a id="text-models"></a>

## 4 文本接口对接

使用当前用户的模型 Key 接入文本对话。先完成密钥与可用模型同步，再选择同步或流式协议；Gateway 不保存星途的完整对话记录。

版本：2.0 收口稿，2026-10-19。本章 API 是现有协议入口；各模型与渠道的组合须实测。星途接入限于独立公司适配，保留官方消息、工具、流式和历史行为。

<a id="text-models--接口和凭证矩阵"></a>

### 接口和凭证矩阵



| 方法 | 地址 | 协议 | 星途如何处理 |
| --- | --- | --- | --- |
| POST | `/v1/chat/completions` | OpenAI Chat | messages 输入，choices 输出；同步/SSE |
| POST | `/v1/completions` | 旧 Completions | prompt 输入，choices[].text；仅兼容旧客户端 |
| POST | `/v1/responses` | Responses | input 输入，output/事件输出 |
| POST | `/v1/responses/compact` | Responses compact | 上下文压缩专用，不套普通对话返回值 |
| POST | `/v1/messages` | Claude Messages | messages/system，content[]；专用 SSE 事件 |
| POST | `/v1beta/models/<MODEL_ID>:generateContent` | Gemini | contents/parts；同步 candidates |
| POST | `/v1beta/models/<MODEL_ID>:streamGenerateContent` | Gemini | contents/parts；按实际流协议读取 |

以上路由均为现有接口；模型和渠道仍需支持对应协议，不能因为路由存在就给全部模型显示该模式。全部示例用 Gateway 用户模型 Key，不能把上游渠道 Key、用户登录 Access Token 或管理员 PAT 填入。

OpenAI 风格优先用 `Authorization: Bearer <FULL_USER_MODEL_API_KEY>`。Claude 客户端还可用 `x-api-key`，Gemini 客户端可用 `x-goog-api-key`；使用对应 SDK 时配置 Gateway 地址，避免把 Key 发到默认官方域名。OpenAI SDK 的 base_url 常为 `https://gateway.example.com/v1`，具体调用不要再重复拼接 /v1。

<a id="text-models--chat-请求字段"></a>

### Chat 请求字段



| 字段 | 类型 | 必填 / 默认 | 约束和用途 |
| --- | --- | --- | --- |
| model | string | 必填 | 选定 Key 的 `/v1/models` 实际 ID |
| messages | array | 必填 | 按顺序携带上下文；图片/工具格式取决于模型 |
| messages[].role | string | 必填 | system/user/assistant/tool；部分模型另有 developer |
| messages[].content | string 或 array | 按消息类型 | 普通文本用 string；视觉输入用类型化内容项 |
| stream | boolean | 可选 | true 读取 SSE，false 读取 JSON |
| temperature / top_p | number | 可选 | 未传交给模型默认；不要把某一个模型默认套给全部模型 |
| max_tokens / max_completion_tokens | integer | 可选 | 非负，网关上限 1073741823；实际模型上限通常更小，按选定模型限制 |
| tools | array | 可选 | 受支持的工具声明；function 参数是 JSON Schema |
| tool_choice | string 或 object | 可选 | auto/none/required 或特定工具，按模型支持 |
| stream_options.include_usage | boolean | 可选 | 渠道支持时可返回 usage；不能要求每帧都有 |
| response_format | object | 可选 | 文本/结构化 JSON 能力依模型支持 |

可选 0、false 与未传不是同一个意思。星途表单只发送用户选定且当前协议支持的字段，不将 undefined 转成 0，也不把显式 temperature=0 丢掉。

<a id="text-models--同步对话"></a>

### 同步对话



```http
POST /v1/chat/completions
Authorization: Bearer <FULL_USER_MODEL_API_KEY>
Content-Type: application/json

{
  "model": "<TEXT_MODEL_ID>",
  "messages": [
    {"role":"system","content":"<SYSTEM_PROMPT>"},
    {"role":"user","content":"<USER_MESSAGE>"}
  ],
  "stream": false
}
```

读取 `choices[].message`，文字通常在 `choices[0].message.content`，工具调用读 `tool_calls`。多轮对话由星途把必要历史按顺序传回，Gateway 不会自动拼接全部会话。模型响应不使用管理接口的 success/data 外壳。

成功响应示例：

```json
{"id":"chatcmpl-example","object":"chat.completion","created":1800000000,"model":"<TEXT_MODEL_ID>","choices":[{"index":0,"message":{"role":"assistant","content":"生成的正文"},"finish_reason":"stop"}],"usage":{"prompt_tokens":20,"completion_tokens":10,"total_tokens":30}}
```

id 为模型响应 ID，不等于 Gateway X-Oneapi-Request-Id，也不等于项目/会话 ID。finish_reason=length 应显示被长度限制截断；tool_calls 表示待执行工具，不是已完成业务。工具运行由星途完成，回填 role=tool 和原 tool_call_id 后继续下一次请求。

<a id="text-models--视觉输入与工具调用"></a>

### 视觉输入与工具调用



支持视觉的文本模型仍调用 chat/completions，不能因为附带图片就路由到图片生成：

```json
{"model":"<VISION_TEXT_MODEL_ID>","messages":[{"role":"user","content":[{"type":"text","text":"描述这张参考图"},{"type":"image_url","image_url":{"url":"https://files.example.com/reference.png"}}]}],"stream":false}
```

本地路径不能由上游读取；先用模型支持的 Data URI 或可访问 URL。素材 asset:// 不是通用视觉 URL，只有明确支持的适配链路才能引用。

工具声明示例：

```json
{"model":"<TOOL_TEXT_MODEL_ID>","messages":[{"role":"user","content":"查询项目进度"}],"tools":[{"type":"function","function":{"name":"get_project_status","description":"查询当前用户的项目进度","parameters":{"type":"object","properties":{"project_id":{"type":"string"}},"required":["project_id"],"additionalProperties":false}}}],"tool_choice":"auto","stream":false}
```

工具返回的 arguments 是 JSON 字符串，先解析并验证再执行；模型生成内容不是授权。若工具属于受控业务插件，每次执行须通过第三章的功能/版本检查，包含模型连续发出的多轮工具调用，不能只在对话开始检查一次。

<a id="text-models--流式与其他文本协议"></a>

### 流式与其他文本协议



同一请求改为 `"stream":true`，使用能发送 POST、JSON 和 Authorization 的流式客户端解析 SSE：

```text
data: {"choices":[{"index":0,"delta":{"content":"<TEXT_DELTA>"}}]}

data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
```

按帧解析，不能把一次网络读取当作完整 JSON。按 index 合并 delta.content，工具参数增量单独处理；usage 帧可能没有 choices。处理协议结束标志、错误与中断，用户停止时取消请求，费用以 Gateway 结算为准。

其他文本协议分别使用 `/v1/responses`、`/v1/messages`、`/v1beta/models/<MODEL_ID>:generateContent` 或 `:streamGenerateContent`，按各自响应解析。路由存在不代表每个模型都支持；不能替换路径后继续套用 choices/delta。

<a id="text-models--responses-请求和响应"></a>

#### Responses 请求和响应



```http
POST /v1/responses
Authorization: Bearer <FULL_USER_MODEL_API_KEY>
Content-Type: application/json

{"model":"<RESPONSES_MODEL_ID>","input":"概括这段内容","max_output_tokens":1024,"stream":false}
```

```json
{"id":"resp-example","object":"response","status":"completed","model":"<RESPONSES_MODEL_ID>","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"概括后的内容"}]}],"usage":{"input_tokens":20,"output_tokens":10,"total_tokens":30}}
```

读取 output 中的 message/content/output_text，不能只拿 output[0] 假定它一定是文本，工具/推理项可能在前。流式按 response.output_text.delta、response.completed、response.failed 等 type 分发；不能以 Chat 的 [DONE] 作为所有协议的唯一结束条件。previous_response_id 等服务端状态功能依上游支持，Gateway 不承诺保存完整 Responses 会话。

<a id="text-models--claude-messages-请求和响应"></a>

#### Claude Messages 请求和响应



```http
POST /v1/messages
Authorization: Bearer <FULL_USER_MODEL_API_KEY>
anthropic-version: 2023-06-01
Content-Type: application/json

{"model":"<CLAUDE_MODEL_ID>","max_tokens":1024,"system":"你是创作助手","messages":[{"role":"user","content":"写一段开场白"}],"stream":false}
```

```json
{"id":"msg-example","type":"message","role":"assistant","model":"<CLAUDE_MODEL_ID>","content":[{"type":"text","text":"开场白正文"}],"stop_reason":"end_turn","usage":{"input_tokens":20,"output_tokens":10}}
```

system 在顶层，不伪装成普通 system 消息。流式处理 message_start、content_block_start/delta/stop、message_delta、message_stop 和 error；tool_use/input_json_delta 与文字分开累积。

<a id="text-models--gemini-请求和响应"></a>

#### Gemini 请求和响应



```http
POST /v1beta/models/<GEMINI_MODEL_ID>:generateContent
Authorization: Bearer <FULL_USER_MODEL_API_KEY>
Content-Type: application/json

{"contents":[{"role":"user","parts":[{"text":"写一句标题"}]}],"generationConfig":{"maxOutputTokens":1024}}
```

```json
{"candidates":[{"content":{"role":"model","parts":[{"text":"生成的标题"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":20,"candidatesTokenCount":10,"totalTokenCount":30}}
```

模型名位于路径，不需要把 OpenAI 的 model/messages 硬塞进 body。parts 可包含文字和其他类型，按类型处理。streamGenerateContent 选择渠道支持的传输格式；SDK 用 SSE 时通常带 alt=sse，按实际 Content-Type 解析，不能把 JSON 数组流与 SSE 混用。

<a id="text-models--模型协议能力表与接入验收"></a>

### 模型协议能力表与接入验收



两端共同维护实际支持表，至少记录公开 model_id、选定 Key/分组、协议路径、同步/流式、视觉、工具、结构化输出、上下文方式、请求限制和验证日期。`/v1/models` 证明 Key 可见模型，不承诺每个模型支持全部协议；`/api/pricing` 的部分元数据也不能替代这张表。

| 接入步骤 | 星途适配要求 | 验收结果 |
| --- | --- | --- |
| 选择协议 | 由已核验模型能力决定，使用相应参数与 parser | 同一模型不因存在路由就自动标全支持 |
| 多轮上下文 | 保留官方历史与工具消息顺序；区分完整历史和上游状态引用 | previous_response_id 不支持时明确失败，不盲目重发付费请求 |
| 工具调用 | 保留 name、call id 和参数增量，完成 schema/权限检查后执行 | 工具参数完整、结果正确回填；停止后不重复执行 |
| 流终止 | 分协议处理完成、失败、长度截断和断流 | HTTP 200 或出现文字都不直接当成功；已有文本保留并标实际状态 |
| 费用与恢复 | 保存响应 ID 和可取得的 Gateway 请求编号，回查服务端账本 | usage 缺失/估算不伪装上游精确值，不客户端重新结算 |

自动 parser/转换测试、实际模型响应及真实星途对话分别记录。必须包含一次同步、一次分帧流、一次工具调用、一次中断和会话/Key 错误；只对明确支持的组合执行。对话基础存在阻断时先处理已有缺陷，不借对接重新设计回答区或发送链路。

<a id="text-models--相关模型能力入口与错误处理"></a>

### 相关模型能力入口与错误处理



语音、向量、重排等若进入星途产品，可复用用户模型 Key，但各自使用现有专用接口：

| 方法 | 现有地址 | 输入与输出 |
| --- | --- | --- |
| POST | `/v1/embeddings` | model/input；返回 data[].embedding |
| POST | `/v1/rerank` | model/query/documents；按重排协议读结果 |
| POST | `/v1/audio/transcriptions` | multipart 的 file/model；返回识别文本 |
| POST | `/v1/audio/translations` | multipart 的 file/model；返回翻译文本 |
| POST | `/v1/audio/speech` | model/input/voice；返回音频二进制或模型支持的流 |
| GET | `/v1/realtime` | WebSocket，需相应客户端和渠道支持 |

这些是能力入口索引，不等于全部套餐都授予。语音等如纳入远程功能控制，要先补稳定 feature_id 和服务端边界，不能借 text.chat 自动授予所有新能力。

OpenAI 常见错误为 `{"error":{"message":"...","type":"...","code":"..."}}`；Claude/Gemini 依协议返回原生错误。处理 HTTP 状态与实际 error 对象，避免写死唯一错误码。401 检查模型 Key，429 区分限流与额度；400 修参数；服务失败允许用户重试，但付费请求超时先回查消费，不能自动循环重发。SSE 开始后 HTTP 仍可能是 200，流内 error 同样要显示失败/中断。

调用后的费用与失败处理见[账号与钱包](#account-login--消费回查与请求结果)。

源码依据：`router/relay-router.go`、`relay/helper/valid_request.go`、`relay/channel/openai/relay-openai.go`、`relaykit/dto/openai_request.go`。协议已读代码核对；真实文本生成尚未两平台联调。

<a id="image-models"></a>

## 5 图片接口对接

图片生成与编辑使用用户自己的模型 Key，图片模型需已对该密钥开放。先按模型支持选择尺寸和结果格式，再把生成文件与星途用户、项目关联保存。

版本：2.0 收口稿，2026-10-19。本章现有接口可先做公司适配；image.generate 和插件远程授权仍待开发，不能把本地 UI 开关写成已经联通 Gateway 权限。

<a id="image-models--接口选择和字段约定"></a>

### 接口选择和字段约定



| 方法 | 地址 | 请求 | 响应 |
| --- | --- | --- | --- |
| POST | `/v1/images/generations` | JSON，OpenAI 兼容文生图 | JSON 或模型支持的 SSE |
| POST | `/v1/images/edits` | JSON 或 multipart，依模型支持 | 图片结果或图片事件 |
| POST | `/api/v3/images/generations` | 火山原生 JSON，仅 Volc Native | 完整读取后的原生响应 |

全部为现有接口。v1 和 v3 的尺寸、质量、参考图字段分别适配；Gateway 当前没有通用图片异步任务查询/取消接口，不能套用视频任务 URL，也不能将 HTTP 超时解释为任务未受理。

| v1 字段 | 类型 | 必填 / 选择规则 |
| --- | --- | --- |
| model | string | 必填，当前 Key 可调用的模型 ID |
| prompt | string | 必填，编辑时为修改指令 |
| n | integer | 可选，正常提交 1–128；模型上限可更低，未传交给适配器默认 |
| size | string | 可选，如 1024x1024；需模型支持，不能给所有模型同一默认 |
| quality | string | 可选，枚举按模型；不能通用写死 standard/hd |
| response_format | string | 可选，url/b64_json，依模型支持 |
| stream | boolean | 可选，仅支持流的链路使用 true |
| image | string / array / file | 编辑条件必填，形式按渠道和请求类型 |
| mask | file 或模型定义格式 | 局部编辑条件使用；尺寸/透明通道等依模型 |
| watermark / seed | boolean / integer | 模型支持才传；显式 false/0 必须保留 |

具体能力表单暂用已确认模型映射。`/v1/models` 只返回模型名，`/api/pricing` 也没有完整质量、尺寸、多图和参考素材规格；不能据此自动生成所有模型参数或准确报价。

<a id="image-models--文生图与图片结果"></a>

### 文生图与图片结果



```http
POST /v1/images/generations
Authorization: Bearer <FULL_USER_MODEL_API_KEY>
Content-Type: application/json

{"model":"<IMAGE_MODEL_ID>","prompt":"<IMAGE_PROMPT>","n":1,"size":"1024x1024"}
```

遍历 `data[]`：有 url 则读取地址，有 b64_json 则解码 Base64。格式、尺寸、quality 和 response_format 的支持取决于模型；n 从 1 开始，网关兼容接口上限 128，模型可能更小。尺寸使用 ASCII x，不用乘号 ×。

Base64 不一定带 Data URI 前缀；URL 不一定永久有效。需要长期保存时由星途保存文件与用户归属，不能把下载次数或本地估价当作账单。

成功返回 URL 示例：

```json
{"created":1800000000,"data":[{"url":"https://files.example.com/generated/image.png","revised_prompt":"实际采用的提示词"}]}
```

Base64 返回示例（占位串不能直接解码）：

```json
{"created":1800000000,"data":[{"b64_json":"<BASE64_IMAGE_BYTES>"}]}
```

遍历全部 data 项，保留返回顺序和 revised_prompt；不要只保存第一张。星途记录调用用户、项目、模型、请求参数、Gateway 请求编号和实际文件，不保存上游密钥。下载图片直链不用附带 Gateway Bearer，防止泄露给 CDN。

<a id="image-models--图片编辑与流式图片"></a>

### 图片编辑与流式图片



编辑使用 `/v1/images/edits`。文件上传示例：

```bash
curl --fail-with-body 'https://gateway.example.com/v1/images/edits' \
  --header 'Authorization: Bearer <FULL_USER_MODEL_API_KEY>' \
  --form 'model=<IMAGE_EDIT_MODEL_ID>' \
  --form 'prompt=<EDIT_PROMPT>' \
  --form 'image=@<LOCAL_IMAGE_PATH>'
```

让客户端生成 multipart boundary，不手工设置 JSON Content-Type。支持 JSON 参考图的渠道也可发送 `{"model":"<IMAGE_EDIT_MODEL_ID>","prompt":"<EDIT_PROMPT>","image":"https://files.example.com/reference.png"}`。本机路径不能放进 JSON 让上游读取；多图和 mask 按模型支持提供。

支持 mask 的编辑示例：

```bash
curl --fail-with-body 'https://gateway.example.com/v1/images/edits' \
  --header 'Authorization: Bearer <FULL_USER_MODEL_API_KEY>' \
  --form 'model=<MASK_EDIT_MODEL_ID>' \
  --form 'prompt=<EDIT_PROMPT>' \
  --form 'image=@<LOCAL_IMAGE_PATH>' \
  --form 'mask=@<LOCAL_MASK_PATH>' \
  --form 'n=1'
```

多图字段名称和数量依适配器要求，不因单图成功就默认 image[] 可用于全部渠道；客户端上传前校验模型支持和文件大小，服务端仍再次校验。素材库中的 asset:// 句柄不能当作所有图片编辑接口的 image 值。

支持图片流的 v1 渠道传 `"stream":true`，按 Content-Type 和事件 type 解析 b64_json/url。OpenAI 图片适配支持 image_generation.partial_image、image_generation.completed，编辑还可能有 image_edit.*；不能套用文本 delta.content。上游一次性返回 JSON 时也可能被转换成 completed 事件，SSE 不保证有中间预览。

事件形态示例：

```text
event: image_generation.partial_image
data: {"type":"image_generation.partial_image","partial_image_index":0,"b64_json":"<PARTIAL_BASE64>"}

event: image_generation.completed
data: {"type":"image_generation.completed","b64_json":"<FINAL_BASE64>"}
```

临时预览不作为最终成果；completed 之后才标记此结果完成。按空行分帧，合并拆开的 data 行；不要假定一次网络读取就是一个事件。错误事件、断开和用户停止要单独处理，未拿到最终图不显示成功。

<a id="image-models--火山原生图片请求"></a>

### 火山原生图片请求



火山原生图片使用 `POST /api/v3/images/generations`，仅 Volc Native 渠道，按模型原生字段构造。当前控制器完整读取上游响应后返回，不作为实时 SSE 通道；没有 `/api/v3/images/edits` 路由。

```http
POST /api/v3/images/generations
Authorization: Bearer <FULL_USER_MODEL_API_KEY>
Content-Type: application/json

{"model":"<VOLC_NATIVE_IMAGE_MODEL_ID>","prompt":"清晨的城市街景","size":"1024x1024","response_format":"url","watermark":false}
```

size 和其他原生字段必须符合所选模型，不能根据旧 PDF 或网页示例自行将 width/height 当成通用必填。该路径只改写模型映射并透传原生 body；当前不支持渠道参数覆盖，配置了覆盖会返回 param_override_not_supported。成功响应以该模型原生 data/url/b64_json 等字段为准，不加 success/data 管理信封。

<a id="image-models--旧媒体工具的适配与交付"></a>

### 旧媒体工具的适配与交付



星途当前媒体工具仍使用旧 `/api/media/images/...` 及旧响应外壳。保留工具名、参数、回调及官方执行语义，在独立公司适配层把请求转换到本章 v1/v3；不把旧 code/data 解析器直接套到模型结果，不为适配新增不存在的通用图片任务接口。

| 输入/结果 | 转换与持久化要求 |
| --- | --- |
| 模型、尺寸、数量、质量 | 来自已核验能力；未填与 0/false 分开；n 仍受服务端安全上限约束 |
| 本地参考图、mask | 按实际协议发送 multipart、支持的 Data URI 或可读 URL；不上传本地路径字符串 |
| 同步 URL/Base64 | 本次请求直接取得结果；验证内容并保存，不继续查询“图片任务” |
| SSE 部分图/最终图 | 预览单独保留，只有最终事件对应最终结果；错误和中断保留准确状态 |
| 结果写盘 | 绑定 origin/user.id、项目、本地生成记录、模型/协议和请求编号；不保存 Key |
| 保存失败 | 优先重取已获结果或重新保存，不能自动再生成一笔付费请求 |

本地处理状态可以表达提交、接收、保存、完成或失败，但只是客户端记录，不得包装成 Gateway 远程图片任务 ID。换号后仍能按归属查看原有成果，新生成使用新账号的身份与 Key。

验收分别记录 URL、Base64、编辑、支持的流式、长请求中断及文件保存失败；按选定模型实际支持选择场景。成功图、真实消费及星途工具返回三项须能关联，自动测试通过不直接勾选真实图片生成。

<a id="image-models--结果保存-权限变化和错误"></a>

### 结果保存、权限变化和错误



图片工作流顺序为：读取本人 image.generate 和插件权限（未来接口）→ 确认模型 Key 可用 → 构造模型支持的请求 → 提交一次 → 展示结果并保存文件 → 回查消费。权限拒绝要发生在预扣费和上游调用前；现有功能尚无第三章的远程权限守卫，不能当作已实现。

401 检查模型 Key；参数错误修正 size/n/参考图；额度和限流按实际错误区分；HTTP 200 的流内 error 也算失败。停止读取、关闭窗口或下载失败都不代表未计费，费用由 Gateway 账本确认。需要恢复时保存请求编号和已有结果，不盲目重新生成。

本地参考文件需要先转为上游可读取的地址或适配器支持的 Base64；图片编辑可直接使用上面的 multipart。需要注册素材时见[视频接口对接中的素材流程](#video-models--参考素材上传与使用)，不要把素材预览临时地址当成永久文件地址。消费回查见[账号与钱包](#account-login--消费回查与请求结果)。

源码依据：`router/relay-router.go`、`relay/helper/valid_request.go`、`relay/channel/openai/relay_image.go`、`controller/volc_native.go`。协议已读代码核对；真实图片生成尚未两平台联调。

<a id="video-models"></a>

## 6 视频接口对接

版本：2.0 收口稿，2026-10-19。现有任务、素材及 Webhook 入口按实际协议接入；远程插件权限与统一发布仍待开发。参考 PDF 差异表保留历史核对结果，本次原路径未取得 PDF 原件，未新增逐页验证结论。

视频按 Wan 与 Seedance 两类接入，Seedance 再区分 v1 兼容和 v3 原生协议。使用当前用户的模型 Key，按提交、保存任务 ID、查询、下载的顺序完成调用。

本章同时补齐参考 PDF 涉及的素材组/素材 CRUD、AK/SK 签名、视频任务生命周期和素材事件。当前源码为可调用能力的依据，PDF 的写法用于参考详细程度；其中未实现或已变化的内容在末节逐项列出。

<a id="video-models--现有接口总表"></a>

### 现有接口总表



| 方法 | 地址 | 凭证 | 作用 |
| --- | --- | --- | --- |
| POST | `/v1/videos` | 用户模型 Key | v1 提交 Wan/Seedance 等视频 |
| POST | `/v1/video/generations` | 用户模型 Key | v1 兼容提交别名 |
| POST | `/v1/videos/:video_id/remix` | 用户模型 Key | 支持的模型做视频再创作，不是通用任务编辑 |
| GET | `/v1/videos/:task_id` | 模型 Key 或用户会话 | 统一 v1 视频对象 |
| GET | `/v1/video/generations/:task_id` | 模型 Key 或用户会话 | 旧适配器 code/data，格式可能不同 |
| POST | `/api/v3/contents/generations/tasks` | 用户模型 Key | Volc Native 原生 Seedance 提交 |
| GET | `/api/v3/contents/generations/tasks/:task_id` | 模型 Key 或用户会话 | 本人原生任务详情 |
| GET | `/api/v3/contents/generations/tasks` | 模型 Key 或用户会话 | 本人 Volc Native 任务列表 |
| DELETE | `/api/v3/contents/generations/tasks/:task_id` | 用户模型 Key | 取消非终态任务 |
| GET | `/v1/videos/:task_id/content-info` | 模型 Key 或用户会话 | 下载交付方式和地址 |
| GET | `/v1/videos/:task_id/content` | 模型 Key 或用户会话 | 代理/重定向交付内容 |
| GET | `/api/task/self` | 用户会话 | 本人异步任务日志列表，含其他平台 |
| POST | `/?Action=...&Version=2024-01-01` | 素材 AK/SK | 素材组/素材 10 项 Action，见下文 |

本人访问仍校验归属和密钥模型限制；管理端跨用户查阅另有后台权限，星途用户客户端不使用管理员身份。视频 ID 仅使用 Gateway 响应 ID，不接受客户端凭空构造上游任务 ID。

<a id="video-models--协议选择"></a>

### 协议选择



| 模型系列 | 协议 | 请求字段 |
| --- | --- | --- |
| Wan，含 Wan 3.0 | v1 | prompt/image(s)；Wan 3.0 另支持 input/parameters |
| Seedance | v1 兼容 | prompt、images、seconds、metadata |
| Seedance | v3 原生 | content[]、duration、resolution 等原生字段 |

v1 提交 `POST /v1/videos`，兼容 `POST /v1/video/generations`；优先查询 `GET /v1/videos/<TASK_ID>`。v3 提交 `POST /api/v3/contents/generations/tasks`，查询同一路径加 `/<TASK_ID>`。

Wan 3.0 是模型版本，仍使用 v1；火山 v3 路径用于 Volc Native 的 Seedance。模型 ID 使用已开通的公开名称。保存 `{用户ID, 模型ID, 协议, 响应id}`，后续按同一协议查询；id 是 Gateway 公开任务 ID。

<a id="video-models--wan-v1"></a>

### Wan：v1



Wan 3.0 示例，适用于百炼渠道映射后的 wan3.0* 模型：

```http
POST /v1/videos
Authorization: Bearer <FULL_USER_MODEL_API_KEY>
Content-Type: application/json

{
  "model": "<WAN3_PUBLIC_MODEL_ID>",
  "input": {"prompt":"<VIDEO_PROMPT>"},
  "parameters": {"resolution":"720P","ratio":"16:9","duration":5,"audio":false,"prompt_extend":true,"watermark":false,"seed":0}
}
```

图生视频在 input 中加入 media，例如：

```json
{
  "prompt": "<VIDEO_PROMPT>",
  "media": [
    {"type":"first_frame","url":"https://files.example.com/first.png"},
    {"type":"last_frame","url":"https://files.example.com/last.png"}
  ]
}
```

这段替换 input，不是独立完整请求。其他媒体类型与数量按模型支持提供。Wan 3.0 也接受平铺 prompt/image/duration/resolution；重复参数优先级为 metadata.parameters > parameters > 顶层字段，建议只写一处。

Wan 3.0 当前时长 2–30 秒、默认 5，不支持 -1 智能时长；分辨率 480P/720P/1080P；比例 adaptive、21:9、16:9、4:3、1:1、3:4、9:16。显式 false 和 0 会保留。

| Wan 3.0 字段 | 类型 | 必填 / 规则 |
| --- | --- | --- |
| model | string | 必填，公开模型名映射到 wan3.0* |
| input.prompt | string | 文本描述；图生视频也按模型要求传入 |
| input.media | array | 图生视频参考媒体，包含 type/url |
| input.media[].type | string | first_frame/last_frame 等，必须模型支持 |
| input.media[].url | string | 模型可读取的媒体地址，不是本地路径 |
| parameters.duration | integer | 2–30 秒，未传默认 5 |
| parameters.resolution | string | 480P / 720P / 1080P，注意大小写 |
| parameters.ratio | string | 上述比例枚举 |
| parameters.audio / prompt_extend / watermark | boolean | 可选，显式 false 必须保留 |
| parameters.seed | integer | 可选，显式 0 必须保留 |

网关通用视频时长安全上限是 3600 秒，不能当成每个模型支持 3600 秒。Wan 3.0 更小的 30 秒上限仍有效；其他模型按其规则校验。

旧版 Wan 使用 prompt、image(s)、duration/seconds、size 和 metadata，音频开关放 `metadata.parameters.audio`。文生视频 size 使用模型接受的像素尺寸，如 1280*720；图生视频常用分辨率档位。Wan 2.7 图生视频会转成 input.media，不能把 Wan 3.0 的全部规则套到旧模型。

<a id="video-models--seedance-v3-原生与-v1-兼容"></a>

### Seedance：v3 原生与 v1 兼容



原生 v3 请求：

```http
POST /api/v3/contents/generations/tasks
Authorization: Bearer <FULL_USER_MODEL_API_KEY>
Content-Type: application/json

{
  "model": "<SEEDANCE_NATIVE_PUBLIC_MODEL_ID>",
  "content": [{"type":"text","text":"<VIDEO_PROMPT>"}],
  "resolution": "720p",
  "ratio": "16:9",
  "duration": 5,
  "generate_audio": false,
  "watermark": false
}
```

参考图加入 content：`{"type":"image_url","role":"first_frame","image_url":{"url":"https://files.example.com/first.png"}}`；尾帧用 last_frame，参考图用 reference_image，均需模型支持。已注册并同步的素材可用 `asset://<ASSET_ID>`，其状态、归属和渠道副本决定能否使用。

content 必须是非空数组。duration 是整数；网关允许 -1 到通用上限，实际模型时长范围与 -1 是否可用仍由模型决定。Seedance 2.5 当前档位只接受 480p/720p/1080p。

<a id="video-models--seedance-原生字段表"></a>

#### Seedance 原生字段表



| 字段 | 类型 | 必填 / 规则 |
| --- | --- | --- |
| model | string | 必填，已开通的 Volc Native 公开模型 ID |
| content | array | 必填且非空，按模型接受的文本与媒体排列 |
| content[].type | string | text / image_url，其他媒体类型由具体模型决定 |
| content[].text | string | type=text 时使用 |
| content[].image_url.url | string | type=image_url 时使用；公网 URL、模型支持的 Data URI 或 asset:// |
| content[].role | string | first_frame / last_frame / reference_image，按模型能力；不强加通用默认 |
| duration | integer | 可选，-1 或非负值仍需具体模型接受，上限 3600 |
| resolution | string | 可选；Seedance 2.5 为 480p / 720p / 1080p，不写通用 4k |
| ratio | string | 依模型支持，如 16:9 / 9:16 / 1:1 / adaptive |
| generate_audio / watermark | boolean | 可选，不传交给上游；false 不是未传 |
| seed | integer | 可选，按模型范围，0 原样保留 |

多素材请求示例：

```json
{"model":"<SEEDANCE_NATIVE_PUBLIC_MODEL_ID>","content":[{"type":"text","text":"参考两张图中的造型，生成办公室场景"},{"type":"image_url","role":"reference_image","image_url":{"url":"asset://<ASSET_ID_1>"}},{"type":"image_url","role":"reference_image","image_url":{"url":"asset://<ASSET_ID_2>"}}],"duration":5,"resolution":"720p","ratio":"16:9","generate_audio":false,"watermark":false}
```

Base64 形式为 `data:image/jpeg;base64,<BASE64_IMAGE_BYTES>`，不能把“PDF 写了单图 30MB”视为所有 Gateway/模型的共同允许值；实际还受请求体、模型、媒体类型和存储策略限制。注册素材也不能保证通过上游内容审核。

v1 兼容请求：

```http
POST /v1/videos
Authorization: Bearer <FULL_USER_MODEL_API_KEY>
Content-Type: application/json

{
  "model": "<SEEDANCE_COMPAT_PUBLIC_MODEL_ID>",
  "prompt": "<VIDEO_PROMPT>",
  "images": ["https://files.example.com/first.png"],
  "seconds": "5",
  "metadata": {"resolution":"720p","ratio":"16:9","generate_audio":false,"watermark":false}
}
```

文生视频省略 images。v1 的时长从 seconds 或 metadata.duration 构造，不要只传顶层 duration；高级选项放 metadata。非文本媒体角色可放 metadata.content，其中 text 项会被顶层 prompt 替换。需要完整保留原生请求结构时使用 v3。

<a id="video-models--查询-取消与下载"></a>

### 查询、取消与下载



提交后先保存响应顶层 id。v1 提交成功的核心字段示例：

```json
{"id":"<GATEWAY_TASK_ID>","object":"video","model":"<PUBLIC_MODEL_ID>","status":"queued","progress":0,"created_at":1900000000}
```

v3 提交响应保留上游原生字段，Gateway 替换 id；最小处理只要求读取 `{"id":"<GATEWAY_TASK_ID>"}`，不要要求它立即包含 status、视频地址或套上 success/data。

| 查询协议 | 状态与结果 |
| --- | --- |
| `GET /v1/videos/<TASK_ID>` | 顶层视频对象；queued/in_progress/completed/failed/unknown；整数 progress，结果 metadata.url，错误 error |
| `GET /api/v3/contents/generations/tasks/<TASK_ID>` | 原生结构；queued/running/succeeded/failed/cancelled/expired；结果 content.video_url |

旧 `GET /v1/video/generations/<TASK_ID>` 返回 code/data，且不同适配器有状态与结果字段差异；新接入统一选上表的查询地址。v3 查询的是 Gateway 保存并更新的任务快照，不是每次直查上游。

完成时的字段示例（省略其他字段）：

```json
{"id":"<GATEWAY_TASK_ID>","status":"completed","progress":100,"metadata":{"url":"https://files.example.com/result.mp4"}}
```

```json
{"id":"<GATEWAY_TASK_ID>","status":"succeeded","content":{"video_url":"https://files.example.com/result.mp4"}}
```

前一段为 v1，后一段为 v3。完成或失败等终态停止轮询；失败展示 error.code/message。查询串行发送，页面恢复后继续查询已有 ID。unknown 不视为成功，不能显示一个不存在的结果。

v3 本人任务列表用 `GET /api/v3/contents/generations/tasks?page_num=1&page_size=20`，返回顶层 items/total；非终态取消用 `DELETE /api/v3/contents/generations/tasks/<TASK_ID>`。取消效果取决于实际状态，不能提前确认退款。当前没有对应的通用 v1 取消入口，不能把同一个 DELETE 路径套到 Wan。

<a id="video-models--v3-列表字段-取消响应和状态流转"></a>

#### v3 列表字段、取消响应和状态流转



| Query | 类型 | 默认 / 边界 |
| --- | --- | --- |
| page_num | integer | 默认 1；超过 500 规范为 500 |
| page_size | integer | 默认 10；最大 100 |
| filter.model | string | 可选，按 Gateway 保存的公开模型名精确筛选 |
| filter.status | string | 可选，按 v3 状态精确筛选 |
| filter.service_tier | string | 可选，按保存的 service_tier 筛选 |

```json
{"items":[{"id":"<GATEWAY_TASK_ID>","model":"<SEEDANCE_NATIVE_PUBLIC_MODEL_ID>","status":"succeeded","content":{"video_url":"https://files.example.com/result.mp4"},"created_at":1800000000}],"total":1}
```

列表只查询当前用户 Gateway 已保存的 Volc Native 任务，不枚举共享上游账号的全部任务；total 为筛选后可访问数。不是所有平台的通用视频历史。

```http
DELETE /api/v3/contents/generations/tasks/<GATEWAY_TASK_ID>
Authorization: Bearer <FULL_USER_MODEL_API_KEY>
```

成功取消的核心形态：

```json
{"id":"<GATEWAY_TASK_ID>","status":"cancelled"}
```

当前取消响应通过本地 FailReason 映射为 status=cancelled，不保证额外合成 error 对象，不能照 PDF 的 failed/error.cancelled 作为唯一成功条件。已终态直接返回原快照；并发完成时可能返回 succeeded，不能在点击取消后立即显示“已取消且全额退款”。成功抢占本地取消状态才由 Gateway 执行一次退款；账本回查是到账依据。

| 协议 | 处理中 | 终态 |
| --- | --- | --- |
| v1 详情 | queued / in_progress | completed / failed；unknown 需回查 |
| v3 详情 | queued / running | succeeded / failed / cancelled / expired |
| 任务日志 | NOT_START / SUBMITTED / QUEUED / IN_PROGRESS | SUCCESS / FAILURE；UNKNOWN 需回查 |

星途保存原始协议状态再映射统一界面，不把 v1 completed、v3 succeeded、日志 SUCCESS 当成同一个原始值。先保存任务 ID 再渲染进度；轮询间隔可从 3–5 秒起并退避，界面关闭后恢复同一任务，不重复提交。

下载先查 `GET /v1/videos/<TASK_ID>/content-info`：data.delivery_mode 为 redirect 时使用 data.url，为 proxy 时携带用户模型 Key 读取 `/v1/videos/<TASK_ID>/content`。不要把 Gateway Bearer 发给 CDN。地址可能过期，长期保存另走文件存储。

普通下载示例：

```bash
curl --fail-with-body --location --output result.mp4 '<VIDEO_URL>'
```

服务器支持 Range 时才使用断点续传；不能承诺任何存储地址固定 24 小时或带宽提升固定倍数。长期保存要在地址有效期内转存实际文件，记录所属用户/任务；url 不等于持久文件。proxy 下载凭证只发 Gateway origin，跟随到其他域名时不转发 Authorization。

<a id="video-models--参考素材上传与使用"></a>

### 参考素材上传与使用



素材 REST 使用用户会话或模型 Key；下面使用用户 Access Token。先用 `GET /api/asset-library/groups` 选择本人分组，或创建一组：

```http
POST /api/asset-library/groups
Authorization: Bearer <USER_ACCESS_TOKEN>
Content-Type: application/json

{"name":"<ASSET_GROUP_NAME>","description":"<DESCRIPTION>"}
```

保存返回的 `data.id` 作为素材分组 ID，然后上传文件：

```bash
curl --fail-with-body 'https://gateway.example.com/api/asset-library/assets' \
  --header 'Authorization: Bearer <USER_ACCESS_TOKEN>' \
  --form 'group_id=<ASSET_GROUP_ID>' \
  --form 'name=<ASSET_NAME>' \
  --form 'type=image' \
  --form 'file=@<LOCAL_IMAGE_PATH>'
```

type 支持 image/video/audio，必须与真实文件类型一致；容量和数量受平台存储策略限制。上传返回 `data.id/status`，保存 Gateway 素材 ID。`GET /api/asset-library/assets?group_id=<ASSET_GROUP_ID>&p=1&page_size=20` 返回 `data.items`，用它回查 processing/ready/unavailable 等状态和 unavailable_reason。

Seedance 原生请求的 content 可引用 `asset://<ASSET_ID>`。上传成功只表示文件和记录已创建，不等于所选生成渠道的副本已经可用；ready 也不能证明每个渠道都已同步。未同步或副本未 active 时可能返回 409，unavailable 素材会被拒绝，需更换素材，不能循环重发生成。Wan 的普通 HTTP(S) 图片 URL 不自动等同于 Seedance 的 asset:// 引用。

预览用 `GET /api/asset-library/assets/<ASSET_ID>/preview` 的 `data.url/expires_at`，过期重新取得；删除用 `DELETE /api/asset-library/assets/<ASSET_ID>`。该库是已注册媒体素材，不是通用 `/v1/files` 上传服务；当前 `/v1/files` 只有未实现占位路由。

有公网接收服务时可接 `GET /api/webhooks/capabilities`、`GET/POST /api/webhook-endpoints` 的媒体事件；否则先轮询任务/素材。Action RPC 使用另一套素材 AK/SK 签名，星途走上述 REST 不需混用。事件格式见公开媒体/素材 Webhook 文档；它们不承担订阅和钱包通知。

<a id="video-models--rest-素材接口补全"></a>

#### REST 素材接口补全



| 方法 | 地址 | 请求 / 返回 |
| --- | --- | --- |
| GET | `/api/asset-library/groups` | 无分页参数时 data 数组；带 p/page_size/search 时 data.items/total/page/page_size |
| POST | `/api/asset-library/groups` | JSON name/description，返回 data.id |
| PUT | `/api/asset-library/groups/:id` | JSON name/description，完整更新 |
| DELETE | `/api/asset-library/groups/:id` | 删除分组，组内有未删除素材时拒绝 |
| GET | `/api/asset-library/assets` | group_id/search/p/page_size，返回分页 data.items |
| POST | `/api/asset-library/assets` | multipart group_id/name/type/file，返回 data 素材 |
| GET | `/api/asset-library/assets/:id/preview` | 返回 data.url/expires_at；variant 按预览服务支持 |
| DELETE | `/api/asset-library/assets/:id` | 删除本人素材 |

分组 name 1–64 字符、description 最长 300；素材名最长 255，type 为 image/video/audio，需与文件类型一致。REST 当前没有素材单条 GET 或改名 PUT；详情可从本人列表取得，若需要单条查询/改名，可使用以下已实现 Action 接口但必须切换为 AK/SK 鉴权。

<a id="video-models--素材-ak-sk-取得与签名-现有"></a>

### 素材 AK/SK 取得与签名（现有）



AK/SK 属于 Gateway 的当前用户，不是上游火山企业主密钥。星途桌面可按用户隔离保存本人的凭证；有服务端时保存在对应用户的安全凭证容器。跨用户不能共用运营账号的 AK/SK。

| 方法 | 地址 | 身份 | 作用 |
| --- | --- | --- | --- |
| GET | `/api/asset-library/access-keys` | 用户会话 | data 数组，只返回 AK 和 secret_hint |
| POST | `/api/asset-library/access-keys` | 用户会话 | `{"name":"星途素材接入"}`，创建并仅此一次返回 SK |
| DELETE | `/api/asset-library/access-keys/:id` | 用户会话 | 停用凭证，路径为数字记录 ID |

名称 1–64 字符，每用户最多 5 个启用凭证。创建成功节选：

```json
{"success":true,"data":{"id":12,"name":"星途素材接入","access_key_id":"<ASSET_ACCESS_KEY_ID>","secret_access_key":"<ASSET_SECRET_ACCESS_KEY>","secret_hint":"<MASKED_HINT>","status":"enabled","last_used_at":0,"created_at":1800000000}}
```

之后列表无法重新取得 SK；遗失需新建并迁移后停用旧凭证。不要在 UI 日志或错误遥测打印完整 SK。

<a id="video-models--签名请求组成"></a>

#### 签名请求组成



统一入口 `POST /?Action=<ACTION>&Version=2024-01-01`，亦有 `/api/v3/` 别名；签名必须使用真正发送的路径。Region=cn-beijing、Service=ark，X-Date 为 UTC `YYYYMMDDTHHMMSSZ`，容许与服务端约 5 分钟偏差。

```http
Content-Type: application/json
X-Date: <UTC_TIMESTAMP>
X-Content-Sha256: <SHA256_OF_EXACT_UTF8_BODY>
Authorization: HMAC-SHA256 Credential=<AK>/<YYYYMMDD>/cn-beijing/ark/request, SignedHeaders=content-type;host;x-content-sha256;x-date, Signature=<SIGNATURE>
```

body 只序列化一次，签名和发送使用同一批 UTF-8 字节。Host 含实际域名及必要端口，不能写死 PDF 的域名；代理不得修改签名所用 Host/path/query。规范请求按 method、URI、排序编码后的 query、规范头、SignedHeaders、body hash 以换行连接。

<a id="video-models--素材组与素材-action-全生命周期-现有"></a>

### 素材组与素材 Action 全生命周期（现有）



以下每项方法都是 POST、Version 都是 2024-01-01，使用上述 AK/SK，不使用 Bearer。成功信封：

```json
{"ResponseMetadata":{"RequestId":"<REQUEST_ID>","Action":"CreateAssetGroup","Version":"2024-01-01","Service":"ark","Region":"cn-beijing"},"Result":{"Id":"<ASSET_GROUP_ID>"}}
```

<a id="video-models--素材组五项-action"></a>

#### 素材组五项 Action



| Action | JSON 请求示例 | Result / 约束 |
| --- | --- | --- |
| CreateAssetGroup | `{"Name":"参考角色组","Description":"正面与侧面","GroupType":"AIGC"}` | `{Id}`；Name 必填 1–64，Description 最长 300，GroupType 可省略或 AIGC |
| GetAssetGroup | `{"Id":"<ASSET_GROUP_ID>"}` | 单组详情，只查本人 |
| ListAssetGroups | `{"PageNumber":1,"PageSize":20,"Filter":{"Name":"参考","GroupIds":["<ASSET_GROUP_ID>"]},"SortBy":"CreateTime","SortOrder":"Desc"}` | Items/TotalCount/PageNumber/PageSize |
| UpdateAssetGroup | `{"Id":"<ASSET_GROUP_ID>","Name":"参考角色组二","Description":"更新说明"}` | `{Id}`；Name/Description 是完整更新 |
| DeleteAssetGroup | `{"Id":"<ASSET_GROUP_ID>"}` | `{}`；有未删除素材时拒绝 |

GetAssetGroup 的真实 Result 形态：

```json
{"Id":"<ASSET_GROUP_ID>","Name":"参考角色组","Description":"正面与侧面","GroupType":"AIGC","ProjectName":"default","CreateTime":"2026-10-16T00:00:00Z","UpdateTime":"2026-10-16T00:00:00Z"}
```

ListAssetGroups 分页从 1 开始、默认 1/20，PageSize 最大 100。Filter.Name 模糊匹配；GroupIds 为精确公开 ID 数组。SortBy 支持 CreateTime/UpdateTime/Name，SortOrder 为 Asc/Desc。不要把 REST 的 p/page_size 或整数数据库主键带进 RPC。

<a id="video-models--素材五项-action"></a>

#### 素材五项 Action



| Action | JSON 请求示例 | Result / 约束 |
| --- | --- | --- |
| CreateAsset | `{"GroupId":"<ASSET_GROUP_ID>","Name":"正面参考图","URL":"https://files.example.com/portrait.png","AssetType":"image"}` | `{Id}`；GroupId/URL 必填；Name 建议显式提供，空则回退下载文件名；类型需明确指定 |
| GetAsset | `{"Id":"<ASSET_ID>"}` | 元数据、Status、FailureReason（失败时）、预览可用情况 |
| ListAssets | `{"PageNumber":1,"PageSize":20,"Filter":{"GroupIds":["<ASSET_GROUP_ID>"],"Statuses":["Active"],"Name":"参考"}}` | Items/TotalCount/PageNumber/PageSize |
| UpdateAsset | `{"Id":"<ASSET_ID>","Name":"正面高清参考图"}` | `{Id}`；只改名称，1–255 字符 |
| DeleteAsset | `{"Id":"<ASSET_ID>"}` | `{}`；删除本人素材 |

CreateAsset 使用可公开读取的 HTTP(S) URL，Gateway 拉取并按存储策略校验类型/大小，支持 image/video/audio 的实际已配置链路；不能用 Base64、file:// 或需要额外鉴权头的源。当前 AssetType 空值会被服务端拒绝，必须显式传入，不依赖 PDF 的 image 默认。类型存在不等于所选上游渠道支持该媒体建模。

GetAsset Result 字段示例：

```json
{"Id":"<ASSET_ID>","GroupId":"<ASSET_GROUP_ID>","Name":"正面参考图","AssetType":"Image","Status":"Active","URL":"https://files.example.com/preview.png","URLExpiresAt":1800000600,"PreviewStatus":"Available","Moderation":{"Strategy":"Default"},"ProjectName":"default","CreateTime":"2026-10-16T00:00:00Z","UpdateTime":"2026-10-16T00:00:00Z"}
```

Status 为 Processing/Active/Failed，Failed 时带 FailureReason。预览签名失败仍返回审核状态，同时 PreviewStatus=Unavailable、PreviewErrorCode=preview_unavailable，URL 可能为空；不能把预览故障当成审核通过或素材不存在。CreateTime/UpdateTime 是 RFC3339 字符串，URLExpiresAt 为 Unix 秒。

ListAssets 必须用 Filter.GroupIds 筛组选中分组；当前控制器不会用顶层 GroupId 做列表过滤，不能照抄 PDF 的这一写法。Statuses 支持 Active、Processing、Failed/Rejected 映射，未知状态不会扩大查询范围。列表同样默认 1/20、最大 100，可用 SortBy/SortOrder；每项结构与 GetAsset 一致。

素材 Active 是本地可用状态，真正生成时还要校验选中渠道的副本和审核状态。获取 asset:// 句柄不保证无需等待或绕过真人审核。上游明确拒绝后 Failed 为同一素材终态；需要修复应创建新素材 ID。

<a id="video-models--action-错误信封"></a>

#### Action 错误信封



```json
{"ResponseMetadata":{"RequestId":"<REQUEST_ID>","Action":"GetAsset","Version":"2024-01-01","Service":"ark","Region":"cn-beijing","Error":{"Code":"ResourceNotFound","Message":"resource not found"}}}
```

| HTTP | ResponseMetadata.Error.Code | 含义 |
| --- | --- | --- |
| 400 | InvalidParameter / InvalidRequest | 参数/Version/JSON 不合法 |
| 400 | InvalidAction | 未支持的 Action，包括当前五个 Webhook Action |
| 401 | AuthFailure | AK/SK、签名、时间或凭证状态不正确 |
| 404 | ResourceNotFound | 不存在或不属于本人 |
| 413 | InvalidRequest / InvalidParameter | Action body 超过 1 MiB 或导入媒体超出策略限制 |
| 409 | InvalidParameter | 当前资源/同步状态冲突 |
| 5xx | InternalError | 平台、存储、下载或上游暂时故障 |

按 HTTP 和 Error 同时判断，不能把缺少 Result 当成空对象成功。跨用户资源保持不可见，不能通过换素材 ID 访问其他用户。

<a id="video-models--素材与任务-webhook-现有-rest-非权益推送"></a>

### 素材与任务 Webhook（现有 REST，非权益推送）



参考 PDF 的五个 Action 与当前可用替代地址逐项对应如下；左列目前全部未实现，调用会得到 InvalidAction，不能使用 AK/SK 直接替代右列的登录凭证：

| PDF 的 Action（当前未实现） | 当前 REST 替代 |
| --- | --- |
| CreateAssetWebhookEndpoint | `POST /api/webhook-endpoints` |
| ListAssetWebhookEndpoints | `GET /api/webhook-endpoints` |
| UpdateAssetWebhookEndpoint | `PUT /api/webhook-endpoints/:id` |
| DeleteAssetWebhookEndpoint | `DELETE /api/webhook-endpoints/:id` |
| TestAssetWebhookEndpoint | `POST /api/webhook-endpoints/:id/test` |

先 `GET /api/webhooks/capabilities` 查询可用类别；平台关闭某类别时，创建/修改订阅该类别的端点会被拒绝。GET/POST/PUT/DELETE 的 `/api/webhook-endpoints` 使用当前用户登录 Access Token；模型 Key 和 AK/SK 不能替代此处的 UserAuth。

| 方法 | 地址 | 请求 / 成功 data |
| --- | --- | --- |
| GET | `/api/webhook-endpoints` | 数组，当前有效端点，无分页 |
| POST | `/api/webhook-endpoints` | name/url/event_types，返回完整端点 |
| PUT | `/api/webhook-endpoints/:id` | 同创建的完整字段，不是部分 PATCH |
| DELETE | `/api/webhook-endpoints/:id` | 停用，success=true |
| POST | `/api/webhook-endpoints/:id/test` | 无业务 body，返回真实探测结果 |

同一套端点还有 `/api/asset-library/webhook-endpoints` 兼容入口，不要各建一遍。每用户最多 5 个启用端点；name 1–64，url 最长 2048，公网 HTTPS、无内嵌用户名密码/fragment，出站执行 SSRF 限制；event_types 至少一项。素材为 asset.active/asset.failed，媒体任务为 task.status_changed，均需部署允许。

```http
POST /api/webhook-endpoints
Authorization: Bearer <USER_ACCESS_TOKEN>
Content-Type: application/json

{"name":"星途服务端素材通知","url":"https://receiver.example.com/webhooks/assets","event_types":["asset.active","asset.failed"]}
```

```json
{"success":true,"data":{"id":"we-example","object":"webhook_endpoint","name":"星途服务端素材通知","url":"https://receiver.example.com/webhooks/assets","event_types":["asset.active","asset.failed"],"status":"enabled","created_at":1800000000,"updated_at":1800000000}}
```

测试成功调用不等于端点可达，需看 data.reachable/http_status/duration_ms/error；测试返回 event_id、webhook_id 供关联。它是实际连通探测，不是 PDF 所写“只入队后返回 EventId”。桌面客户端没有稳定公网接收地址时直接轮询，不给每台电脑配置公网端点；有共享接收服务才按 Gateway 用户归属分发通知。

<a id="video-models--素材事件与乱序处理"></a>

#### 素材事件与乱序处理



Gateway POST 到接收地址，携带 Content-Type=application/json、User-Agent=New-API/1.0、webhook-id、webhook-timestamp；当前没有独立事件签名密钥。需要权威确认时用当前用户的 AK/SK GetAsset 或 REST 列表回查，不能信任任意客户端自报的回调内容。

```json
{"id":"evt-example","object":"event","type":"asset.failed","created_at":1800000000,"owner_user_id":123,"data":{"id":"<ASSET_ID>","group_id":"<ASSET_GROUP_ID>","status":"Failed","failure_reason":"policy_rejected","updated_at":1800000000,"state_version":2}}
```

asset.active 对应 Active、state_version=1；asset.failed 对应 Failed、版本=2。接收端先持久化，再回 2xx，异步更新业务；webhook-id 去重，同素材只接受严格更高 state_version。收到 2 后不被迟到的 1 覆盖；无版本旧事件先回查。失败素材阻止新引用，但不删除已完成生成成果。

task.status_changed 使用任务状态枚举和单调 state_version，响应快照可能省略/截断，收到后可按 task_id 读取权威详情。不能按素材固定 1/2 解释任务版本。

当前请求超时约 10 秒，失败退避重试，保留至 72 小时，单间隔上限 12 小时；同端点重试 webhook-id 和 body 不变，timestamp 可变化。没有承诺严格有序、恰好一次或全部历史回放。媒体/素材通知不包含钱包、订阅、插件权限或版本发布事件，这些另按第三章回查。

<a id="video-models--ak-sk-到视频结果的-python-示例-现有协议"></a>

### AK/SK 到视频结果的 Python 示例（现有协议）



以下为可改写的服务端/桌面主进程参考实现，依赖 requests。所有凭证从环境读取；代码只演示函数定义，只有显式调用 create_video 才会提交付费视频任务，素材创建需另行调用。运行前确认模型、素材归属/副本和参数已可用；示例未进行真实生成验证。

```python
import hashlib
import hmac
import json
import os
import time
from datetime import datetime, timezone
from urllib.parse import urlencode, urlsplit

import requests

BASE = os.environ["GATEWAY_BASE_URL"].rstrip("/")
AK = os.environ["GATEWAY_ASSET_AK"]
SK = os.environ["GATEWAY_ASSET_SK"]
MODEL_KEY = os.environ["GATEWAY_MODEL_KEY"]


def action_call(action, payload):
    target = urlsplit(BASE)
    if (target.scheme != "https" or not target.hostname or target.username
            or target.password or target.query or target.fragment
            or target.path not in ("", "/")):
        raise ValueError("GATEWAY_BASE_URL must be an HTTPS origin")
    now = datetime.now(timezone.utc)
    x_date = now.strftime("%Y%m%dT%H%M%SZ")
    date = now.strftime("%Y%m%d")
    body = json.dumps(payload, ensure_ascii=False, separators=(",", ":")).encode()
    digest = hashlib.sha256(body).hexdigest()
    query = urlencode(sorted({"Action": action, "Version": "2024-01-01"}.items()))
    signed = "content-type;host;x-content-sha256;x-date"
    headers_text = (
        f"content-type:application/json\nhost:{target.netloc}\n"
        f"x-content-sha256:{digest}\nx-date:{x_date}\n"
    )
    canonical = "\n".join(["POST", "/", query, headers_text, signed, digest])
    scope = f"{date}/cn-beijing/ark/request"
    to_sign = "\n".join([
        "HMAC-SHA256", x_date, scope,
        hashlib.sha256(canonical.encode()).hexdigest(),
    ])
    key = SK.encode()
    for item in (date, "cn-beijing", "ark", "request"):
        key = hmac.new(key, item.encode(), hashlib.sha256).digest()
    signature = hmac.new(key, to_sign.encode(), hashlib.sha256).hexdigest()
    headers = {
        "Content-Type": "application/json", "X-Date": x_date,
        "X-Content-Sha256": digest,
        "Authorization": (
            f"HMAC-SHA256 Credential={AK}/{scope}, "
            f"SignedHeaders={signed}, Signature={signature}"
        ),
    }
    response = requests.post(
        f"{BASE}/?{query}", headers=headers, data=body,
        timeout=(10, 60), allow_redirects=False,
    )
    response.raise_for_status()
    data = response.json()
    if data.get("ResponseMetadata", {}).get("Error"):
        raise RuntimeError("asset action rejected; inspect sanitized error")
    return data["Result"]


def create_video(group_id, asset_id, model_id, prompt):
    # group_id/asset_id must belong to this user; no implicit asset creation.
    asset = action_call("GetAsset", {"Id": asset_id})
    if asset.get("GroupId") != group_id or asset.get("Status") != "Active":
        raise RuntimeError("asset is not available in the requested group")
    headers = {"Authorization": f"Bearer {MODEL_KEY}"}
    response = requests.post(
        f"{BASE}/api/v3/contents/generations/tasks", headers=headers,
        json={
            "model": model_id, "duration": 5, "resolution": "720p",
            "ratio": "16:9", "generate_audio": False, "watermark": False,
            "content": [
                {"type": "text", "text": prompt},
                {"type": "image_url", "role": "reference_image",
                 "image_url": {"url": f"asset://{asset_id}"}},
            ],
        }, timeout=(10, 120), allow_redirects=False,
    )
    response.raise_for_status()
    task_id = response.json()["id"]
    # Persist task_id with user/model/protocol before starting polling.
    return task_id


def wait_for_video(task_id, timeout_seconds=600):
    deadline = time.monotonic() + timeout_seconds
    while time.monotonic() < deadline:
        response = requests.get(
            f"{BASE}/api/v3/contents/generations/tasks/{task_id}",
            headers={"Authorization": f"Bearer {MODEL_KEY}"},
            timeout=(10, 30), allow_redirects=False,
        )
        response.raise_for_status()
        task = response.json()
        if task.get("status") == "succeeded":
            return task["content"]["video_url"]
        if task.get("status") in ("failed", "cancelled", "expired"):
            raise RuntimeError("task reached a non-success terminal state")
        time.sleep(4)
    raise TimeoutError("keep task_id and resume polling; do not resubmit")
```

创建分组和导入素材使用同一个 action_call：`action_call("CreateAssetGroup", {...})` → 保存 Result.Id；`action_call("CreateAsset", {...})` → 保存新素材 ID；用 GetAsset 等待 Active，再 create_video，保存任务后 wait_for_video。不要把素材还在 Processing 当作失败而不断重建，也不自动重试结果不明的创建请求。

<a id="video-models--旧媒体链路替换-任务恢复与验收"></a>

### 旧媒体链路替换、任务恢复与验收



星途的旧 `/api/media/videos/...` 模型列表、提交、轮询与取消不能原样转发给本仓库。模型目录用选定 Key 的 `/v1/models` 并结合实测能力；提交按 Wan/Seedance 和 v1/v3 分开，返回按各协议解析。保留现有工具调用契约，通过独立公司适配完成转换。

| 链路 | 必须保存/判断 | 验收重点 |
| --- | --- | --- |
| 提交 | origin、user.id、model_id、协议、公开 task_id、可取得的请求编号 | 拿到 id 后再轮询；超时结果不明先查记录，不自动重新提交 |
| 轮询与重启 | 按原协议查同一公开 id，保留最后响应和实际终态 | 重开窗口/网络恢复继续原任务；不将所有状态硬套旧 code/data |
| 取消 | 当前协议/模型是否支持；取消响应与后续快照 | Wan 不伪造通用取消；取消与完成竞争按最终结果和账本确认 |
| 下载与保存 | content-info/内容地址、用户归属、本地成果 | 链接失效按原任务重取；失败不重新生成；不转发 Gateway Bearer 到外部 CDN |
| 费用 | 请求和消费关联、账务来源、实际结算 | 移除或替换固定 100 积分/秒假定；无报价接口时不展示为准确报价 |
| 素材 | group/asset 归属、Active、所选渠道副本可用 | Processing 等待，Failed 明确处理；不能仅凭 asset:// 字符串断言可用 |
| Webhook | 事件归属、重复/乱序和最终回查 | 回调用于收敛任务/素材状态，不授予插件权益、不自动触发新生成 |

Wan v1、Seedance v1、Seedance v3 分别保留一次提交到结果的证据；未开通或不支持的组合列明限制。再验证轮询中断恢复、错误终态、支持的取消、下载保存及消费回查。素材/回调另记实际结果，某个视频成功不能替代整条素材生命周期验收。

新功能撤权只阻断相关新执行，已有任务查取/取消和成果回收按归属继续。统一版本不合规也不能删除原视频文件、原素材配置或账务记录。此处为待实施适配要求，不是新增路由或本次真实生成记录。

<a id="video-models--参考-pdf-与当前实现差异"></a>

### 参考 PDF 与当前实现差异



| PDF 中的内容 | 当前核对结果 | 本手册处理 |
| --- | --- | --- |
| 五个 Create/List/Update/Delete/TestAssetWebhookEndpoint Action | HandleVolcAssetAction 只处理前述 10 项素材 Action，这 5 项未实现 | 使用现有 REST 端点管理；若要求保留 PDF 的 AK/SK 契约需另补实现 |
| ListAssets 顶层 GroupId 筛选 | 列表实际读取 Filter.GroupIds | 示例改成 Filter.GroupIds |
| 素材 CreatedAt 数字、AssetType 默认 image | 实际 CreateTime/UpdateTime 为 RFC3339，空 AssetType 不通过 | 明确实际字段和显式类型 |
| GET 素材状态 Available | 当前使用 Active/Processing/Failed | 按实际枚举解析 |
| v3 提交固定有 id/model/status/created_at | 返回保留上游字段，只保证替换公开 id | 最小只依赖 id，详情另查 |
| v3 id 直接示为上游 cgt-* | 返回 Gateway 公开任务 ID | 不拼上游 ID、不跨协议混用 |
| 统一 4k、固定 5/10 秒、固定参数默认 | 不同模型能力不同，Seedance 2.5 只到 1080p | 分模型说明，不写成通用能力 |
| asset:// 百分百绕过真人审核 | 素材仍有上游审核和渠道副本要求 | 明确状态/归属/同步条件 |
| 取消返回 failed/error.cancelled 且一定释放全部配额 | 当前成功映射 status=cancelled，终态/并发完成可能不取消，退款由状态竞争结果决定 | 按返回快照与账本确认，不强求 error 对象 |
| 视频必定 TOS CDN、24h、固定提速 | 存储和交付方式可变 | 查询 content-info，不承诺固定供应商/有效期/性能 |
| Python 示例与无限轮询 | 缺少请求超时和 cancelled/expired 等终态边界 | 提供可解析、有限时轮询和单次提交示例 |

缺少 Webhook Action 是实现缺口，不通过文档改名假装已提供；其余差异按当前源码和所选模型说明。公开文档的素材/Webhook 页面可继续参考，但本章明确的现有契约优先用于两平台接入。

任务历史、提交超时和消费回查统一见[账号与钱包](#account-login--消费回查与请求结果)。当前没有公开的用户参数报价接口，不能固定按 100 积分/秒估价。

源码依据：`router/video-router.go`、`relay/channel/task/{ali,doubao,volcnative}/`、`controller/{volc_native,video_proxy,task,asset_library}.go`、`service/assetlibrary/`。协议已读代码核对并有本地适配测试；真实生成、素材与回调尚未两平台联调。
