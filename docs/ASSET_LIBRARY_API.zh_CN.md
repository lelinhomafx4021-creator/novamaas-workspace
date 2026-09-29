# 素材库与火山 Action API 兼容说明

平台素材库同时提供面向控制台的文件上传接口，以及面向下游客户的火山方舟 Action API 兼容接口。下游侧使用平台签发的 AK/SK，不会接触渠道上配置的火山凭据；平台素材 ID 在请求转发时再按渠道映射为对应的上游素材 ID。

## 下游 AK/SK

用户登录控制台后，进入「素材库」，点击页面右上角的「API 接入」即可创建和吊销下游 AK/SK。弹窗会同时展示当前站点的官方 SDK Endpoint、`/api/v3/` 兼容请求地址、签名参数和已支持的 Action，便于直接交付给下游客户。

创建时只需填写密钥用途名称。创建成功后必须立即复制 Access Key ID 和 Secret Access Key；Secret Access Key 只显示一次，关闭提示后无法再次查看。如有遗失，应吊销旧密钥并重新创建，不应要求管理员从数据库中导出密文。

也可以通过以下控制台接口自动化管理素材库访问密钥：

- `GET /api/asset-library/access-keys`
- `POST /api/asset-library/access-keys`，请求体为 `{"name":"automation"}`
- `DELETE /api/asset-library/access-keys/{id}`

每个用户最多可同时拥有 5 个有效密钥。创建响应只返回一次 `secret_access_key`，后续列表只显示 AK 和 SK 尾号；删除采用禁用状态保留审计记录。SK 使用 AES-GCM 加密保存，密钥来源依次为 `STORAGE_CREDENTIAL_ENCRYPTION_KEY`、`CRYPTO_SECRET`、`SESSION_SECRET`。生产环境应固定配置第一项，并纳入备份和轮换方案；缺少可用主密钥时拒绝创建 AK/SK。

## 兼容路径与 Action

与火山官方 SDK 一样，可将 API Endpoint 配置为平台域名并请求根路径：

```text
POST https://gateway.example.com/?Action=CreateAsset&Version=2024-01-01
```

同时提供 `POST /api/v3/?Action=...&Version=2024-01-01` 作为网关部署兼容别名。签名必须使用客户端实际请求的路径，根路径和 `/api/v3/` 的签名不能混用。

当前支持以下 Action：

- `CreateAssetGroup`、`ListAssetGroups`、`GetAssetGroup`、`UpdateAssetGroup`、`DeleteAssetGroup`
- `CreateAsset`、`ListAssets`、`GetAsset`、`UpdateAsset`、`DeleteAsset`

请求和响应采用火山 `ResponseMetadata` / `Result` 信封，版本固定为 `2024-01-01`，Service 为 `ark`，Region 为 `cn-beijing`。`ProjectName` 作为协议兼容字段接受，平台租户隔离仍以 AK 所属用户为准，响应中的项目名为 `default`。

`CreateAsset` 接收 `GroupId`、`URL`、`AssetType` 和可选 `Name`。平台先使用 SSRF 防护下载公网 HTTP/HTTPS 素材，按存储策略校验 MIME、类型和最大文件大小，写入平台永久对象存储，再返回平台素材 ID；各火山或 YooFang 渠道副本继续由后台同步。存在已启用的上游素材渠道时，新素材先返回 `Status=Processing`，首次确认上游可用后改为 `Status=Active`；没有启用上游渠道时仍直接返回 `Active`。素材 URL 导入完成前客户端请求会保持连接，因此下游应设置与大文件上传相匹配的超时，并使用自己的幂等控制避免超时重试产生重复素材。

列表接口支持官方的页码分页，也支持 `MaxResults` / `NextToken`：

- `PageNumber` 从 1 开始，`PageSize` 最大 100。
- `MaxResults` 最大 100；存在下一页时响应返回仅对本平台有效的 `NextToken`。
- `Filter.Name` 模糊搜索名称，`Filter.GroupIds` 精确筛选素材组，`ListAssets` 还接受 `Filter.Statuses`。
- `SortBy` 支持 `CreateTime`、`UpdateTime`，素材列表额外支持 `GroupId`；`SortOrder` 支持 `Asc`、`Desc`。

## AK/SK 签名

下游签名沿用火山 HMAC-SHA256 V4 结构：

```text
Authorization: HMAC-SHA256 Credential=<AK>/<YYYYMMDD>/cn-beijing/ark/request, SignedHeaders=content-type;host;x-content-sha256;x-date, Signature=<hex>
X-Date: 20260922T083045Z
X-Content-Sha256: <SHA256(request-body)>
Content-Type: application/json
```

平台会重新计算请求体哈希、Canonical URI、排序后的 Canonical Query、Canonical Headers 和四级派生签名，并使用常量时间比较验签。`X-Date` 与凭据日期必须一致，服务器只接受时间偏差不超过 5 分钟的请求。反向代理必须保留客户端签名时使用的 `Host`，并保持请求路径与 Query 不变。

## 素材库大列表

控制台素材列表按服务端分页，每页最多加载 40 条；名称、素材 ID、MIME 类型和上传者搜索在数据库侧完成。页面只保留当前页卡片，图片使用浏览器懒加载，预览签名 URL 仅在卡片接近可视区域时请求，因此素材总量超过 1,000 时不会在首屏创建同等数量的 DOM、媒体请求或预览签名请求。

素材卡片展示上传者和上传时间。管理员查看全局范围时可以按上传者检索；普通用户仍只能看到自己的素材。数据库查询以素材归属、状态和创建时间索引为基础，部署到生产前仍应使用目标数据库和真实对象存储做容量、带宽与分页延迟压测。

## 素材状态 Webhook

### 企业 AK/SK 管理接口

企业可使用同一套素材 AK/SK 签名管理回调，不必先登录控制台。请求仍为 `POST /api/v3/?Action=...&Version=2024-01-01`（也支持已签名的根路径）。下面这些 Action 是本平台扩展，不是火山官方 Action；客户需使用能签署自定义 Action 的客户端。

| Action | JSON 请求字段 | Result |
| --- | --- | --- |
| CreateAssetWebhookEndpoint | Name、URL、EventTypes | 端点对象，含一次性 signing_secret |
| ListAssetWebhookEndpoints | `{}` | Items、TotalCount；不返回密钥明文 |
| UpdateAssetWebhookEndpoint | Id、Name、URL、EventTypes | 更新后的端点对象；完整替换名称、URL、订阅类型 |
| DeleteAssetWebhookEndpoint | Id | `{}`，禁用端点 |
| RotateAssetWebhookEndpointSecret | Id | 端点对象，含新 signing_secret；旧密钥立即失效 |
| TestAssetWebhookEndpoint | Id | EventId；仅表示已入队，不保证已经送达 |

创建请求示例：

```json
{
  "Name": "ERP material notifications",
  "URL": "https://customer.example.com/webhooks/assets",
  "EventTypes": ["asset.active", "asset.failed"]
}
```

响应仍使用 `ResponseMetadata` / `Result` 信封。Result 内的端点对象沿用控制台接口的字段命名：`id`、`name`、`url`、`event_types`、`status`、`signing_secret_hint`、`created_at`、`updated_at` 等。后续请求中的 `Id` 填返回的 `Result.id`。端点归属由签名 AK 所属用户决定，不接受客户端指定其他用户；同一企业账号签发的 AK 共享该账号端点，不能访问其他账号的端点。

### 素材状态查询与图片预览

AK/SK 的 `GetAsset` 与 `ListAssets` 返回素材元数据和审核状态；查询成功为 HTTP 200，素材是否可用需看 `Status`。审核拒绝返回 `Status=Failed` 和 `FailureReason`，不会因审核失败隐藏素材。

图片 URL 可生成时，额外返回 `PreviewStatus=Available` 和 `URLExpiresAt`（Unix 秒），客户可使用 `URL` 预览，过期后重新查询。预览签名或存储发生异常时，仍返回素材元数据和审核状态，`URL` 为空，`PreviewStatus=Unavailable`、`PreviewErrorCode=preview_unavailable`，不返回内部存储错误。一条素材预览失败不会使整页列表失败。素材不存在、无权限或元数据查询失败仍按原错误响应处理。

```json
{
  "Result": {
    "Id": "asset-example-001",
    "Status": "Failed",
    "FailureReason": "sensitive_content",
    "URL": "",
    "PreviewStatus": "Unavailable",
    "PreviewErrorCode": "preview_unavailable"
  }
}
```

以上为省略其他字段的示例。`PreviewStatus` 只表示图片预览是否可用，与审核状态独立，不能据此决定是否生成视频。`CreateAsset` 的 AK/SK 响应仍仅返回 `Id`；应随后调用 `GetAsset` 或接收事件确认状态。

### 控制台配置与事件投递

下游客户在控制台「素材库 → Webhooks」填写自己的公网 HTTPS 回调地址。平台集中轮询上游素材状态；首次确认可用，或后续复审将素材从可用改判为失败时，平台先更新本地素材状态，再向订阅端点发送事件。Webhook 只负责提醒，`GetAsset` / `ListAssets` 返回的本地状态始终是最终依据。

也可以通过登录态控制台接口管理端点：

- `GET /api/asset-library/webhook-endpoints`
- `POST /api/asset-library/webhook-endpoints`
- `PUT /api/asset-library/webhook-endpoints/{id}`
- `DELETE /api/asset-library/webhook-endpoints/{id}`
- `POST /api/asset-library/webhook-endpoints/{id}/rotate-secret`
- `POST /api/asset-library/webhook-endpoints/{id}/test`

创建或更新请求示例：

```json
{
  "name": "production",
  "url": "https://customer.example.com/webhooks/assets",
  "event_types": ["asset.active", "asset.failed"]
}
```

每个用户最多可有 5 个有效端点。创建和轮换响应中的 `signing_secret` 只展示一次，后续列表仅返回尾号；密钥使用与 AK/SK 相同的 AES-GCM 主密钥体系加密保存。回调地址在投递前同时经过 URL 校验和 SSRF 防护。平台直连投递不会跟随重定向；启用请求 Worker 的部署应让 Worker 同样把 3xx 视为失败。

事件信封和签名头采用 OpenAI Webhook 相同的通用结构，便于下游复用现有接收逻辑：

```http
POST /webhooks/assets HTTP/1.1
Content-Type: application/json
webhook-id: wh_...
webhook-timestamp: 1790580000
webhook-signature: v1,<base64-hmac-sha256>
```

```json
{
  "id": "evt_...",
  "object": "event",
  "type": "asset.failed",
  "created_at": 1790580000,
  "data": {
    "id": "asset-20260923163433-giwbv",
    "group_id": "group-20260923163432-cwlhg",
    "status": "Failed",
    "failure_reason": "sensitive_content",
    "updated_at": 1790580000,
    "state_version": 2
  }
}
```

签名原文为 `<webhook-id>.<webhook-timestamp>.<原始请求体>`，使用端点的 `whsec_...` 密钥做 HMAC-SHA256 后 Base64 编码。下游应使用原始请求体验签、校验时间戳，并以 `webhook-id` 去重；业务处理前先快速返回 2xx。非 2xx 或网络错误会指数退避重试，最长保留 72 小时；同一投递的 `webhook-id` 和请求体在重试时保持不变。

收到 `asset.failed` 后，素材 ID 不变，下游应把该 ID 标记为失败/不可用，并阻止新的生成任务继续引用它；不要删除或复用这个 ID。平台会作废排队中的旧 `asset.active`，但已发出的请求仍可能延迟到达。Webhook 可能重复或乱序，下游必须按素材 ID 在数据库中原子比较 `data.state_version`，仅在收到的版本大于已保存版本时更新状态。`asset.active` 的版本为 1，终态 `asset.failed` 的版本为 2；同一素材失败后不会恢复，重新上传使用新 ID。不要仅比较秒级 `updated_at`，因为两次状态变化可能发生在同一秒。缺少版本的历史事件应通过 `GetAsset` 查询对账，不直接覆盖本地状态。
