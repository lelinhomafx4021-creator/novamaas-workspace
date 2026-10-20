# 视频模型对接

管理目标：通过公司平台提供视频任务能力，星途负责提交、状态跟踪、恢复查询及成果保存。平台已有基础任务能力，星途旧媒体提交、查询及取消流程仍待适配，完整链路尚未验收。

## 一、范围与职责

先确定实际入口、模型及调用协议。兼容视频链路与 Seedance 原生链路分别核对任务响应、查询及取消规则，不能混用状态和标识。常规媒体入口还须核对当前产品配置，具体关系见[图片模型对接](/admin-manual?article=image-models)。

平台准备模型、渠道、用户密钥、素材条件及账务依据；星途适配请求、任务记录、恢复和成果关联。旧固定积分提示须随适配更新，以平台实际价格和消费为准。

## 二、任务生命周期

| 阶段 | 交付要求 |
| --- | --- |
| 素材准备 | 确认参考素材已经上传、可访问且满足模型引用条件 |
| 任务提交 | 使用选定模型及协议，保存平台公开任务标识 |
| 状态查询 | 按原环境、账号和协议查询，记录真实状态及错误 |
| 断网与重启恢复 | 根据已保存任务恢复查询，避免重复生成 |
| 任务取消 | 按链路能力和当前状态执行，以最终结果更新展示 |
| 下载与归档 | 保存实际视频文件及本地位置，关联原项目或会话 |
| 费用核查 | 依据最终任务状态及平台记录核对消费，退款单独确认 |

任务记录保存模型、协议、公开任务标识、当前状态、错误和本地成果位置。模型密钥沿用受保护配置，使用归属关联，不写入任务正文或回报。

## 三、异常与账务处理

提交超时且结果不明时先回查实际状态。取消与完成并发时以最终任务结果为准；发起取消不代表退款完成。素材准备、生成任务及成果下载分别记录，媒体任务回调与套餐、插件权益同步分别对接。

远端链接需核对有效期及访问条件；长期成果保存实际文件，避免历史项目只保留已失效地址。

## 四、已有接口与字段

### 4.1 路径及协议选择

下列写操作使用用户模型 API Key；查询可复用已支持的模型密钥或本人登录会话，但始终核对任务归属。本期适配优先固定一种协议完成闭环。

| 操作 | 路径 | 请求 / 响应 |
| --- | --- | --- |
| v1 提交 | POST /v1/videos | JSON，兼容别名 /v1/video/generations |
| v1 查询 | GET /v1/videos/:task_id | 顶层视频对象；旧别名查询的封装可能不同 |
| v3 提交 | POST /api/v3/contents/generations/tasks | Volc Native 原生 JSON |
| v3 查询 | GET /api/v3/contents/generations/tasks/:task_id | 本人原生任务快照 |
| v3 列表 | GET /api/v3/contents/generations/tasks | 本人已保存的 v3 任务，不枚举上游共享账号任务 |
| v3 取消 | DELETE /api/v3/contents/generations/tasks/:task_id | 以返回实际终态为准 |
| 文件交付 | GET /v1/videos/:task_id/content-info | 按 data.delivery_mode 选择 redirect 或 proxy |
| 代理下载 | GET /v1/videos/:task_id/content | 本平台鉴权；不向外部 CDN 转发平台 Bearer |

### 4.2 提交字段

| v1 请求字段 | 类型 | 必填 / 规则 |
| --- | --- | --- |
| `model` | string | 必填，实际公开模型 ID |
| `prompt` | string | 文本生成必填；按模型说明处理其他输入 |
| `images` | string[] | 图片参考场景条件使用，URL / Data URI 等须模型支持 |
| `seconds` | string | 可选，时长秒数字符串，如 "5"；客户端按实际模型范围选择，网关通用上限 3600 秒 |
| `size` | string | 可选，按模型支持 |
| `metadata` | object | 可选，具体链路的高级参数；Seedance 兼容路径用 metadata.resolution、ratio、generate_audio、watermark 等 |

Wan 3.0 另有 input / parameters 适配，不能直接复用 Seedance metadata 字段。Seedance v1 时长按 seconds 或适配器的 metadata.duration 处理，不能只发一个未经核对的顶层 duration。原始任务标识仅使用平台返回的 id。

| Seedance v3 请求字段 | 类型 | 必填 / 规则 |
| --- | --- | --- |
| `model` | string | 必填，已开通 Volc Native 的公开模型 ID |
| `content` | object[] | 必填且非空，按选定模型排列文本及媒体 |
| `content[].type` | string | 必填，text / image_url，其他媒体按模型适配 |
| `content[].text` | string | type=text 时必填 |
| `content[].image_url.url` | string | type=image_url 时必填；模型支持的 URL、Data URI 或 asset:// 素材引用 |
| `content[].role` | string | 媒体角色按模型支持，如 first_frame / last_frame / reference_image |
| `duration` | integer | 可选，网关接受 -1 至 3600 的整数，实际模型允许范围更窄；-1 仅模型支持时表示智能时长 |
| `resolution` | string | 可选；当前 Seedance 2.5 适配验证 480p / 720p / 1080p，其他模型分别确认 |
| `ratio` | string | 可选，如 16:9 / 9:16 / 1:1 / adaptive，按模型支持 |
| `generate_audio` / `watermark` | boolean | 可选，显式 false 原样保留 |
| `seed` | integer | 可选，按模型支持范围，0 不自动视为未传 |

原生请求示例：

```json
{
  "model": "<实际原生视频模型ID>",
  "content": [{"type":"text","text":"办公室窗边的绿植随微风轻动"}],
  "duration": 5, "resolution": "720p", "ratio": "16:9", "watermark": false
}
```

### 4.3 响应、任务状态及取消

| 字段 / 协议 | 类型 | 接收规则 |
| --- | --- | --- |
| 提交 `id` | string | 立即保存平台公开任务 ID；其他字段不保证提交时全部返回 |
| v1 `object` / `model` / `status` | string | 视频对象类型、公开模型、当前状态 |
| v1 `progress` | integer | 当前进度，不能将 100 独立当作任务成功 |
| v1 `created_at` / `completed_at` / `expires_at` | integer | Unix 秒，完成和到期字段可能省略 |
| v1 `error.code` / `error.message` | string | 失败时解析，不读成订阅错误封装 |
| v3 `model` / `status` | string | 原生任务模型与状态 |
| v3 `content.video_url` | string | 成功时的视频来源；先核对可用结果再归档 |
| v3 `error` / `usage` | object | 有返回时保留并按原生协议解析 |
| v3 列表 `items` / `total` | object[] / integer | 本人任务快照及筛选后数量 |

v3 列表 Query：page_num 为 integer，默认 1、最大 500；page_size 为 integer，默认 10、最大 100；filter.model、filter.status、filter.service_tier 为可选 string 精确筛选项。

| 协议 | 处理中 | 终态 |
| --- | --- | --- |
| v1 视频对象 | queued / in_progress | completed / failed；unknown 回查 |
| v3 原生对象 | queued / running | succeeded / failed / cancelled / expired |
| 平台任务日志 | NOT_START / SUBMITTED / QUEUED / IN_PROGRESS | SUCCESS / FAILURE；UNKNOWN 回查 |

取消成功可返回 `{"id":"<平台任务ID>","status":"cancelled"}`，不保证合成 error 对象；若完成抢先则可能返回 succeeded。客户端保留原协议状态，不在点击取消后直接显示退款到账。

文件交付信息使用 success/message/data 封装：data.delivery_mode:string 为 redirect 时使用 data.url:string；为 proxy 时带本平台凭据读取 content 路径，不能无鉴权取包或把 Bearer 发给结果存储域。原生素材注册、上传、Action 签名与媒体 Webhook 字段已有单独说明，见[视频 API](/docs?article=videos)、[素材库 API](/docs?article=assets)及[素材回调](/docs?article=asset-webhooks)；按选定入口接入，不与插件配置或权益回报混用。

## 五、验收标准

每条拟开放链路分别验证实际入口、素材引用、提交、状态查询、断网与重启恢复、成功及失败终态、下载保存、历史重新打开和费用归属。支持取消的链路补验取消与完成竞争场景。

平台任务能力、客户端适配和完整桌面验收分别记录，本版不标记真实生成、取消或退款已通过。统一交付记录见[管理总览](/admin-manual?article=account-login)。
