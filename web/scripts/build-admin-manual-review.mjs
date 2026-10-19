import assert from 'node:assert/strict'
import fs from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

import { lexer, walkTokens } from 'marked'

const repositoryRoot = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  '../..'
)
const manualRoot = path.join(repositoryRoot, 'docs/admin-manual')
const output = path.join(
  repositoryRoot,
  'docs/review/星途与Gateway接口对接手册.md'
)
const chapterNames = [
  '01-account-login.md',
  '02-subscriptions.md',
  '03-api-keys.md',
  '04-text-models.md',
  '05-image-models.md',
  '06-video-models.md',
]
const designNames = [
  'ADMIN_MANUAL.zh_CN.md',
  'STARPATH_FEATURE_ACCESS.zh_CN.md',
  'STARPATH_PLUGIN_INTEGRATION_RESEARCH.zh_CN.md',
  'STARPATH_INTEGRATION_DELIVERY_PLAN.zh_CN.md',
]
const chapters = await Promise.all(
  chapterNames.map(async (name, index) => {
    const content = await fs.readFile(path.join(manualRoot, name), 'utf8')
    const [heading, ...lines] = content.trimEnd().split('\n')
    assert.ok(heading.startsWith('# '), `${name}: missing chapter title`)
    return {
      name,
      number: index + 1,
      id: name.slice(3, -3),
      title: heading.slice(2),
      content,
      body: lines.join('\n').trim(),
    }
  })
)
const designs = await Promise.all(
  designNames.map(async (name) => ({
    filename: path.join(repositoryRoot, 'docs/design', name),
    content: await fs.readFile(
      path.join(repositoryRoot, 'docs/design', name),
      'utf8'
    ),
  }))
)

function anchor(text) {
  return text
    .toLowerCase()
    .replaceAll(/[^\p{L}\p{N}]+/gu, '-')
    .replaceAll(/^-|-$/g, '')
}

function headings(content) {
  const counts = new Map()
  return lexer(content)
    .filter((token) => token.type === 'heading')
    .map((token) => {
      const base = anchor(token.text)
      const count = counts.get(base) ?? 0
      counts.set(base, count + 1)
      return { ...token, id: count ? `${base}-${count}` : base }
    })
}

const chapterByID = new Map(chapters.map((chapter) => [chapter.id, chapter]))
const contentManifest = await fs.readFile(
  path.join(manualRoot, 'content.go'),
  'utf8'
)
assert.deepEqual(
  [...contentManifest.matchAll(/"(\d{2}-[^"\n]+\.md)"/g)].map(
    (match) => match[1]
  ),
  chapterNames,
  'Backend and review chapter manifests differ'
)
assert.deepEqual(
  (await fs.readdir(manualRoot)).filter((name) => name.endsWith('.md')).sort(),
  chapterNames,
  'Review the six-chapter manifest before adding another article'
)

let jsonExamples = 0
let tables = 0
let links = 0
for (const document of [
  ...chapters.map((chapter) => ({
    filename: path.join(manualRoot, chapter.name),
    content: chapter.content,
  })),
  ...designs,
]) {
  const tokens = lexer(document.content)
  const localLinks = []
  walkTokens(tokens, (token) => {
    if (token.type === 'code') {
      const fence = token.raw.match(/^(`{3,}|~{3,})/)?.[1]
      assert.ok(
        !fence || token.raw.trimEnd().endsWith(fence),
        `${document.filename}: unclosed code fence`
      )
    }
    if (token.type === 'code' && token.lang === 'json') {
      JSON.parse(token.text)
      jsonExamples++
    }
    if (token.type === 'code' && token.lang === 'http') {
      const body = token.text
        .split(/\r?\n\r?\n/)
        .slice(1)
        .join('\n\n')
        .trim()
      if (body.startsWith('{') || body.startsWith('[')) {
        JSON.parse(body)
        jsonExamples++
      }
    }
    if (token.type === 'table') {
      tables++
      for (const row of token.rows) {
        assert.equal(
          row.length,
          token.header.length,
          `${document.filename}: inconsistent table row`
        )
      }
      for (const row of token.raw.trimEnd().split('\n').slice(2)) {
        const cells = row
          .replace(/^\s*\|/, '')
          .replace(/\|\s*$/, '')
          .split(/(?<!\\)\|/)
        assert.equal(
          cells.length,
          token.header.length,
          `${document.filename}: malformed table row ${row}`
        )
      }
    }
    if (token.type === 'link') localLinks.push(token.href)
  })
  for (const href of localLinks) {
    if (/^(?:https?:|mailto:)/i.test(href)) continue
    const url = new URL(href, 'https://manual.example.com/')
    let target = document.filename
    if (url.pathname === '/admin-manual') {
      const chapter = chapterByID.get(url.searchParams.get('article'))
      assert.ok(chapter, `${document.filename}: unknown chapter in ${href}`)
      target = path.join(manualRoot, chapter.name)
    } else if (!href.startsWith('#')) {
      assert.ok(!href.startsWith('/'), `Unexpected absolute file link: ${href}`)
      target = path.resolve(
        path.dirname(document.filename),
        decodeURIComponent(href.split('#')[0])
      )
    }
    const targetContent = await fs.readFile(target, 'utf8')
    if (url.hash) {
      const fragment = decodeURIComponent(url.hash.slice(1))
      assert.ok(
        headings(targetContent).some((heading) => heading.id === fragment),
        `${document.filename}: missing anchor ${href}`
      )
    }
    links++
  }
}

const sections = chapters.map((chapter) => {
  const sourceHeadings = headings(chapter.content)
  const renamed = new Map(
    sourceHeadings.map((heading) => [
      heading.id,
      `${chapter.id}--${heading.id}`,
    ])
  )
  const tokens = lexer(chapter.body)
  let headingIndex = 1
  const blocks = tokens.map((token) => {
    if (token.type !== 'heading') return token.raw
    const source = sourceHeadings[headingIndex++]
    return `<a id="${renamed.get(source.id)}"></a>\n\n${'#'.repeat(Math.min(token.depth + 1, 6))} ${token.text}\n\n`
  })
  let body = blocks.join('').trimEnd()
  body = body.replaceAll(
    /\]\(#([^)]*)\)/g,
    (_, fragment) => `](#${chapter.id}--${decodeURIComponent(fragment)})`
  )
  body = body.replaceAll(
    /\]\(\/admin-manual\?article=([\w-]+)(?:#([^)]*))?\)/g,
    (_, id, fragment) => {
      assert.ok(chapterByID.has(id), `Unknown chapter ${id}`)
      return `](#${id}${fragment ? `--${decodeURIComponent(fragment)}` : ''})`
    }
  )
  const index = sourceHeadings
    .filter((heading) => heading.depth === 2)
    .map((heading) => `  - [${heading.text}](#${renamed.get(heading.id)})`)
  return {
    index: [`- [${chapter.number} ${chapter.title}](#${chapter.id})`, ...index],
    body: `<a id="${chapter.id}"></a>\n\n## ${chapter.number} ${chapter.title}\n\n${body}`,
  }
})

const introduction = `# 星途与 Gateway 接口对接手册

版本：2.0 收口稿。更新日期：2026-10-19。

聚合网关（Gateway）管理账号、钱包、套餐与模型调用；功能权限、插件目录、配置生效回报及统一发布是本册明确标记的待开发能力。星途负责公司接口适配、本人状态展示与后续受控执行。

本审阅稿由六章正文生成；保留既有 article ID。现有 API 已按源码核对，不等于真实两端联调通过；所有 \`/api/apps/starpath/...\` 均未实现。统一版本由后台指定，所有获授权用户同一目标，平台/架构只选择同版本文件。

星途当前先复验现有功能，套餐只读；不因接口存在恢复购买或改造官方对话、工具、技能、MCP 核心。原参考 PDF 的差异核对保留于视频章，本次未取得原路径 PDF 源文件，未新增逐页复核结果。

开发交付、源码接点、阶段责任、兼容迁移和验收场景见[实施与交付计划](../design/STARPATH_INTEGRATION_DELIVERY_PLAN.zh_CN.md)。手册状态与阅读隔离说明见[设计说明](../design/ADMIN_MANUAL.zh_CN.md)。编辑 \`docs/admin-manual/\` 后在 \`web/\` 运行 \`bun run docs:admin-manual\`，不要直接维护第二份正文。

## 阅读目录

${chapters.map((chapter) => `${chapter.number}. [${chapter.title}](#${chapter.id})`).join('\n')}

## 详细索引

${sections.flatMap((section) => section.index).join('\n')}
`
const rendered = `${introduction}\n${sections.map((section) => section.body).join('\n\n')}\n`
const renderedAnchors = new Set(
  [...rendered.matchAll(/<a id="([^"]+)"><\/a>/g)].map((match) => match[1])
)
walkTokens(lexer(rendered), (token) => {
  if (token.type === 'link' && token.href.startsWith('#')) {
    assert.ok(
      renderedAnchors.has(decodeURIComponent(token.href.slice(1))),
      `Review copy has a missing anchor: ${token.href}`
    )
  }
})

if (process.argv.includes('--check')) {
  assert.equal(
    await fs.readFile(output, 'utf8'),
    rendered,
    'Review copy is stale; run bun run docs:admin-manual'
  )
} else {
  await fs.mkdir(path.dirname(output), { recursive: true })
  await fs.writeFile(output, rendered)
}
console.log(
  `Manual ${process.argv.includes('--check') ? 'checked' : 'generated'}: 6 chapters, ${designs.length} design documents, ${jsonExamples} JSON examples, ${tables} tables, ${links} local links.`
)
