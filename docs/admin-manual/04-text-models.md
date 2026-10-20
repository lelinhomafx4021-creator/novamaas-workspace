# 文本模型对接

管理目标：星途通过公司平台的 OpenAI 兼容连接使用文本模型，平台统一管理供应商、模型权限及账务。现有连接能力已具备，完整桌面对话链路待联调。

## 一、职责与接入前提

平台为目标用户准备可见且可调用的模型，核对渠道、映射、分组、模型限制和额度。星途沿用现有对话、上下文、流式、工具及历史保存机制，在公司平台适配处完成连接。

用户按[账户与平台密钥](/admin-manual?article=api-keys)配置模型密钥并同步模型。模型调用使用模型密钥，账户资料使用登录会话，分别核对身份和费用归属。

## 二、对接要求

| 环节 | 交付要求 |
| --- | --- |
| 模型选择 | 使用实际同步目录，默认模型保持在可用范围内 |
| 消息与上下文 | 复用现有会话和历史结构，保持多轮上下文正确 |
| 流式与工具 | 保留事件、工具参数及结果、停止、错误和恢复行为 |
| 能力开放 | 视觉、附件及工具按所选模型的实际验证结果开放 |
| 账务核查 | 在平台按实际密钥账户回查调用、用量及费用 |

首期以现有兼容连接为基础，保持原有对话和扩展执行契约；其他协议按明确需求单独适配。

## 三、已有请求与响应字段

首期复用 POST /v1/chat/completions，Content-Type 为 application/json，Authorization 使用用户模型 API Key；客户端继续走现有对话适配层。下表列出本期要对齐的字段，不要求每个模型支持所有可选参数。

| 请求字段 | 类型 | 必填 / 规则 |
| --- | --- | --- |
| `model` | string | 必填，使用当前密钥目录中的实际模型 ID |
| `messages` | object[] | 必填，按顺序传已有历史与本次输入 |
| `messages[].role` | string | 必填，system / user / assistant / tool；developer 等按模型支持 |
| `messages[].content` | string 或 object[] | 普通文本为字符串；视觉、工具等按原适配结构传递 |
| `messages[].tool_calls` / `tool_call_id` | object[] / string | 工具调用与工具结果按现有协议关联，不丢失 ID |
| `stream` | boolean | 可选，true 读取 SSE，false 读取 JSON |
| `temperature` / `top_p` | number | 可选，仅在所选模型支持时发送，显式 0 保留 |
| `max_tokens` / `max_completion_tokens` | integer | 可选，非负且不超过网关上限 1073741823；模型上限可能更低 |
| `tools` | object[] | 可选，工具声明及参数 JSON Schema 沿用既有格式 |
| `tool_choice` | string 或 object | 可选，auto / none / required 或指定工具，按模型支持 |
| `stream_options.include_usage` | boolean | 可选，仅在当前渠道支持时使用 |
| `response_format` | object | 可选，结构化输出按实际模型支持 |

最小非流式请求示例，不额外写死温度和长度默认值：

```json
{"model":"<实际模型ID>","messages":[{"role":"user","content":"请介绍你的能力"}],"stream":false}
```

| JSON 成功响应字段 | 类型 | 解析规则 |
| --- | --- | --- |
| `id` / `object` / `model` | string | 回答标识、对象类型、实际返回模型，不使用请求序号替代 |
| `created` | integer | Unix 秒 |
| `choices` | object[] | 按 index 读取结果，不能假定任意响应必有非空文本 |
| `choices[].index` | integer | 结果序号 |
| `choices[].message.role` / `content` | string / string或null | 普通回答；工具调用时文本可为空 |
| `choices[].message.tool_calls` | object[] | 有工具调用时按现有执行循环处理 |
| `choices[].finish_reason` | string 或 null | 正常结束、长度限制或工具调用等，不统一视为失败 |
| `usage.prompt_tokens` / `completion_tokens` / `total_tokens` | integer | 若响应提供则记录；平台实际账务为最终消费依据 |

流式使用 `choices[].delta`，文本通常位于 delta.content；工具参数可能分片，按 index 与调用 ID 累积。usage 可在单独末帧返回，choices 可为空；处理 SSE data、结束标记及错误沿用现有客户端，不能把每一帧都当作完整 JSON 回答。平台错误通常为 `error` 对象，先检查 HTTP 和协议错误，不按订阅接口的 success 包装读取。

Responses、Claude Messages 和 Gemini 是其他已有协议，保持原路由和字段；本次公司平台对接不顺带重写这些执行链。扩展某条协议时单独列明模型、真实请求与验收记录。

## 四、联调与验收

1. 保存密钥，核对目录、默认模型及权限。
2. 新建对话完成短文本和多轮交流，核对回答及上下文。
3. 验证流式、停止、失败提示、重试和历史恢复；支持附件及工具的模型分别验证对应能力。
4. 回查平台记录，核对实际模型、用量及费用所属账户。

目录同步、接口可达和真实对话成功分别记录。客户端接入及版本改造均须回归现有对话和基础扩展；图片、视频的独立生成按各自章节验收。
