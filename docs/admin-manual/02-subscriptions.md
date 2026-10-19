# 套餐与订阅

版本：2.0 收口稿，2026-10-19。Gateway 现有用户购买和管理接口按下文说明；星途当前阶段仅展示套餐与订阅，购买、支付和偏好写入不属于已启用的客户端流程。商品详情与插件权益 API 仍为待开发契约。

聚合网关（Gateway）是套餐、订阅和额度账本的管理中心。运营人员在 Gateway 后台配置套餐及插件权益；星途只展示允许购买的内容、提交用户购买请求、读取最终权益并落实界面和执行限制。

本章标为“现有”的接口已经存在于当前分支；“拟新增”是待开发契约，不能作为已上线接口调用。套餐的价格/额度定义、展示详情、功能授权是三个关联到同一 `plan_id` 的对象。现有套餐接口只实现其中的价格/额度定义，插件权益与详细介绍需按下文补齐。

## 业务对象和完整接入顺序

| 对象 | 唯一标识 | 负责什么 | 生效方式 |
| --- | --- | --- | --- |
| 套餐定义 | `plan.id` | 价格、期限、额度、重置、支付方式 | 管理员维护，用户读取可购项 |
| 用户订阅 | `subscription.id` | 某用户实际持有的额度和有效期 | 支付回调或管理员发放后创建 |
| 套餐详情（拟新增） | `plan_id` | 亮点、详细说明、展示顺序与角标 | 只影响商品展示，不授予权限 |
| 套餐功能（拟新增） | `plan_id + feature_id` | 授予模型功能、插件管理和指定插件 | 服务端按有效订阅计算最终权限 |
| 用户覆盖（拟新增） | `user_id + feature_id` | 单人试用、禁用、隐藏、到期恢复 | 优先于套餐，仍不能越过全局关闭 |

运营流程：创建套餐定义 → 配置详情 → 配置套餐功能 → 预览指定用户结果 → 上架。现有接口只能完成定义和上架；后面三个配置/预览能力要开发完成才能形成完整商品。

用户流程：登录 → 查询套餐与本人订阅 → 展示价格和权益 → 用户选择支付方式 → Gateway 收款/扣钱包并发放订阅 → 重新读取订阅、钱包、最终功能和插件目录 → 星途立即更新可见项。客户端不能凭支付页返回参数自行发放权益。

## 本章现有接口总表

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

## 用户端与管理端分工

用户登录后并行拉取 `/api/subscription/plans`、`/api/subscription/self`、`/api/user/self`，分别得到可购套餐、持有订阅、钱包余额。不要把套餐定义当成用户已经购买的订阅，也不要让客户端传入任意用户 ID 查询其他人的权益。

普通用户请求使用自己的 Access Token。下面的管理端请求由 Gateway 管理后台使用管理员身份发送。若以后接独立运营服务，管理员凭证只能保存在该服务端；星途安装包只持有用户凭证，不提供第二套可独立授权的运营后台。

## 查询套餐与订阅

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

### 套餐列表成功响应

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

### 用户订阅完整字段说明

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

## 用户购买与扣费来源

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

## 管理端操作

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

### 管理用户订阅

| 操作 | 接口与请求 | 结果 |
| --- | --- | --- |
| 查询 | `GET /api/subscription/admin/users/123/subscriptions` | `data[].subscription`，包括历史记录 |
| 新增 | `POST /api/subscription/admin/users/123/subscriptions`，`{"plan_id":7}` | 从套餐创建订阅，不收款；成功后回查列表 |
| 兼容绑定入口 | `POST /api/subscription/admin/bind`，`{"user_id":123,"plan_id":7}` | 与上面的新增操作二选一，不能都调用 |
| 立即失效 | `POST /api/subscription/admin/user_subscriptions/42/invalidate`，无请求体 | 该订阅停止生效，保留记录 |
| 删除记录 | `DELETE /api/subscription/admin/user_subscriptions/42` | 硬删除，后续历史列表也不再返回 |

上述请求全部携带管理员凭证。`123` 是用户 ID、`7` 是套餐 ID、`42` 是用户订阅 ID，不能互换。新增与付费购买共用套餐次数限制。失效不是退款，也不是支付渠道侧的自动续费取消；删除更不能替代退款。当前没有任意 `PUT /user_subscriptions/:id` 修改结束时间或额度的接口，星途不要构造不存在的“订阅编辑”请求。

管理端创建/修改/上下架套餐、发放订阅，以及用户购买，均要求 Gateway 已确认支付合规；星途不能通过换成管理员身份绕开该前提。手册仅说明这一接口条件，不要求客户端修改平台合规配置。

### 重置指定用户的指定套餐额度（现有）

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

### 重置一个套餐的全部有效订阅（现有）

```http
POST /api/subscription/admin/plans/7/subscriptions/reset
Authorization: Bearer <ADMIN_ACCESS_TOKEN_OR_PAT>
Content-Type: application/json

{"advance_reset_time":true}
```

路径 7 为套餐 ID；请求体必须是 JSON 对象。该接口不使用 body.plan_id。行为与上项相同，但跨该套餐全部持有人；没有匹配时成功且计数为 0。返回相同计数结构，user_count 是去重用户数。此接口有全体影响，Gateway 后台应明确显示目标套餐与操作范围。

重置、失效和删除控制器当前没有与套餐编辑相同的 requirePaymentCompliance 检查；鉴权仍要求管理员。两项重置会记录管理审计。它们都不是付款、退款、补发新订阅或变更插件授权的入口；没有客户端幂等键，超时先回查结果再决定是否操作。

## 套餐详情和插件权益如何配置（拟新增）

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

### 常见运营操作与两端结果

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

### 错误与支付确认边界

现有订阅接口的参数、业务与数据库失败主要通过 `success:false/message` 返回，不能假定全部为 400 或存在稳定 error_code。常见失败包括支付合规未确认、套餐不存在/未启用、购买次数超限、钱包不足、钱包购买关闭、组不存在。鉴权失败另由中间件决定 HTTP 状态；UI 同时处理网络、HTTP 和业务三层结果。

未来展示/授权接口采用第三章的稳定错误码：400 INVALID_ARGUMENT、401 AUTH_REQUIRED、403 FORBIDDEN、404 RESOURCE_NOT_FOUND、409 POLICY_REVISION_CONFLICT、503 ENTITLEMENTS_UNAVAILABLE。支付创建超时仍是“结果待确认”，不能按这些设计错误码推断已退款。

## 操作完成后的回查

购买、发放、失效或删除成功后，按[密钥与插件权益](/admin-manual?article=api-keys#权益变化后的同步)统一回查。到期由 Gateway 的维护任务处理，星途按返回状态显示，不自行延长订阅。

当前没有统一的用户订阅订单列表/详情接口；还缺客户端操作幂等键、订单与具体订阅的持久化关联，以及订阅/钱包权益通知。超时后先回查，不自动重复购买或发放；已有支付回调去重不等于客户端重发请求会去重。升级差价、下周期切换、取消续费和退款也没有通用接口，需要另定业务规则。

## 套餐对接任务与验收

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
