# ⚠️ 提交说明 / PR Notice
> [!IMPORTANT]
>
> - 请提供**人工撰写**的简洁摘要，避免直接粘贴未经整理的 AI 输出。

## 📝 变更描述 / Description

给管理员加「供应商测试」页：填上游 Base URL、Key、模型，直连对方做基础验收、缓存、压测。不走 Channel / relay / 计费，不新建表。

测完用页面上的判定标准打分，数字可改，表格马上重算，不必重跑。判定只有正常 / 偏慢 / 无法对照。空温度、top_p 不发送。语料原文发送，不做垫字。供应商缺字段就跳过。基础验收只在流式连通性展示模型回复。

**合进公司 `main` 时不要带个人 CD。** 从本 PR 排除：

- `.github/workflows/cd-aliyun.yml`
- `.github/workflows/ci-draft.yml`
- `deploy/aliyun/`
- `PR.md`（本文只作 GitHub 正文，不要进仓库）
- `web/src/i18n/locales/_reports/`

官方文件动过原语句的只有两处，合之前看一眼：

1. `router/api-router.go`：原 `gzip.Gzip(gzip.DefaultCompression)` 换成带 `WithExcludedPaths`，排除 `/api/supplier-test/runs`。其它 API 仍 gzip。下面再挂 `POST /api/supplier-test/models` 和 `/runs`，AdminAuth。
2. `model/user.go`：管理员 / root 默认侧栏 map 整段重写对齐，原有 channel、models 等开关都还在，只多了 `supplier_test: true`。

其余官方挂钩都是加项：侧栏菜单、模块开关、`/supplier-test` 路由、`env.d.ts` 允许读 `.txt`。没有删渠道、用户、计费代码。

新逻辑在 `service/suppliertest/` 和 `web/src/features/supplier-test/`。`git --stat` 行数会被 7 份 i18n 和语料 txt 撑大，审代码看 runner、client、前端工作台即可。

当前 git 用户不是仓库历史核心作者。实现过程有 AI 辅助。

## 🚀 变更类型 / Type of change
- [ ] 🐛 Bug 修复 (Bug fix) - *请关联对应 Issue，避免将设计取舍、理解偏差或预期不一致直接归类为 bug*
- [x] ✨ 新功能 (New feature) - *重大特性建议先通过 Issue 沟通*
- [ ] ⚡ 性能优化 / 重构 (Refactor)
- [ ] 📝 文档更新 (Documentation)

## 🔗 关联任务 / Related Issue
- Closes # (如有)

## 🧭 与上游关系 / Upstream Relationship
- 与上游关系：NovaMaaS 下游专属
- 差异摘要：管理员诊断页，不改变网关转发、计费或渠道运行时语义，不写入 README 关键差异表。

## ✅ 提交前检查项 / Checklist
- [x] **人工确认:** 我已亲自整理并撰写此描述，没有直接粘贴未经处理的 AI 输出。
- [x] **非重复提交:** 我已搜索现有的 Issues 与 PRs，确认不是重复提交。
- [x] **Bug fix 说明:** 若此 PR 标记为 `Bug fix`，我已提交或关联对应 Issue，且不会将设计取舍、预期不一致或理解偏差直接归类为 bug。
- [x] **变更理解:** 我已理解这些更改的工作原理及可能影响。
- [x] **范围聚焦:** 合公司时已排除个人 CI/CD 与无关报告文件；功能代码限于供应商测试及其挂钩。
- [x] **上游同步:** 下游专属，不更新 `UPSTREAM.md`。
- [x] **本地验证:** 已在本地运行并通过测试或手动验证，维护者可以据此复核结果。
- [x] **安全合规:** 代码中无敏感凭据，且符合项目代码规范。

## 📸 运行证明 / Proof of Work

- `go test ./service/suppliertest ./controller ./model -count=1` 通过
- `cd web && bun run typecheck` 通过
- `oxlint` 对 `src/features/supplier-test` 0 warning / 0 error
- `vitest src/features/supplier-test` 通过（判定尺子、页面布局）

手动：管理员打开「供应商测试」→ 填目标 → 标签页跑基础 / 缓存 / 压测 → 底下展开判定标准改数字，对照表马上重算。请在 GitHub PR 里补一张页面截图。

---

**建议 Title:** `feat: add admin supplier test workbench`

**合向:** 公司 `main`（`yeruyi1024/novamaas-workspace`）

**说明:** 个人 fork 上的阿里云 CD 继续留在 `feat/supplier-test` 自己用，不要放进这个公司 PR。
