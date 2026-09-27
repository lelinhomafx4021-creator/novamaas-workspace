# 发版公告视觉与文案规范

## 本期主张

**主题：从素材接入到移动经营，让 AI 服务更容易交付和推广。**

| 能力 | 上线状态 | 面向客户的价值表达 |
| --- | --- | --- |
| 素材库 | 已上线 | 沿用火山方舟素材 Action API 的请求形态，使用平台 AK/SK 与地址接入，降低已有集成的迁移改造量；在平台管理素材、渠道副本及视频生成调用。实际支持范围以[接口说明](ASSET_LIBRARY_API.zh_CN.md)为准。 |
| 微信小程序 | 发版预告 | 在手机上查看 Token 用量、任务和账户动态，在客户沟通现场展示模型目录及服务入口。正式开放时间另行公告。 |

表达顺序固定为：**状态 → 可执行的能力 → 商业收益 → 适用边界**。已上线和即将上线必须使用不同的状态标签，避免把预告写成可立即使用的功能。不承诺零改造迁移、自动获客或尚未开放的支付能力。

## 固定视觉语言

- **基准图**：[素材库与小程序发版插图](images/releases/asset-library-miniapp-preview.png)。后续生成应把这张图作为风格参考；替换工作任务和道具，保持人物、线条、纸张质感与叙事方法一致。
- **基调**：温暖、可信的工作场景卡通插画。用人物动作解释产品用途，不以抽象科技装置代替功能本身。
- **人物与构图**：固定为两位圆头简笔人物在同一工作台协作。夸张圆头、简化躯干和四肢、粗细略有变化的深色手绘轮廓、少量有辨识度的表情。左侧人物执行“接入、整理、复用”，右侧人物执行“查看、展示、经营”；用交接动作、箭头或视线形成从左到右的业务流。画面须能让人不用阅读文字也看出动作与结果。业务道具占据画面中心横带，适合裁切成弹窗横幅。
- **色彩**：暖纸白 `#f7f1e7` 为底，炭黑 `#272524` 画轮廓，柔和钴蓝 `#3d6eae` 表示平台操作，鼠尾草绿 `#7e9b69` 表示流转，陶土橙 `#c8734f` 表示移动触达。后续图片沿用这一组色彩。
- **笔触与场景**：手绘墨线、平面纸片色块与轻微纸张纹理；暖光、木桌和少量植物提供真实办公氛围。人物是画面主角；屏幕、卡片、手机等道具可以比人物更具体，但只画出能辨认用途的图形，避免复杂真实界面。保持成熟的商业插画质感，避免幼儿贴纸、科幻霓虹和泛化的 AI 脑图。
- **业务叙事**：一张图只讲一个可见的“操作 → 交付 → 使用结果”过程。本期左侧从素材库挑选图片/视频卡片并交给同事，右侧在手机上查看用量并向客户展示素材。下期根据实际已上线能力替换动作；预告功能要用“正在展示或准备”表达，不画成已经全面可用的承诺。
- **文字**：生成图内不放文字、数字、Logo 或水印。标题、状态标签和按钮放在图片外侧的网页元素中，确保多语言、无障碍和后续改版可控。
- **尺寸与交付**：保留约 16:9、至少 1600 × 900 的无损源图作风格参考；核心人物、手势和主要道具须能裁切为约 2.5:1 的弹窗横幅。网页展示优先从源图导出 WebP，尽量控制在 300 KiB 左右；逐张检查线条、表情和道具，不为追求体积牺牲可读性。

### 可复用生成提示词

每次发版替换方括号中的业务主体，保留画面语言：

```text
Use case: ads-marketing. Asset type: wide B2B SaaS release illustration, 16:9 landscape.
Reference style: match the attached asset-library-miniapp-preview.png illustration's two expressive round-headed colleagues, warm sunlit office, thick imperfect charcoal linework, textured cut-paper color blocks, and specific business props. Do not reproduce its old product story.
Scene: exactly TWO friendly round-headed stick-figure colleagues collaborate at one bright shared worktable. On the left, one person visibly [does the new product task] with recognizable objects. On the right, the other person receives the result and visibly [uses it for the business outcome]. Make the exchange of the object or result the visual center; show an intelligible left-to-right action, not an abstract symbol.
Style: sophisticated warm editorial cartoon, expressive simple stick-figure bodies, hand-drawn charcoal ink outlines with slightly irregular stroke widths, flat cut-paper shapes, subtle paper texture; mature commercial illustration.
Palette: warm ivory #f7f1e7, charcoal #272524, muted cobalt #3d6eae, sage #7e9b69, terracotta #c8734f.
Composition: keep both figures and functional objects in the center horizontal band; make the story readable after a centered crop to 2.5:1. Place title outside the illustration in HTML.
No logos, words, letters, numerals, watermarks, extra people, sci-fi devices, neon effects, generic AI brains, money symbols, or unreadable dense dashboards.
```

下一期沿用这份规范和基准图，先明确当次发版的一个操作及其商业结果，再生成新图片。保留原图作参考，不覆盖历史文件；每次使用新的文件名、OSS 对象路径及版本标识。

## 本期图片交付与 OSS 使用

| 项目 | 信息 |
| --- | --- |
| 已上传展示文件 | [`docs/images/releases/asset-library-miniapp-preview.webp`](images/releases/asset-library-miniapp-preview.webp) |
| 文件名 / MIME | `asset-library-miniapp-preview.webp` / `image/webp` |
| 像素 | 1672 × 941，约 16:9 |
| 压缩图大小 | 227,376 字节，约 222 KiB；比原 PNG 少约 90% |
| 压缩图 SHA-256 | `c9b6e8265fb5f5d48ff7d180893aa0fca2adfda98ab5b347be48615bfb5d6e48` |
| 已上传 WebP 对象键 | `releases/202609_001/asset-library-miniapp-preview.webp` |
| 保留的原图 | [`asset-library-miniapp-preview.png`](images/releases/asset-library-miniapp-preview.png)，2,269,215 字节；SHA-256 `171be83512fcaf2abe4a92e6f7daa44896674735199da3f084f27eb0df741b16` |
| 已上传原图对象键 | `release/202609_001/asset-library-miniapp-preview.png` |

本期压缩图从原 PNG 以 `cwebp -q 88 -m 6 -mt -sharp_yuv -metadata none` 导出，保留原始像素尺寸。下一期先用同样的导出参数制作网页图，再根据实际画面检查画质和文件大小。

由产品负责人上传 WebP 到 OSS，并提供浏览器可长期公开访问的 HTTPS 地址。上传时设置 `Content-Type: image/webp`；使用新的版本化对象键，不覆盖已上传的 PNG 原图或其他发版图片。可设置长期缓存，但换图时必须换文件名或对象键。上线前用无登录浏览器打开地址，确认图片可访问且不会过期。不要将带 `Expires`、`OSSAccessKeyId`、`Signature` 的临时签名链接写入前端构建配置：它会失效，并把临时凭据暴露给所有浏览器。本期[公开 WebP 地址](https://nova-maas-aitoken-public.oss-cn-shenzhen.aliyuncs.com/releases/202609_001/asset-library-miniapp-preview.webp)已验证返回 `image/webp`，内容 SHA-256 与本地文件一致。

本期 PNG 原图已上传到 `nova-maas-aitoken-prod` 存储桶；WebP 压缩图已上传到 `nova-maas-aitoken-public` 存储桶。

内置发版卡片默认从本期公开 WebP 地址加载；图片不再从站内 `/releases/...` 加载。未来发版可在构建 `web/` 时用 `VITE_FEATURED_RELEASE_IMAGE_URL` 指定新的长期有效 HTTPS 地址，覆盖默认图片；同时必须更新版本标识、图片与文案。后台新增的普通发版公告，在“图片地址”字段填入对应 OSS HTTPS 地址即可。

## 发布与展示规则

系统设置 → 内容 → 公告可新增“发版更新”：填写标题、正文、发布日期、唯一版本标识，图片填写已上传 OSS 的 HTTPS 地址。图片和正文会一起出现在登录后的发版弹窗；未到发布日期的公告不会展示。每个账号在同一浏览器中，每个版本标识只提示一次。后续有实质性新发版时必须换一个版本标识；修改同一标识的文案不会再次弹出。

本期素材库与小程序预告作为内置发版卡片发布。违规提醒由服务端按用户和自然日记录确认：最近七天有违规记录时每天最多弹出一次；发版卡片与待确认的违规提醒会合并到同一弹窗。普通系统公告仍保留在通知中心，违规提醒弹出时也会展示。发版已读状态保存在当前浏览器，清理浏览器数据或换设备后可能再次看到发版卡片。
