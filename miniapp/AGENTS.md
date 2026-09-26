# AGENTS.md — WeChat Mini Program Conventions

## Scope

These rules apply to all files under `miniapp/`.

## Runtime and dependencies

- This is an independent Taro 4 + React 18 application. Do not import code from `web/src`.
- Use Bun for dependency installation and scripts. Keep `miniapp/bun.lock` independent from the Web lockfile.
- Use Taro components and APIs instead of browser DOM, `window`, `document`, or `localStorage`.
- Keep the package light. Prefer direct imports and do not add a cross-platform UI framework without an architecture decision.

## Internationalization

- All user-visible text must use `react-i18next`.
- Every key must be present in en, zh, zh-TW, fr, ja, ru, and vi.
- English is the fallback language. Page configuration titles are fallback-only; pages must update their runtime title with `usePageTitle`.

## API and security

- Configure the backend through `MINIAPP_API_BASE_URL`; never hard-code production hosts.
- Keep authentication, billing, and authorization rules on the Go server. Do not duplicate server business rules in the client.
- Never store AppSecret, payment keys, `session_key`, or backend administrative credentials in the mini program.
- Version local-storage keys and store only the minimum required client state.

## Required checks

Run these commands from `miniapp/` before completing a task:

```bash
bun run typecheck
bun run lint
bun run test
bun run build:weapp
```

Update `docs/design/WECHAT_MINIAPP_PROJECT_PLAN.zh_CN.md` whenever a tracked `MP-*` task starts, completes, becomes blocked, or changes scope.
