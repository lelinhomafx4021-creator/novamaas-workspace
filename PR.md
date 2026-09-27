# ⚠️ 提交说明 / PR Notice
> [!IMPORTANT]
>
> - 本 PR 由维护者整理，代码实现与验证过程使用了 AI 辅助；合并前请由维护者完成最终审查。

## 📝 变更描述 / Description

新增管理员可用的供应商验收工作台，用于直接验证上游模型、缓存、流式响应、并发能力以及视频任务提交与查询。测试请求不经过业务 Channel、relay 或计费链路。

同时补充正式视频任务的轮询历史：复用现有请求体归档表记录状态、HTTP 状态码、错误和响应 JSON；内容变化时新增记录，完全相同时累计次数；大字段会截断，记录保留 90 天，归档失败不会影响任务轮询、退款或结算。管理员可在任务日志详情中分页查看时间线。

本次整理还完成以下修正：

- 供应商测试后端接口、前端路由和侧栏入口统一使用管理员权限；系统侧栏模块开关可按部署需要启用或关闭该入口。
- 撤回对后端用户默认侧栏权限结构的修改，减少对现有权限模型的侵入。
- 流式响应支持保活，并识别缺少 `finish_reason` / `[DONE]` 的异常提前结束。
- 并发压测使用统一起跑点，进度按完成数单调递增，避免并发回调导致界面进度倒退。
- 已合入公司最新 `main`，PR 分支未修改公司现有 CI/CD 工作流。

## 🚀 变更类型 / Type of change

- [x] 🐛 Bug 修复 (Bug fix)
- [x] ✨ 新功能 (New feature)
- [x] ⚡ 性能优化 / 重构 (Refactor)
- [x] 📝 文档更新 (Documentation)

## 🔗 关联任务 / Related Issue

- Closes #（如有请补充）

## 🧭 与上游关系 / Upstream Relationship

- 与上游关系：NovaMaaS 下游专属。
- 差异摘要：新增供应商验收工具和视频轮询诊断记录；不改变现有业务转发、渠道路由与计费语义。

## ✅ 提交前检查项 / Checklist

- [ ] **人工确认:** 维护者已审阅代码和本文，并确认行为符合预期。
- [ ] **非重复提交:** 维护者已确认没有重复 Issue 或 PR。
- [x] **变更理解:** 权限边界、流式生命周期、并发统计和轮询归档路径已逐项检查。
- [x] **范围聚焦:** 未修改公司现有 CI/CD 工作流。
- [x] **上游同步:** 分支已包含公司最新 `main`；本 PR 没有移植独立上游功能提交。
- [x] **本地验证:** 后端、前端、竞态检查和生产构建均已通过。
- [x] **安全合规:** 未提交敏感凭据；供应商测试仅管理员可用，并可通过侧栏模块配置关闭入口。

## 📸 运行证明 / Proof of Work

- `go test ./...`：通过。
- `go test -race ./service/suppliertest ./controller`：通过。
- `bun run typecheck`：通过。
- 供应商测试与任务日志相关前端测试：18 个文件、75 个用例通过。
- 改动文件 `oxlint`：通过。
- `bun run build`：通过。

建议标题：`feat: add admin supplier testing and video polling diagnostics`

来源分支：`lelinhomafx4021-creator/novamaas-workspace:feat/supplier-test-pr`
