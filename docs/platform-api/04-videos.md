# 视频生成

视频接入方式取决于模型系列和已开通渠道。Wan 使用 v1 接口；Seedance 可使用 v1 兼容或火山原生 v3。v3 是协议路径版本，不代表 Wan 3.0 必须走 v3。以下示例中的模型、任务和素材 ID 均需替换成当前账号可用的实际值。

## 协议与请求路径

| 系列 / 操作 | 请求方式 | 路径 | 说明 |
| --- | --- | --- | --- |
| Wan 或 Seedance 兼容提交 | POST | `/v1/videos` | 兼容别名 `/v1/video/generations` |
| v1 查询 | GET | `/v1/videos/{task_id}` | 顶层视频对象、整数 progress |
| Seedance 原生提交 | POST | `/api/v3/contents/generations/tasks` | 需要对应的 Volc Native 渠道 |
| v3 查询 / 列表 | GET | `/api/v3/contents/generations/tasks/{id}` / 同路径不带 ID | Gateway 保存的任务快照及本人 v3 任务列表 |
| v3 取消 | DELETE | `/api/v3/contents/generations/tasks/{id}` | 以实际返回的任务终态为准 |
| 文件交付信息 | GET | `/v1/videos/{task_id}/content-info` | 按响应选择直连或网关代理下载 |

请求携带 `Authorization: Bearer YOUR_API_KEY`。保存提交响应里的 Gateway 任务 ID，后续查询使用该 ID。提交成功表示任务已受理，仍需轮询到终态；网络超时不要自动重复创建付费任务。

素材库可使用 REST 或 Action RPC，详见“素材库 API”。素材 ID 用于引用已注册媒体，上传与渠道同步完成后才能调用；素材仍受模型支持范围、审核结果及账号归属约束，不保证审核通过。

## 一、Seedance 原生 v3 流程

### 1. 提交视频生成任务 (CreateTask) · 增

使用 content 数组传入文本和媒体，具体的媒体角色、尺寸、时长、音频能力由所选 Seedance 模型与渠道决定。接口不会替客户端保证所有模型具备相同特性。

- **接口路径**：`POST /api/v3/contents/generations/tasks`
- **请求头**：
  - `Content-Type: application/json`
  - `Authorization: Bearer YOUR_API_KEY`（在控制台「令牌」页面创建并获取）

#### 请求参数说明

| 参数名 | 类型 | 必选 | 默认值 | 完整说明与业务建议 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | — | 模型标识：`doubao-seedance-2-0`、`doubao-seedance-2-0-fast-260128` 等 |
| `content` | array | 是 | — | 结构化内容列表，由文本提示词与图片对象数组构成（官方标准规范，无平铺限制） |
| `content[].type` | string | 是 | — | 内容类型：`"text"`（文本描述）或 `"image_url"`（图像来源） |
| `content[].text` | string | 条件 | — | 文本提示词，描述画面主体、动作动态、环境运镜、质感细节等（中英文均可） |
| `content[].image_url.url` | string | 条件 | — | 支持三种格式：<br>1. **素材库句柄**：`asset://{asset_id}`（需完成注册和所选渠道的同步）<br>2. **公网直链**：`https://...`（HTTP/HTTPS 直连直链）<br>3. **Base64 编码**：`data:image/jpeg;base64,{data}`（需符合模型和平台请求大小限制） |
| `content[].role` | string | 否 | 由模型决定 | 图片角色定位：<br>• `"first_frame"`：首帧图（从该图开始动态演化）<br>• `"last_frame"`：尾帧图（向该图演化收尾）<br>• `"reference_image"`：人像/风格特征参考图（保持多图一致性） |
| `resolution` | string | 否 | 由模型决定 | 按模型选择；当前 Seedance 2.5 接受 `480p`、`720p`、`1080p`，不能将 `4k` 视为通用选项 |
| `ratio` | string | 否 | 由模型决定 | 画面宽高比：`16:9`、`9:16`、`1:1`、`4:3`、`3:4`、21:9、`adaptive`（自适应首图） |
| `duration` | integer | 否 | 由模型决定 | 网关校验整数和通用时长边界；实际支持时长与 `-1` 智能时长能力由模型决定 |
| `generate_audio` | boolean | 否 | 由模型决定 | 是否生成音频；需要所选模型支持 |
| `watermark` | boolean | 否 | 由模型决定 | 是否请求水印；以所选模型支持范围为准 |

#### (1) 素材库句柄调用示例

```bash
curl --fail-with-body --request POST 'https://gateway.ai.shilijia.xyz/api/v3/contents/generations/tasks' \
  --header 'Authorization: Bearer YOUR_API_KEY' \
  --header 'Content-Type: application/json' \
  --data-raw '{
  "model": "doubao-seedance-2-0",
  "resolution": "720p",
  "ratio": "16:9",
  "duration": 5,
  "generate_audio": true,
  "watermark": false,
  "content": [
    {
      "type": "text",
      "text": "严格参考输入图片中的人物形象生成视频，保持人物外貌、脸型、发型和神态高度一致，自然微笑，动作流畅，在科技办公室中专注工作，电影级质感。"
    },
    {
      "type": "image_url",
      "role": "reference_image",
      "image_url": {
        "url": "asset://YOUR_ASSET_ID_1"
      }
    },
    {
      "type": "image_url",
      "role": "reference_image",
      "image_url": {
        "url": "asset://YOUR_ASSET_ID_2"
      }
    }
  ]
}'
```

提交响应示例（至少读取 id，其他字段不保证立即返回）：

```json
{
  "id": "task_EXAMPLE_GATEWAY_ID",
  "model": "doubao-seedance-2-0",
  "status": "queued",
  "created_at": 1790237730
}
```

保存返回的 `id`，用于后续查询渲染状态。

#### (2) 公网直链调用示例

适合已有稳定公网对象存储直链的场景：

```json
{
  "model": "doubao-seedance-2-0",
  "resolution": "720p",
  "ratio": "16:9",
  "duration": 5,
  "generate_audio": true,
  "content": [
    {
      "type": "text",
      "text": "古风人物在庭院中优雅挥袖，落叶飘落，高清画质。"
    },
    {
      "type": "image_url",
      "role": "first_frame",
      "image_url": {
        "url": "https://tos-cn-beijing.volces.com/assets/scene.jpg"
      }
    }
  ]
}
```

#### (3) 本地 Base64 直传调用示例

模型支持 Data URI 时可提交本地图片的 Base64 内容；文件大小仍受请求限制和上游限制约束：

```json
{
  "model": "doubao-seedance-2-0",
  "resolution": "720p",
  "ratio": "16:9",
  "duration": 5,
  "content": [
    {
      "type": "text",
      "text": "高科技发布会演讲，动作自然，写实风格。"
    },
    {
      "type": "image_url",
      "role": "first_frame",
      "image_url": {
        "url": "data:image/jpeg;base64,/9j/4AAQSkZJRg..."
      }
    }
  ]
}
```

---

### 2. 查询单个任务状态与渲染结果 (GetTask) · 查

- **接口路径**：`GET /api/v3/contents/generations/tasks/{id}`
- **请求头**：`Authorization: Bearer YOUR_API_KEY`

```bash
curl --fail-with-body --request GET 'https://gateway.ai.shilijia.xyz/api/v3/contents/generations/tasks/task_EXAMPLE_GATEWAY_ID' \
  --header 'Authorization: Bearer YOUR_API_KEY'
```

#### 成功响应结构 (HTTP 200 · status == "succeeded" 即渲染完成)

```json
{
  "id": "task_EXAMPLE_GATEWAY_ID",
  "model": "doubao-seedance-2-0",
  "status": "succeeded",
  "content": {
    "video_url": "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/doubao-seedance-2-0/video_xxx.mp4?X-Tos-Algorithm=...",
    "cover_url": "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/doubao-seedance-2-0/cover_xxx.jpg?X-Tos-Algorithm=..."
  },
  "usage": {
    "completion_tokens": 125,
    "total_tokens": 125
  },
  "created_at": 1790237730,
  "updated_at": 1790237765
}
```

#### 任务状态流转说明

| 状态值 (`status`) | 说明 | 业务下一步建议 |
| --- | --- | --- |
| `queued` | 排队中 | 建议间隔 3~5 秒继续轮询 |
| `running` | 渲染处理中 | 建议间隔 3~5 秒继续轮询 |
| `succeeded` | 渲染成功 | 提取 `content.video_url`，立即下载或流式转存 |
| `failed` | 任务失败 | 检查 `error` 字段说明，停止轮询 |
| `cancelled` / `expired` | 已取消 / 已过期 | 停止轮询，不展示为生成成功 |

---

### 3. 分页查询视频任务列表 (ListTasks) · 查

仅列出当前用户的 v3 任务，不会列出同一上游账号下其他用户或 Wan v1 的任务。

- **接口路径**：`GET /api/v3/contents/generations/tasks`
- **请求参数 (Query)**：
  - `page_num`：页码，从 1 开始，最大支持 500（默认 1）
  - `page_size`：每页条数，默认 10，最大支持 100
  - `filter.model`：按模型标识精确筛选（如 `doubao-seedance-2-0`）
  - `filter.status`：按状态筛选（`queued` / `running` / `succeeded` / `failed`）

```bash
curl --fail-with-body --request GET 'https://gateway.ai.shilijia.xyz/api/v3/contents/generations/tasks?page_num=1&page_size=10&filter.status=succeeded' \
  --header 'Authorization: Bearer YOUR_API_KEY'
```

响应示例：

```json
{
  "items": [
    {
      "id": "task_EXAMPLE_GATEWAY_ID",
      "model": "doubao-seedance-2-0",
      "status": "succeeded",
      "content": {
        "video_url": "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/doubao-seedance-2-0/video_xxx.mp4?X-Tos-..."
      },
      "created_at": 1790237730
    }
  ],
  "total": 1
}
```

---

### 4. 取消/终止视频任务 (CancelTask) · 删

用于请求取消尚未结束的 v3 任务。已结束任务直接返回其当前状态；取消与完成并发时，以持久化后的返回状态为准。不能仅凭请求返回 HTTP 200 就预先确认取消或退款。

- **接口路径**：`DELETE /api/v3/contents/generations/tasks/{id}`
- **请求头**：`Authorization: Bearer YOUR_API_KEY`

```bash
curl --fail-with-body --request DELETE 'https://gateway.ai.shilijia.xyz/api/v3/contents/generations/tasks/task_EXAMPLE_GATEWAY_ID' \
  --header 'Authorization: Bearer YOUR_API_KEY'
```

响应示例 (HTTP 200)：

```json
{
  "id": "task_EXAMPLE_GATEWAY_ID",
  "model": "doubao-seedance-2-0",
  "status": "cancelled"
}
```

---

## 二、视频下载与保存

先取 `/v1/videos/{task_id}/content-info`。当 data.delivery_mode 为 redirect 时读取 data.url，下载时不携带 Gateway 的 Authorization；为 proxy 时携带模型 Key 请求 `/v1/videos/{task_id}/content`。视频地址来自具体渠道或平台存储，不保证总是某一家 CDN，也不保证统一有效期、速度或断点续传能力。

需要长期保存的文件应及时转存至自己的存储；临时地址过期时重新查询任务或交付信息。下载示例：

```bash
curl --fail-with-body -L -o 'video.mp4' '<VIDEO_URL>'
```

## 三、v1 视频协议

Wan 使用该入口；Seedance 也可使用兼容请求。需要保留 Seedance 原生 content 数组及模型原生字段时，选择前述 v3。两种请求的字段结构和状态格式应分别处理。

### 1. 网关兼容视频生成接口 (POST /v1/video/generations)

支持将素材库 `asset://` 句柄置于 `metadata.content` 列表中传入：

```bash
curl --fail-with-body --request POST 'https://gateway.ai.shilijia.xyz/v1/video/generations' \
  --header 'Authorization: Bearer YOUR_API_KEY' \
  --header 'Content-Type: application/json' \
  --data-raw '{
  "model": "doubao-seedance-2-0",
  "prompt": "商务男士在明亮办公室内演讲，动作自然生动，电影级画质",
  "metadata": {
    "duration": 5,
    "ratio": "16:9",
    "resolution": "720p",
    "generate_audio": false,
    "watermark": false,
    "content": [
      {
        "type": "text",
        "text": "商务男士在明亮办公室内演讲，动作自然生动，电影级画质"
      },
      {
        "type": "image_url",
        "role": "reference_image",
        "image_url": {
          "url": "asset://YOUR_ASSET_ID"
        }
      }
    ]
  }
}'
```

提交成功响应：

```json
{
  "id": "task_TjyIFnCBCqj4UgdIkpP7mHFnZDbqZkx4",
  "task_id": "task_TjyIFnCBCqj4UgdIkpP7mHFnZDbqZkx4",
  "object": "video",
  "model": "doubao-seedance-2-0",
  "status": "queued",
  "progress": 0
}
```

#### 阿里百炼 Wan3.0 请求参数

以下兼容适用于百炼渠道的 `wan3.0-video`、`wan3.0-video-prime`，包括映射到这些模型的别名。不同百炼渠道均适用；其他渠道类型和百炼旧模型继续使用原有规范。

现有请求无需调整。接口仍为 `POST /v1/video/generations`（也支持 `/v1/videos`），请求头使用 `Content-Type: application/json` 和 `Authorization: Bearer YOUR_API_KEY`。

```json
{
  "model": "wan3.0-video",
  "prompt": "金毛小狗在阳光明媚的草地上奔跑，电影级画面",
  "image": "https://example.com/sample.png",
  "audio": false,
  "resolution": "720P",
  "ratio": "16:9",
  "duration": 5,
  "prompt_extend": true,
  "watermark": false,
  "seed": 0
}
```

将图片地址替换为实际地址；文生视频可省略 `image`。官方 `input/parameters` 和已有的 `metadata.input/metadata.parameters` 写法同样支持，无需迁移参数位置。响应和任务查询继续使用网关视频格式。

| 参数 | 支持范围 |
| --- | --- |
| `audio` | 布尔值；兼容字符串 `"true"` / `"false"`，`false` 请求关闭音轨 |
| `resolution` | `480P`、`720P`、`1080P`，兼容小写；网关默认 `720P` |
| `ratio` | `adaptive`、`21:9`、`16:9`、`4:3`、`1:1`、`3:4`、`9:16` |
| `size` | 原有分辨率或预设像素尺寸，如 `720P`、`1280*720`、`1280x720`，服务端转换为官方字段 |
| `duration` | 2～30 秒，默认 5 秒；暂不支持 `-1` 智能时长，参考视频还需符合上游总时长限制 |
| `prompt_extend` / `watermark` | 布尔值，兼容字符串布尔值；智能改写默认开启 |
| `seed` | `-1` 或 `0`～`2147483647`；不指定时由上游随机生成 |

明确传入的 `false`、`0` 会保留。重复参数按 `metadata.parameters > parameters > 顶层字段` 逐字段合并；显式 `resolution/ratio` 优先于从 `size` 推导的值。`metadata.input` 的同名字段优先于顶层 `input`；明确给出的 `input.media` 优先于从旧图片字段生成的媒体列表。建议同一参数只写一次。

Wan3 渠道的参数覆盖在请求格式转换和参数校验前执行。条件可用 `original_model` 匹配用户提交的模型名，例如精确匹配 `wan3.0-video-480p` 后设置顶层 `size` 为 `480p`，随后转换为上游 `parameters.resolution: "480P"`。模型映射独立负责把公开模型名替换成上游模型名，模型后缀不会自动决定分辨率。`mode: "full"` 要求名称完全一致。

参数覆盖针对用户请求结构执行，仍遵循上述字段优先级：覆盖顶层 `size` 不会压过请求中显式的 `resolution`；需要强制覆盖时，应覆盖实际使用的 `parameters.resolution` 或 `metadata.parameters.resolution`。覆盖后的时长会重新校验，并用于计费估算和上游请求。原始请求快照保留用户提交的内容。

百炼旧模型（例如 Wan2.5）关闭音频仍使用 `metadata.parameters.audio: false`，类型及支持范围保持原有适配器行为。媒体类型和素材限制见[Wan3.0 官方文档](https://help.aliyun.com/zh/model-studio/wan3-video-generation-api-reference)。

### 2. 查询 v1 视频任务

优先使用 `GET /v1/videos/{task_id}`：

```bash
curl --fail-with-body 'https://gateway.ai.shilijia.xyz/v1/videos/<TASK_ID>' \
  --header 'Authorization: Bearer YOUR_API_KEY'
```

完成时的核心字段示例：

```json
{
  "id": "<TASK_ID>",
  "object": "video",
  "model": "<MODEL_ID>",
  "status": "completed",
  "progress": 100,
  "metadata": {"url":"https://files.example.com/result.mp4"}
}
```

状态为 queued / in_progress / completed / failed / unknown；progress 为整数。失败读取 error.code/message。旧 `GET /v1/video/generations/{task_id}` 仍可使用，但返回 code/data 且各适配器字段存在差异，不能套用上面的顶层解析。当前没有通用的 v1 DELETE 取消入口。
