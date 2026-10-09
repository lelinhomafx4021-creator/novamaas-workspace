# 素材库 API

素材组和素材均使用 Action RPC：所有请求为 POST，Action 与 Version 放在 URL Query，业务参数放在 JSON Body。

## 地址与鉴权

```text
POST /?Action=YOUR_ACTION&Version=2024-01-01
```

| 配置 | 值 |
| --- | --- |
| Version | 2024-01-01 |
| Region | cn-beijing |
| Service | ark |
| 签名算法 | HMAC-SHA256 V4 |

```http
Authorization: HMAC-SHA256 Credential=YOUR_AK/YYYYMMDD/cn-beijing/ark/request, SignedHeaders=content-type;host;x-content-sha256;x-date, Signature=YOUR_SIGNATURE
X-Date: YOUR_UTC_TIMESTAMP
X-Content-Sha256: YOUR_BODY_SHA256
Content-Type: application/json
```

素材导入 URL 必须是公网可直接访问的 HTTP/HTTPS 图片直链，不支持 Base64。

## 素材组 · 创建

`POST /?Action=CreateAssetGroup&Version=2024-01-01`

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| Name | string | 是 | 素材组名称 |
| Description | string | 否 | 素材组业务描述 |
| GroupType | string | 否 | 固定值 AIGC |

```json
{
  "Name": "参考角色组",
  "Description": "正面及侧面参考图片",
  "GroupType": "AIGC"
}
```

```json
{
  "ResponseMetadata": {
    "RequestId": "YOUR_REQUEST_ID",
    "Action": "CreateAssetGroup",
    "Version": "2024-01-01",
    "Service": "ark",
    "Region": "cn-beijing"
  },
  "Result": {
    "Id": "YOUR_GROUP_ID"
  }
}
```

## 素材组 · 查询详情

`POST /?Action=GetAssetGroup&Version=2024-01-01`

```json
{
  "Id": "YOUR_GROUP_ID"
}
```

```json
{
  "ResponseMetadata": {
    "RequestId": "YOUR_REQUEST_ID"
  },
  "Result": {
    "Id": "YOUR_GROUP_ID",
    "Name": "参考角色组",
    "Description": "正面及侧面参考图片",
    "GroupType": "AIGC",
    "CreatedAt": 1790236860
  }
}
```

## 素材组 · 查询列表

`POST /?Action=ListAssetGroups&Version=2024-01-01`

| 参数 | 类型 | 默认值 / 说明 |
| --- | --- | --- |
| PageNumber | integer | 1，从 1 开始 |
| PageSize | integer | 20，最大 100 |
| Filter.Name | string | 名称模糊搜索 |
| Filter.GroupIds | string[] | 素材组 ID 列表 |
| SortBy | string | CreateTime / UpdateTime |
| SortOrder | string | Asc / Desc |

```json
{
  "PageNumber": 1,
  "PageSize": 20,
  "Filter": {
    "Name": "参考"
  }
}
```

```json
{
  "ResponseMetadata": {
    "RequestId": "YOUR_REQUEST_ID"
  },
  "Result": {
    "TotalCount": 1,
    "PageNumber": 1,
    "PageSize": 20,
    "Items": [
      {
        "Id": "YOUR_GROUP_ID",
        "Name": "参考角色组",
        "Description": "正面及侧面参考图片",
        "CreatedAt": 1790236860
      }
    ]
  }
}
```

## 素材组 · 更新

`POST /?Action=UpdateAssetGroup&Version=2024-01-01`

```json
{
  "Id": "YOUR_GROUP_ID",
  "Name": "参考角色组（更新）",
  "Description": "更新后的描述"
}
```

```json
{
  "ResponseMetadata": {
    "RequestId": "YOUR_REQUEST_ID"
  },
  "Result": {
    "Id": "YOUR_GROUP_ID"
  }
}
```

## 素材组 · 删除

`POST /?Action=DeleteAssetGroup&Version=2024-01-01`

```json
{
  "Id": "YOUR_GROUP_ID"
}
```

```json
{
  "ResponseMetadata": {
    "RequestId": "YOUR_REQUEST_ID"
  },
  "Result": {}
}
```

## 素材 · 导入

`POST /?Action=CreateAsset&Version=2024-01-01`

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| GroupId | string | 是 | 所属素材组 ID |
| Name | string | 是 | 素材名称 |
| URL | string | 是 | 公网 HTTP/HTTPS 图片直链，不接受 Base64 |
| AssetType | string | 否 | 默认 image，支持 image / video |

```json
{
  "GroupId": "YOUR_GROUP_ID",
  "Name": "正面参考图",
  "URL": "https://files.example.com/reference.jpg",
  "AssetType": "image"
}
```

```json
{
  "ResponseMetadata": {
    "RequestId": "YOUR_REQUEST_ID",
    "Action": "CreateAsset",
    "Version": "2024-01-01"
  },
  "Result": {
    "Id": "YOUR_ASSET_ID"
  }
}
```

Result.Id 即素材 ID。确认素材 Active 后，在视频请求中填写 `asset://YOUR_ASSET_ID`。

## 素材 · 查询详情

`POST /?Action=GetAsset&Version=2024-01-01`

```json
{
  "Id": "YOUR_ASSET_ID"
}
```

```json
{
  "ResponseMetadata": {
    "RequestId": "YOUR_REQUEST_ID"
  },
  "Result": {
    "Id": "YOUR_ASSET_ID",
    "GroupId": "YOUR_GROUP_ID",
    "Name": "正面参考图",
    "AssetType": "image",
    "Status": "Active",
    "URL": "https://files.example.com/reference.jpg",
    "CreatedAt": 1790236870
  }
}
```

| Status | 含义 |
| --- | --- |
| Active | 就绪可用 |
| Processing | 处理中 |
| Failed | 失败，读取 FailureReason |

## 素材 · 查询列表

`POST /?Action=ListAssets&Version=2024-01-01`

| 参数 | 类型 | 默认值 / 说明 |
| --- | --- | --- |
| PageNumber | integer | 1 |
| PageSize | integer | 20，最大 100 |
| Filter.GroupIds | string[] | 素材组 ID 列表 |
| Filter.Statuses | string[] | 按状态筛选，例如 ["Active"] |
| Filter.Name | string | 名称模糊搜索 |

```json
{
  "GroupId": "YOUR_GROUP_ID",
  "PageNumber": 1,
  "PageSize": 20
}
```

```json
{
  "ResponseMetadata": {
    "RequestId": "YOUR_REQUEST_ID"
  },
  "Result": {
    "TotalCount": 1,
    "PageNumber": 1,
    "PageSize": 20,
    "Items": [
      {
        "Id": "YOUR_ASSET_ID",
        "GroupId": "YOUR_GROUP_ID",
        "Name": "正面参考图",
        "AssetType": "image",
        "Status": "Active",
        "URL": "https://files.example.com/reference.jpg"
      }
    ]
  }
}
```

## 素材 · 更新

`POST /?Action=UpdateAsset&Version=2024-01-01`

```json
{
  "Id": "YOUR_ASSET_ID",
  "Name": "正面参考图（高清特写）"
}
```

```json
{
  "ResponseMetadata": {
    "RequestId": "YOUR_REQUEST_ID"
  },
  "Result": {
    "Id": "YOUR_ASSET_ID"
  }
}
```

## 素材 · 删除

`POST /?Action=DeleteAsset&Version=2024-01-01`

```json
{
  "Id": "YOUR_ASSET_ID"
}
```

```json
{
  "ResponseMetadata": {
    "RequestId": "YOUR_REQUEST_ID"
  },
  "Result": {}
}
```
