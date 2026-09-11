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
})
