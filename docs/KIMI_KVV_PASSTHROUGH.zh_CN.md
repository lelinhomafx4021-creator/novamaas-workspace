# OpenAI 渠道的 Kimi KVV 透传模式

## 适用边界

该模式用于上游已经提供 OpenAI Chat Completions 接口、同时保留 Kimi 扩展语义的场景。上游必须能够在直连时通过目标 KVV 测试；网关只负责透明传递这些能力，不会为普通 OpenAI 上游伪造 Kimi 的缓存、思考、动态工具或 token 统计语义。

当前透传范围仅为：

- 客户端入口：`/moonshot/v1/chat/completions`
- 渠道类型：OpenAI
- 上游协议：OpenAI Chat Completions
- 请求：保留原始 JSON 字段和值，仅在配置模型映射时改写顶层 `model`
- 响应：保留上游 JSON/SSE 字段，不补造 usage；配置模型映射时将顶层 `model` 恢复为客户端看到的模型名

以下入口继续使用现有兼容模拟逻辑，不属于本模式：

- `/moonshot/v1/responses`
- `/moonshot/anthropic/v1/messages`

网关鉴权、渠道选择、请求大小和计费安全校验、额度预扣与结算仍然生效。因此，这不是字节级 TCP 代理，而是 Kimi Chat Completions 语义的透明转发。

## 配置方式

在渠道管理中创建或编辑 OpenAI 渠道，在“渠道额外设置”的“Moonshot 上游模式”中选择“Kimi 兼容透传”。对应的渠道 `setting` 为：

```json
{
  "moonshot_facade_mode": "kimi_passthrough"
}
```

为避免改变 KVV 可观察到的请求或响应，启用该模式时不能同时配置：

- 强制格式化响应
- 将思考内容转换到正文
- 渠道系统提示词或系统提示词拼接
- 参数覆盖
- HTTP 状态码映射

请求头覆盖和模型映射仍可使用；模型映射只修改顶层 `model`。

未配置该字段或设置为 `emulate` 时，行为保持不变：`/moonshot` 会把普通 OpenAI 上游转换为 Moonshot 兼容响应。这种模拟模式不等同于原生 Kimi 能力，不能据此判断 KVV 已通过。

## 验证方法

先对上游地址直连运行同一套 KVV，再通过网关的 `/moonshot/v1/chat/completions` 运行。两次结果应在以下能力上保持一致：

1. 非流式和流式响应均保留上游 Kimi 扩展字段。
2. `reasoning_effort=max`、`thinking`、历史思考内容、`messages[].tools` 和 `partial` 不被降级或删除。
3. usage 中的 prompt、completion、cache read/write 和 thinking/reasoning token 统计来自上游，不由网关补造。
4. SSE 数据块内容和结束顺序保持上游语义，网关只统一发送最终 `[DONE]`。
5. 使用模型映射时，上游收到映射后的模型，客户端响应看到原模型名。

只有“上游直连通过、网关入口也通过”才能证明链路支持 KVV。若直连上游失败，应先由上游补齐 Kimi 行为；透传模式无法把不具备 Kimi 语义的 OpenAI 服务升级成 Kimi 原生实现。
