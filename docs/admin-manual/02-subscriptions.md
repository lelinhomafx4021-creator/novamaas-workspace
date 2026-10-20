# 套餐与订阅管理

管理目标：由聚合平台维护套餐及用户订阅，星途统一展示套餐目录、本人持有情况及对应使用状态。基础管理与订阅读取已具备，完整展示、历史关联资料和套餐功能权益仍需补齐。

## 一、业务规则

套餐决定需订阅功能的使用资格、额度和期限。插件目录、显示、排序及目标版本由全局配置决定，未订阅、已订阅和到期账号使用同一受管目录。

| 对象 | 管理口径 |
| --- | --- |
| 套餐定义 | 维护名称、说明、价格、货币、周期、额度、重置及排序；推荐、卖点和功能授予为待补资料 |
| 用户订阅 | 每次发放形成一份订阅来源，记录用户、套餐、状态、期限、额度及来源 |
| 功能权益 | 平台按本人全部有效订阅合并授予；一份失效后重新计算，保留其他有效来源 |
| 账务边界 | 钱包余额、订阅额度及模型密钥预算分别核算；额度耗尽不直接等同订阅到期 |

星途当前购买按钮禁用，首期采用管理发放与本人只读展示。自动购买、续费、退款及升降级另按明确业务方案实施。

## 二、管理员操作流程

1. 在聚合平台“订阅”中创建或编辑套餐，填写基础资料并核对启用状态、额度及重置周期。
2. 在“用户”中核对目标用户 ID，进入该用户的“管理订阅”，选择套餐并新增订阅，确认来源、记录数量、期限和额度。
3. 使用该用户登录星途，从账户菜单进入“订阅套餐”并刷新，对照套餐目录和本人记录。
4. 调整订阅前确认目标和影响范围；操作后分别检查平台记录、星途展示及相关功能使用状态。

| 管理动作 | 影响范围与核查要求 |
| --- | --- |
| 新增订阅 | 增加一份来源，原有订阅保留；管理发放与支付结果分别记录 |
| 用户页重置额度 | 处理该用户、该套餐的有效订阅，即使从某一条记录进入也须核对实际处理数量 |
| 套餐页重置额度 | 处理该套餐的全部有效订阅；重置前确认“推进下次重置时间”选项 |
| 到期或作废 | 作用于对应订阅；功能权益接通后重新计算全部有效来源 |
| 套餐下架 | 停止新目录展示，既有有效订阅及本人历史继续保留 |
| 删除、退款或升降级 | 明确目标记录、旧权益、期限和余额处理，分别核对业务与账务结果 |

额度重置保持原订阅有效期。平台支付合规设置会限制套餐操作；未确认时，用户套餐目录也可能为空，排查时先核对平台设置。

## 三、接口与展示要求

复用 GET /api/subscription/plans 和 GET /api/subscription/self，管理员操作沿用现有管理接口。新增资料与功能结果兼容扩展既有响应，客户端不自报套餐作为授权依据。

| 数据 | 平台交付要求 | 星途展示与处理 |
| --- | --- | --- |
| 套餐目录 | 套餐身份、名称、说明、价格、货币、周期、额度及排序 | 按实际单位展示，目录价格不当作用户已付款金额 |
| 推荐与卖点 | 可保存、读取和清空的关联展示资料 | 取代本地营销表作为正式来源，清空后同步移除 |
| 本人订阅 | 状态、起止时间、总额度、已用、剩余及下次重置 | 分开显示有效和历史；总额度为零显示“不限额”，无重置时间不渲染无效日期 |
| 历史套餐资料 | 提供本人订阅关联资料；需固定的历史信息使用发放时快照 | 下架后仍可识别名称；确实缺失时保留编号并提示 |
| 有效功能 | 稳定功能标识、授予来源、可用状态、期限及不可用原因 | 更新使用按钮及订阅提示，不隐藏、重排目录或分配不同版本 |

当前卡片主要展示名称、状态和日期，完整额度仍待补齐；历史下架套餐可能回退为“套餐 #ID”，需要补本人关联资料。本人订阅读取失败时目前会回退为空列表，扩展时必须区分“无订阅”与“读取失败”。

读取失败保留最后一次有效数据并标注未更新；换号清理旧账号结果并校验迟到响应。授权过期后按总览的使用规则处理，不能将故障展示为零余额或无限授权。

## 四、已有接口字段

### 4.1 套餐目录与管理保存

GET /api/subscription/plans 使用用户登录会话，无请求体；成功 `data` 为数组，每项为 `{"plan":{...}}`。后台 GET /api/subscription/admin/plans 使用管理员会话。POST 同一管理地址创建，PUT /api/subscription/admin/plans/:id 编辑，请求体均为 `{"plan":{...}}`；编辑并非局部 PATCH，提交完整编辑资料，避免省略的值被写成零或空串。

以下为 `plan` 的已有字段；读取时均按实际返回解析，必填列描述管理保存要求。

| 字段 | 类型 | 保存要求 | 含义、单位与规则 |
| --- | --- | --- | --- |
| `id` | integer | 服务端生成 | 套餐 ID；编辑以路径 ID 为准 |
| `title` | string | 必填 | 去空白后非空的套餐名 |
| `subtitle` | string | 可空 | 简要说明 |
| `price_amount` | number | 必填 | 金额 0–9999；读取时不视为本人已支付金额 |
| `currency` | string | 当前保存为 USD | 当前控制器统一保存 USD，不按任意货币计费 |
| `duration_unit` | string | 必填 | year / month / day / hour / custom；省略归为 month |
| `duration_value` | integer | 非 custom 时 >0 | 非 custom 下小于等于 0 会归为 1 |
| `custom_seconds` | integer | custom 时 >0 | 自定义有效期秒数；保存接口未完整验证全部期限组合，发放前仍须校验 |
| `enabled` | boolean | 明确提交 | 是否出现在新套餐目录中 |
| `sort_order` | integer | 明确提交 | 列表按本值降序，再按 id 降序 |
| `total_amount` | integer | >=0 | 原始 quota，0 为不限额；与用户订阅额度快照分别维护 |
| `quota_reset_period` | string | 必填 | never / daily / weekly / monthly / custom；未知值归 never |
| `quota_reset_custom_seconds` | integer | custom 时 >0 | 自定义额度重置间隔秒数 |
| `allow_balance_pay` | boolean | 新建默认 true | 是否允许钱包购买；编辑省略时保留原值 |
| `allow_wallet_overflow` | boolean | 新建默认 true | 新订阅额度耗尽后是否可回退钱包；编辑省略时保留原值 |
| `max_purchase_per_user` | integer | >=0 | 同套餐购买次数限制，0 不限 |
| `upgrade_group` / `downgrade_group` | string | 可空 | 已存在的平台用户分组；不是插件或功能 ID |
| `stripe_price_id` / `creem_product_id` / `waffo_pancake_product_id` | string | 可空 | 支付产品关联，本期客户端只读，不据此开放购买 |
| `created_at` / `updated_at` | integer | 服务端维护 | Unix 秒 |

### 4.2 本人订阅

GET /api/subscription/self 成功 `data` 包含 `billing_preference`、`subscriptions` 和 `all_subscriptions`。两个数组每项均包一层 `subscription`，历史数组也包含当前有效记录，不能直接拼接两个数组造成重复。

| 字段路径 | 类型 | 含义与规则 |
| --- | --- | --- |
| `billing_preference` | string | subscription_first / wallet_first / subscription_only / wallet_only；为计费偏好 |
| `subscriptions` | object[] | 当前 active 且未到结束时间的订阅，额度耗尽仍可能在其中 |
| `all_subscriptions` | object[] | 本人全部有效及历史记录 |
| `*.subscription.id` / `user_id` / `plan_id` | integer | 分别为订阅、持有人、套餐 ID；user_id 必须匹配当前账号 |
| `*.subscription.amount_total` / `amount_used` | integer | 周期总额度及已用原始 quota；有限额度剩余量为 max(0, total-used)，total=0 为不限额 |
| `*.subscription.start_time` / `end_time` | integer | Unix 秒；到期后不参与功能授予 |
| `*.subscription.status` | string | active / expired / cancelled，不与订单支付状态混用 |
| `*.subscription.source` | string | order / admin，说明发放来源 |
| `*.subscription.last_reset_time` / `next_reset_time` | integer | Unix 秒，0 表示无该次重置时间；以服务端日程为准 |
| `*.subscription.upgrade_group` / `prev_user_group` / `downgrade_group` | string | 发放时的分组及回退资料 |
| `*.subscription.allow_wallet_overflow` | boolean | 此份订阅的钱包回退快照 |
| `*.subscription.created_at` / `updated_at` | integer | Unix 秒，不等于付款确认时间 |

`*` 表示两个数组内的每一项。成功响应节选，示例历史数组中仍包含有效项：

```json
{
  "success": true,
  "message": "",
  "data": {
    "billing_preference": "subscription_first",
    "subscriptions": [{"subscription": {"id": 42, "user_id": 123, "plan_id": 7, "amount_total": 500000, "amount_used": 100000, "status": "active", "start_time": 1900000000, "end_time": 1902592000, "next_reset_time": 0}}],
    "all_subscriptions": [{"subscription": {"id": 42, "user_id": 123, "plan_id": 7, "amount_total": 500000, "amount_used": 100000, "status": "active", "start_time": 1900000000, "end_time": 1902592000, "next_reset_time": 0}}]
  }
}
```

当前控制器读库失败可能返回成功且空数组，尚不能可靠表达“无订阅”。新增权益必须独立可靠读取账本，不能沿用该回退结果计算授权。

### 4.3 管理订阅的请求与结果

下表接口均要求管理员身份，`:id` 按所在行分别指用户、套餐或订阅，不能互换。新增发放按套餐产生期限及额度，不接受客户端自填到期时间。

| 方法与路径 | 请求字段 | 成功 data 与注意事项 |
| --- | --- | --- |
| POST /api/subscription/admin/users/:id/subscriptions | `plan_id`: integer，必填且 >0 | null 或含提示的对象；回查用户列表取得实际订阅记录 |
| POST /api/subscription/admin/bind | `user_id`、`plan_id`: integer，必填且 >0 | 与管理发放相同，不代表支付成功 |
| GET /api/subscription/admin/users/:id/subscriptions | 无请求体 | 本人章节同形的 subscription 包装数组 |
| POST /api/subscription/admin/users/:id/subscriptions/reset | `plan_id`: integer，必填；`advance_reset_time`: boolean，可选默认 true | `plan_id`、`matched_count`、`reset_count`、`user_count`: integer；`advance_reset_time`: boolean |
| POST /api/subscription/admin/plans/:id/subscriptions/reset | `advance_reset_time`: boolean，可选默认 true | 同上，作用于全体持有人；路径为套餐 ID，不读 body.plan_id |
| POST /api/subscription/admin/user_subscriptions/:id/invalidate | 无请求体 | 操作后回查；只作废目标订阅 |
| DELETE /api/subscription/admin/user_subscriptions/:id | 无请求体 | 操作后回查；删除记录不等于退款 |
| PATCH /api/subscription/admin/plans/:id | `enabled`: boolean，必填 | 只调整套餐上架状态 |

重置中 false 仅清已用额度，true 同时推进下次重置时间；有效期不变。返回 matched_count=0 是成功但没有匹配，不应显示“已重置一条”。

## 五、拟新增展示与功能权益字段

沿用已有两条读取路由，拟通过 Query `include_starpath=1` 主动请求扩展；未传保持原协议。旧服务忽略该 Query 时仍可能返回基础数据，客户端须识别扩展缺失并提示尚未支持，不能默认授予。

| 位置与字段 | 类型 | 必填 / 规则 |
| --- | --- | --- |
| 套餐目录每项 `starpath.revision` | integer | 扩展响应必有，独立于套餐 ID |
| `starpath.presentation.badge` | string | 必有，0–32 字符，空串移除角标 |
| `starpath.presentation.recommended` | boolean | 必有，推荐展示，不影响价格及权益 |
| `starpath.presentation.highlights` | string[] | 必有，0–20 条，每条 1–200 字符；[] 清空 |
| `starpath.presentation.description_markdown` | string | 必有，可空，最多 20000 字符；渲染净化 |
| `starpath.presentation.terms_markdown` | string | 必有，可空，最多 10000 字符；不能代替支付规则 |
| `starpath.feature_ids` | string[] | 必有，稳定功能 ID；[] 表示该套餐不授予功能，不改变免费能力 |
| 管理保存 `starpath.expected_revision` | integer | 提交扩展时必填，首次 0；冲突 409；服务端不接受客户端写 revision |
| 本人 `data.plan_summaries[]` | object[] | 扩展响应必有，仅本人已持有套餐；每项 plan_id:integer、title:string、subtitle:string，补历史名称 |
| 本人 `data.feature_entitlements.version` | string | 扩展响应必有，服务端生成，不透明版本标识 |
| `data.feature_entitlements.expires_at` | integer | 扩展响应必有，Unix 秒，按有效来源下一次变化时间与授权快照期限取较早值 |
| `data.feature_entitlements.items[].feature_id` | string | 必有，稳定功能 ID，与全局插件使用条件关联 |
| `items[].enabled` | boolean | 必有，本人是否具有该功能资格；仍须满足全局开关和运行条件 |
| `items[].reason` | string | 必有，allowed / subscription_required / subscription_expired / globally_disabled |
| `items[].expires_at` | integer 或 null | 必有，全部有效来源合并后的资格期限；免费或当前无有效授予时为 null，是否有资格仍以 enabled 为准 |
| `items[].source_subscription_ids` | integer[] | 必有，全部有效授予来源；免费功能可为空 |

`items[]` 均位于 `data.feature_entitlements` 下。关闭功能不等于套餐到期，账本读取失败返回 503 ENTITLEMENTS_UNAVAILABLE，不合成“没有订阅”。未知功能默认不授予。展示与功能关联拟随既有管理员套餐保存请求扩展，基础套餐与扩展资料须在同次事务中保存；原控制器尚未支持这些字段。

拟定保存扩展节选如下，提交时 `plan` 仍须带完整计费定义：

```json
{
  "plan": {"id": 7, "title": "示例套餐"},
  "starpath": {
    "expected_revision": 0,
    "presentation": {"badge": "", "recommended": false, "highlights": [], "description_markdown": "", "terms_markdown": ""},
    "feature_ids": ["plugin:example.tool"]
  }
}
```

示例功能仅用于说明关联，不代表已有收费插件。`plan_summaries` 名称来自本人订阅关联或发放快照，不开放任意下架套餐详情；首次补齐历史时不能伪造过去的价格和说明。多份订阅合并权益后，快照期限仍须在任一来源变化时重新计算。

## 六、开发与验收

本章对应 DP-02 和 DP-04。平台开发交付展示资料、历史关联及服务端权益计算；星途开发完成解析、卡片、刷新和功能使用状态；产品运营维护展示内容与套餐授予关系。

验收覆盖套餐创建与编辑、展示资料清空、订阅发放、多份订阅、额度重置、到期、作废、下架历史、读取失败及换号。使用未订阅与有效订阅两个账号证明：目录和版本相同，相关使用资格及本人额度、期限不同；到期后保留目录与成果。

每项分别记录平台保存、本人接口和星途实际结果。跨章节交付及证据要求见[管理总览](/admin-manual?article=account-login)。
