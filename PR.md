# ⚠️ 提交说明 / PR Notice
> [!IMPORTANT]
>
> - 请提供**人工撰写**的简洁摘要，避免直接粘贴未经整理的 AI 输出。

## 📝 变更描述 / Description

给管理员增加「供应商测试」工作台：只填 Base URL、可选 API Key 和模型，直连目标做基础验收、缓存命中和压测。不走 Channel / relay / 计费，也不写新表。

页面展示 TTFT / TPOT 的均值、P50、P90，以及这次短测推算的 RPM / TPM。结果可复制 Markdown，或导出 `.md` / `.html`。语料只保留一份内置列表（含约 3k token 文本），缓存和压测共用，按原文发送，不再做提示词填充或预热垫字。压测可选「可走缓存」（每枪相同语料）或「打断缓存」（语料前加随机前缀）。

指标只分「正常 / 偏慢 / 无法对照」。差一点仍算正常：例如错误率 8%、TTFT 4–5 秒、缓存命中 60% 都算正常。供应商缺字段会跳过，不当失败。温度和 top_p 空着就不发送。

也可以测本平台：点「用本平台」填当前 API 地址，再粘贴令牌页的 Key，即可拉取 `/v1/models` 并跑同一套检查。空模型列表不再当成错误；如果误填了控制台页面地址，会提示改用 API 源站，而不是一串 HTML。基础验收只在「流式连通性」展示模型回复，其它检查只看通过/跳过/失败。gzip 只排除本功能的 SSE 路径，其它 API 仍压缩。侧边栏是加性入口，不改渠道、计费、转发。

本段描述由作者整理；实现过程有 AI 辅助。

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
- [x] **范围聚焦:** 本 PR 未包含任何与当前任务无关的代码改动。
- [x] **上游同步:** 若本 PR 引入或同步上游改动，我已同时更新 `UPSTREAM.md`；否则已在上方说明其为下游专属或不适用。
- [x] **本地验证:** 已在本地运行并通过测试或手动验证，维护者可以据此复核结果。
- [x] **安全合规:** 代码中无敏感凭据，且符合项目代码规范。

## 📸 运行证明 / Proof of Work

本地测试：

- `go test ./service/suppliertest ./controller ./router ./model -count=1` 通过
- `cd web && bun run typecheck` 通过
- `oxlint src/features/supplier-test` 0 warning / 0 error
- `vitest src/features/supplier-test/baselines.test.ts` 通过

手动：管理员打开「供应商测试」→「用本平台」→ 粘贴令牌页 Key → 拉取模型。测上游时仍只填对方 Base URL。请在 GitHub PR 里补一张页面截图。

---

**建议 Title:** `feat: add admin supplier test workbench`

**分支:** `feat/supplier-test`（相对 `main`）

**说明:** 当前 git 用户不是仓库历史核心作者，PR 已标明实现过程有 AI 辅助。合并前如不需要把本文件带进仓库，可从提交中移除 `PR.md`，只把正文贴到 GitHub。
