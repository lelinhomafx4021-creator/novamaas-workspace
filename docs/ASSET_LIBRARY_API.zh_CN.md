# 素材库与火山 Action API 兼容说明

平台素材库同时提供面向控制台的文件上传接口，以及面向下游客户的火山方舟 Action API 兼容接口。下游侧使用平台签发的 AK/SK，不会接触渠道上配置的火山凭据；平台素材 ID 在请求转发时再按渠道映射为对应的上游素材 ID。

## 下游 AK/SK

用户登录控制台后可通过以下接口管理素材库访问密钥：

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

`CreateAsset` 接收 `GroupId`、`URL`、`AssetType` 和可选 `Name`。平台先使用 SSRF 防护下载公网 HTTP/HTTPS 素材，按存储策略校验 MIME、类型和最大文件大小，写入平台永久对象存储，再返回平台素材 ID；各火山或 YooFang 渠道副本继续由后台同步。素材 URL 导入完成前客户端请求会保持连接，因此下游应设置与大文件上传相匹配的超时，并使用自己的幂等控制避免超时重试产生重复素材。

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
