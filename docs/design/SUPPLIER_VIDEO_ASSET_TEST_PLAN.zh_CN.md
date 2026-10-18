# 视频测试与官方产物验证：一份报告、两个章节

日期：2026-10-18。基于 `main` 实现，按最新确认收敛执行与报告逻辑。

## 执行流程

```text
准备素材 → 提交一个视频请求 → 记录 Task ID
        → 首次排队返回（一次）→ 首次执行中返回（一次）→ 视频最终返回
        → 使用独立官方 AK/SK 提交视频 URL → 记录 QueryID → 官方验证结果
```

后台继续轮询至终态。相同状态的后续返回不再进入报告；整份报告只有一条轮询汇总，分别列出生成和验证的查询次数及查询总耗时。没有实际观察到排队或执行中状态时标为未记录，不补造状态。超时、取消或错误保留实际诊断并注明未取得终态。

## 第一部分：视频生成

- 实际发送的请求体与提交回执，包含 Task ID、接口、HTTP 状态及请求 ID。
- 首次排队、首次执行中返回，各保留一份。
- 最终查询响应及视频地址。GET 查询注明无请求体；运行中响应不能标为最终成功。
- 首次状态各保留一次，重复轮询只进入共用汇总；素材引用保留在实际请求中。

## 第二部分：官方产物验证

- 关联同一次生成的 Task ID、视频 URL。
- `CreateArkOfficialResultQuery` 的实际请求体、回执及 QueryID。
- `GetArkOfficialResult` 的实际请求体及最终返回。
- 来源特征命中和模型匹配分别展示；别名缺少可靠映射时标为无法确认。

直接把生成返回的原视频 URL 交给火山官方，不增加视频下载、转存、哈希或额外产物处理阶段，也不要求配置视频暂存策略。补查已有 QueryID 只查验证结果，不重新生成视频或重复创建已受理查询。

```text
POST https://ark.cn-beijing.volcengineapi.com/?Action=CreateArkOfficialResultQuery&Version=2024-01-01
{"Type":"ContentURL","ContentURL":"生成返回的原视频 URL"}

POST https://ark.cn-beijing.volcengineapi.com/?Action=GetArkOfficialResult&Version=2024-01-01
{"QueryID":"已受理的查询 ID"}
```

官方 AK/SK 单独配置并加密保存；设置接口不回显 Secret。调用固定官方端点、禁止凭据随重定向泄漏，按 AK 限流。创建已尝试但无 QueryID 时保留实际回执，不自动重发无法确认是否受理的请求。

## 素材与导出

素材库只用一个默认分组：有则复用，无则自动创建。无需选择分组，不新增按测试、任务或场景划分的隔离。沿用已有素材上传、查询、预览和生成时渠道映射能力。支持 URL、Base64、平台素材 ID 与有序多图；按首帧、首尾帧、多图参考校验用途和数量。

只生成一份完整报告，视频生成和官方产物验证为其中两个连续章节，共用标题、任务信息、三项结论和轮询汇总，不提供报告切换或拆分导出。复用文本报告的品牌页眉、A4 样式、摘要卡和打印逻辑，一次复制 Markdown 或导出 HTML、PDF 即包含两部分。同一份固定执行记录是所有导出的数据源；修改表单不会改变已执行请求。常规结果目标为 1–3 页：压缩标题、摘要卡、元信息和章节间距，JSON 只去除格式空白，不删字段、不损失大整数、false、0 或字符串空白。长 JSON 仍完整跨页展示，没有固定高度、滚动裁切或页数截断。

报告顶部增加“兼容 v3 接口”和“支持素材库”两个独立人工勾选项，默认不勾选，不根据接口路径、素材或自动检查结果推断；仅勾选的说明进入报告，两项都未勾选时整行隐藏。按当前执行记录分别保留勾选状态，Markdown、HTML 和 PDF 保持一致。

普通 JSON 直接展示。Base64 在正文中显示类型、大小和 SHA256，完整原文附件复用已有私有请求快照存储与权限检查；该附件存储不阻塞视频生成和官方验证。凭据与 URL 签名值脱敏。没有原始提交记录的 Task ID 查询明确标注缺失。

服务端保留现有 SSE 和手工查询接口；SSE 只发送新增或变化的记录。报告用 TEXT JSON 保存，MySQL 使用 LONGTEXT，兼容 SQLite／PostgreSQL TEXT。管理员可通过以下接口读取固定记录与附件：

- `GET /api/supplier-test/video/runs/:id`
- `GET /api/supplier-test/video/runs/:id/evidence`
- `GET/PUT /api/supplier-test/video/verification-settings`
- `POST /api/supplier-test/video/assets/query`
- `POST /api/supplier-test/video/assets/upload`

## 验证范围

自动化用例覆盖首次排队／执行中去重、轮询次数、提交与终态原文、显式零值、多素材和默认分组、QueryID 补查不重建、失败与缺字段、记录归属和密钥保护、一份报告包含两部分的三种导出以及文本报告回归。执行前端类型检查、lint、构建和长 JSON PDF 分页检查，并补齐七种语言。

数据库集成测试已有 SQLite 与本地 MySQL 验证；PostgreSQL 需通过 `SUPPLIER_TEST_POSTGRES_DSN` 提供实例，未运行时明确注明。模拟官方协议用例不代表真实火山联调；真实测试需要可用的生成渠道、官方 AK/SK 以及火山可读取的视频 URL。

## 参考

- [供应商测试说明](SUPPLIER_TEST.zh_CN.md)
- [平台素材库接口](../ASSET_LIBRARY_API.zh_CN.md)
- [PDF 品牌与导出规范](PDF_DOCUMENT_BRANDING.zh_CN.md)
- [官方产物验证](https://docs.volcengine.com/docs/ark/verify_modelark_artifact?lang=zh)
