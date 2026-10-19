# 视频接口对接

版本：2.0 收口稿，2026-10-19。现有任务、素材及 Webhook 入口按实际协议接入；远程插件权限与统一发布仍待开发。参考 PDF 差异表保留历史核对结果，本次原路径未取得 PDF 原件，未新增逐页验证结论。

视频按 Wan 与 Seedance 两类接入，Seedance 再区分 v1 兼容和 v3 原生协议。使用当前用户的模型 Key，按提交、保存任务 ID、查询、下载的顺序完成调用。

本章同时补齐参考 PDF 涉及的素材组/素材 CRUD、AK/SK 签名、视频任务生命周期和素材事件。当前源码为可调用能力的依据，PDF 的写法用于参考详细程度；其中未实现或已变化的内容在末节逐项列出。

## 现有接口总表

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

## 协议选择

| 模型系列 | 协议 | 请求字段 |
| --- | --- | --- |
| Wan，含 Wan 3.0 | v1 | prompt/image(s)；Wan 3.0 另支持 input/parameters |
| Seedance | v1 兼容 | prompt、images、seconds、metadata |
| Seedance | v3 原生 | content[]、duration、resolution 等原生字段 |

v1 提交 `POST /v1/videos`，兼容 `POST /v1/video/generations`；优先查询 `GET /v1/videos/<TASK_ID>`。v3 提交 `POST /api/v3/contents/generations/tasks`，查询同一路径加 `/<TASK_ID>`。

Wan 3.0 是模型版本，仍使用 v1；火山 v3 路径用于 Volc Native 的 Seedance。模型 ID 使用已开通的公开名称。保存 `{用户ID, 模型ID, 协议, 响应id}`，后续按同一协议查询；id 是 Gateway 公开任务 ID。

## Wan：v1

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

## Seedance：v3 原生与 v1 兼容

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

### Seedance 原生字段表

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

## 查询、取消与下载

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

### v3 列表字段、取消响应和状态流转

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

## 参考素材上传与使用

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

### REST 素材接口补全

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

## 素材 AK/SK 取得与签名（现有）

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

### 签名请求组成

统一入口 `POST /?Action=<ACTION>&Version=2024-01-01`，亦有 `/api/v3/` 别名；签名必须使用真正发送的路径。Region=cn-beijing、Service=ark，X-Date 为 UTC `YYYYMMDDTHHMMSSZ`，容许与服务端约 5 分钟偏差。

```http
Content-Type: application/json
X-Date: <UTC_TIMESTAMP>
X-Content-Sha256: <SHA256_OF_EXACT_UTF8_BODY>
Authorization: HMAC-SHA256 Credential=<AK>/<YYYYMMDD>/cn-beijing/ark/request, SignedHeaders=content-type;host;x-content-sha256;x-date, Signature=<SIGNATURE>
```

body 只序列化一次，签名和发送使用同一批 UTF-8 字节。Host 含实际域名及必要端口，不能写死 PDF 的域名；代理不得修改签名所用 Host/path/query。规范请求按 method、URI、排序编码后的 query、规范头、SignedHeaders、body hash 以换行连接。

## 素材组与素材 Action 全生命周期（现有）

以下每项方法都是 POST、Version 都是 2024-01-01，使用上述 AK/SK，不使用 Bearer。成功信封：

```json
{"ResponseMetadata":{"RequestId":"<REQUEST_ID>","Action":"CreateAssetGroup","Version":"2024-01-01","Service":"ark","Region":"cn-beijing"},"Result":{"Id":"<ASSET_GROUP_ID>"}}
```

### 素材组五项 Action

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

### 素材五项 Action

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

### Action 错误信封

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

## 素材与任务 Webhook（现有 REST，非权益推送）

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

### 素材事件与乱序处理

Gateway POST 到接收地址，携带 Content-Type=application/json、User-Agent=New-API/1.0、webhook-id、webhook-timestamp；当前没有独立事件签名密钥。需要权威确认时用当前用户的 AK/SK GetAsset 或 REST 列表回查，不能信任任意客户端自报的回调内容。

```json
{"id":"evt-example","object":"event","type":"asset.failed","created_at":1800000000,"owner_user_id":123,"data":{"id":"<ASSET_ID>","group_id":"<ASSET_GROUP_ID>","status":"Failed","failure_reason":"policy_rejected","updated_at":1800000000,"state_version":2}}
```

asset.active 对应 Active、state_version=1；asset.failed 对应 Failed、版本=2。接收端先持久化，再回 2xx，异步更新业务；webhook-id 去重，同素材只接受严格更高 state_version。收到 2 后不被迟到的 1 覆盖；无版本旧事件先回查。失败素材阻止新引用，但不删除已完成生成成果。

task.status_changed 使用任务状态枚举和单调 state_version，响应快照可能省略/截断，收到后可按 task_id 读取权威详情。不能按素材固定 1/2 解释任务版本。

当前请求超时约 10 秒，失败退避重试，保留至 72 小时，单间隔上限 12 小时；同端点重试 webhook-id 和 body 不变，timestamp 可变化。没有承诺严格有序、恰好一次或全部历史回放。媒体/素材通知不包含钱包、订阅、插件权限或版本发布事件，这些另按第三章回查。

## AK/SK 到视频结果的 Python 示例（现有协议）

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

## 旧媒体链路替换、任务恢复与验收

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

## 参考 PDF 与当前实现差异

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

任务历史、提交超时和消费回查统一见[账号与钱包](/admin-manual?article=account-login#消费回查与请求结果)。当前没有公开的用户参数报价接口，不能固定按 100 积分/秒估价。

源码依据：`router/video-router.go`、`relay/channel/task/{ali,doubao,volcnative}/`、`controller/{volc_native,video_proxy,task,asset_library}.go`、`service/assetlibrary/`。协议已读代码核对并有本地适配测试；真实生成、素材与回调尚未两平台联调。
