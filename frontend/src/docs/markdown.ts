import DOMPurify from 'dompurify'
import { marked } from 'marked'

import type { DocHeading, RenderedMarkdown } from './types'

marked.setOptions({
  breaks: true,
  gfm: true,
})

const FORBIDDEN_TAGS = [
  'script',
  'style',
  'iframe',
  'object',
  'embed',
  'form',
  'input',
  'button',
  'textarea',
  'select',
]

export function renderMarkdown(source: string, options: { collapsedHeadings?: readonly string[]; documentationPath?: string } = {}): RenderedMarkdown {
  const parsed = marked.parse(source) as string
  const sanitized = DOMPurify.sanitize(parsed, {
    FORBID_TAGS: FORBIDDEN_TAGS,
    FORBID_ATTR: ['style'],
  })
  const container = document.createElement('div')
  container.innerHTML = sanitized

  const headings: DocHeading[] = []
  const slugCounts = new Map<string, number>()

  container.querySelectorAll<HTMLHeadingElement>('h1, h2, h3').forEach((heading) => {
    const text = heading.textContent?.trim() || 'section'
    const baseSlug = createHeadingSlug(text)
    const count = (slugCounts.get(baseSlug) ?? 0) + 1
    slugCounts.set(baseSlug, count)
    const id = count === 1 ? baseSlug : `${baseSlug}-${count}`
    heading.id = id

    if (heading.tagName === 'H2' || heading.tagName === 'H3') {
      headings.push({
        id,
        text,
        level: heading.tagName === 'H2' ? 2 : 3,
      })
    }
  })

  container.querySelectorAll<HTMLAnchorElement>('a[href]').forEach((link) => {
    const href = link.getAttribute('href') || ''
    if (options.documentationPath && /^\/docs(?:[?#]|$)/.test(href)) {
      link.setAttribute('href', options.documentationPath + href.slice('/docs'.length))
    }
    if (/^https?:\/\//i.test(href)) {
      link.target = '_blank'
      link.rel = 'noopener noreferrer'
    }
  })

  container.querySelectorAll('table').forEach((table) => {
    const wrapper = document.createElement('div')
    wrapper.className = 'docs-table-wrap'
    table.replaceWith(wrapper)
    wrapper.append(table)
  })

  container.querySelectorAll<HTMLPreElement>('pre').forEach((pre) => {
    const code = pre.querySelector('code')
    const languageClass = Array.from(code?.classList ?? []).find((name) => name.startsWith('language-'))
    const language = languageClass?.slice('language-'.length).toUpperCase() || 'CODE'
    const wrapper = document.createElement('div')
    wrapper.className = 'docs-code-block'
    const toolbar = document.createElement('div')
    toolbar.className = 'docs-code-block__toolbar'
    const label = document.createElement('span')
    label.textContent = language
    const copyButton = document.createElement('button')
    copyButton.type = 'button'
    copyButton.setAttribute('data-copy-code', '')
    copyButton.setAttribute('aria-label', `复制 ${language} 代码`)
    copyButton.textContent = '复制'
    toolbar.append(label, copyButton)
    pre.replaceWith(wrapper)
    wrapper.append(toolbar, pre)
  })

  for (const heading of Array.from(container.querySelectorAll('h2'))) {
    if (!options.collapsedHeadings?.includes(heading.textContent || '')) continue
    const sections: Element[] = []
    let next = heading.nextElementSibling
    while (next && next.tagName !== 'H2' && next.tagName !== 'H1') {
      sections.push(next)
      next = next.nextElementSibling
    }
    const detail = document.createElement('details')
    detail.className = 'docs-disclosure'
    const summary = document.createElement('summary')
    const body = document.createElement('div')
    body.className = 'docs-disclosure-content'
    heading.replaceWith(detail)
    summary.append(heading)
    body.append(...sections)
    detail.append(summary, body)
  }

  return { html: container.innerHTML, headings }
}

function createHeadingSlug(text: string): string {
  return text
    .normalize('NFKC')
    .toLowerCase()
    .trim()
    .replace(/[\s_]+/g, '-')
    .replace(/[^\p{Letter}\p{Number}-]+/gu, '')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '') || 'section'
}
