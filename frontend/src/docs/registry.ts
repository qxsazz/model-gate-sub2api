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
import sdkSource from '@/content/docs/tutorial/sdk.md?raw'
import groupsSource from '@/content/docs/platform/groups.md?raw'
import vipSource from '@/content/docs/membership/vip.md?raw'
import rewardsSource from '@/content/docs/membership/rewards.md?raw'
import rechargeSource from '@/content/docs/membership/recharge.md?raw'
import referralSource from '@/content/docs/membership/referral.md?raw'
import achievementsSource from '@/content/docs/achievements/overview.md?raw'
import tokenSource from '@/content/docs/achievements/token.md?raw'
import checkinSource from '@/content/docs/achievements/check-in.md?raw'
import activitiesSource from '@/content/docs/achievements/activities.md?raw'
import collectionSource from '@/content/docs/achievements/collection.md?raw'
import toolsSource from '@/content/docs/tools/recommended.md?raw'
import setupSource from '@/content/docs/tools/environment.md?raw'
import privacySource from '@/content/docs/help/security.md?raw'
import serviceSource from '@/content/docs/about/service.md?raw'
import errorsSource from '@/content/docs/troubleshooting/http-errors.md?raw'
import networkSource from '@/content/docs/troubleshooting/network.md?raw'
import recordsSource from '@/content/docs/troubleshooting/records.md?raw'
import accountIssuesSource from '@/content/docs/troubleshooting/account-issues.md?raw'

const optionalSections: Record<string, string[]> = {
  'api-reference': ['Responses 请求', 'Anthropic Messages 请求', '图像与其他能力'],
  'claude-code': ['配置冲突', '排查顺序'],
  codex: ['常见问题'],
  cursor: ['排查建议'],
  sdk: ['生产应用注意事项'],
  recommended: ['CC Switch · 配置管理', 'Cherry Studio · 桌面客户端', 'Cline · 编程助手', 'Continue · 编辑器模型配置', 'OpenCode · 终端编程', 'Aider · 终端代码协作'],
  'http-errors': ['400 · 请求格式错误', '401 · 鉴权失败', '403 · 权限与余额', '404 · 地址与模型', '413 · 请求体过大', '429 · 限流与额度', '499 · 请求被取消', '500 · 服务内部异常', '502 · 上游请求失败', '503 · 服务暂不可用', '504 · 等待超时'],
  'account-issues': ['登录失效或刷新失败', '支付完成但余额未更新', 'VIP 等级、分组或奖励不符合预期', '兑换码失败', '错误记录入口报 403'],
}

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
      article('sdk', 'SDK 调用示例', '使用 Python 和 JavaScript 连接 Model-Gate。', sdkSource),
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
      article('groups', '模型与分组', '理解分组选择、模型权限和接入协议。', groupsSource),
      article('rate-limits', '限流与重试', '构建更稳定的请求与重试策略。', rateLimitsSource),
      article('billing', '计费与用量', '查看余额、用量和账单记录。', billingSource),
    ],
  },
  {
    slug: 'membership', title: '充值与 VIP', description: '从累计充值到会员权益。',
    articles: [
      article('recharge', '充值与兑换', '了解支付到账、兑换余额与成长计量。', rechargeSource),
      article('vip', 'VIP 等级与权益', '累计有效充值，逐级开启会员权益。', vipSource),
      article('rewards', '累充奖励', '各档奖励、领取条件及退款追回规则。', rewardsSource),
      article('referral', '邀请返利', '会员等级对应的邀请返利比例。', referralSource),
    ],
  },
  {
    slug: 'achievements', title: '成就与活动', description: '收藏每一次成长。',
    articles: [
      article('overview', '成就收藏册', '勋章、进度、领取与佩戴玩法预告。', achievementsSource, 'upcoming'),
      article('token', 'Token 成长成就', '累计调用用量的三档成长里程碑。', tokenSource, 'upcoming'),
      article('check-in', '签到与全勤', '连续签到、补签卡及全勤奖励。', checkinSource, 'upcoming'),
      article('activities', '活动与盲盒', '排行榜、答题与盲盒收藏玩法。', activitiesSource, 'upcoming'),
      article('collection', '充值与收藏成就', '充值荣誉、分类收藏及终章成就。', collectionSource, 'upcoming'),
    ],
  },
  {
    slug: 'tools', title: '工具推荐', description: '选择适合自己的工作方式。',
    articles: [
      article('recommended', '工具选型指南', '桌面、编辑器、终端与配置管理工具。', toolsSource),
      article('environment', '环境准备', 'Windows、macOS 与 Linux 的接入准备。', setupSource),
    ],
  },
  {
    slug: 'about',
    title: '了解我们',
    description: '了解服务原则、安全与隐私底线。',
    articles: [
      article('service', '了解 MODEL-GATE', '接入清晰、计费可查、故障可追踪。', serviceSource),
      article('security', '安全与隐私 · 服务底线', '凭据保护、账户隔离与清晰的数据边界。', privacySource),
    ],
  },
  {
    slug: 'troubleshooting', title: '常见问题', description: '按报错定位原因，查看记录并逐步排查。',
    articles: [
      article('faq', '排错导航', '从 HTTP 状态、报错正文和请求记录开始。', faqSource),
      article('http-errors', 'HTTP 报错速查', '401、403、429 及常见网关状态的排查方法。', errorsSource),
      article('network', '网络、超时与流式异常', '排查连接失败、断流和 HTTP 200 中的错误事件。', networkSource),
      article('records', '查看记录与反馈问题', '查看用量与错误记录，准备安全的排障信息。', recordsSource),
      article('account-issues', '账户、充值与 VIP 问题', '区分登录、支付、权益与奖励的业务限制。', accountIssuesSource),
    ],
  },
]

function article(
  slug: string,
  title: string,
  description: string,
  source: string,
  status: 'available' | 'upcoming' = 'available',
): DocArticle {
  return {
    slug,
    title,
    description,
    source,
    updatedAt: ['service', 'security', 'faq', 'http-errors', 'network', 'records', 'account-issues', 'recharge', 'rewards', 'collection', 'overview', 'vip', 'check-in'].includes(slug) ? '2026-10-02' : '2026-10-01',
    status,
    collapsedHeadings: optionalSections[slug] || [],
  }
}

export function resolveDocument(category?: string, page?: string): ResolvedDocument | null {
  if (category === 'help') {
    if (!page || page === 'faq') return resolveDocument('troubleshooting', 'faq')
    if (page === 'security') return resolveDocument('about', 'security')
    return null
  }
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
