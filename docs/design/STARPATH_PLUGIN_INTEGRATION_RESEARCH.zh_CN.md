# 星途插件接入调研与实施契约

调研日期：2026-10-16；接点复核与交付规划更新：2026-10-19，手册 2.0 收口稿。规则：Gateway 全权统一指定版本，所有获授权用户同一目标，平台/架构仅选择同版本的不同包。范围：只读核对源码、补接入文档；没有安装插件、修改星途、修改业务接口或触发外部调用。

本次复核星途 `60d7591960120f22d3544b8e88abb16cf7f3526f` 的账号/套餐适配、旧媒体路径及插件接点，并读取当前稳定性规则；该工作区另有未提交改动，不作为已验收发行基线。下文插件清单和固定运行时细节保留原调研基线，不冒称全部在本轮重新安装验证。任务和验收编号统一见[实施与交付计划](STARPATH_INTEGRATION_DELIVERY_PLAN.zh_CN.md)。

结论：Gateway 管商品、套餐、最终权限和发布决策；星途负责运行时接入和落实。先完成“媒体能力适配”和“一个受控业务工具插件”两条链路。随包模型适配、消息渠道、核心扩展、用户安装插件分别接入，不能都塞进同一个可卸载商品列表。

手册入口：[密钥与插件权益](../admin-manual/03-api-keys.md)。通用远程 API、套餐授权、发布/更新/回滚契约沿用该章；本文件补充实际插件清单、源码接点、安装产物、执行边界和实施顺序。

## 调研基线与证据等级

| 项目 | 核对结果 |
| --- | --- |
| 星途应用版本 | package.json 为 2026.9.23 |
| 固定 OpenClaw | v2026.8.1；本地源码 HEAD 为 ea806575e6450e4d1efdfc72c19f04be982a1b9b |
| 随包插件声明 | package.json 的 openclaw.plugins 共 18 项；声明不代表当前设备均已安装/启用 |
| 本地扩展 | openclaw-extensions 中 5 个 manifest |
| 现有用户插件管理 | Electron preload → plugins:* IPC → PluginManager → user_plugins / OpenClaw |
| Gateway 远程管理 | 当前没有 /api/apps/starpath/...；本册这些地址全为待开发契约 |
| 外部资料 | 官方 v2026.8.1 文档与当前官方文档交叉核对；实现以项目固定版本和已应用补丁为准 |

“已确认”指从源码读到，不代表真实端到端测试通过；“拟定”是实施要求；“待核对”不能当作已支持。固定版本的插件 API 仍需按目标包测试，不能照当前官网的新接口直接替换现有 SDK。

## 真实插件清单与接入分类

### 18 项随包声明

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

### 5 个本地扩展

| runtime_id | 注册能力 | 当前接点 | 如何纳入管理 |
| --- | --- | --- | --- |
| ask-user-question | AskUserQuestion | 本机 /askuser 回调 | 核心确认能力保留；不能因 plugins.manage 隐藏而关闭 |
| lobster-media-generation | lobsterai_image_generate、lobsterai_video_generate、lobsterai_skin_manage | 本机 /media-generation/tool 回调 | 保留扩展加载，在业务动作处分别检查图片/Wan/Seedance；不能整包当一个付费开关 |
| lobsterai-model-compat | 显式模型/思考档位适配 | modelProfiles / thinkingProfiles | 宿主兼容基础设施，随客户端/运行时维护 |
| mcp-bridge | 按配置暴露 MCP 工具 | 旧桥接协议 | 现有主链路使用原生 mcp.servers；不能为做权限而重新强制切回旧桥 |
| lobster-decision | decision_evaluate | 本机 /decision/tool | 实验能力默认关闭；本次不自动恢复或售卖 |

插件、技能和 MCP 分别维护。DOCX/XLSX/PPTX/PDF 来自技能和文档运行时，不是本表中的四个独立原生插件；禁止把它们改登记成可随意卸载的付费插件以绕过既有保留要求。

### Gateway 商品目录首期准入范围

首期 `/api/apps/starpath/admin/plugins` 只接经核对的独立业务工具插件。核心/provider/随包渠道不允许通过该目录创建另一份同 runtime_id 的包，也不套用普通用户卸载、强制更新或回滚流程。渠道未来可有远程功能授权，但安装和凭据仍沿用现有专用链路。

尚未取得真实业务插件商品清单；手册中的 example.storyboard 为对接样例，不是本次发现的已安装商品。nsp-clawguard 只有明确版本的兼容处理源码，不能据此断言用户已经安装或需要将其上架售卖。

## 现有链路：从按钮到运行时

### 插件管理的真实调用顺序

1. PluginsSettings/PluginConfigPage 调用 window.electron.plugins。
2. preload 将请求转到 plugins:* IPC，所有本地文件和进程操作由主进程执行。
3. handlers 调用 PluginManager，读写 user_plugins，并在安装/卸载/启停/配置后调用 syncOpenClawConfig。
4. 安装器把 npm 包打成 tgz、把 Git 源检出后打包，或使用 ClawHub/本地来源，再交给固定 OpenClaw CLI。
5. OpenClaw 在独立 staging 中安装；星途识别 manifest.id，切换到用户 third-party-extensions 目录，记录版本。
6. 配置同步生成 plugins.load.paths、allow、entries，再按影响分类请求重启本地 OpenClaw。
7. 运行时发现/校验/加载插件，注册工具、消息渠道等能力；模型调用工具后才到对应 execute 或本机回调。

这里的“本地 OpenClaw gateway”是客户端运行时，不是远程聚合网关 Gateway。远程管理响应不会自动修改本地加载器，必须经星途主进程适配。

### 现有 IPC 完整表

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

### 三层目录与元数据

| 位置 | 当前用途 | 接入要求 |
| --- | --- | --- |
| 应用 resources/cfmind 或开发 vendor/openclaw-runtime/current | 固定运行时及随包资源 | 不在活动客户端目录重建/覆盖；随宿主版本维护 |
| runtime 的 third-party-extensions | 构建时同步的随包渠道/本地扩展 | 不与用户安装目标混用，不随意覆盖 SDK 依赖 |
| userData/third-party-extensions | 用户安装的插件 | 现有 PluginManager 主目标，保存跨应用升级的包 |
| userData/openclaw/state/extensions | OpenClaw 其他安装来源 | 当前也参与发现；受控 ID 必须排除影子副本绕过 |
| userData/plugin-install-staging | 安装中间和切换备份 | 在扫描目录之外，保留同卷 rename 语义 |
| user_plugins | plugin_id/source/spec/version/enabled/config | 当前按插件 ID 全局存储，没有 Gateway user_id/origin 隔离 |

不能把某个目录存在当成插件已经可运行。配置 enabled=true 仍需 load.paths 能发现、allow 允许、schema 合法、宿主兼容且实际注册成功。

## Gateway 与星途的新增接缝

### 身份、绑定和本地状态

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

### 配置同步的准确插入点

在 openclawConfigSync 现有托管 entries/allow 合并处计算受控插件的 effective_enabled：先保留核心/provider/现有渠道的原规则，再对已登记业务插件收紧。

- entries 使用 manifest runtime_id，不用 npm 名或构建目录名。
- 从原 existingAllow 中去掉受控业务插件的旧许可，再加入当前有效可运行的受控项，防止残留 allow 扩大授权。
- 对受控插件设置最终 entries.enabled；不能因为 raw user_plugins.enabled=true 又覆盖前面的拒绝。
- 不覆盖其他插件的原有配置、memory slots、原生 mcp.servers；不以清空全部 allow/deny 或删除目录来撤权。
- 当前全部插件变更均分类为 Restart。权限刷新内容没变时不重写配置/重启；确有需要时合并一次操作，并在安全点处理运行中的对话。

仅改配置还不够：已加载工具、在途回调和后台任务要在真实执行边界再次检查。首次登录未取到快照不闪现受控入口，换号清掉旧决策并丢弃迟到响应。

### 受控安装的 IPC 输入

建议保留原普通用户插件 IPC，同时增加或等价实现两个受控入口：

| 拟定 IPC | renderer 提交 | 主进程自行取得 |
| --- | --- | --- |
| plugins:install-managed | pluginId | 当前账号、check 决策、download/install 授权、固定包和摘要 |
| plugins:update-managed | pluginId | 同上，并验证实际当前 release 与回滚方向 |

renderer 不传可任意替换的 URL、registry、文件路径或 SHA256；不能以“我知道 release_id”为授权。现有泛用 install/local/git、检测导入、旧目录发现和改名安装也要识别受控 runtime_id，避免从另一个入口加载影子包绕过策略。

## 插件作者具体交付什么

### 最小包结构

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

### 业务工具入口示例

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

### 两种业务插件的计费路径

| 类型 | 实际执行 | Gateway 责任 |
| --- | --- | --- |
| 调用模型的公司插件 | 工具 → 本机桥接 → 用户模型 Key → 现有模型 API | 功能/Key/模型/额度校验后预扣费，按实际调用结算 |
| 纯本地插件 | 工具 → 主进程许可检查 → 本地逻辑 | 只提供远程权益，当前不因此新增云端调用费用 |

如要按“插件执行一次”另外收费，需要独立服务边界、预扣/结算/失败退款和幂等业务设计；当前没有这一计费接口。不能用调用插件 API 的次数、客户端上报或下载次数直接扣钱包，也不能在现有模型费用外暗加一笔。

## 现有媒体插件怎么接 Gateway

### 保持工具协议，替换主进程适配

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

### 工具字段到网关字段

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

### 工具结果、权限与历史

星途继续返回 `content[]/isError/details`，不把 Gateway 原生 JSON 直接丢给模型当作旧 code/data。媒体插件原有 status 动作会自适应轮询，保持这一能力，避免模型重复发起多轮忙轮询。

查询时保留原 Gateway 状态，同时映射既有本地媒体状态：v1 completed 与 v3 succeeded 均为完成，queued/running/in_progress 为处理，失败/取消/过期分别保留原因。先保存 `{origin,user_id,account_generation,protocol,model_id,gateway_task_id}` 再开始查询；不把上游 ID 或临时 UI 作业 ID当 Gateway ID。

image.generate、video.wan、video.seedance 分别在新生成动作检查；同一媒体插件不能只配一个 plugin:lobster-media-generation 开关。status、本人旧任务下载/取消及历史查看按原所有权保留。皮肤工具复用媒体扩展，但不能因本次接入恢复或扩大当前产品未开放的皮肤功能。

现有工具说明包含“必须有订阅”和“只有排队可取消”等旧企业假设，主进程还有固定 100 积分/秒估算。切换公司适配时同步校正这些说明：Gateway 支持钱包/订阅扣费偏好，v3 取消按真实状态判断，准确报价当前缺失；不能继续向用户展示固定估价或承诺不可取消。对话工具参数和返回协议保持原约定。

## 消息渠道、模型适配、技能和 MCP 的边界

### 消息渠道

微信/钉钉/飞书/企业微信的连接、扫码和平台凭据继续走星途现有 IM 控制器与 channels 配置，Gateway 不代理它们全部消息流，也不将用户的微信/邮箱凭据当成模型 Key。

将来需要按套餐限制某个渠道，可登记独立功能（例如 im.weixin，当前尚未登记/实现），在设置入口、配置保存、建立连接和入站业务触发处接同一授权。仅隐藏导航不会断开已有连接；停用要安全停止接收新任务并保留凭据，不能卸载随包渠道。

远端 IM 发件人 ID 不是 Gateway user_id。若多个远端成员共用一个客户端，必须明确调用归属是当前安装的登录账号还是经过绑定的成员账号；没有映射时不能自动让任意发件人消耗当前用户钱包。该业务规则尚未确认，不能将“一个插件连接成功”验收为多用户计费接通。

### 模型 provider

平台配置当前锁到 Gateway /v1 的公司 provider，Key 和模型仍需正确关联。随包 provider 的注册/兼容代码保留，不因模型未对某用户开放就卸载 provider。云端能力以模型 Key、用户组、模型白名单和 Gateway 实际路由为准，客户端菜单不能扩大范围。

### 技能与 MCP

技能授权若以后纳入范围，应在技能发现/启用及其真正调用工具的边界设计；SKILL.md 本身不是可执行软件包的授权证据。MCP 当前经原生 mcp.servers 配置和运行管理，逐工具权限不能只改旧 mcp-bridge manifest。两者首期只保留现状和接缝，不因插件方案改写加载与工具协议。

## 发布包、升级和回滚如何落实

### 现有能力与缺口

| 项目 | 已确认的现有行为 | 必须补齐 |
| --- | --- | --- |
| 版本来源 | npm/clawhub 检查远端最新；update 不传固定版本 | 受控插件只接受 Gateway 决策和 immutable release |
| staging | 与目标同卷，CLI 安装、manifest ID 检查、rename | 发布前检查期望 runtime_id/version/hash 与当前账号决策 |
| 切换失败恢复 | rename 失败恢复有效旧目录 | 不能把它称为运行健康失败后的持久回滚 |
| 切换成功 | publisher 立即删除旧备份 | 保留到配置同步和运行健康验证完成，再按保留策略清理 |
| 本机存储 | addUserPlugin 更新时 enabled=true，config 未被 upsert 覆盖 | 托管流程保留原启停意愿/配置，不能靠安装默认扩大授权 |
| 兼容处理 | nsp-clawguard 有精确版本的专用适配 | 新包不得默认套同补丁；按包/版本核对，不伪报“适用所有插件” |
| 原生依赖 | Electron/Node、平台架构、host peer/junction 参与 | 发布前验证目标组合，不能把 macOS 包标成跨平台 |

### 发布产物契约补充（拟定）

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

### 安装到运行成功的阶段

download authorize → 下载固定包 → hash/大小/身份/格式检查 → staging 安装和依赖校验 → install authorize → 等待安全点并切换 → 记录本机版本 → 合并配置 → 必要的运行时重启 → 健康检查 → report succeeded → 清理旧代备份。

同一插件安装/升级串行，避免两次操作交错；每次 await 后检查账号代次和取消状态，特别是发布目录与写本机元数据前。中途换号保留可恢复操作记录，不把 A 的安装结果认作 B 的权益。

健康检查至少确认插件被发现、schema 通过、工具实际注册、桥接接通；纯注册成功不证明真实业务成功。模型计费测试应在明确测试请求下进行，不能把每次健康检查实现成付费生成。

### 回滚的两种情况

- 安装中失败：恢复当前操作保留的旧目录及对应本地元数据/配置版本，重新核对当前权限；不要只回退文件而保留数据库新版本号。
- 已运行版本出问题：Gateway 把唯一当前版本策略切换为 rollback，星途按授权取得指定安全包，确认配置/数据可逆后安装。现有 publisher 没有保留全部历史版本，不能假定本机一定有旧包。

旧文件中的绝对 symlink/junction 不跨机器复用，不覆盖当前运行时目录。运行健康失败时保留诊断和可恢复路径，先恢复可用版本；不可逆数据迁移必须另有恢复方案，不能以降级代码冒充数据恢复。

## 执行校验覆盖范围

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

## 两个平台的实施顺序与验收

| 阶段 | Gateway 交付 | 星途交付 | 完成条件 |
| --- | --- | --- | --- |
| A 现有媒体接通 | 使用已有 Key/模型/图片/视频/素材接口 | 公司媒体适配器，保留工具与账号隔离 | 图片同步、Wan、Seedance v3、取消/下载和消费回查各自闭环 |
| B 权限基础 | 目录、套餐、覆盖、本人权限、专用 Key 绑定 | 单一权限快照、按账号关联配置、IPC/业务守卫 | A/B 换号不串权，过期拒绝新执行，历史可用 |
| C 业务插件样例 | 先登记一个真实已审阅插件 | 作者包、主进程桥接、安装配置、真实 execute | manifest/工具/runtime ID 一致，拒绝时没有外部副作用 |
| D 发布控制 | 不可变包、兼容约束、统一版本/强制更新/全体回滚 | 精确安装、保留旧代、健康检查和结果上报 | 下载后撤销、断网/换号、切换失败与运行失败均能正确处理 |
| E 渠道和其他扩展 | 依业务规则补功能，不伪造现有能力 | 原有 IM/技能/MCP 接缝按确认范围落实 | 原平台行为保留，未知归属不误扣费 |

A 可先使用现有接口推进；B–D 需要两端开发，当前均未完成。消息渠道多用户归属、真实付费插件清单、目标发布平台和可逆迁移规则尚待实施时确认，不影响先完成现有媒体与权限基础设计。

| 必验场景 | 可观察结果 |
| --- | --- |
| 三种 ID 不同 | qqbot/openclaw-qqbot/qqbot 不混用，email 等别名不产生重复 entries |
| 默认或个人 hidden | 入口/搜索隐藏，IPC/工具/后台调用均不能新执行 |
| 用户只关闭管理页 | 已明确授权业务工具仍按其权限运行，核心确认工具保留 |
| 本机 desired_enabled=false | 后台授权或更新不能把用户手动关闭项重新打开 |
| 多账号同一设备 | 包可共享，配置/密钥/授权隔离，旧响应不回写 |
| batch-save 一项不合法 | 不留下前半批保存后仍返回整体失败的状态 |
| 旧目录或本地重装影子 ID | 不能绕过受控目录和版本门槛 |
| 切换成功但注册失败 | 不上报 succeeded，可恢复旧文件/元数据/兼容配置 |
| 图片返回同步结果 | 工具收到完成结果，不去查不存在的远程图片任务 |
| Wan 不支持通用取消 | 明确反馈，保留任务；不伪造 cancelled 或退款 |
| 官方 SDK/运行时升级 | 固定版本准入与实际执行重新验证，不能只看主应用版本 |

## 源码证据与官方资料

文件行号为此次调研快照，后续编辑会变化；以符号与行为重新定位。

| 仓库/文件 | 核对符号/位置 | 支持结论 |
| --- | --- | --- |
| 星途 package.json | openclaw.version/plugins | 版本和 18 项随包清单 |
| 星途 openclaw-extensions/*/openclaw.plugin.json | id/contracts/configSchema | 5 个本地扩展身份与注册声明 |
| 星途 src/main/preload.ts | plugins，约 1146 行 | renderer 到 IPC 的现有契约 |
| 星途 src/main/ipcHandlers/plugins/handlers.ts | registerPluginHandlers | 11 项 handler、批量保存及旧更新调用 |
| 星途 src/main/plugins/pluginManager.ts | listPlugins/installPlugin/getPluginConfigSchema/checkPluginUpdates | 隐藏、来源、安装、schema、latest 更新 |
| 星途 src/main/plugins/pluginInstallPublisher.ts | publishStagedPluginDirectory | 同卷切换、失败恢复、成功即清旧备份 |
| 星途 src/main/coworkStore.ts | listUserPlugins/addUserPlugin，约 3711 行 | plugin_id 全局存储及启停/配置行为 |
| 星途 src/main/libs/openclawConfigSync.ts | pluginEntries/trustedPluginAllow，约 2535–2688 行 | manifest ID、allow、渠道/核心状态和 load.paths |
| 星途 src/main/libs/openclawConfigImpact.ts | classifyPluginConfigChange | 四类插件变更均为 Restart |
| 星途 src/main/libs/openclawLocalExtensions.ts | findThirdPartyExtensionsDir/listAvailableOpenClawExtensionManifests | 用户和随包目录的区别 |
| 星途 src/main/libs/mcpBridgeServer.ts | MediaGenerationRequest/mediaCallbackUrl | 本机回调请求和监听边界 |
| 星途 src/main/main.ts | handleMediaGenerationCallback，约 6044 行 | 旧媒体协议、现有账号隔离与计费提示 |
| 星途 src/shared/auth/platformConfig.ts | lockPlatformConfig | 公司 provider 锁定到 Gateway /v1 |
| 星途 src/main/plugins/nspClawguardCompatibility.ts | NSP_CLAWGUARD/readSupportedNspClawguardPackage | 精确包/manifest 版本例外 |
| OpenClaw src/plugins/hook-before-tool-call-result.ts | PluginHookBeforeToolCallResult | 工具前置阻断能力及适用边界 |
| Gateway router/api-router.go / relay-router.go / video-router.go | 当前路由 | 现有服务接口与旧企业 API 的差异 |

官方资料：

- [固定 v2026.8.1 的插件 manifest](https://github.com/openclaw/openclaw/blob/v2026.8.1/docs/plugins/manifest.md)：核对身份、schema 和配置约束。
- [固定 v2026.8.1 的工具插件](https://github.com/openclaw/openclaw/blob/v2026.8.1/docs/plugins/tool-plugins.md)：核对工具扩展方式；本示例继续沿用项目实际 registerTool 接口。
- [官方 SDK 稳定性说明](https://docs.openclaw.ai/plugins/sdk-overview)：SDK 为实验契约，需要固定宿主并验证声明兼容范围。
- [官方安装说明](https://docs.openclaw.ai/cli/plugins/install)：安装来源和产物处理的参考；具体命令能力以项目固定版本为准。

本轮只完成调研与文档。没有以“已写接口方案”代替真实两端联调，没有读取用户私有插件配置、登录凭据或真实聊天记录。
