# 媒体任务与素材审核 Webhook

Webhook 将异步任务状态推送到外部程序，用于更新业务状态、触发后续队列或提示用户。支持个人订阅和系统全量通知；素材审核和媒体生成共用端点管理、数据库事件队列与后台投递器。

## 配置

1. 管理员开启媒体任务 Webhook 后，打开 **任务日志 → 钩子配置**，填写名称和公网 HTTPS 回调地址。此入口固定订阅 `task.status_changed`，无需选择事件。
2. 管理员开启素材库 Webhook 后，在 **素材库 → 钩子配置** 配置素材回调地址，选择素材可用或审核失败事件。
3. 点击端点的测试按钮，立即发出测试请求。页面显示是否成功、HTTP 状态码和耗时；接收端返回 HTTP `2xx` 表示成功。

两个入口分别只展示本类别的端点和事件。已有配置保留，关闭类别时暂停对应通知，不会清除端点。旧版本创建的混合订阅在两个入口均可看到；编辑名称或地址会影响两类通知，页面会提示。编辑时保留仍启用的另一类别订阅，若另一类别已关闭，保存前提示将移除该订阅。删除混合端点会停止两类通知，页面会明确提示。每个账号最多 5 个有效端点，两类合计；管理员在“全部”日志视图配置的个人端点仍只订阅自己的任务。

### 系统全量通知

系统管理员（Root）打开 **系统设置 → 运维 → 系统钩子配置**，分别配置：

- **素材库 Webhook**：类别总开关，控制用户配置和素材通知。可选系统地址接收所有账号的素材可用、审核失败事件。
- **媒体任务 Webhook**：类别总开关，控制用户配置和媒体任务通知。可选系统地址接收所有账号的任务状态变化，包含任务 ID、状态和响应快照。

两个类别默认关闭。开启并保存后，用户才能新增、编辑和测试相应类别的端点，且新事件进入通知队列。管理员可不填系统地址，仅允许个人订阅；填写有效系统地址后，系统额外接收所有账号的事件，个人未配置端点时也生效。同一事件可分别发送到个人端点与系统端点，不重复占用个人端点额度。

管理页面需先打开对应草稿开关，才能编辑或测试地址；Root 可在保存之前诊断地址，用户权限以已保存的开关为准。地址可为空，非空时必须符合公网 HTTPS 格式。任一配置无效，全部配置均不写入。关闭类别会拒绝个人端点的新增、编辑和测试（HTTP `403`），停止该类别所有个人及系统通知、新事件入队与后续重试；已有配置仍可读取或删除，已经发出的请求无法撤回。重新开启后不回放关闭期间的事件。

### PDF 接口手册

PDF 生成与下载默认关闭。系统管理员（Root）在 **系统设置 → 运维 → 系统钩子配置 → 钩子接口文档** 中开启下载开关并保存后，素材库和媒体任务的 **钩子配置** 分别显示本类别的文档下载按钮；系统配置页面同时提供两类下载。关闭并保存后，后端立即拒绝新的下载请求（HTTP `403`），用户重新打开配置窗口后隐藏入口；该开关不影响通知发送和连通性测试。

两类通知均关闭时，手册开关开启仍保留用户入口，可查看和下载文档；页面不提供端点新增、编辑或测试功能。

下载时必须指定 `scope=asset_library` 或 `scope=media_tasks`；缺失或无效返回 HTTP `400`。两份 PDF 按类别动态生成：素材库手册仅解释 `asset.active`、`asset.failed`，媒体任务手册仅解释 `task.status_changed`。客户手册只包含回调方式、请求头、事件信封、业务字段、JSON 示例、枚举、接收端响应和重试约定；端点管理与连通性测试 API 留在本开发文档中。JSON 示例均为格式化且可解析的对象；字段表解释类型、含义、可选性，并单列状态和原因枚举。PDF 使用系统文档相同的内嵌中文字体、A4 信笺、颜色、页码和品牌来源：平台名称、平台 Logo、运营主体名称与 Logo、可见页脚。管理员修改品牌设置后重新下载即生效。手册不包含实际配置地址或客户数据。

开发维护人员也可使用本地命令生成独立手册，不需要数据库或 Docker；本地命令不经过面向用户的下载开关：

```bash
go run ./cmd/webhook-manual -scope asset_library
go run ./cmd/webhook-manual -scope media_tasks
```

默认分别输出 `output/pdf/asset_library-webhook-api-manual.pdf` 与 `output/pdf/media_tasks-webhook-api-manual.pdf`。命令也提供 `-output`、`-platform-name`、`-platform-logo`、`-operating-name`、`-operating-logo` 和 `-footer` 参数；页面下载自动使用当前系统品牌配置。

## 事件范围

| 事件 | 触发条件 |
| --- | --- |
| `task.status_changed` | 异步视频、Suno 音乐等主任务模型首次保存，以及后续持久化状态变化，包括成功、失败、超时及取消。 |
| `asset.active` | 素材首次确认可用。 |
| `asset.failed` | 素材被上游审核明确拒绝。 |
| `webhook.test` | 用户手动测试端点。 |

任务状态使用平台统一值：`NOT_START`、`SUBMITTED`、`QUEUED`、`IN_PROGRESS`、`SUCCESS`、`FAILURE`、`UNKNOWN`。只更新进度或响应体而状态相同时不会重复生成事件。订阅生效后产生新事件，不回放历史任务。

当前覆盖“任务日志”对应的主异步任务模型；Midjourney 的独立绘图任务模型及同步返回的图片生成接口不产生 `task.status_changed`。

## 请求格式

平台发送 HTTPS `POST`，`Content-Type: application/json`。示例：

```json
{
  "id": "evt_example",
  "object": "event",
  "type": "task.status_changed",
  "created_at": 1791264000,
  "owner_user_id": 7,
  "data": {
    "task_id": "task_public_example",
    "platform": "54",
    "status": "SUCCESS",
    "state_version": 3,
    "progress": "100%",
    "response": {
      "status": "succeeded",
      "content": {
        "video_url": "https://media.example/result.mp4"
      }
    },
    "response_sha256": "<响应内容摘要>"
  }
}
```

- `data.task_id` 是客户端查询使用的公开任务 ID；不会附带渠道密钥、内部计费信息或任务私有字段。
- `owner_user_id` 是事件所属用户 ID；系统接收端可据此将不同账号的事件路由到外部队列。系统测试事件不带此字段。
- `data.platform` 沿用任务模型的平台标识：视频渠道通常为渠道类型的数字字符串（如 DoubaoVideo 为 `54`），Suno 为 `suno`。
- `data.response` 是当前已保存的任务响应快照，各供应商结构可能不同；失败事件可包含 `fail_reason`。暂无响应时为 `null`。
- 响应体采用现有任务轮询记录的大小边界：Base64 内容和超过 4 KiB 的字符串替换为省略提示，设置 `response_truncated`；处理后仍超过 64 KiB，或原始数据超过 8 MiB 时，响应为 `null` 并设置 `response_omitted`。提供 `response_sha256` 便于关联快照。完整结果和最新状态通过对应的任务查询接口获取。
- 每个任务的 `state_version` 在状态变化时递增。历史任务从首次变更开始生成版本；新任务从 1 开始。
- 请求头 `webhook-id` 为该端点投递的稳定 `wh_...` ID，重试保持不变；`webhook-timestamp` 为本次发送时间。同一事件发往多个端点时共享 `evt_...`，但投递 ID 不同。

## 接收与重试

接收端应先将事件可靠保存到自己的队列，再尽快返回 `2xx`。单次请求最长 10 秒；网络异常及非 `2xx` 响应按指数退避重试，最长 72 小时，退避间隔上限 12 小时。重试复用原始事件响应快照，不读取任务的后续响应覆盖它。

这是**至少一次投递**，不保证跨事件到达顺序。接收端按 `webhook-id` 去重，并按任务 ID 原子保存最大 `state_version`，防止迟到的处理状态覆盖成功或失败状态。事件保存成功但 HTTP 响应丢失，或发送后平台数据库写回失败，都可能重试。

删除端点后停止后续发送，已发出的请求无法撤回。编辑地址后，尚未发送和重试的事件使用新地址；修改订阅只影响后续生成的事件。延续已有素材 webhook 的直接 JSON 投递协议，不附带签名密钥；需要核实业务状态时使用任务查询 API。

## 管理 API

使用平台账号的管理 API 访问凭证（`Authorization: Bearer <access_token>`），权限范围与页面一致。模型调用令牌不用于配置端点。

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/api/webhook-endpoints` | 列出本账号的有效端点。 |
| `POST` | `/api/webhook-endpoints` | 新建端点。 |
| `PUT` | `/api/webhook-endpoints/{id}` | 编辑名称、地址和订阅。 |
| `DELETE` | `/api/webhook-endpoints/{id}` | 禁用端点。 |
| `POST` | `/api/webhook-endpoints/{id}/test` | 无请求体；立即发送测试并返回连通结果。 |
| `GET` | `/api/webhooks/system` | Root 读取系统类别配置。 |
| `PUT` | `/api/webhooks/system` | Root 原子保存两类通知配置和手册开关。 |
| `POST` | `/api/webhooks/system/{topic}/test` | Root 测试指定地址；`topic` 为 `asset_library` 或 `media_tasks`。 |
| `GET` | `/api/webhooks/capabilities` | 已登录用户读取 `data.asset_library_enabled`、`data.media_tasks_enabled` 和 `data.manual_enabled`（均为 boolean），不返回系统地址。 |
| `GET` | `/api/webhooks/manual.pdf?scope=asset_library` 或 `?scope=media_tasks` | 已登录用户在手册开关开启时下载对应类别当前品牌的 PDF；未指定有效类别返回 `400`，关闭返回 `403`。 |

新建和编辑请求体：

```json
{
  "name": "外部任务队列",
  "url": "https://customer.example/webhooks/media",
  "event_types": [
    "task.status_changed"
  ]
}
```

管理 API 仍接受旧版本的混合订阅，以兼容已有端点；新建页面按素材与媒体任务分开配置。

原 `/api/asset-library/webhook-endpoints` 路径及其子路径继续提供兼容接口，该路径的测试投递继续使用 `asset.webhook_test` 事件类型。

系统配置 PUT 请求体和 GET/PUT 的 `data`（两个类别对象和 `manual_enabled` 均必填，不接受部分 PUT；`manual_enabled` 为 boolean，默认 false）：

```json
{
  "asset_library": {
    "enabled": true,
    "url": "https://operator.example/hooks/assets"
  },
  "media_tasks": {
    "enabled": true,
    "url": "https://operator.example/hooks/tasks"
  },
  "manual_enabled": true
}
```

系统测试请求体：

```json
{
  "url": "https://operator.example/hooks/tasks"
}
```

测试接口立即发送 HTTPS `POST` 的 `webhook.test` 事件，返回示例：

```json
{
  "success": true,
  "data": {
    "reachable": true,
    "http_status": 204,
    "duration_ms": 18,
    "event_id": "evt_test_example",
    "webhook_id": "wh_test_example"
  }
}
```

| 测试出参 | 类型 | 说明 |
| --- | --- | --- |
| `reachable` | boolean | 收到 HTTP `2xx` 且限量读取响应未出现错误时为 `true`。 |
| `http_status` | integer | 回调 HTTP 状态；未取得响应时为 `0`。 |
| `duration_ms` | integer | 测试耗时，毫秒。 |
| `event_id` | string | 测试事件 ID，可关联接收端日志。 |
| `webhook_id` | string | 本次投递 ID，同时出现在请求头。 |
| `error` | string，可选 | 失败时的说明，不回显 URL 凭据或接收端响应体。 |

外层 `success: true` 表示管理请求执行成功，必须继续检查 `data.reachable`。非 `2xx`、网络异常或超时返回 `reachable: false`。测试最多等待 10 秒，不写入持久投递队列，不自动重试；直接请求不跟随重定向。

## 实现与运维

- 生产者在任务或素材状态事务中写入不可变投递记录；队列失败时状态事务回滚，外部 HTTP 在事务提交后由独立 worker 执行。
- 共用现有 `asset_webhook_endpoints` / `asset_webhook_deliveries` 表，避免迁移丢失旧配置和队列；任务表新增内部 `webhook_version` 字段，由现有自动迁移完成。
- `asset_webhook_deliveries` 位于主数据库，既是待发送队列，也是投递记录；包含事件与投递 ID、端点、事件类型、请求快照、状态、尝试次数、回调 HTTP 状态、最后错误和时间戳。它不写入独立的日志数据库。测试接口直接发送，不写入这张投递表；目前没有投递历史页面或查询 API。
- 系统配置持久保存在同一端点表，使用系统归属的两条固定端点，用户管理接口不可读取或编辑。系统与个人投递使用同一套租约、重试和清理规则。
- 每个节点运行 4 个独立投递循环，通过数据库条件更新竞争租约；一分钟租约，发送前复核所有权，数据库查询和 HTTP 共用租约截止时间。复用了 [PR #48](https://github.com/yeruyi1024/novamaas-workspace/pull/48) 的过期租约防护。
- 素材的旧激活事件取代规则继续保留；媒体任务事件通过状态版本处理顺序，不丢弃已产生的中间状态事件。
- 主节点每小时清理终态投递记录：成功/已取代保留 30 天，最终失败保留 90 天。待发送、处理中和可重试记录不参与清理。
- 复用现有 SSRF 防护和 worker 出站设置；直接 HTTP 投递不跟随重定向。投递失败及写回失败记录在服务日志中。

素材审核的详细语义见 [素材上游审核拒绝与重传](design/ASSET_CONTENT_REJECTION.zh_CN.md)。
