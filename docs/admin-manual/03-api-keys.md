# 密钥与插件权益

登录后用当前用户的会话管理其模型密钥，再把可用模型和套餐权益同步到星途。模型 Key、钱包额度、订阅权益和插件开关各有职责，统一在本章完成配置与回查。

版本：2.0 收口稿，2026-10-19。现有密钥接口、待开发的权限/插件/统一发布契约，以及两端实施规划分节说明；规划和接口样例不代表业务功能已经上线。

Gateway 负责功能目录、插件商品资料、套餐授予、个人例外、版本包和发布策略；星途负责读取、显示、安装和执行。星途本地配置只能进一步限制，不能成为另一份能扩大服务端授权的规则源。本章 `/api/token/...` 等为现有能力，全部 `/api/apps/starpath/...` 为拟新增、尚未实现。

## 密钥和模型目录接口总表（现有）

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

## 获取星途使用的用户密钥

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

## 密钥配置与修改

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

## 模型与插件可见性

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

## 必须补齐的用户功能与插件权限

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

### 本人权限与专用模型 Key 的完整契约

`GET /api/apps/starpath/entitlements` 无请求体、无 user_id 参数，仅接受当前用户登录身份。data 的 schema_version 为整数，目前拟定 1；application_id 固定 starpath；user_id 为认证用户 ID；effective_version 是比较变化的不透明字符串；server_time/valid_until 是 Unix 秒；refresh_after_seconds 为建议回查秒数；features 是可见功能数组，每项 id/state 必填、reason_code 可选。有效期最长 60 秒，不能晚于最近的订阅/覆盖到期；空数组不等于全部开放。与服务端目录不一致或未知 ID 都拒绝默认开放。

`POST /api/apps/starpath/model-key` 使用用户 Access Token，body 为 `{}`，不允许指定 user_id/application_id/group/权限。按当前用户和 starpath 原子取得或初始化唯一当前绑定，重复和并发请求返回同一 token_id，不需要靠密钥名查重：

```json
{"success":true,"message":"","data":{"application_id":"starpath","token_id":42,"created":false,"status":1}}
```

首次创建 created=true；有效绑定已存在则 false。status 沿用现有 Key 的 1/2/3/4，停用/过期/耗尽照实返回，不自动新建绕过；绑定的 Key 被删除时返回 409 APP_KEY_REBIND_REQUIRED，需用户显式确认新绑定，不能默默把另一个普通 Key 认作专用 Key。首期不新增自动轮换接口，主动轮换需另定明确流程。

此接口不返回明文；有 token_id 后由现有 `POST /api/token/42/key` 取明文。应用绑定必须由服务端保护，普通 Key 编辑不得取消；专用 Key 在星途外调用也受应用功能约束。接口幂等不等于所有现有 token 创建请求都幂等，后者仍按本章现有流程处理。

### 可信应用身份与插件来源的边界

专用 Key 只能证明服务端保存的应用归属，不能证明一次通用模型请求实际来自哪个插件。Gateway 应通过认证身份、Key 绑定、路由及已验证的模型/业务动作判断可检查的功能；不得信任客户端自报 plugin_id、X-App-ID、feature_id 或安装版本作为授权依据。

例如两个插件都请求 `/v1/chat/completions`，仅看 Key 或请求头无法区分调用来源。通用文本路由只能落实它可确定的 text.chat 等能力；如果某插件要求服务端强制限制其专属云业务，需先建设可验证的独立业务服务边界。没有这条边界时，该插件只能验收受支持客户端的本机执行限制，不能宣称已做到防修改客户端的远程强制许可。

图片/视频等云能力同样要覆盖别名协议、模型映射及素材入口，在鉴权和参数验证之后、预扣费与上游副作用之前检查。历史查询、取消和成果回收继续按资源归属放行；关闭“新生成”不应阻止用户取回已有成果。纯本地工具、办公技能及 MCP 的授权仍按各自既有规则，本次不重写其核心执行链。

### 配置生效回报与管理端观察（拟新增）

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

## Gateway 后台具体控制哪些内容（拟新增）

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

### 功能目录字段和读取

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

### 套餐授权的读取、替换和生效

`GET /api/apps/starpath/admin/plans/7/features`：管理员；套餐必须存在，下架套餐允许管理。

```json
{"success":true,"message":"","data":{"plan_id":7,"revision":2,"feature_ids":["image.generate","plugins.manage","plugin:example.storyboard"]}}
```

`PUT` 同地址，必填 expected_revision（>=0）、feature_ids（数组，最多 500 个，去重且只能包含已登记 ID）、reason（1–500 字符）。空数组表示清空此套餐全部授予，不能表示“不修改”。保存返回 `{plan_id,revision}`；订阅账本不变，功能计算立即使用新规则，相关缓存失效。详细请求见第二章。

功能 available=false 时可以保留套餐关联，服务端仍不给用户授予；后台应显示全局关闭原因。解除全局关闭后恢复按原关联计算。

### 用户单独覆盖和恢复继承

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

### 三个业务例子

| 用户情况 | 计算过程 | 结果 |
| --- | --- | --- |
| 无套餐，无个人覆盖 | X 默认 disabled，全局可用 | 可见购买入口，不能运行 |
| 有套餐 7，又有个人 hidden | 套餐先授予，再被个人覆盖 | 各入口隐藏，不能通过 IPC 或计划任务运行 |
| 套餐 7 到期，套餐 8 仍授予 X | 只合并当前有效订阅，套餐 8 仍命中 | X 继续使用，不因一份到期全部关闭 |

全局关闭会使以上全部变为 hidden。订阅有效判断必须包含 start_time <= now < end_time；额度耗尽不改变授权，只影响实际计费是否通过。

## 真实插件如何接入：源码调研与落地细则

本节按星途 2026.9.23、固定 OpenClaw v2026.8.1 与当前 Gateway 路由核对，包含已确认行为及明确标注的待开发接缝。这里只补文档，未改星途/业务接口、未安装插件或进行真实联调。Gateway 管规则和发布决策，星途沿现有接口适配。

### 真实插件清单与接入分类

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

### 现有链路：从按钮到运行时

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

### Gateway 与星途的新增接缝

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

### 插件作者具体交付什么

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

### 现有媒体插件怎么接 Gateway

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

### 消息渠道、模型适配、技能和 MCP 的边界

**消息渠道**

微信/钉钉/飞书/企业微信的连接、扫码和平台凭据继续走星途现有 IM 控制器与 channels 配置，Gateway 不代理它们全部消息流，也不将用户的微信/邮箱凭据当成模型 Key。

将来需要按套餐限制某个渠道，可登记独立功能（例如 im.weixin，当前尚未登记/实现），在设置入口、配置保存、建立连接和入站业务触发处接同一授权。仅隐藏导航不会断开已有连接；停用要安全停止接收新任务并保留凭据，不能卸载随包渠道。

远端 IM 发件人 ID 不是 Gateway user_id。若多个远端成员共用一个客户端，必须明确调用归属是当前安装的登录账号还是经过绑定的成员账号；没有映射时不能自动让任意发件人消耗当前用户钱包。该业务规则尚未确认，不能将“一个插件连接成功”验收为多用户计费接通。

**模型 provider**

平台配置当前锁到 Gateway /v1 的公司 provider，Key 和模型仍需正确关联。随包 provider 的注册/兼容代码保留，不因模型未对某用户开放就卸载 provider。云端能力以模型 Key、用户组、模型白名单和 Gateway 实际路由为准，客户端菜单不能扩大范围。

**技能与 MCP**

技能授权若以后纳入范围，应在技能发现/启用及其真正调用工具的边界设计；SKILL.md 本身不是可执行软件包的授权证据。MCP 当前经原生 mcp.servers 配置和运行管理，逐工具权限不能只改旧 mcp-bridge manifest。两者首期只保留现状和接缝，不因插件方案改写加载与工具协议。

### 发布包、升级和回滚如何落实

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

### 执行校验覆盖范围

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

## 插件列表、详情与显示控制（拟新增）

当前星途 `plugins:list/install/uninstall/set-enabled/save-config/check-updates/update` 是本地 IPC；安装器已支持 npm/clawhub/git/local，当前 update 对 npm/clawhub 重新安装未固定版本的源。它们还没有消费 Gateway 最终授权或发布策略。`plugins:*` 不是可从网络调用的 Gateway HTTP API。

星途的主应用另有 `appUpdateGrayClient.ts`，调用 `/api/client-updates/check` 并期待旧 `code:0` 信封；当前 Gateway 路由没有这个地址。该实现也属于主应用更新，不能当作已经具备 Gateway 插件发布能力。本章拟定插件 API 统一使用 success/data，接入时需要单独适配。

### 插件目录管理接口

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

### 用户列表和详情响应

```json
{"success":true,"message":"","data":{"page":1,"page_size":20,"total":1,"catalog_revision":12,"effective_version":"policy-17","items":[{"plugin_id":"example.storyboard","runtime_id":"example.storyboard","feature_id":"plugin:example.storyboard","name":"示例分镜插件","summary":"根据提示词整理分镜","icon_url":"https://cdn.example.com/plugins/storyboard.png","category":"创作","sort_order":10,"state":"disabled","reason_code":"PLAN_REQUIRED","purchase_plan_ids":[7],"min_client_version":"1.2.0"}]}}
```

详情 data 为同一项加 `description_markdown` 和 `revision`。disabled 可返回允许公开的介绍和可购套餐 ID，不能返回包下载授权；hidden 不得从列表、详情、搜索或更新检查泄露元数据。purchase_plan_ids 只包含当前可购、确实授予该功能的套餐，空数组不编造“购买后可用”。

列表没有安装状态：安装版本、本地启停和配置由星途本机提供，按 plugin_id 合并。用户本地 disabled 不会改写 Gateway 套餐规则；Gateway enabled 也不会强行把用户手动关闭的插件打开。元数据刷新以 catalog_revision 判断变化，授权有效期仍由本人 entitlements 决定，不能用目录缓存延长授权。

### 星途每个入口的执行约束

| 位置 | 读取与处理 |
| --- | --- |
| 设置/侧栏/商店/搜索/快捷键 | 同一份权限和目录快照；hidden 无入口，disabled 显示原因 |
| 插件安装/升级 | plugins.manage=enabled 且插件功能 enabled，目录可安装、版本可用；运维强制更新可在不开放管理页的情况下单独提示获授权用户 |
| 插件配置/启用 | 主进程 IPC 校验，不能只由 renderer 禁用按钮 |
| 插件加载/工具调用/计划任务 | 根据可信 runtime_id 再校验插件功能和版本门槛；旧缓存不得绕过 |
| 模型/云服务调用 | Gateway 在计费前按真实身份/专用 Key、模型与功能边界检查 |
| 历史和已受理任务 | 按所有权继续查询、取消、下载；不因新执行被禁而删除成果 |

纯本地插件执行由受支持星途版本落实；任意修改客户端无法获得同等服务端强制保证。卸载、关闭本地插件和本地配置清理应保留安全收尾通道；失权不应让用户连停用或备份既有数据都无法完成。

## 插件版本、统一发布、强制更新和回滚（拟新增）

用户已明确采用全权统一控制：每个受控插件只有一份当前版本策略，所有获授权用户执行同一目标版本。Gateway 指定版本、升级时限、暂停、撤销与回滚；星途执行，没有用户自主选版入口。套餐和个人覆盖继续决定能否使用插件，不参与版本分群。

不同操作系统/架构可以使用不同安装文件，但这些文件必须属于同一个插件目标版本。客户端过旧或平台不支持时明确返回兼容问题，不能自动给这类用户下发另一条旧版本策略。本节全部接口仍为拟定，当前未实现。

### 发布对象与状态

| 对象 | ID / 版本 | 生命周期 |
| --- | --- | --- |
| 插件元数据 | plugin_id / revision | draft → listed → unlisted，可重新上架 |
| 发布包 | release_id / version | draft → published → withdrawn；published 内容不可变 |
| 当前统一策略 | plugin_id / revision | 每插件唯一；active / paused，修改事务提交即更新当前规则 |
| 本人下载/安装决策 | decision_id | 按当前账号权限、统一策略和兼容性签发，短时有效 |
| 本机安装操作 | operation_id | downloaded → installing → succeeded / failed；失败不代表已回滚成功 |

版本为三段数字 major.minor.patch。当前目标以后台指定的精确版本为准，已安装更高版本也不自动视为合规；降低版本必须有后台明确 rollback 指令和兼容检查。发布包只登记文件；设置统一策略才决定全体用户接下来应使用哪个版本。

### 发布管理接口总表

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

### 登记、发布和撤销版本

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

### 指定全体统一版本与升级时限

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

### 星途检查更新：请求与完整响应

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

### 下载和安装前重新授权

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

### 安装结果上报

```http
POST /api/apps/starpath/plugin-updates/report
Authorization: Bearer <USER_ACCESS_TOKEN>
Idempotency-Key: <REPORT_KEY>
Content-Type: application/json

{"operation_id":"<OPERATION_ID>","plugin_id":"example.storyboard","release_id":"rel-example-140-mac-arm64","phase":"succeeded","installed_version":"1.4.0","error_code":null}
```

phase 为 downloaded/installing/succeeded/failed；failed 必须带稳定 error_code，不上传环境变量、密钥或完整本地路径。operation_id 只能由归属用户上报；按允许状态推进，不因乱序覆盖终态；同内容重复返回原结果，冲突终态返回 409 INSTALL_REPORT_CONFLICT。成功返回 `{accepted:true}`。结果上报只用于 Gateway 后台统计，不能凭“我装好了”获得额外功能，也不能当作远程证明客户端未被修改。

### 全体统一回滚与故障恢复

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

### 刷新、断网、换号和错误契约

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

## 实施规划与交付门槛

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

### 后台配置如何落到客户端

1. Gateway 管理端先 GET 资源及 revision，提交完整对象、expected_revision 和 reason；冲突后重读，不覆盖别人已保存的规则。
2. 服务端在同一资源事务中校验、写入与递增版本，记录管理审计，并让所有节点的相关权限/决策失效。多个资源的多个请求不伪装成跨接口原子成功。
3. 星途主进程按当前 origin/user.id/登录代次读取本人快照，统一驱动界面与执行入口，再发 config-reports。正常网络下目标 60 秒内生效，实际联调测量；云侧鉴权无需等待本机刷新，回报不作为授权凭证。
4. 安装与执行另查当前统一策略、包兼容与本机 desired_enabled；不把“有权益”变成强制开启用户手动关闭的插件。
5. 旧 Key 和 user_plugins 不按名称或当前登录账号批量认领。先保留原记录、核验归属，再做可回退关联；包可共享，配置、凭据和授权不可跨账号共享。

### 必测场景与交接证据

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

## 两个平台逐项验收（待实施）

### 一个插件从建档到用户可用的完整对接

1. Gateway root 建档插件（draft），对应功能初始 hidden；登记固定版本包、核验并 publish。
2. Gateway root 预览平台/客户端兼容，保存唯一 release-policy，统一指定目标版本和期限；这一步只决定版本，不授予使用权限。
3. Gateway 配置套餐详情和 feature_ids，包含该插件及需要的 plugins.manage；按需要设置功能 available=true、default_state=disabled，再将插件 listed。普通用户仍只看可购买说明。
4. Gateway 已存在用户有效订阅（由已授权的购买或管理员发放建立），星途回查本人订阅和 entitlements，确认该插件变为 enabled。星途当前仍为只读套餐；未来启用购买须另过支付闭环验收。
5. 星途读取本人 plugins 详情，check 得到获准 release；download 授权 → 下载核验 → install 再授权 → 原子安装/健康检查 → report。
6. 用户执行时，主进程按 runtime_id 检查功能、版本和本地启停；调用云模型时 Gateway 再校验模型 Key、功能与额度，然后预扣费/转发。
7. 后续改显示资料只刷新目录；改套餐授权刷新权限；发新版本走发布/统一策略；撤权和强制更新分别阻断相应的新执行，历史成果保留。

这条链路明确 Gateway 为主控端，星途没有第二套可以自行授予套餐、开放插件或越过版本策略的规则。

### 验收矩阵

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

## 权益变化后的同步

登录、换号、购买/发放完成、订阅失效及页面恢复时，统一执行：

1. 读取 `/api/user/self` 与 `/api/subscription/self`，确认用户、钱包、订阅和分组。
2. 读取 `/api/user/models`、`/api/user/self/groups`，刷新模型选择。
3. 读取选定的 `/api/token/42`，确认启停、过期、组和模型限制。
4. 获取当前账号选定 Key 后读取 `/v1/models`，更新星途实际调用配置；明文仅按调用需要获取和保存。
5. 上述拟新增权限接口实现后，再刷新最终功能快照；当前没有该接口，不能把本地开关或模型列表当成它的等价替代。

订阅变更可能改变用户组，但不会自动改写密钥。group 为空通常继续跟随用户；固定高档组的密钥在降级后可能被拒绝；模型白名单和密钥预算也不会因续订自动扩大。不要把订阅剩余量写进 remain_quota。

当前没有专用的订阅/钱包/插件变更推送，先回查同步。媒体 Webhook 不承担权益通知。星途现有 Gateway 登录与订阅读取已接入，但模型 Key/目录自动关联和旧媒体协议转换仍需客户端完成。

源码依据：`controller/{token,user,model,group,pricing}.go`、`middleware/auth.go`、`model/subscription.go`、`model/token.go`；星途 `maasAuth.ts`、`platformConfig.ts`、`enterpriseConfigSync.ts`、`Settings.tsx`、插件 IPC 和 `openclawConfigSync.ts`、企业账号菜单及 `product.json`。现有能力来自源码核对；新增权限契约仅为方案，自动配置与权益同步尚未完成两平台联调。
