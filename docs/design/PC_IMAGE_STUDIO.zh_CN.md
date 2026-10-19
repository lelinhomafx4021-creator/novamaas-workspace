# PC 图片工作室重构

日期：2026-10-19

新分支 `codex/pc-image-studio-pro` 从公司 `main` 的 `112fb826fd9c395417d05b046f8130a0832bb8eb` 创建。业务基础取自 [PC 生图 PR #69](https://github.com/yeruyi1024/novamaas-workspace/pull/69) 的 `460fc0e7dc181ae581ca18004dde0e61783cb787`，保留模型权限、API Key 计费、提示词优化、幂等重试、OSS 原图、30 天文件保留、素材库副本和图片审查边界。

界面重新设计为独立的深石墨色工作室，使用冷蓝交互色、原创光学装置主视觉和独立语义变量。参考图只用于三栏布局：左侧工作空间、中间画布与底部输入、右侧生成记录。所有配色、背景和组件样式均重新设计，主题变量限定于工作室及其弹窗。保留站点品牌和原有项目归属信息。

桌面画布和记录分别滚动，提示词编辑器固定在中间区域底部；窄屏改为单栏滚动，并在顶部提供新建和记录定位。选择历史记录后，视图和键盘焦点回到画布；新建创作仅清空当前草稿和选择，保留历史。个人记录只显示图像、提示词、模型、日期和状态，完整审查信息仍由审查页承载。

展示层进一步精简：通用布局、间距与响应式规则使用 Tailwind，独立 CSS 从 841 行缩减为 146 行，仅保留主题、组件变体和主视觉规则。访客与登录后的记录空状态共用结构；业务流程不变。响应式断点统一使用 rem，平板网格按内容安排高度，避免输入区与记录区重叠。

## 主视觉

- 文件：`web/public/images/image-studio/orbit.jpg`，1600 × 900，约 355 KiB。
- 使用内置 imagegen 生成，原始 PNG 转为 JPEG 以控制交付体积。
- 原创科技装置插画，用于空画布。示例按钮只填入提示词，不会自动发起付费模型调用。
- 图片生成提示词如下：

```text
Use case: stylized-concept. Asset type: panoramic artwork for the empty canvas of a professional AI image creation studio. Primary request: an elegant futuristic optical sculpture, a precision machined silver titanium torus encircling a luminous icy blue glass core, fine concentric orbital rings and sparse suspended refractive glass fragments. Scene/backdrop: deep near-black graphite studio fading into darkness, a subtle horizon of light and restrained fine star particles. Composition: cinematic 16:9 landscape, sculpture at the right third, left half mostly clean dark negative space for HTML headline, sculpture occupies 60 percent of height, nothing cropped. Style: premium physically based 3D render, sophisticated industrial art direction, realistic brushed metal, luminous caustics and delicate blue edge light, high contrast but calm. Palette: black graphite, silver, ice blue, a tiny hint of warm reflection. Constraints: artwork only, no interface, no lettering, no watermark, no logos, no purple, no green, no cyberpunk city, no people. Make it look like a premium creative software product's editorial artwork, not a generic gradient wallpaper.
```

## 验证

- 前端最终全量 126 个文件、696 项测试通过；相关 5 个文件、40 项测试也已单独通过。
- 本机 Node 的 Web Storage 全局对象会覆盖 jsdom 的 localStorage。测试启动使用 `NODE_OPTIONS=--no-experimental-webstorage bun run test --maxWorkers=3`，不修改业务代码。
- `bun run typecheck`、受影响文件 lint、生产构建通过。
- Go 的 model、storage、imagestudio、controller、router、authz、common 和 relay/channel 八个包回归通过；根模块与 relaykit 的独立构建通过。
- 实际浏览器检查 1440 × 900 桌面、1024px 平板和 375px 窄屏，无横向溢出。1440 × 900 下示例卡完整位于画布内，输入区保持可见；平板的记录区位于完整编辑器之后。
- 复用 PC PR 已有的 MySQL 数据验证管理员登录、模型读取、两条生成记录、历史选择、1254px OSS 原图加载及预览弹窗。本轮未新增或删除业务图片记录。

本地凭据、数据库连接和验证截图不进入提交；原工作目录及 PR #69 均保留。
