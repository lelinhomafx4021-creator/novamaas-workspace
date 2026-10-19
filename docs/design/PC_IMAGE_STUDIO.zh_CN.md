# PC 图片工作室重构

日期：2026-10-19

新分支 `codex/pc-image-studio-pro` 从公司 `main` 的 `112fb826fd9c395417d05b046f8130a0832bb8eb` 创建。业务基础取自 [PC 生图 PR #69](https://github.com/yeruyi1024/novamaas-workspace/pull/69) 的 `460fc0e7dc181ae581ca18004dde0e61783cb787`，保留模型权限、API Key 计费、提示词优化、幂等重试、OSS 原图、30 天文件保留、素材库副本和图片审查边界。

界面重新设计为独立的深石墨色工作室，使用冷蓝交互色、原创光学装置主视觉和独立语义变量。当前采用创作区和 300px 生成记录两栏，新建入口在顶部。空画布的引导、灵感按钮和编辑器统一居中，内容宽度最多 960px；主视觉融入背景，灵感示例改为紧凑按钮。所有配色、背景和组件样式均重新设计，主题变量限定于工作室及其弹窗。保留站点品牌和原有项目归属信息。

桌面查看结果时，画布和记录分别滚动，编辑器保持在画布下方，图片按剩余画布高度缩放。窗口高度不足 760px 时，创作区整体滚动，避免压缩结果区域；空画布同时压缩主视觉和上下间距。窄屏改为单栏，灵感按钮按两列排列，顶部提供新建和记录定位。选择历史记录后，视图和键盘焦点回到画布；新建创作仅清空当前草稿和选择，保留历史。输入框支持收起、放大和拖动调整高度，切换时保留草稿和参考图；收起后显示一行草稿摘要，并紧邻引导区域。个人记录只显示图像、提示词、模型、日期和状态，完整审查信息仍由审查页承载。

通用布局、间距与响应式规则使用 Tailwind，独立 CSS 仅保留主题、组件变体、主视觉和画布高度约束。删除左侧导航、说明卡、重复画布工具栏、灵感长段落及其冗余展示断言；访客与登录后的记录空状态共用结构。窄屏按内容安排高度，避免输入区与记录区重叠。

参考图支持文件选择、拖入本地文件或网页图片，最多 8 张，每张不超过 20 MiB，格式为 PNG、JPEG 或 WebP。历史缩略图拖入时使用完整原图链接。图片进入私有 OSS 的独立临时对象，7 天后由现有存储清理队列删除。生成请求仅提交对象 ID；服务端校验归属、状态和有效期后重新签名，通过通用图片请求的 `image` URL 数组转发，继续使用既有权限、渠道和计费链路。参考图移除只解除本次引用，生成结果仍按 30 天规则保留。

## 主视觉

- 文件：`web/public/images/image-studio/orbit.jpg`，1600 × 900，约 355 KiB。
- 使用内置 imagegen 生成，原始 PNG 转为 JPEG 以控制交付体积。
- 原创科技装置插画，用于空画布。示例按钮只填入提示词，不会自动发起付费模型调用。
- 图片生成提示词如下：

```text
Use case: stylized-concept. Asset type: panoramic artwork for the empty canvas of a professional AI image creation studio. Primary request: an elegant futuristic optical sculpture, a precision machined silver titanium torus encircling a luminous icy blue glass core, fine concentric orbital rings and sparse suspended refractive glass fragments. Scene/backdrop: deep near-black graphite studio fading into darkness, a subtle horizon of light and restrained fine star particles. Composition: cinematic 16:9 landscape, sculpture at the right third, left half mostly clean dark negative space for HTML headline, sculpture occupies 60 percent of height, nothing cropped. Style: premium physically based 3D render, sophisticated industrial art direction, realistic brushed metal, luminous caustics and delicate blue edge light, high contrast but calm. Palette: black graphite, silver, ice blue, a tiny hint of warm reflection. Constraints: artwork only, no interface, no lettering, no watermark, no logos, no purple, no green, no cyberpunk city, no people. Make it look like a premium creative software product's editorial artwork, not a generic gradient wallpaper.
```

## 验证

- 前端最终全量 126 个文件、699 项测试通过，包含工作室的 20 项流程测试，覆盖收起后的草稿摘要和新建草稿清除校验状态。
- 本机 Node 的 Web Storage 全局对象会覆盖 jsdom 的 localStorage。测试启动使用 `NODE_OPTIONS=--no-experimental-webstorage bun run test --maxWorkers=1`，不修改业务代码。
- `bun run typecheck`、受影响文件 lint、生产构建通过。
- Go 的 model、storage、imagestudio、controller、router 五个相关包回归通过；根模块构建通过。
- 实际浏览器检查 1440 × 900 桌面、1440 × 700 短窗口和 375px 窄屏，无横向溢出。桌面的主视觉、灵感和编辑器共享宽度和间距，收起后保持相邻；结果预览完整位于可用画布内。窄屏的记录区位于完整编辑器之后。
- 复用本地 MySQL 验证管理员登录、模型读取和历史记录；文件上传与网页图片拖入均成功落到 OSS，1600px 参考图预览正常，存储账本的到期时间确认为创建后 604800 秒。实际生图调用中，本地 API 令牌验证通过，上游返回 `Invalid token`，因此未验证到真实成图；未修改密钥和渠道配置。

本地凭据、数据库连接和验证截图不进入提交；原工作目录及 PR #69 均保留。
