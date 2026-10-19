# 文本接口对接

使用当前用户的模型 Key 接入文本对话。先完成密钥与可用模型同步，再选择同步或流式协议；Gateway 不保存星途的完整对话记录。

版本：2.0 收口稿，2026-10-19。本章 API 是现有协议入口；各模型与渠道的组合须实测。星途接入限于独立公司适配，保留官方消息、工具、流式和历史行为。

## 接口和凭证矩阵

| 方法 | 地址 | 协议 | 星途如何处理 |
| --- | --- | --- | --- |
| POST | `/v1/chat/completions` | OpenAI Chat | messages 输入，choices 输出；同步/SSE |
| POST | `/v1/completions` | 旧 Completions | prompt 输入，choices[].text；仅兼容旧客户端 |
| POST | `/v1/responses` | Responses | input 输入，output/事件输出 |
| POST | `/v1/responses/compact` | Responses compact | 上下文压缩专用，不套普通对话返回值 |
| POST | `/v1/messages` | Claude Messages | messages/system，content[]；专用 SSE 事件 |
| POST | `/v1beta/models/<MODEL_ID>:generateContent` | Gemini | contents/parts；同步 candidates |
| POST | `/v1beta/models/<MODEL_ID>:streamGenerateContent` | Gemini | contents/parts；按实际流协议读取 |

以上路由均为现有接口；模型和渠道仍需支持对应协议，不能因为路由存在就给全部模型显示该模式。全部示例用 Gateway 用户模型 Key，不能把上游渠道 Key、用户登录 Access Token 或管理员 PAT 填入。

OpenAI 风格优先用 `Authorization: Bearer <FULL_USER_MODEL_API_KEY>`。Claude 客户端还可用 `x-api-key`，Gemini 客户端可用 `x-goog-api-key`；使用对应 SDK 时配置 Gateway 地址，避免把 Key 发到默认官方域名。OpenAI SDK 的 base_url 常为 `https://gateway.example.com/v1`，具体调用不要再重复拼接 /v1。

## Chat 请求字段

| 字段 | 类型 | 必填 / 默认 | 约束和用途 |
| --- | --- | --- | --- |
| model | string | 必填 | 选定 Key 的 `/v1/models` 实际 ID |
| messages | array | 必填 | 按顺序携带上下文；图片/工具格式取决于模型 |
| messages[].role | string | 必填 | system/user/assistant/tool；部分模型另有 developer |
| messages[].content | string 或 array | 按消息类型 | 普通文本用 string；视觉输入用类型化内容项 |
| stream | boolean | 可选 | true 读取 SSE，false 读取 JSON |
| temperature / top_p | number | 可选 | 未传交给模型默认；不要把某一个模型默认套给全部模型 |
| max_tokens / max_completion_tokens | integer | 可选 | 非负，网关上限 1073741823；实际模型上限通常更小，按选定模型限制 |
| tools | array | 可选 | 受支持的工具声明；function 参数是 JSON Schema |
| tool_choice | string 或 object | 可选 | auto/none/required 或特定工具，按模型支持 |
| stream_options.include_usage | boolean | 可选 | 渠道支持时可返回 usage；不能要求每帧都有 |
| response_format | object | 可选 | 文本/结构化 JSON 能力依模型支持 |

可选 0、false 与未传不是同一个意思。星途表单只发送用户选定且当前协议支持的字段，不将 undefined 转成 0，也不把显式 temperature=0 丢掉。

## 同步对话

```http
POST /v1/chat/completions
Authorization: Bearer <FULL_USER_MODEL_API_KEY>
Content-Type: application/json

{
  "model": "<TEXT_MODEL_ID>",
  "messages": [
    {"role":"system","content":"<SYSTEM_PROMPT>"},
    {"role":"user","content":"<USER_MESSAGE>"}
  ],
  "stream": false
}
```

读取 `choices[].message`，文字通常在 `choices[0].message.content`，工具调用读 `tool_calls`。多轮对话由星途把必要历史按顺序传回，Gateway 不会自动拼接全部会话。模型响应不使用管理接口的 success/data 外壳。

成功响应示例：

```json
{"id":"chatcmpl-example","object":"chat.completion","created":1800000000,"model":"<TEXT_MODEL_ID>","choices":[{"index":0,"message":{"role":"assistant","content":"生成的正文"},"finish_reason":"stop"}],"usage":{"prompt_tokens":20,"completion_tokens":10,"total_tokens":30}}
```

id 为模型响应 ID，不等于 Gateway X-Oneapi-Request-Id，也不等于项目/会话 ID。finish_reason=length 应显示被长度限制截断；tool_calls 表示待执行工具，不是已完成业务。工具运行由星途完成，回填 role=tool 和原 tool_call_id 后继续下一次请求。

## 视觉输入与工具调用

支持视觉的文本模型仍调用 chat/completions，不能因为附带图片就路由到图片生成：

```json
{"model":"<VISION_TEXT_MODEL_ID>","messages":[{"role":"user","content":[{"type":"text","text":"描述这张参考图"},{"type":"image_url","image_url":{"url":"https://files.example.com/reference.png"}}]}],"stream":false}
```

本地路径不能由上游读取；先用模型支持的 Data URI 或可访问 URL。素材 asset:// 不是通用视觉 URL，只有明确支持的适配链路才能引用。

工具声明示例：

```json
{"model":"<TOOL_TEXT_MODEL_ID>","messages":[{"role":"user","content":"查询项目进度"}],"tools":[{"type":"function","function":{"name":"get_project_status","description":"查询当前用户的项目进度","parameters":{"type":"object","properties":{"project_id":{"type":"string"}},"required":["project_id"],"additionalProperties":false}}}],"tool_choice":"auto","stream":false}
```

工具返回的 arguments 是 JSON 字符串，先解析并验证再执行；模型生成内容不是授权。若工具属于受控业务插件，每次执行须通过第三章的功能/版本检查，包含模型连续发出的多轮工具调用，不能只在对话开始检查一次。

## 流式与其他文本协议

同一请求改为 `"stream":true`，使用能发送 POST、JSON 和 Authorization 的流式客户端解析 SSE：

```text
data: {"choices":[{"index":0,"delta":{"content":"<TEXT_DELTA>"}}]}

data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
```

按帧解析，不能把一次网络读取当作完整 JSON。按 index 合并 delta.content，工具参数增量单独处理；usage 帧可能没有 choices。处理协议结束标志、错误与中断，用户停止时取消请求，费用以 Gateway 结算为准。

其他文本协议分别使用 `/v1/responses`、`/v1/messages`、`/v1beta/models/<MODEL_ID>:generateContent` 或 `:streamGenerateContent`，按各自响应解析。路由存在不代表每个模型都支持；不能替换路径后继续套用 choices/delta。

### Responses 请求和响应

```http
POST /v1/responses
Authorization: Bearer <FULL_USER_MODEL_API_KEY>
Content-Type: application/json

{"model":"<RESPONSES_MODEL_ID>","input":"概括这段内容","max_output_tokens":1024,"stream":false}
```

```json
{"id":"resp-example","object":"response","status":"completed","model":"<RESPONSES_MODEL_ID>","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"概括后的内容"}]}],"usage":{"input_tokens":20,"output_tokens":10,"total_tokens":30}}
```

读取 output 中的 message/content/output_text，不能只拿 output[0] 假定它一定是文本，工具/推理项可能在前。流式按 response.output_text.delta、response.completed、response.failed 等 type 分发；不能以 Chat 的 [DONE] 作为所有协议的唯一结束条件。previous_response_id 等服务端状态功能依上游支持，Gateway 不承诺保存完整 Responses 会话。

### Claude Messages 请求和响应

```http
POST /v1/messages
Authorization: Bearer <FULL_USER_MODEL_API_KEY>
anthropic-version: 2023-06-01
Content-Type: application/json

{"model":"<CLAUDE_MODEL_ID>","max_tokens":1024,"system":"你是创作助手","messages":[{"role":"user","content":"写一段开场白"}],"stream":false}
```

```json
{"id":"msg-example","type":"message","role":"assistant","model":"<CLAUDE_MODEL_ID>","content":[{"type":"text","text":"开场白正文"}],"stop_reason":"end_turn","usage":{"input_tokens":20,"output_tokens":10}}
```

system 在顶层，不伪装成普通 system 消息。流式处理 message_start、content_block_start/delta/stop、message_delta、message_stop 和 error；tool_use/input_json_delta 与文字分开累积。

### Gemini 请求和响应

```http
POST /v1beta/models/<GEMINI_MODEL_ID>:generateContent
Authorization: Bearer <FULL_USER_MODEL_API_KEY>
Content-Type: application/json

{"contents":[{"role":"user","parts":[{"text":"写一句标题"}]}],"generationConfig":{"maxOutputTokens":1024}}
```

```json
{"candidates":[{"content":{"role":"model","parts":[{"text":"生成的标题"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":20,"candidatesTokenCount":10,"totalTokenCount":30}}
```

模型名位于路径，不需要把 OpenAI 的 model/messages 硬塞进 body。parts 可包含文字和其他类型，按类型处理。streamGenerateContent 选择渠道支持的传输格式；SDK 用 SSE 时通常带 alt=sse，按实际 Content-Type 解析，不能把 JSON 数组流与 SSE 混用。

## 模型协议能力表与接入验收

两端共同维护实际支持表，至少记录公开 model_id、选定 Key/分组、协议路径、同步/流式、视觉、工具、结构化输出、上下文方式、请求限制和验证日期。`/v1/models` 证明 Key 可见模型，不承诺每个模型支持全部协议；`/api/pricing` 的部分元数据也不能替代这张表。

| 接入步骤 | 星途适配要求 | 验收结果 |
| --- | --- | --- |
| 选择协议 | 由已核验模型能力决定，使用相应参数与 parser | 同一模型不因存在路由就自动标全支持 |
| 多轮上下文 | 保留官方历史与工具消息顺序；区分完整历史和上游状态引用 | previous_response_id 不支持时明确失败，不盲目重发付费请求 |
| 工具调用 | 保留 name、call id 和参数增量，完成 schema/权限检查后执行 | 工具参数完整、结果正确回填；停止后不重复执行 |
| 流终止 | 分协议处理完成、失败、长度截断和断流 | HTTP 200 或出现文字都不直接当成功；已有文本保留并标实际状态 |
| 费用与恢复 | 保存响应 ID 和可取得的 Gateway 请求编号，回查服务端账本 | usage 缺失/估算不伪装上游精确值，不客户端重新结算 |

自动 parser/转换测试、实际模型响应及真实星途对话分别记录。必须包含一次同步、一次分帧流、一次工具调用、一次中断和会话/Key 错误；只对明确支持的组合执行。对话基础存在阻断时先处理已有缺陷，不借对接重新设计回答区或发送链路。

## 相关模型能力入口与错误处理

语音、向量、重排等若进入星途产品，可复用用户模型 Key，但各自使用现有专用接口：

| 方法 | 现有地址 | 输入与输出 |
| --- | --- | --- |
| POST | `/v1/embeddings` | model/input；返回 data[].embedding |
| POST | `/v1/rerank` | model/query/documents；按重排协议读结果 |
| POST | `/v1/audio/transcriptions` | multipart 的 file/model；返回识别文本 |
| POST | `/v1/audio/translations` | multipart 的 file/model；返回翻译文本 |
| POST | `/v1/audio/speech` | model/input/voice；返回音频二进制或模型支持的流 |
| GET | `/v1/realtime` | WebSocket，需相应客户端和渠道支持 |

这些是能力入口索引，不等于全部套餐都授予。语音等如纳入远程功能控制，要先补稳定 feature_id 和服务端边界，不能借 text.chat 自动授予所有新能力。

OpenAI 常见错误为 `{"error":{"message":"...","type":"...","code":"..."}}`；Claude/Gemini 依协议返回原生错误。处理 HTTP 状态与实际 error 对象，避免写死唯一错误码。401 检查模型 Key，429 区分限流与额度；400 修参数；服务失败允许用户重试，但付费请求超时先回查消费，不能自动循环重发。SSE 开始后 HTTP 仍可能是 200，流内 error 同样要显示失败/中断。

调用后的费用与失败处理见[账号与钱包](/admin-manual?article=account-login#消费回查与请求结果)。

源码依据：`router/relay-router.go`、`relay/helper/valid_request.go`、`relay/channel/openai/relay-openai.go`、`relaykit/dto/openai_request.go`。协议已读代码核对；真实文本生成尚未两平台联调。
