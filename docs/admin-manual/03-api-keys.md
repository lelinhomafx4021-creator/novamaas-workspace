# 账户与平台密钥

管理目标：统一账户资料与订阅身份，明确模型调用的凭据、权限和费用归属。账户登录和模型配置已有源码依据，完整桌面操作及实际消费仍待联调。

## 一、身份与凭据边界

| 凭据或资料 | 用途 | 管理要求 |
| --- | --- | --- |
| 账户登录会话 | 读取本人资料、订阅及拟建的安装计划，提交设备结果 | 使用当前认证账号，换号隔离本人数据 |
| 用户模型密钥 | 获取可用模型并发起模型调用 | 校验归属、状态、期限、预算、分组及模型限制 |
| 上游供应商密钥 | 聚合平台连接供应商 | 由平台保管，不下发给星途 |
| 插件私有配置 | 连接器和插件自身所需凭据 | 沿用本机受保护配置，不进入全局目录或设备回报 |

当前登录不会自动领取模型密钥；退出账户时现有模型密钥仍保留，换号须单独核对密钥所有者。模型费用归实际密钥所属账户，不能仅按星途登录账号判断。自动领取和随账号绑定密钥属于后续方案。

插件使用权益、模型密钥有效性和调用额度分别校验。涉及模型调用的插件还须满足全局启用、版本、本机开关及对应使用条件。

## 二、平台准备与用户配置

1. 平台维护上游渠道、模型映射、用户分组和计费规则，确认目标模型具备可用来源。
2. 为目标用户准备模型密钥，核对权限及额度；管理员登录凭据和上游供应商密钥由平台保管。
3. 用户在星途“设置 → 模型与平台”保存平台密钥，客户端校验并自动同步模型目录及默认模型；再次保存同一密钥也会重新校验。
4. 在对话区选择同步模型完成实际调用，回查平台记录，确认调用结果、用量及费用归属。

账户登录位于星途侧栏，订阅通过账户菜单查看。平台连接地址由产品配置统一管理，普通用户无需填写供应商地址。当前其他供应商路由已停用并保留兼容记录，后续适配应保持现有边界。

## 三、已有账户接口字段

星途已有主进程账户适配，继续复用其独立会话和 Cookie 存储。以下是当前平台协议，Access Token 不作为模型密钥使用。

| 接口 | 请求字段 / 凭据 | 成功结果 |
| --- | --- | --- |
| POST /api/user/login | JSON `username:string`、`password:string`，必填 | 普通登录返回会话包；启用二次认证时先返回挑战 |
| POST /api/user/login/2fa | JSON `flow_token:string`、`code:string`，必填 | 验证成功后返回会话包；flow_token 不能作为访问凭据 |
| POST /api/user/auth/refresh | Refresh Cookie；`X-Auth-Session:string` 指明预期 sid | 轮换 Refresh Cookie，返回新的会话包 |
| POST /api/user/auth/logout | 当前会话及预期 sid，沿用现有适配 | 撤销当前会话并清理相应 Cookie |
| GET /api/user/self | Authorization: Bearer 用户 Access Token | `data` 为本人资料，不含新模型密钥 |

短信登录已有独立 challenge_token 流程，不能将手机号和验证码填入密码登录字段；其他登录方式继续沿用现有账户适配，不在本期重做。

| 会话包 `data` 字段 | 类型 | 返回条件与含义 |
| --- | --- | --- |
| `access_token` | string | 正式登录或刷新成功必有，仅存于受保护会话层 |
| `token_type` | string | 正式会话必有，Bearer |
| `access_expires_at` | integer | 正式会话必有，Unix 秒 |
| `session.sid` | string | 正式会话必有，用于预期会话校验，不是 user.id |
| `session.current` | boolean | 当前会话标记 |
| `session.login_method` | string | 实际登录方式 |
| `session.created_at` / `last_active_at` / `expires_at` | integer | Unix 秒 |
| `session.ip` / `user_agent` | string | 会话设备记录，不作为可信设备认证依据 |
| `user.id` / `username` / `display_name` | integer / string / string | 正式会话的本人身份及显示资料 |
| `require_2fa` | boolean | 密码校验后的挑战响应为 true，此时尚未登录完成 |
| `flow_token` / `expires_at` / `sms_available` | string / integer / boolean | 挑战分支返回的凭证、Unix 秒期限和短信能力，不与正式会话字段混用 |

刷新凭据通过 Set-Cookie 返回，不在 JSON 中返回 refresh_token。账户相关失败可使用顶层 `code`，如 AUTH_TOKEN_EXPIRED、AUTH_SESSION_REVOKED、AUTH_UNAUTHORIZED；与总览中拟新增的 error.code 分别解析。并发刷新交给现有会话层串行处理，换号后拒收旧账号迟到结果。

本人资料中本期使用 `id:integer`、`username:string`、`display_name:string`、`role:integer`、`status:integer`、`group:string`、`quota:integer`、`used_quota:integer`、`request_count:integer`。quota 是钱包原始额度，不是订阅剩余量；其他平台账户字段按现有接口保留，不需要全部复制到星途界面。

## 四、已有模型目录与密钥字段

| 接口 | 凭据 | 字段与处理 |
| --- | --- | --- |
| GET /v1/models | 当前模型 API Key | 顶层 `object:string`、`data:object[]`；每项 `id:string` 为模型实际调用名，模型目录不含完整参数规格 |
| GET /api/usage/token/ | 当前模型 API Key | 成功顶层为 `code:true`、message、data，不能只检查 success |
| GET /api/token/ | 当前用户 Access Token | 管理本人密钥的分页资料；返回的 key 为掩码，不能用于模型调用 |
| POST /api/token/:id/key | 当前用户 Access Token | 校验本人归属后 `data.key:string` 返回真实密钥；读取成功不代表客户端已自动配置 |

用量接口 `data` 字段如下，均为已有返回字段：

| 字段 | 类型 | 含义与单位 |
| --- | --- | --- |
| `object` / `name` | string | token_usage / 密钥名称 |
| `total_granted` / `total_used` / `total_available` | integer | 总、已用、剩余原始 quota，均为此密钥预算，不等于钱包或订阅总余额 |
| `unlimited_quota` | boolean | true 时不按 total_available=0 判断预算耗尽，仍有账号、模型及账务限制 |
| `model_limits_enabled` | boolean | 是否启用模型限制 |
| `model_limits` | object | 可用模型限制映射，按实际结构读取，不当作模型详情数组 |
| `expires_at` | integer | Unix 秒，0 表示无到期限制；与旧 dashboard/billing 的毫秒字段不同 |

星途继续采用“用户明确保存模型密钥 → 校验 → 同步模型”的流程。管理本人密钥接口可作为后续授权选择入口，当前不自动创建、读取或绑定某条密钥。掩码、登录令牌和上游凭据都不能填入模型配置；日志、安装计划及设备回报均不包含真实密钥。

## 五、异常处理与验收

| 场景 | 预期处理 |
| --- | --- |
| 密钥无效或模型目录读取失败 | 提示失败并保留原有效配置 |
| 模型目录为空或格式异常 | 拒绝保存为可用平台配置 |
| 主动清空密钥 | 清除当前平台调用和模型可用状态，保持旧供应商路由停用 |
| 退出或切换账号 | 明确当前模型密钥归属，隔离本人订阅及待执行安装计划 |
| 重启客户端 | 按受保护配置恢复有效模型设置，核对默认模型可用性 |

验收包括正确密钥、错误密钥、主动清空、默认模型、重启保留及真实调用归属。模型目录可见仅代表发现模型，不自动证明视觉、附件、工具或媒体协议已经适配。完整调用验收见[文本模型对接](/admin-manual?article=text-models)。
