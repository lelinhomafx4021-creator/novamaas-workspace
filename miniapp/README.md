# 微信小程序

这是当前仓库中的独立 Taro 4 + React 18 小程序应用。它复用现有 Go API 和业务规则，但不导入 `web/src` 的 DOM 组件或 React 19 运行时。

长期计划、任务编号、依赖关系与验收证据见 [`docs/design/WECHAT_MINIAPP_PROJECT_PLAN.zh_CN.md`](../docs/design/WECHAT_MINIAPP_PROJECT_PLAN.zh_CN.md)。

## 本地启动

要求：Bun、微信开发者工具，以及正在运行的本仓库 Go 后端。

```bash
cd miniapp
bun install
MINIAPP_API_BASE_URL=http://127.0.0.1:3000 bun run dev:weapp
```

然后在微信开发者工具中导入 `miniapp/`。开发骨架使用 `touristappid`，需要登录、真机、合法域名或支付能力时，在本机的 `project.private.config.json` 中配置真实 AppID；该文件不会提交。

`MINIAPP_API_BASE_URL` 会在构建时写入客户端，只能放公开的 API 基地址，不能放任何 Secret。正式环境必须使用已在微信公众平台配置的 HTTPS 合法域名。

## 验证

```bash
bun run typecheck
bun run lint
bun run test
MINIAPP_API_BASE_URL=http://127.0.0.1:3000 bun run build:weapp
```

构建产物位于 `dist/`。首页调用公开的 `/api/status` 验证平台连接；当前已覆盖微信登录与账号连接、首页概览、模型目录、API Key、流式对话、用量/任务/账单、钱包与充值码、订阅余额购买、账户语言、登录会话、签到推广和隐私注销。微信支付 V3 必须等待运营方提供主体资质、商户配置和合规结论，不会使用 WebView 或客户端入账方式绕过。

后端管理员需要通过现有系统设置能力配置以下键，并在最后启用登录：

```text
wechat_miniapp.app_id
wechat_miniapp.app_secret
wechat_miniapp.request_timeout_seconds
wechat_miniapp.enabled
```

也可以在仓库根目录 `.env` 或部署环境中设置以下变量。显式环境变量优先于数据库系统设置；系统环境变量又优先于 `.env` 中的同名值：

```dotenv
WECHAT_MINIAPP_APP_ID=wx_your_app_id
WECHAT_MINIAPP_APP_SECRET=replace_with_your_app_secret
WECHAT_MINIAPP_ENABLED=true
WECHAT_MINIAPP_REQUEST_TIMEOUT_SECONDS=5
```

AppSecret 只保存在服务端设置中。公开状态接口只返回 `wechat_miniapp_login` 就绪布尔值；小程序不会收到 AppSecret、OpenID 或微信 `session_key`。

## 目录

```text
config/             Taro 构建配置
src/api/            API 地址、请求封装和接口适配
src/components/     小程序轻量共享组件
src/hooks/          页面级 Taro/React Hook
src/i18n/           七语言资源与语言检测
src/auth/           版本化本地会话、微信登录和绑定/注册适配
src/account/        账户偏好解析与高风险确认规则
src/pages/          五个 Tab 页面及功能详情页面
```

## 边界

- Web 与小程序依赖、锁文件和构建完全隔离。
- 跨端共享优先使用后端 API 契约；未来确需共享 TypeScript 时，只能放无运行时依赖的纯类型。
- 身份、计费、权限、限流和支付最终状态都以后端为准。
