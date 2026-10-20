# 插件管理与统一版本

管理目标：在聚合平台统一维护受管插件目录、显示、排序、全局启停和目标发布；星途执行本机展示、安装与运行，并回报实际状态。订阅只影响需订阅功能的使用资格、额度和期限。

现状：本机插件管理及安装流程已有，全局目录、发布记录、安装计划和设备回报尚未接通。本章为拟定开发与管理约定。

## 一、管理原则与生效范围

| 管理事项 | 本期规则 |
| --- | --- |
| 目录与版本 | 相同环境和兼容条件下，所有账号使用同一受管目录、排序和唯一目标发布 |
| 订阅差异 | 未订阅提示订阅，有效订阅按授予可用，到期提示续订；目录和版本保持一致 |
| 显示与启用 | 隐藏改变入口展示，全局停用限制新操作；两个状态分别维护 |
| 本机事实 | 是否安装、实际版本、私有配置和本机开关按设备记录，与全局策略合并展示 |
| 纳管边界 | 公司受管插件应用平台规则，未纳管用户自装插件沿用原机制，基础扩展保留原行为 |
| 生效确认 | 支持协议的客户端联网刷新后应用；离线、未适配旧版及无近期回报的设备标记未确认 |

使用顺序为：**全局启用 → 免费规则或有效套餐授予 → 版本要求 → 本机启用及必要调用条件。** 全局关闭优先，多份有效订阅合并授予，失去全部来源后只调整相关使用资格。云端新操作在真实服务边界校验，模型调用另核对密钥和额度。

## 二、现有插件清单与纳管方式

源码核查范围包括安装声明、五个本地扩展、实际缓存、插件页面和配置同步。当前“插件”页只展示用户安装记录，并过滤随包及内部扩展；连接器通过各自连接配置管理，应保持两类边界。

### 2.1 现有连接器

| 名称 | 包声明标识 → 实际运行标识 | 源码及本地缓存版本 |
| --- | --- | --- |
| 钉钉 | dingtalk-connector → dingtalk-connector | 0.8.26 |
| 飞书 | openclaw-lark → openclaw-lark | 2026.7.16 |
| 企业微信 | wecom-openclaw-plugin → wecom-openclaw-plugin | 2026.5.25 |
| 微信 | openclaw-weixin → openclaw-weixin | 2.4.3 |
| 邮箱 | clawemail-email → email | 0.9.16 |

前四项在“远程连接”中配置，邮箱有独立设置。以上为 2026-10-19 的源码与本地缓存快照，尚未作为公司统一目标发布，也不代表收发链路已验收。邮箱包名与运行标识不同，必须登记映射。

### 2.2 分类管理

| 类别 | 当前对象或能力 | 首期处理 |
| --- | --- | --- |
| 用户安装插件 | npm、ClawHub、Git及本地安装，已有启停、配置和更新流程 | 作为通用安装接入对象，明确哪些纳入公司管理 |
| 随包连接器 | 上述通信及邮箱连接器 | 纳入身份、版本及状态盘点，沿用客户端受控发行；独立升级另验兼容与恢复 |
| 基础扩展 | ask-user-question、mcp-bridge、lobsterai-model-compat及运行时浏览器等 | 保留原过滤和执行规则，不因公司目录为空而关闭，不作为默认收费对象 |
| 媒体扩展 | lobster-media-generation及现有工具、回调 | 保留工具契约，按图片和视频章适配 |
| 实验扩展 | lobster-decision | 保持现有条件，不为联调自动启用或列为正式商品 |

安装声明还包含模型供应商和历史渠道适配项，不能直接当作商品目录。SDK、技能及 MCP 服务与独立插件分别管理。当前未确认公司收费插件清单；可先用已有飞书入口验证展示，用钉钉作身份和只读版本对照，用合格独立测试包验证安装流程。

## 三、职责与后台管理

| 责任方 | 维护内容 | 交付结果 |
| --- | --- | --- |
| 产品运营 | 功能说明、免费或需订阅规则、各套餐授予关系 | 稳定功能标识与明确使用规则 |
| 平台 Root | 目录、位置、排序、开关、目标发布、期限、暂停和撤销 | 可追踪的全局配置版本及管理记录 |
| 插件维护者 | 开发、打包、测试、兼容及恢复说明 | 精确版本、固定文件、摘要和运行检查要求 |
| 包源或公司文件存储 | 保存并提供登记文件，私有文件提供短期下载条件 | 可下载且与登记摘要一致的文件 |
| 星途客户端 | 应用规则、下载校验、调用安装器、运行检查及回报 | 真实设备安装与运行结果 |

平台业务服务不需要下载并安装所有插件。可登记稳定的精确 npm 版本或固定文件；需要留存副本时，由发布流程将验收包上传公司文件存储。后台新增目录不会自动抓取、执行来源的最新代码。

兼容的独立插件可使用通用卡片与安装器。需要新增专用页面、宿主能力或改变随包结构的功能，须先完成客户端开发和发行。

## 四、发布与安装流程

1. **登记目录。** 确认稳定插件标识、真实包名、运行标识、管理类别和展示位置，兼容验证完成后开放。
2. **准备固定包。** 核对精确版本、文件摘要、系统/架构、宿主兼容、配置保留、运行检查和恢复条件。
3. **登记发布。** Root 提交固定文件及测试资料，取得发布标识；已发布身份、版本和文件资料不可覆盖，修复须登记新发布。
4. **指定统一目标。** 在全局配置引用已登记、未撤销且兼容资料齐全的发布，核对开关与期限后整体发布。仅登记新包不会改变当前目标。
5. **申请并执行安装。** 客户端发现差异后提示，首期由用户确认并避开运行任务。下载前取得安装计划，校验文件、身份及版本，切换前在线复核，再交给现有安装器。
6. **检查、恢复与回报。** 分别核对实际注册和运行结果；成功后清理恢复副本，失败时安全恢复兼容旧版或明确阻断。平台依据实际回报跟进设备。

目标按发布标识和精确版本核对，既能升级也能明确回滚。来源出现更高版本不会自动改变目标，失败后不得退回来源最新版本。超过统一升级期限时，对不满足必要版本的受管功能停止新操作并提示更新，不在任务执行中强行替换文件。静默安装另行规划。

### 4.1 现有安装器的改造要求

| 来源或能力 | 已有依据 | 必须补齐 |
| --- | --- | --- |
| npm 独立插件 | 安装器已有 version 参数；当前更新入口未传入版本 | 改传平台精确版本及固定 registry，增加下载文件摘要、实际身份和版本核验 |
| 固定压缩包 | 安装器可接本地来源 | 下载校验后交给安装器，保留受管来源及身份映射，后续仍跟随平台目标 |
| Git、ClawHub等 | 当前参数不能一概保证锁定到同一文件 | 首期转为可核验的固定包；无法固定的来源明确提示不支持受管安装 |
| 目录原子切换 | 切换失败可恢复旧目录，成功后立即清理旧备份 | 延后清理，补运行检查失败时的恢复资料，保护用户配置及本机禁用选择 |

本机原为禁用的插件安装后仍禁用，回报“已安装、未启用”，运行检查延后到实际启用后，恢复资料保留须覆盖该状态。不可逆数据迁移阻止自动回退并说明处理方式。

客户端改造接点为现有 plugins:update、插件来源处理及目录发布器；通过公司适配层连接平台目标，保留现有对话、工具和基础扩展契约。

### 4.2 下架、停用、撤销和卸载

| 动作 | 实际影响 |
| --- | --- |
| 隐藏入口 | 所有受管账号统一隐藏，对应运行状态另按启用规则处理 |
| 目录下架 | 停止新发现和新安装，保留已有包、配置和成果 |
| 全局停用 | 所有受管账号停止相应新操作，订阅不能重新开放 |
| 暂停安装 | 拒绝新的安装和切换计划，保留当前目标及期限 |
| 撤销发布 | 拒绝该发布的新受管下载计划及安装，已下载包切换前仍须复核；已安装设备另按停用或回滚处理 |
| 明确回滚 | 统一指定已验证安全目标，核对兼容和恢复条件后执行并回报 |
| 本机卸载 | 沿用现有用户操作；平台隐藏、下架或订阅到期不自动删除配置及成果 |

撤销作用于公司受管链路，外部公开文件可能仍存在，不能据此标记外部副本已删除或全部设备已卸载。

## 五、接口与最小数据约定

接口路径及权限统一见[管理总览](/admin-manual?article=account-login)。全局配置发布携带原配置版本，冲突时重新读取，只对客户端暴露完整发布结果。本人权益复用 GET /api/subscription/self，不据此隐藏、重排插件或改变目标。

| 数据对象 | 必须提供的内容 | 接收要求 |
| --- | --- | --- |
| 全局目录 | 稳定插件标识、运行映射、管理类别、上下架、位置、显示/启用、排序和配置版本 | 按身份合并本机记录，保留内部过滤 |
| 固定发布 | 发布标识、插件及精确版本、来源定位、系统/架构文件、大小与 SHA-256、客户端及宿主兼容、配置与运行检查要求 | 文件资料不可覆盖，不接受 latest、浮动范围或未固定分支 |
| 安装请求 | 设备、客户端/宿主版本、系统/架构、本机版本、所见配置与目标、操作标识及申请/切换复核阶段 | 服务端绑定登录账号，核对目标、上下架、暂停/撤销和兼容，订阅不改变安装包与版本 |
| 安装计划 | 操作标识、配置版本、目标发布、精确版本、匹配文件及摘要、有效期和恢复要求 | 下载前取得，切换前在线复核；绑定账号、设备、插件、发布和文件，不得改作其他安装 |
| 设备回报 | 设备、操作、配置版本、目标及实际版本、应用/安装/运行检查阶段、结果、时间、脱敏错误和恢复结果 | 同一操作按阶段幂等记录，未回报标记未知，安装成功不代替运行检查成功 |

安装计划使用登录会话，不使用模型密钥。全局配置不得包含私有凭据和下载凭证，私有文件短期地址仅由计划返回。错误按现有响应封装提供稳定原因，至少区分目标变化、发布撤销、安装暂停、不兼容、计划过期、身份失效和文件校验失败。

全局配置按平台环境和兼容条件缓存，本人权益按账号隔离。换号清除旧账号数据及待执行计划，重新取得当前身份下的同一全局目标；离线不能使用旧计划切换。读取失败保留最后有效配置并提示，不能报告策略已应用或安装成功。

### 5.1 全局配置字段（拟新增）

GET /api/apps/starpath/config 的 Query 提交 `client_version`、`host_version`、`os`、`arch`，均为必填 string；os 使用 darwin / win32 / linux，arch 使用 arm64 / x64，其他组合待适配后明确增加。兼容条件不能从账号或 User-Agent 猜测；同环境、同兼容条件的结果保持一致。成功返回 `data` 如下，表内字段均须返回：

| 字段 | 类型 | 含义与约束 |
| --- | --- | --- |
| `schema_version` | integer | 首期 1；不支持的版本阻止应用 |
| `config_revision` | integer | 服务端递增发布版本；旧版本响应不得覆盖新版本 |
| `published_at` | integer | Unix 秒，配置实际发布时间 |
| `refresh_interval_seconds` | integer | 建议刷新间隔秒数，首期拟 60，允许联调调整 |
| `plugins` | object[] | 完整受管目录，空数组仅代表无受管项，基础扩展仍按原规则运行 |
| `plugins[].plugin_id` | string | 平台稳定标识，登记后不随改名变化 |
| `plugins[].runtime_id` | string | 对应 manifest 实际运行标识，邮箱等允许与包名不同 |
| `plugins[].package_name` | string | 真实包名，不使用显示名称或目录名代替 |
| `plugins[].name` / `description` | string | 显示名称及纯文本说明 |
| `plugins[].management_type` | string | independent / bundled；本期只对合格 independent 包提供独立安装 |
| `plugins[].placement` | string | plugins / remote_connections / email_settings；只映射已支持入口，不是任意路由或执行地址 |
| `plugins[].listed` | boolean | 是否上架；false 不允许新发现、新安装 |
| `plugins[].visible` | boolean | 是否显示入口；不改变订阅或本机安装事实 |
| `plugins[].enabled` | boolean | 是否允许新的受管操作；全局关闭优先 |
| `plugins[].sort_order` | integer | 展示降序，值相同按 plugin_id 升序保持稳定 |
| `plugins[].required_feature_ids` | string[] | 使用需同时满足的稳定功能 ID；[] 为无需额外订阅授予，仍受全局开关约束 |
| `plugins[].target_release_id` | string 或 null | 统一目标发布；尚未纳入独立安装的随包项可为 null |
| `plugins[].target_version` | string 或 null | 服务端从目标发布得出，不接受客户端自报；无目标时 null |
| `plugins[].install_paused` | boolean | 是否暂停新的下载和切换计划 |
| `plugins[].upgrade_deadline` | integer 或 null | Unix 秒，目标必须达到的期限；无强制期限时 null |

已有安装可能仍需响应下架或隐藏，完整快照保留对应记录；`visible` 是界面规则，不承诺元数据保密。物理移除目录记录另做迁移，客户端不能把“本次未看到”解释为卸载命令。配置只含非敏感元数据，不带包下载凭证或私有配置。

GET /api/apps/starpath/admin/config 返回完整未筛减的管理快照。PUT 同地址提交 `expected_revision:integer`、`reason:string`、`plugins:object[]`，三项必填；首次版本为 0。plugins 是完整替换，子项使用上表字段，但 `target_version` 由服务端派生，不由管理员提交。reason 为 1–500 字符；所有项验证通过后原子发布，否则不改变原配置。清空须明确提交 []，不存在的目标、撤销包及未知功能标识均拒绝保存。

发布请求节选，示例沿用已有随包飞书，暂不指派独立包：

```json
{
  "expected_revision": 3,
  "reason": "恢复飞书入口",
  "plugins": [{
    "plugin_id": "openclaw-lark", "runtime_id": "openclaw-lark", "package_name": "openclaw-lark",
    "name": "飞书", "description": "飞书连接配置", "management_type": "bundled", "placement": "remote_connections",
    "listed": true, "visible": true, "enabled": true, "sort_order": 10, "required_feature_ids": [],
    "target_release_id": null, "install_paused": false, "upgrade_deadline": null
  }]
}
```

该例假设目录仅有这一项；实际发布须带完整目录。成功 data 返回已发布的完整配置和新 config_revision。引用旧 revision 返回 409 POLICY_REVISION_CONFLICT，不自动重试覆盖。

### 5.2 固定发布与文件字段（拟新增）

POST /api/apps/starpath/admin/plugin-releases 登记请求如下；表中“必填”均指提交字段。`release_id`、`artifacts[].artifact_id`、`created_at` 和 `revoked_at` 由服务端返回，不能由客户端覆盖。

| 请求字段 | 类型 | 必填 | 含义与约束 |
| --- | --- | --- | --- |
| `plugin_id` | string | 是 | 已登记的 independent 插件 |
| `runtime_id` / `package_name` | string | 是 | 与目录及实际包身份对应 |
| `version` | string | 是 | 精确 package.json 版本，不接受 latest、范围或分支名 |
| `compatibility.client_range` | string | 是 | 已验证的客户端版本范围，必须能按版本规则解析 |
| `compatibility.host_versions` | string[] | 是 | 已测试宿主精确版本，非空；不以客户端版本代替 |
| `compatibility.node_majors` | integer[] | 是 | 已验证 Node 主版本，非空；安装时核对实际宿主环境 |
| `config_schema_version` | integer | 是 | >0，配置读写和恢复的版本依据 |
| `health_check_profile` | string | 是 | 客户端已支持的运行检查标识，不接收远程脚本 |
| `rollback_supported` | boolean | 是 | 是否具备已验证恢复条件 |
| `rollback_notes` | string | 是 | 配置/数据恢复说明；不支持回滚时写明人工处理方式 |
| `test_evidence_ref` | string | 是 | 脱敏测试记录引用，不能仅写“已测试” |
| `artifacts` | object[] | 是 | 至少一个固定文件，不重复声明同一系统/架构组合 |
| `artifacts[].os` / `arch` | string | 是 | 对应支持的系统及架构，不能将本机包标为任意平台 |
| `artifacts[].format` | string | 是 | 首期 npm_tgz；其他格式须先适配安装器 |
| `artifacts[].source_type` | string | 是 | npm_registry / fixed_archive |
| `artifacts[].source_uri` | string | 是 | 精确 tarball 地址或公司固定文件引用；不夹带账号密码，私有存储鉴权另行配置 |
| `artifacts[].registry_url` | string | 条件 | npm_registry 时必填固定 registry；fixed_archive 时 null |
| `artifacts[].size_bytes` | integer | 是 | >0，实际文件字节数 |
| `artifacts[].sha256` | string | 是 | 64 位十六进制摘要；下载后校验实际文件 |

成功 data 为上列对象加服务端生成的发布及文件 ID、created_at:integer、revoked_at:integer或null、revoke_reason:string。已登记的身份、版本、文件与兼容资料不可覆盖；修复另建发布。固定 tgz 的摘要不等于安装依赖已固定，发布流程还须验证依赖解析和可复现产物，不满足时不开放受管更新。

GET 同一管理地址要求 Query `plugin_id:string`，可选 `page:integer` 默认 1、`page_size:integer` 默认 20、范围 1–100；成功 data 为 `items:object[]`、`total:integer`、`page:integer`、`page_size:integer`，items 使用上述完整发布结构，包括撤销记录。仅 Root 可读原始存储引用，客户端全局配置只读目标摘要资料。

POST /api/apps/starpath/admin/plugin-releases/:release_id/revoke 的 body 为 `{"reason":"撤销原因"}`，reason 必填且 1–500 字符；返回发布 ID、revoked_at、revoke_reason。重复撤销不重复创建撤销事件。撤销立即拒绝该发布的新计划及切换复核，已安装设备是否停用或回滚仍按全局配置另行发布。

### 5.3 安装计划请求与响应（拟新增）

POST /api/apps/starpath/plugins/:plugin_id/install-plan 使用当前用户登录会话。登录账号由服务端绑定，不接收 body.user_id。安装的包与版本不按订阅分配。

| 请求字段 | 类型 | 必填 | 含义与约束 |
| --- | --- | --- | --- |
| `operation_id` | string | 是 | 客户端为一次安装生成 UUID；申请、复核和回报沿用同一个值 |
| `device_id` | string | 是 | 本机随机安装标识，不用用户名或硬件序列号代替 |
| `phase` | string | 是 | download / switch，分别表示下载申请、切换前复核 |
| `plan_id` | string | 条件 | switch 时必须提供下载阶段取得的计划 ID |
| `client_version` / `host_version` | string | 是 | 客户端版本、宿主实际版本 |
| `node_major` | integer | 是 | 实际运行宿主 Node 主版本 |
| `os` / `arch` | string | 是 | 与固定文件匹配的系统及架构 |
| `installed_release_id` | string 或 null | 是 | 本机已验证受管发布，无记录时 null |
| `installed_version` | string 或 null | 是 | 本机实际包版本，未安装为 null |
| `config_revision` | integer | 是 | 当前观察到的配置版本 |
| `target_release_id` | string | 是 | 当前观察到的目标，必须与平台现目标一致 |

| 成功 data 字段 | 类型 | 返回要求 |
| --- | --- | --- |
| `plan_id` / `operation_id` / `device_id` / `plugin_id` | string | 必有，绑定计划、操作、设备和插件 |
| `config_revision` | integer | 必有，校验通过时的当前配置版本 |
| `phase` | string | 必有，与本次申请阶段一致 |
| `target_release_id` / `target_version` | string | 必有，唯一目标和精确版本 |
| `artifact.artifact_id` / `format` | string | 必有，匹配文件身份及格式 |
| `artifact.url` | string 或 null | download 时给 HTTPS 下载地址；switch 返回 null，不重新选包 |
| `artifact.size_bytes` / `sha256` | integer / string | 必有，登记时的字节数和摘要 |
| `artifact.expires_at` | integer 或 null | 下载地址有效期；switch 无地址时为 null |
| `expires_at` | integer | 必有，当前阶段计划有效期；下载地址不得长于此期限 |
| `preserve_config` | boolean | 必有，首期 true；不因更新清除私有配置或本机禁用选择 |
| `health_check_profile` | string | 必有，采用客户端已支持的检查 |
| `rollback_supported` / `rollback_notes` | boolean / string | 必有，明确恢复能力与限制 |

switch 复核仍绑定同一账号、设备、插件、目标及 artifact_id；目标变化、撤销、暂停、兼容不符或过期均拒绝。切换授权只在 expires_at 前使用，已知失效不继续切换；配置发布与本机文件替换不构成跨网络原子事务，平台只能依据回报确认实际结果。

相同账号、设备、operation_id 和 phase 的重复申请应返回相同未过期计划；同一键改变绑定字段返回 IDEMPOTENCY_CONFLICT。已过期、策略改变或换号后重新读取规则并创建新操作，不复用旧授权。下载文件只依据计划，renderer 不提交任意替换地址。

下载申请示例，路径插件为 `example.tool`；这是协议示例，不表示已有该商品：

```json
{
  "operation_id": "00000000-0000-4000-8000-000000000001",
  "device_id": "00000000-0000-4000-8000-000000000002",
  "phase": "download",
  "client_version": "<客户端实际版本>", "host_version": "<宿主实际版本>", "node_major": 24,
  "os": "darwin", "arch": "arm64", "installed_release_id": null, "installed_version": null,
  "config_revision": 4, "target_release_id": "rel_example"
}
```

对应成功响应示例，零值摘要仅占位，必须替换为真实文件摘要：

```json
{
  "success": true,
  "message": "",
  "data": {
    "plan_id": "plan_example", "operation_id": "00000000-0000-4000-8000-000000000001",
    "device_id": "00000000-0000-4000-8000-000000000002", "plugin_id": "example.tool",
    "config_revision": 4, "phase": "download", "target_release_id": "rel_example", "target_version": "1.0.0",
    "artifact": {
      "artifact_id": "artifact_example", "format": "npm_tgz", "url": "https://files.example.com/example-tool-1.0.0.tgz",
      "size_bytes": 1024, "sha256": "0000000000000000000000000000000000000000000000000000000000000000", "expires_at": 1900000060
    },
    "expires_at": 1900000060, "preserve_config": true,
    "health_check_profile": "<客户端已支持的检查标识>", "rollback_supported": true, "rollback_notes": "保留并恢复安装前配置及有效包"
  }
}
```

下载、staging 安装和本机检查完成后，先等待空闲，再用相同绑定字段、phase=switch 及 plan_id 在线复核；复核成功后才能切换。上述示例的节点版本、期限、文件大小和恢复说明均不作为正式发布默认值。

### 5.4 设备结果回报字段（拟新增）

POST /api/apps/starpath/plugin-reports 使用用户登录会话，服务端记账号及 received_at；设备自报的时间不作为服务端授权依据。

| 请求字段 | 类型 | 必填 | 含义与约束 |
| --- | --- | --- | --- |
| `report_id` | string | 是 | 单条回报 UUID，网络重试沿用 |
| `device_id` / `plugin_id` | string | 是 | 实际设备及插件 |
| `operation_id` | string 或 null | 是 | 安装相关阶段必须关联已有操作；独立配置应用或本机卸载可为 null |
| `config_revision` | integer | 是 | 此次实际应用或执行依据的配置版本 |
| `sequence` | integer | 是 | >0，同设备、插件、配置和操作内递增；迟到低序号不覆盖较新事实 |
| `phase` | string | 是 | config / download / install / health / rollback / uninstall |
| `status` | string | 是 | success / failed / deferred / not_run；deferred 表示任务忙等延期，not_run 表示尚未执行检查 |
| `target_release_id` | string 或 null | 是 | 操作目标；无独立目标的随包项可为 null |
| `actual_release_id` | string 或 null | 是 | 本机核验后的实际受管发布，未安装或无法确认时 null |
| `actual_version` | string 或 null | 是 | 本机实际版本，未安装时 null |
| `local_enabled` | boolean | 是 | 用户的本机启停选择，不伪称全局已启用 |
| `reported_at` | integer | 是 | 客户端 Unix 秒，用于记录及排查 |
| `reason_code` | string 或 null | 是 | 成功时 null；异常或延期使用稳定原因，如 ARTIFACT_MISMATCH、RUNTIME_CHECK_FAILED、TASK_BUSY、LOCAL_DISABLED |
| `error_message` | string | 否 | 脱敏说明，最多 1000 字符，不含凭据、命令原文或私有配置 |

成功 data 为 `report_id:string`、`accepted:boolean`、`received_at:integer`。同 report_id 同内容幂等返回；同 ID 不同内容返回 IDEMPOTENCY_CONFLICT，不能覆盖旧记录。安装相关回报须核对登录账号与计划绑定；换号后待上传旧回报保留原归属，不能改为新账号提交。

本机禁用时 install=success 后可回报 health=not_run、reason_code=LOCAL_DISABLED；实际启用后再用新 report_id 和递增 sequence 回报检查。平台区分“文件安装成功”和“运行检查成功”，离线或没有近期回报仍为未确认，不能由目录发布结果推断全部设备已生效。

## 六、开发交付与验收

本章对应 DP-01、DP-03、DP-04 和 DP-05。平台开发交付目录、发布、计划及回报接口；星途开发接入入口、安装器与运行检查；插件维护者交付固定文件、兼容和恢复依据。

| 验收场景 | 通过标准 |
| --- | --- |
| 两个账号统一展示 | 飞书隐藏后均隐藏，恢复后均恢复，目标一致；飞书不预设收费 |
| 订阅使用差异 | 实际需订阅功能在未订阅、有效、到期时正确变化，目录及目标保持一致 |
| 新增与更新 | 登记发布不自动改变目标；来源有更高版本仍只安装公司目标 |
| 下载后策略变化 | 目标变更、暂停或撤销后拒绝旧计划，保留当前有效安装 |
| 文件与兼容校验 | 摘要、运行身份、版本或兼容不符时拒绝覆盖并回报原因 |
| 运行失败与回滚 | 区分已安装和运行正常，失败安全恢复或阻断，配置与成果保留 |
| 重试、换号及运行中 | 同一设备安装串行且可重试；换号重取计划，任务中延后安装 |
| 随包与基础能力 | 不直接套用独立覆盖流程，现有连接器、对话及基础扩展回归通过 |

当前源码盘点与管理方案不能替代真实安装、运行或远程策略验收。实施顺序及统一证据要求见[管理总览](/admin-manual?article=account-login)。
