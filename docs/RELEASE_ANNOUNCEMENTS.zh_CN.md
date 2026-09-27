# 发版公告视觉与文案规范

## 本期主张

**主题：从素材接入到移动经营，让 AI 服务更容易交付和推广。**

| 能力 | 上线状态 | 面向客户的价值表达 |
| --- | --- | --- |
| 素材库 | 已上线 | 沿用火山方舟素材 Action API 的请求形态，使用平台 AK/SK 与地址接入，降低已有集成的迁移改造量；在平台管理素材、渠道副本及视频生成调用。实际支持范围以[接口说明](ASSET_LIBRARY_API.zh_CN.md)为准。 |
| 微信小程序 | 发版预告 | 在手机上查看 Token 用量、任务和账户动态，在客户沟通现场展示模型目录及服务入口。正式开放时间另行公告。 |

表达顺序固定为：**状态 → 可执行的能力 → 商业收益 → 适用边界**。已上线和即将上线必须使用不同的状态标签，避免把预告写成可立即使用的功能。不承诺零改造迁移、自动获客或尚未开放的支付能力。

## 固定视觉语言

- **基调**：温暖、可信的工作场景卡通插画。用人物动作解释产品用途，不以抽象科技装置代替功能本身。
- **人物与构图**：固定为两位圆头简笔人物在同一工作台协作。左侧人物执行“接入、整理、复用”，右侧人物执行“查看、展示、经营”；画面须能让人不用阅读文字也看出动作与结果。业务道具占据画面中心横带，适合裁切成弹窗横幅。
- **色彩**：暖纸白 `#f7f1e7` 为底，炭黑 `#272524` 画轮廓，柔和钴蓝 `#3d6eae` 表示平台操作，鼠尾草绿 `#7e9b69` 表示流转，陶土橙 `#c8734f` 表示移动触达。后续图片沿用这一组色彩。
- **笔触**：手绘墨线、平面纸片色块与轻微纸张纹理。人物简练有表情，办公道具清晰具体；保持成熟的商业插画质感，避免幼儿贴纸、科幻霓虹和复杂写实界面。
- **文字**：生成图内不放文字、数字、Logo 或水印。标题、状态标签和按钮放在图片外侧的网页元素中，确保多语言、无障碍和后续改版可控。
- **尺寸**：推荐约 16:9、至少 1600 × 900 的源图；核心人物和动作须能裁切为约 2.5:1 的弹窗横幅。导出 PNG/WebP，尽量控制在 2 MiB 内。

### 可复用生成提示词

每次发版替换方括号中的业务主体，保留画面语言：

```text
Use case: ads-marketing. Asset type: wide B2B SaaS release illustration, 16:9 landscape.
Scene: exactly TWO friendly round-headed stick-figure colleagues collaborate at one bright shared worktable. On the left, one person visibly [does the new product task] with recognizable objects. On the right, the other person receives the result and visibly [uses it for the business outcome]. Show an intelligible left-to-right action, not an abstract symbol.
Style: sophisticated warm editorial cartoon, expressive simple stick-figure bodies, hand-drawn charcoal ink outlines, flat cut-paper shapes, subtle paper texture; mature commercial illustration.
Palette: warm ivory #f7f1e7, charcoal #272524, muted cobalt #3d6eae, sage #7e9b69, terracotta #c8734f.
Composition: keep both figures and functional objects in the center horizontal band; make the story readable after a centered crop to 2.5:1. Place title outside the illustration in HTML.
No logos, words, letters, numerals, watermarks, extra people, sci-fi devices, neon effects, generic AI brains, or money symbols.
```

本期图片位于 `web/public/releases/asset-library-miniapp-preview.png`。下一期沿用相同视觉语言，替换业务主体并使用新的文件名与版本标识。

## 发布与展示规则

系统设置 → 内容 → 公告可新增“发版更新”：填写标题、正文、发布日期、唯一版本标识，按需填写 HTTPS 图片地址或站内 `/...` 路径。图片和正文会一起出现在登录后的发版弹窗；未到发布日期的公告不会展示。每个账号在同一浏览器中，每个版本标识只提示一次。后续有实质性新发版时必须换一个版本标识；修改同一标识的文案不会再次弹出。

本期素材库与小程序预告作为内置发版卡片发布。违规提醒由服务端按用户和自然日记录确认：最近七天有违规记录时每天最多弹出一次；发版卡片与待确认的违规提醒会合并到同一弹窗。普通系统公告仍保留在通知中心，违规提醒弹出时也会展示。发版已读状态保存在当前浏览器，清理浏览器数据或换设备后可能再次看到发版卡片。
