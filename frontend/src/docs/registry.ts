import type { DocArticle, DocGroup, DocLocation, ResolvedDocument } from './types'

import apiKeySource from '@/content/docs/tutorial/api-key.md?raw'
import apiReferenceSource from '@/content/docs/tutorial/api-reference.md?raw'
import quickStartSource from '@/content/docs/tutorial/quick-start.md?raw'
import claudeCodeSource from '@/content/docs/clients/claude-code.md?raw'
import codexSource from '@/content/docs/clients/codex.md?raw'
import cursorSource from '@/content/docs/clients/cursor.md?raw'
import billingSource from '@/content/docs/platform/billing.md?raw'
import rateLimitsSource from '@/content/docs/platform/rate-limits.md?raw'
import faqSource from '@/content/docs/help/faq.md?raw'

export const DEFAULT_DOC_LOCATION: DocLocation = {
  category: 'tutorial',
  page: 'quick-start',
}

export const docsNavigation: DocGroup[] = [
  {
    slug: 'tutorial',
    title: '开始使用',
    description: '从创建密钥到发出第一个请求。',
    articles: [
      article('quick-start', '快速开始', '用几分钟完成首次 Model-Gate API 调用。', quickStartSource),
      article('api-key', 'API Key', '创建、保存和轮换你的访问密钥。', apiKeySource),
      article('api-reference', 'API 参考', '了解兼容端点、请求头和响应约定。', apiReferenceSource),
    ],
  },
  {
    slug: 'clients',
    title: '客户端接入',
    description: '将常用 AI 开发工具连接到 Model-Gate。',
    articles: [
      article('claude-code', 'Claude Code 接入', '在 Claude Code 中使用 Model-Gate。', claudeCodeSource),
      article('codex', 'Codex 接入', '配置 Codex CLI 的 API 地址和凭据。', codexSource),
      article('cursor', 'Cursor 接入', '在 Cursor 中使用兼容模型。', cursorSource),
    ],
  },
  {
    slug: 'platform',
    title: '平台规则',
    description: '理解限流、重试、余额和用量。',
    articles: [
      article('rate-limits', '限流与重试', '构建更稳定的请求与重试策略。', rateLimitsSource),
      article('billing', '计费与用量', '查看余额、用量和账单记录。', billingSource),
    ],
  },
  {
    slug: 'help',
    title: '帮助',
    description: '排查常见的接入问题。',
    articles: [
      article('faq', '常见问题', 'Model-Gate 使用过程中的高频问题。', faqSource),
    ],
  },
]

function article(
  slug: string,
  title: string,
  description: string,
  source: string,
): DocArticle {
  return {
    slug,
    title,
    description,
    source,
    updatedAt: '2026-09-11',
  }
}

export function resolveDocument(category?: string, page?: string): ResolvedDocument | null {
  const requestedCategory = category || DEFAULT_DOC_LOCATION.category
  const group = docsNavigation.find((entry) => entry.slug === requestedCategory)
  if (!group) {
    return null
  }

  const requestedPage = page || (
    requestedCategory === DEFAULT_DOC_LOCATION.category
      ? DEFAULT_DOC_LOCATION.page
      : group.articles[0]?.slug
  )
  const document = group.articles.find((entry) => entry.slug === requestedPage)
  if (!document) {
    return null
  }

  return {
    location: { category: group.slug, page: document.slug },
    group,
    document,
  }
}

export function getAdjacentDocuments(category: string, page: string): {
  previous: DocArticle | null
  next: DocArticle | null
} {
  const flattened = docsNavigation.flatMap((group) => group.articles)
  const current = resolveDocument(category, page)?.document
  const currentIndex = current ? flattened.indexOf(current) : -1

  if (currentIndex < 0) {
    return { previous: null, next: null }
  }

  return {
    previous: flattened[currentIndex - 1] ?? null,
    next: flattened[currentIndex + 1] ?? null,
  }
}

export function findDocumentLocation(article: DocArticle): DocLocation | null {
  for (const group of docsNavigation) {
    if (group.articles.includes(article)) {
      return { category: group.slug, page: article.slug }
    }
  }
  return null
}
