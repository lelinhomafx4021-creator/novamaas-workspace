# 平台对接手册设计说明

更新日期：2026-10-19。版本：2.0 收口稿。六章正文、实施计划与受保护阅读功能独立整理于 `codex/admin-manual`，基线为 `company/main` 的 `112fb826f`。原工作区其他任务及本地规则不纳入交付。

## 2.0 本轮交付与状态

| 交付 | 内容与状态 |
| --- | --- |
| 六章正文 | 保持六个 article ID；补充账号/Key 关联、只读套餐边界、可信云动作、媒体适配、配置生效回报和实施验收 |
| 权限与发布契约 | 全部 `/api/apps/starpath/...` 仍待开发；配置回报与安装回报分开，统一目标版本不按用户或套餐分流 |
| 实施交付计划 | [双方源码接点、阶段/责任、数据兼容、场景与上线门槛](STARPATH_INTEGRATION_DELIVERY_PLAN.zh_CN.md) |
| 阅读功能 | 后端 embed、默认关闭的 root 开关、管理员鉴权、前端独立路由及失败清缓存；不是插件权限服务 |
| 合并审阅稿 | [单份完整手册](../review/星途与Gateway接口对接手册.md)由六章生成；不再人工维护第二份正文 |
| 验证工具 | `web/scripts/build-admin-manual-review.mjs` 检查示例/链接/表格/目录一致性；已有 isolation 脚本检查生产构建与公开内容 |

本次只读复核星途账号、套餐、旧媒体路径和插件接点，并纳入 2026-10-19 稳定性计划：先验现有功能，套餐当前只读；不借手册恢复购买或改造官方对话、工具、技能、MCP。应用 Key 仅证明应用归属，共用通用模型接口不能据客户端自报插件 ID 承诺插件级云强制授权；纯本地限制的边界在第三章明确说明。

原 PDF 在原路径未找到，本次沿用视频章的历史差异矩阵，以当前源码核对现有契约，不新增逐页检查结论。没有真实支付、模型生成、插件安装、发布、迁移或两端业务联调；它们分别列入后续验收。

### 本轮验证记录

本次在独立工作区对当前文件执行，结果如下：

- 六章和四份设计文档：81 段 JSON/HTTP body、115 个表格、22 个内部链接及锚点检查通过；合并稿与生成结果一致，六章 99 段代码原文完整保留。
- Python 示例 1 段通过 AST 语法检查，TypeScript 示例 2 段通过语法转译；没有执行示例，不作为功能集成或类型联调证明。
- `GOWORK=off go test ./controller ./model ./middleware ./router ./docs/admin-manual ./docs/platform-api -count=1` 通过。运行时移除外部数据库测试 DSN；前四包执行测试，两个文档包只有编译、没有测试用例。
- 前端相关 5 个文件 37 项测试、`bun run typecheck`、涉及文件格式检查通过；lint 无 error，现有阅读组件保留 3 条 warning。测试使用 `NODE_OPTIONS=--no-experimental-webstorage` 避免 Node 的实验性 Web Storage 干扰 jsdom，不改业务代码绕过测试。
- 本轮重新生产构建通过；隔离检查覆盖 242 个构建文件、8 个 public 文件、7 篇公开文档和 2 个 OpenAPI。手册正文没有进入公开内容或静态包。
- 仅保留阅读功能使用的 3 个新文案，七种语言均有值；原 locale key/value 全部保留。原工作区其他翻译和本地规则不进入本次交付。

未启动或变更业务服务、数据库、渠道及客户端；未进行真实模型/支付/插件安装、三数据库业务联调或原生多平台验收。以下 2026-10-16 记录只作为历史，不代表新增业务能力已实现或本轮真实联调通过。

### 维护命令

在 `web/` 执行 `bun run docs:admin-manual` 更新合并稿，执行 `bun run check:admin-manual` 检查六章、四份设计文档、示例及内部链接。完成新的生产构建后执行 `bun run check:admin-manual-isolation`；缺少 dist 时检查应失败，不能复用旧构建充当本轮证据。

## 2026-10-16 历史设计与验证

## 统一版本控制的最新决定

用户明确要求全权统一控制。每插件一份当前版本策略，由 Gateway 指定精确目标、升级期限、暂停、撤销和回滚；所有获授权用户执行同一目标。账号/套餐仍控制使用权益，平台/架构只匹配同版本的不同文件。

已移除按用户、套餐、比例分配版本的契约和旧发布批次接口，替换为 `/api/apps/starpath/admin/plugins/:plugin_id/release-policy` 的 GET/PUT 与兼容预览。客户端没有自主选版入口；过渡期限由后台指定，届时安装版必须匹配目标，兼容性不满足时明确提示。所有这些接口仍为待开发方案。

## 本次 PDF 对照修订

用户确认：聚合网关为管理和规则中心，星途是接入并执行的一端；“插件更新”包含版本发布、更新提示、强制更新、统一版本和回滚。不得将两端已有的零散能力描述成已经完成统一管理。

本次内容直接进入 `docs/admin-manual/` 六章，通过既有受保护手册接口提供；合并可审阅版本在 [星途与 Gateway 接口对接手册](../review/星途与Gateway接口对接手册.md)。不改变既有 article ID，不新增阅读大类。

| 章节 | 本次补充 |
| --- | --- |
| 账号与钱包 | 两平台职责/接入时序、凭证边界、账号会话接口索引 |
| 套餐与订阅 | 19 项现有用户/管理接口，补两个遗漏的重置入口；完整字段、快照/动态行为、返回差异；拟定商品详情与功能授予接口 |
| 密钥与插件权益 | 密钥接口清单；逐项权限配置；插件列表/详情；版本登记、发布/撤销、统一策略/兼容预览、用户检查/授权/上报；强更/统一版本/回滚、断网和换号契约 |
| 文本接口 | Chat/Responses/Claude/Gemini 字段、请求响应、工具/视觉输入、流事件和相关能力索引 |
| 图片接口 | v1/v3 字段、同步/Base64/编辑/mask/流式响应、结果保存和失败处理 |
| 视频接口 | Wan/Seedance 字段、列表/取消/下载；REST 素材、AK/SK、10 项 Action CRUD、Webhook REST/事件、Python 签名与轮询；PDF 差异矩阵 |

现有能力以本仓库路由/控制器/模型和星途相关源码为依据。所有 `/api/apps/starpath/...` 为规划，未加入路由或 OpenAPI；原生 Webhook 的 5 个 Action 在 PDF 中存在、当前代码未实现，文档明确转用现有 REST，不伪称已支持。

星途仓库只读核对插件 handlers/manager、maasAuth/platformConfig、企业 UI 配置与 appUpdateGrayClient。发现本地插件更新仍重装未固定版本的源；主应用灰度客户端调用的 `/api/client-updates/check` 在当前 Gateway 无对应路由。新契约要求 Gateway 精确版本决策及受控安装适配，不能复用 latest 冒充发布控制。

文档中的套餐价格、插件 ID、版本、SHA256 和时间均为契约示例；不是生产配置或真实发布记录。未执行支付、模型生成、发布或数据库变更。

### 插件源码调研与接入细则补充

用户继续要求调研真实插件接入，并达到逐字段、逐链路的说明深度。已只读核对星途插件管理/安装/配置同步/工具回调/账号隔离、18 项随包声明和 5 个本地扩展，以及 OpenClaw v2026.8.1 固定源码与官方资料。

- 新增[插件接入调研](STARPATH_PLUGIN_INTEGRATION_RESEARCH.zh_CN.md)，关键内容已完整并入第三章，保留六章结构。
- 明确 package_id/runtime_id/channel_id 三种标识差异、核心/provider/渠道/业务工具的接入边界，列出 11 项现有 IPC 和作者包/工具示例。
- 补齐 user_plugins 当前缺少账号维度、desired_enabled 与有效状态分离、配置 schema/内部凭据、allow/entries 合并和真实执行守卫。
- 细化旧媒体工具到现有图片/Wan/Seedance 接口的转换，说明无通用图片任务、无通用 Wan 取消、旧计费和订阅提示的差异。
- 修订发布契约为首期 npm_tgz 和固定宿主/Node/schema/包身份检查，明确现有目录切换恢复不等于健康检查后回滚；补传递依赖与历史元数据恢复要求。
- 星途与 OpenClaw 源码只读；未重启客户端、安装插件、改业务 API、发起生成或发布操作。文档语法/链接/示例和内容隔离分别检查，功能开发及真实两端联调仍未完成。

本次检查通过：合并审阅版重新生成，六章不变；10 份相关文档的 JSON/HTTP 示例、Python 语法和内部链接检查通过；调研版/第三章/合并版共 3 处 TypeScript 样例语法转译通过（不是类型集成或执行验证）；Markdown 表格解析通过。文档包编译和现有构建内容隔离检查通过。星途关键源文件摘要及 Git 状态与开始时一致。本轮只改 4 份既有 Markdown 并新增调研文件，没有功能测试或真实联调结果。

### PDF 对照修订验证（此前记录）

- 六章正文、两份设计说明和合并审阅版共 9 份 Markdown；JSON 示例/HTTP body 可解析，Python 代码可通过语法检查，内部链接与锚点可解析。合并审阅版由六章当前内容生成，包含详细索引。
- 使用现有 marked 解析六章：94 段代码示例、55 个表格，未出现表格列数错误或未闭合代码块。
- `GOWORK=off go test ./docs/admin-manual ./docs/platform-api` 通过包编译；两个包没有测试用例，该结果不代表新增业务功能已测试。
- `bun run check:admin-manual-isolation` 通过，检查现有 239 个构建文件、8 个 public 文件、7 篇公开文档和 2 个 OpenAPI；本轮未改前端源码，未重新生成生产构建。
- `git diff --check` 与 Markdown 空白检查通过。当前修改范围仅上述 9 份文档；原工作区其他修改保留。

历史工作分支 `codex/admin-manual` 最初从 `company/main` 的 `ce9e31f1e` 建立。初始设计经用户“全部写吧，有问题问”确认后实施；后续改为星途 × Gateway 开发对接手册。该阶段没有提交或推送；2026-10-19 已在独立工作区对齐本文件顶部的新基线，原工作区无关变更保留。

## 1. 当前阅读结构与两平台分工

| 章节 | 内容 | 文件 / article ID |
| --- | --- | --- |
| 账号与钱包 | 注册/登录、会话、个人资料、钱包积分、充值和消费回查 | `docs/admin-manual/01-account-login.md` / `account-login` |
| 套餐与订阅 | 用户查询和购买、管理端套餐和订阅操作、支付后回查 | `docs/admin-manual/02-subscriptions.md` / `subscriptions` |
| 密钥与插件权益 | 用户密钥获取/维护、模型与插件可见性、权益变更统一同步 | `docs/admin-manual/03-api-keys.md` / `api-keys` |
| 文本接口对接 | 同步对话、流式与其他文本协议 | `docs/admin-manual/04-text-models.md` / `text-models` |
| 图片接口对接 | 文生图、图片编辑、流式图片与结果保存 | `docs/admin-manual/05-image-models.md` / `image-models` |
| 视频接口对接 | Wan v1、Seedance v1/v3、任务查询/取消/下载、参考素材 | `docs/admin-manual/06-video-models.md` / `video-models` |

保留既有 article ID。账号和钱包、密钥和插件分别合并；三种模型各自成章，视频章内再分 Wan 和 Seedance。没有运维、排错、审计或“待验证方法”附录，接口限制就近说明；订阅同步与消费回查集中维护。

手册已覆盖：充值报价/支付/到账记录及 CUSTOM 数量边界；选定模型 Key 的 `/v1/models` 目录；生成超时、请求 ID 和消费记录；视频 v1/v3 响应示例；素材分组、文件上传、状态与渠道副本可用条件。缺口只在相关章节说明，不新增两个平台的业务能力。

Gateway 保存账号、钱包/订阅账本、模型密钥、渠道及生成任务。星途负责界面、项目/对话、协议适配、按用户隔离的调用配置及插件执行。用户模型调用用自己的 Key；运营账号只用于服务端管理操作。复用 Gateway 账号时以其 user.id 关联，不必另建账号体系。

手册面向星途开发人员，仍只对已获权限的 Gateway 管理员开放。界面名为“平台对接手册”，保留原内部键、目录和 URL，避免无关迁移。现有接口按源码说明；用户确认必须补齐的插件权限方案在第三章明确标为“未实现”，不能把拟定请求示例或本地测试当成真实两平台联调完成。

## 2. 提供方式、权限与隔离

### 现有用户文档

`docs/platform-api/content.go` 只嵌入该目录的七篇 Markdown，`GET /api/docs` 返回其内容。`HeaderNavModuleAuth("docs")` 决定是否开放、是否需要登录：旧配置 docs:true 可匿名读取；关闭或无效配置拒绝。公开搜索仅针对该接口返回内容。

角色值：普通用户 1、管理员 10、root 100。UserAuth/AdminAuth/RootAuth 复用后台凭证解析，校验用户有效性、启用状态和最低角色；接受登录 Access Token 或面板 PAT。模型 API Key 不能作为手册后台身份。

### 手册策略

配置键 `AdminManualEnabled`，默认字符串 false。仅 root 通过 `/api/option/` 修改；只接受 true/false，拒绝无效值。配置沿用 options 表；该键不是敏感凭证，不加入 GET option 的秘密键过滤，也不进入匿名 status、用户资料或导航配置。

| 身份 | 开关关闭 | 开关开启 |
| --- | --- | --- |
| 匿名 | 不显示入口；接口 404 | 不显示入口；接口 404 |
| 普通用户 | 同上 | 同上 |
| 管理员 | 同上 | 可见、可读取 |
| root | 同上，无预览例外 | 可见、可读取 |

- `GET /api/admin-manual/access` 仅返回可用能力；`GET /api/admin-manual` 返回六章正文。两者都支持有/无尾斜线，不经预鉴权跳转暴露接口。
- 专用 AdminManualAuth 每次读取持久化策略；缺失、非法值、数据库错误均关闭。有效身份还需 role >= admin。未授权响应复用 RelayNotFound，与不存在接口相同。
- 响应 private/no-store，Vary Authorization，X-Robots-Tag noindex/nofollow/noarchive。前端按用户与会话隔离查询缓存，失权、退出、换号时清除正文；查询关闭全局 GET 去重，避免 StrictMode 复用已取消请求。
- 根开关写入使用可检查保存错误的事务流程，沿用 option.update 管理写审计；只记录键和操作者，不记录配置值。失败不产生成功审计。

### 存放与入口

正文在 `docs/admin-manual/`，后端 embed，显式六篇清单，与公开文档清单分离。.dockerignore 只允许该后端包必需的 Go/Markdown。正文不进入 OpenAPI、公开搜索、前端静态资源或站点地图。

前端共用现有文档阅读器，在文档内切换；管理侧栏有授权时显示直达入口。root 在系统设置 → 站点设置控制开关。SPA `/admin-manual` 守卫失败显示通用不存在页，数据接口真实返回 HTTP 404；不依赖浏览器导航携带 Authorization 来保护静态应用壳。

## 3. 接口核对与缺口归并

### 范围纠正

用户明确此次只做接口文档总结。误加的订阅订单查询、支付回执扩展、密钥创建回执、CORS 改动及对应测试已撤回；相关业务文件与本轮开始前一致。路由只保留已授权的手册提供接口，未新增订阅、密钥或模型业务接口。星途仓库始终只读。

手册区分当前接口与拟定方案。没有统一订阅订单查询、创建密钥不返回 ID、跨域请求编号未暴露、插件权益和报价接口未提供等，均作为现有能力缺口说明，不将建议写成已实现功能。按用户后续确认，插件权限为必须建设项，本轮仍只完善文档和接口方案；详细契约见 [星途功能与插件权限对接方案](STARPATH_FEATURE_ACCESS.zh_CN.md)。

### 两平台现有能力与剩余缺口

已读本仓库真实路由、控制器、模型及适配器；星途仓库只读参考。草稿不是事实来源，未修改星途运行链路或业务数据库。

| 所属章节 | 现有能力与应补的对接 | 尚缺能力 / 条件 |
| --- | --- | --- |
| 账号与钱包 | 星途已有 Gateway 登录、Cookie 续期、钱包和订阅读取；已有资料修改、找回密码、设备管理、充值与消费查询 | 单独星途账号体系才需额外身份映射；网页跨站代理与桌面 Cookie 容器分开处理 |
| 套餐与订阅 | 用户查询/购买，管理端查询、发放、失效、删除；支付回调已有部分去重 | 统一订阅订单查询与订阅关联、客户端操作幂等；升级差价、下周期切换、退款需业务规则 |
| 密钥与插件权益 | 用户 Key CRUD（创建不返回 ID）、可用模型、部分 pricing 元数据；星途本机企业配置能隐藏部分入口，但不是远程按用户授权 | 必须建设：Gateway 功能目录/套餐授权/用户覆盖/最终权限读取，星途显示与执行校验、可信应用 Key、同步与失效；现已补接口方案，未实现 |
| 文本/图片/视频 | 现有文本、图片、Wan v1 与 Seedance v1/v3；任务列表、素材上传/预览、媒体事件 | 完整模型参数能力目录、按用户参数报价；普通文件库/完整云端生成历史按产品范围决定 |

关键代码边界：

- 星途 maasAuth.ts 已有登录/余额/订阅读取；platformConfig.ts 仍取单独的 provider.apiKey/models。main.ts 媒体链路还调用旧 `/api/media/...` 并按 code:0 解析，需客户端适配现有 v1/v3，不必复制旧 API 到 Gateway。
- 星途视频费用提示仍有固定 100 积分/秒估算，不能视为 Gateway 实际价格。可先取消不准确预估，或后补用户报价接口。
- `/api/token/` 只操作当前用户密钥，创建只返回 success/message，没有 ID 或明文；管理员身份不自动变成其他用户。订阅改变分组不自动改写所有密钥。
- `/v1/videos/:id` 是顶层视频对象；旧 `/v1/video/generations/:id` 为 code/data 且适配器响应有差异。Seedance v1 的 seconds/metadata 与 v3 content/duration 分开。
- 已有 `/api/task/self`、`/api/log/self`、`/api/log/self/stat`、`/api/pricing`，不要误报为全部缺失。pricing 只提供部分能力元数据且受导航开放策略控制。
- 媒体/素材事件不包含订阅、钱包和插件变化。桌面端不作为稳定公网回调接收服务，无接收服务时先回查。
- 公开 `docs/platform-api/04-videos.md` 已校正 v1/v3 选择、Seedance 2.5 分辨率、审核与 CDN/有效期承诺、取消终态、Gateway 任务 ID 及 v1 查询响应。公开文档只描述模型调用，不引入管理员手册的账号、订单或管理内容。

不为方便接口而重复创建账本、任务或账号体系。账号/钱包/订阅/模型的一次聚合读取可后补，首版可组合已有接口。插件授权与两平台权限联动已确认为必须建设，不能仅通过隐藏按钮验收；本轮只交付方案。完整模型参数能力目录/报价、受限管理身份、完整云端历史、应用更新/公告等按实际产品范围另排。

### 2026-10-16 功能与插件权限方案补充（此前记录）

- 阅读结构仍为六章，新增内容集中在第三章，不新增用户阅读大类；本地合并审阅版同步更新。
- 明确星途旧企业后台入口、本机企业 UI 配置、Gateway 登录三者的边界，避免误报已有远程管理员端。
- 拟定 `/api/apps/starpath/...` 的本人权限、专用 Key、目录、套餐授权、用户覆盖与管理预览契约；全部标为未实现，未加入路由或 OpenAPI。
- 细化多套餐并集、用户覆盖、全局关闭、订阅/覆盖到期、下架与额度变化、断网和换号、执行及计费前检查。详细文档单独保存于 `docs/design/STARPATH_FEATURE_ACCESS.zh_CN.md`。
- 本次文档结构、示例、链接、第三章同步和公开内容隔离检查通过，范围比对仅四份 Markdown 变化，星途仓库无新增变更；未重新构建或运行功能测试，不沿用下文先前的业务测试作为新增权限能力的测试证明。

## 4. 实现文件与此前验证记录

| 范围 | 文件 |
| --- | --- |
| 策略、鉴权与配置 | model/admin_manual.go、middleware/admin_manual.go、model/option.go、controller/option.go |
| 内容与路由 | controller/admin_manual.go、router/api-router.go、docs/admin-manual/content.go、六篇正文、.dockerignore |
| 前端阅读与入口 | features/admin-manual、features/api-docs/page.tsx、文档渲染器、侧栏和 public-header、admin-manual 路由 |
| root 开关 | system-settings/site/admin-manual-section、相关类型/注册/缓存失效；七种语言翻译 |
| 自动检查 | router/admin_manual_test.go、middleware/admin_manual_test.go、相关前端 __tests__、web/scripts/check-admin-manual-isolation.mjs |

### 2026-10-16 验证结果

已核对：

- 支付四个控制器、controller/token.go、middleware/cors.go 均无相对原分支的差异；误加的四个业务实现/测试文件已移除。
- router/api-router.go 只保留手册路由变更。正文不再描述不存在的订阅订单查询或密钥创建返回 ID。
- 公开视频文档保留纯文档纠错：按 Wan/Seedance 选 v1/v3；校正分辨率、审核、下载有效期、取消及查询响应说明。

已测试通过：

- `GOWORK=off go test ./controller ./model ./middleware ./router ./docs/admin-manual ./docs/platform-api -count=1`。显式移除外部数据库测试 DSN，使用隔离临时测试库。
- 文档权限矩阵与隔离检查通过：6 篇手册、239 个现有构建文件、8 个公开文件、7 篇用户文档、2 个 OpenAPI。
- 手册 24 段、公开文档 33 段 JSON 示例可解析；代码块、内部链接/锚点、占位域名及 git diff --check 通过。
- 本地通过 make server 重新加载撤回后的代码：误加的订单列表/详情地址均返回 404，CORS 恢复原行为。浏览器确认手册将订阅订单查询标为现有缺口，不再展示不存在的接口。
- 页面结果：`/tmp/platform-integration-docs-only.jpg`。没有提交或推送。

已读代码核对：各接口字段、凭证、扣费来源、同步和模型协议差异。未验证：真实支付、模型生成和两平台业务联调；缺口没有实施。

此前已通过的相关检查：后端鉴权/控制器/模型、文本/图片和 Wan/Seedance 适配测试；前端相关 7 个文件 48 项、typecheck、涉及文件 lint/format 和生产构建。全量前端曾一次通过、另一次出现无关用例不稳定，不能据此宣称全量持续通过。

已读代码推断：正文各接口、凭证、扣费来源、同步与协议差异来自源码核对。未验证：星途真实联调、线上支付、真实文本/图片/视频生成、第三方素材/回调、完整跨数据库/多节点业务验收。上述能力没有因文档整理而变成已实现或已实测。
