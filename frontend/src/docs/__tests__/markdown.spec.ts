import { describe, expect, it } from 'vitest'

import { renderMarkdown } from '@/docs/markdown'

describe('renderMarkdown', () => {
  it('creates stable unique heading ids and a level-two/three table of contents', () => {
    const result = renderMarkdown(`
# 文档标题
## 创建密钥
### 安全建议
## 创建密钥
`)

    expect(result.headings).toEqual([
      { id: '创建密钥', text: '创建密钥', level: 2 },
      { id: '安全建议', text: '安全建议', level: 3 },
      { id: '创建密钥-2', text: '创建密钥', level: 2 },
    ])
    expect(result.html).toContain('id="创建密钥"')
    expect(result.html).toContain('id="创建密钥-2"')
  })

  it('removes executable markup, event handlers, frames, and dangerous links', () => {
    const result = renderMarkdown(`
<script>alert('xss')</script>
<img src="x" onerror="alert('xss')">
<iframe src="https://example.com"></iframe>
<a href="javascript:alert(1)">危险链接</a>
`)

    expect(result.html).not.toMatch(/script|onerror|iframe|javascript:/i)
  })

  it('marks external links for safe new-tab navigation while preserving internal links', () => {
    const result = renderMarkdown(`
[Model-Gate 文档](/docs?cat=help&page=faq)
[外部网站](https://example.com)
`)

    const container = document.createElement('div')
    container.innerHTML = result.html
    const links = container.querySelectorAll('a')

    expect(links[0]?.getAttribute('target')).toBeNull()
    expect(links[1]?.getAttribute('target')).toBe('_blank')
    expect(links[1]?.getAttribute('rel')).toBe('noopener noreferrer')
  })

  it('turns fenced code into a copyable, labelled code block', () => {
    const result = renderMarkdown('```bash\necho model-gate\n```')

    expect(result.html).toContain('class="docs-code-block"')
    expect(result.html).toContain('data-copy-code')
    expect(result.html).toContain('BASH')
  })
  it('contains wide tables inside a responsive scroll wrapper', () => {
    expect(renderMarkdown('| 等级 | 门槛 |\n| --- | --- |\n| VIP 1 | $100 |').html).toContain('docs-table-wrap')
  })
  it('folds optional sections without hiding required sections or losing heading anchors', () => {
    const result = renderMarkdown('# 文档\n## 基础配置\n必须先读。\n## 进阶排查\n补充内容。\n### 网络\n检查地址。\n## 费用\n关键规则。', { collapsedHeadings: ['进阶排查'] })
    const container = document.createElement('div'); container.innerHTML = result.html
    const detail = container.querySelector('details')!
    expect(detail.hasAttribute('open')).toBe(false)
    expect(detail.querySelector('summary #进阶排查')).not.toBeNull()
    expect(detail.textContent).toContain('补充内容')
    expect(detail.textContent).not.toContain('关键规则')
    expect(result.headings.map(heading => heading.text)).toContain('进阶排查')
  })
})
