# 视频创作画布：最终报告

分支 `codex/video-studio-canvas`，基于 company/main，14 个提交。范围是第一期 PC 端加共用后端，小程序不在本期。

## 一 交付内容

顶部导航新增“视频创作”，进入后是一块无限画布。打开页面自动进入最近的项目，没有项目就自动创建。

用户流程：添加生成节点，选 API 密钥和模型，写提示词，可附参考图，点“生成”。节点右侧出现结果节点，显示状态，完成后内嵌播放和下载。可对提示词做一键优化。页面在桌面、平板、手机上都能用。

## 二 关键设计

| 主题 | 做法 |
| --- | --- |
| 提交链路 | 画布请求先转成标准 `/v1/video/generations` 请求体，再走原有的渠道选择、重试、计费、适配器。对外 `/v1/videos` 契约不变，有回归测试。 |
| 适配器 | 新增可选接口 `TaskSubmitResponseParser`，sora、doubao、ali 各自实现“只解析”。原 `DoResponse` 等于解析加写响应。 |
| 幂等 | 每次点“生成”新建 `client_request_id`，（用户，请求号）唯一索引。同请求重放返回原记录，同号不同内容返回 409。哈希取自客户端原始输入。 |
| 密钥 | 节点保存 `token_id`（密钥编号，不是密钥本身）。服务端按“编号加当前用户”查，取其分组和模型白名单，提交时在渠道分发处再校验。别人的密钥返回 404，禁用或过期返回 403。 |
| 模型规则 | 后台选项 `video_studio.model_rules`：名字关键字对应协议，按顺序第一条命中。默认 `seedance`、`wan`、`h3`。加新模型家族只需加一条规则。 |
| 提示词优化 | 后台选项 `video_studio.prompt_model` 指定文本模型，走现有聊天接口，按所选密钥的分组计费。留空则不显示按钮。 |
| 存储 | 参考图存为 `StorageObject`（用途 `video_studio_reference`），默认临时保存，可手动长期保存。按 sha256 去重，有配额。注册 Seedance 素材时复用同一对象。生成的视频不转存。 |
| 项目保存 | 乐观锁，冲突返回 409 并停止自动保存。草稿上限 60 KiB，保证落在 MySQL 的 TEXT 内。 |

### 参数

| 模型 | 协议 | 分辨率 | 参考图 |
| --- | --- | --- | --- |
| Seedance 2.0 | seedance | 480p、720p、1080p、4k | 最多 9 张，可用素材库 |
| Seedance 2.0 fast | seedance | 480p、720p | 同上 |
| Seedance 2.5 | seedance | 480p、720p、1080p | 同上 |
| 万相 3 | wan3 | 480P、720P | 最多 9 张 |
| MiniMax-H3 | openai_video | 768P、1080P | 本期仅文生视频 |

Seedance 的分辨率取自仓库定价表，480p 按 720p 计价。万相和 H3 的分辨率由你提供。比例沿用通用列表，时长上限用 `relaycommon.MaxTaskDurationSeconds`，万相为 2 到 30 秒。

## 三 规模

对比 company/main：80 个文件，新增 6884 行，删除 95 行。

| 部分 | 行数 |
| --- | --- |
| Go 生产代码 | 1915 |
| Go 测试 | 695 |
| 前端源码（不含语言文件和生成的路由树） | 3546 |

后端新增 `model/video_studio.go`、`service/videostudio/`、`service/storage/video_studio.go`、`controller/video_studio.go`、`setting/model_setting/video_studio.go`。修改处集中在 `controller/relay.go`（抽出 `submitVideoTask`）、三个适配器、`middleware/distributor.go`、`service/assetlibrary/service.go`、`model/option.go`。前端在 `web/src/features/video-studio/`，另有后台设置卡片和导航开关。

## 四 核查结果

| 检查 | 结果 |
| --- | --- |
| Go 构建、`relaykit` 独立构建、`go vet`、`gofmt` | 通过 |
| Go 测试（controller、service、relay、model、middleware、setting、pkg） | 通过 |
| 视频创作的 8 个控制器测试 | SQLite 和本地 MySQL 都通过 |
| 前端类型检查、oxlint | 通过 |
| vitest | 112 个文件、595 个用例通过 |
| 前端生产构建 | 通过 |
| 浏览器 | 桌面、平板（820px）、手机（390px）无横向溢出；配置区折叠、按密钥收窄模型已点过 |
| 代码自查 | 无 TODO、无调试输出、无 `encoding/json` 直接调用、无跨库风险写法 |

精简：内联了只有一处调用的 `ValidateVideoStudioPromptModel`；把“切换模型保留哪些参数”从组件挪进 `lib/draft.ts` 并补测试；`properties-panel` 拆出 `model-config` 后回到 206 行。其余只有一处调用的函数是 controller、service、model 分层之间的导出函数，或带专属规则的函数，保留。

## 五 已知限制

1. 生成的视频不转存。上游地址过期后视频打不开，记录和节点还在。
2. 没有用真实渠道和 OSS 联调。提交、计费、状态流转用模拟上游验证，真实视频的播放、跨域、格式兼容没有人看过。
3. 万相适配器只转换以 `wan3.0` 开头的模型名（`relay/channel/task/ali/adaptor.go:151`）。后台规则可让别的万相名字走万相协议，但适配器不会转换，可能被上游拒绝。
4. 万相适配器本身仍接受 1080P，画布里只提供 480P、720P。
5. 关键字过短会误命中，例如 `h3`、`wan` 出现在别的模型名里。可在后台改长。
6. 没有“失败后自动换 v1 重发”，因为换格式重试可能重复生成、重复扣费。想走通用 v1，把模型关键字设为 openai_video。
7. 旧草稿里的 `group` 字段换成了 `token_id`，旧项目的分组按默认分组处理。
8. 密钥选择器只取前 100 把。
9. 迁移只在 SQLite 和 MySQL 验证过，PostgreSQL 没跑。
10. 画面比例和 Seedance 以外的时长范围没有对照厂商文档核实。
11. `reference-panel.tsx` 282 行，超过约 200 行的建议，没拆。

## 六 上线前待办

| 事项 | 说明 |
| --- | --- |
| 真实渠道端到端 | Seedance、万相、H3 各提交一次，确认播放、下载、计费、失败退款 |
| PostgreSQL | 补跑迁移和视频创作测试 |
| 推分支、开 PR | 用仓库 PR 模板并注明 AI 辅助。README 下游差异清单需补一条，要等有真实 PR 链接 |
| 万相命名 | 确认实际模型名后决定是否放宽适配器的前缀限制 |

后续可选：视频转存到 OSS（默认关闭的策略）、H3 参考图（需 multipart）、沉浸模式、小程序端、拆分 `reference-panel.tsx`。
