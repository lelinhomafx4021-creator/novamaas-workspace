# 图片接口对接

图片生成与编辑使用用户自己的模型 Key，图片模型需已对该密钥开放。先按模型支持选择尺寸和结果格式，再把生成文件与星途用户、项目关联保存。

版本：2.0 收口稿，2026-10-19。本章现有接口可先做公司适配；image.generate 和插件远程授权仍待开发，不能把本地 UI 开关写成已经联通 Gateway 权限。

## 接口选择和字段约定

| 方法 | 地址 | 请求 | 响应 |
| --- | --- | --- | --- |
| POST | `/v1/images/generations` | JSON，OpenAI 兼容文生图 | JSON 或模型支持的 SSE |
| POST | `/v1/images/edits` | JSON 或 multipart，依模型支持 | 图片结果或图片事件 |
| POST | `/api/v3/images/generations` | 火山原生 JSON，仅 Volc Native | 完整读取后的原生响应 |

全部为现有接口。v1 和 v3 的尺寸、质量、参考图字段分别适配；Gateway 当前没有通用图片异步任务查询/取消接口，不能套用视频任务 URL，也不能将 HTTP 超时解释为任务未受理。

| v1 字段 | 类型 | 必填 / 选择规则 |
| --- | --- | --- |
| model | string | 必填，当前 Key 可调用的模型 ID |
| prompt | string | 必填，编辑时为修改指令 |
| n | integer | 可选，正常提交 1–128；模型上限可更低，未传交给适配器默认 |
| size | string | 可选，如 1024x1024；需模型支持，不能给所有模型同一默认 |
| quality | string | 可选，枚举按模型；不能通用写死 standard/hd |
| response_format | string | 可选，url/b64_json，依模型支持 |
| stream | boolean | 可选，仅支持流的链路使用 true |
| image | string / array / file | 编辑条件必填，形式按渠道和请求类型 |
| mask | file 或模型定义格式 | 局部编辑条件使用；尺寸/透明通道等依模型 |
| watermark / seed | boolean / integer | 模型支持才传；显式 false/0 必须保留 |

具体能力表单暂用已确认模型映射。`/v1/models` 只返回模型名，`/api/pricing` 也没有完整质量、尺寸、多图和参考素材规格；不能据此自动生成所有模型参数或准确报价。

## 文生图与图片结果

```http
POST /v1/images/generations
Authorization: Bearer <FULL_USER_MODEL_API_KEY>
Content-Type: application/json

{"model":"<IMAGE_MODEL_ID>","prompt":"<IMAGE_PROMPT>","n":1,"size":"1024x1024"}
```

遍历 `data[]`：有 url 则读取地址，有 b64_json 则解码 Base64。格式、尺寸、quality 和 response_format 的支持取决于模型；n 从 1 开始，网关兼容接口上限 128，模型可能更小。尺寸使用 ASCII x，不用乘号 ×。

Base64 不一定带 Data URI 前缀；URL 不一定永久有效。需要长期保存时由星途保存文件与用户归属，不能把下载次数或本地估价当作账单。

成功返回 URL 示例：

```json
{"created":1800000000,"data":[{"url":"https://files.example.com/generated/image.png","revised_prompt":"实际采用的提示词"}]}
```

Base64 返回示例（占位串不能直接解码）：

```json
{"created":1800000000,"data":[{"b64_json":"<BASE64_IMAGE_BYTES>"}]}
```

遍历全部 data 项，保留返回顺序和 revised_prompt；不要只保存第一张。星途记录调用用户、项目、模型、请求参数、Gateway 请求编号和实际文件，不保存上游密钥。下载图片直链不用附带 Gateway Bearer，防止泄露给 CDN。

## 图片编辑与流式图片

编辑使用 `/v1/images/edits`。文件上传示例：

```bash
curl --fail-with-body 'https://gateway.example.com/v1/images/edits' \
  --header 'Authorization: Bearer <FULL_USER_MODEL_API_KEY>' \
  --form 'model=<IMAGE_EDIT_MODEL_ID>' \
  --form 'prompt=<EDIT_PROMPT>' \
  --form 'image=@<LOCAL_IMAGE_PATH>'
```

让客户端生成 multipart boundary，不手工设置 JSON Content-Type。支持 JSON 参考图的渠道也可发送 `{"model":"<IMAGE_EDIT_MODEL_ID>","prompt":"<EDIT_PROMPT>","image":"https://files.example.com/reference.png"}`。本机路径不能放进 JSON 让上游读取；多图和 mask 按模型支持提供。

支持 mask 的编辑示例：

```bash
curl --fail-with-body 'https://gateway.example.com/v1/images/edits' \
  --header 'Authorization: Bearer <FULL_USER_MODEL_API_KEY>' \
  --form 'model=<MASK_EDIT_MODEL_ID>' \
  --form 'prompt=<EDIT_PROMPT>' \
  --form 'image=@<LOCAL_IMAGE_PATH>' \
  --form 'mask=@<LOCAL_MASK_PATH>' \
  --form 'n=1'
```

多图字段名称和数量依适配器要求，不因单图成功就默认 image[] 可用于全部渠道；客户端上传前校验模型支持和文件大小，服务端仍再次校验。素材库中的 asset:// 句柄不能当作所有图片编辑接口的 image 值。

支持图片流的 v1 渠道传 `"stream":true`，按 Content-Type 和事件 type 解析 b64_json/url。OpenAI 图片适配支持 image_generation.partial_image、image_generation.completed，编辑还可能有 image_edit.*；不能套用文本 delta.content。上游一次性返回 JSON 时也可能被转换成 completed 事件，SSE 不保证有中间预览。

事件形态示例：

```text
event: image_generation.partial_image
data: {"type":"image_generation.partial_image","partial_image_index":0,"b64_json":"<PARTIAL_BASE64>"}

event: image_generation.completed
data: {"type":"image_generation.completed","b64_json":"<FINAL_BASE64>"}
```

临时预览不作为最终成果；completed 之后才标记此结果完成。按空行分帧，合并拆开的 data 行；不要假定一次网络读取就是一个事件。错误事件、断开和用户停止要单独处理，未拿到最终图不显示成功。

## 火山原生图片请求

火山原生图片使用 `POST /api/v3/images/generations`，仅 Volc Native 渠道，按模型原生字段构造。当前控制器完整读取上游响应后返回，不作为实时 SSE 通道；没有 `/api/v3/images/edits` 路由。

```http
POST /api/v3/images/generations
Authorization: Bearer <FULL_USER_MODEL_API_KEY>
Content-Type: application/json

{"model":"<VOLC_NATIVE_IMAGE_MODEL_ID>","prompt":"清晨的城市街景","size":"1024x1024","response_format":"url","watermark":false}
```

size 和其他原生字段必须符合所选模型，不能根据旧 PDF 或网页示例自行将 width/height 当成通用必填。该路径只改写模型映射并透传原生 body；当前不支持渠道参数覆盖，配置了覆盖会返回 param_override_not_supported。成功响应以该模型原生 data/url/b64_json 等字段为准，不加 success/data 管理信封。

## 旧媒体工具的适配与交付

星途当前媒体工具仍使用旧 `/api/media/images/...` 及旧响应外壳。保留工具名、参数、回调及官方执行语义，在独立公司适配层把请求转换到本章 v1/v3；不把旧 code/data 解析器直接套到模型结果，不为适配新增不存在的通用图片任务接口。

| 输入/结果 | 转换与持久化要求 |
| --- | --- |
| 模型、尺寸、数量、质量 | 来自已核验能力；未填与 0/false 分开；n 仍受服务端安全上限约束 |
| 本地参考图、mask | 按实际协议发送 multipart、支持的 Data URI 或可读 URL；不上传本地路径字符串 |
| 同步 URL/Base64 | 本次请求直接取得结果；验证内容并保存，不继续查询“图片任务” |
| SSE 部分图/最终图 | 预览单独保留，只有最终事件对应最终结果；错误和中断保留准确状态 |
| 结果写盘 | 绑定 origin/user.id、项目、本地生成记录、模型/协议和请求编号；不保存 Key |
| 保存失败 | 优先重取已获结果或重新保存，不能自动再生成一笔付费请求 |

本地处理状态可以表达提交、接收、保存、完成或失败，但只是客户端记录，不得包装成 Gateway 远程图片任务 ID。换号后仍能按归属查看原有成果，新生成使用新账号的身份与 Key。

验收分别记录 URL、Base64、编辑、支持的流式、长请求中断及文件保存失败；按选定模型实际支持选择场景。成功图、真实消费及星途工具返回三项须能关联，自动测试通过不直接勾选真实图片生成。

## 结果保存、权限变化和错误

图片工作流顺序为：读取本人 image.generate 和插件权限（未来接口）→ 确认模型 Key 可用 → 构造模型支持的请求 → 提交一次 → 展示结果并保存文件 → 回查消费。权限拒绝要发生在预扣费和上游调用前；现有功能尚无第三章的远程权限守卫，不能当作已实现。

401 检查模型 Key；参数错误修正 size/n/参考图；额度和限流按实际错误区分；HTTP 200 的流内 error 也算失败。停止读取、关闭窗口或下载失败都不代表未计费，费用由 Gateway 账本确认。需要恢复时保存请求编号和已有结果，不盲目重新生成。

本地参考文件需要先转为上游可读取的地址或适配器支持的 Base64；图片编辑可直接使用上面的 multipart。需要注册素材时见[视频接口对接中的素材流程](/admin-manual?article=video-models#参考素材上传与使用)，不要把素材预览临时地址当成永久文件地址。消费回查见[账号与钱包](/admin-manual?article=account-login#消费回查与请求结果)。

源码依据：`router/relay-router.go`、`relay/helper/valid_request.go`、`relay/channel/openai/relay_image.go`、`controller/volc_native.go`。协议已读代码核对；真实图片生成尚未两平台联调。
