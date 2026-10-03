<template>
  <div ref="shell" class="docs-page" :class="{ 'is-dark': isDark, 'docs-page--embedded': embedded }" data-testid="docs-shell">
    <DocsHeader v-if="!embedded" :is-dark="isDark" @open-menu="mobileMenuOpen = true" @toggle-theme="toggleTheme" />
    <button v-else class="docs-workspace-menu" type="button" @click="mobileMenuOpen = true"><Icon name="document" size="md" />文档目录</button>


    <div class="docs-layout">
      <aside class="docs-layout__sidebar">
        <DocsSidebar
          :navigation="docsNavigation"
          :overview="isOverview"
          :current-category="isOverview ? undefined : resolved?.location.category"
          :current-page="isOverview ? undefined : resolved?.location.page"
        />
      </aside>

      <main class="docs-main">
        <DocsOverview v-if="isOverview" :navigation="docsNavigation" />
        <DocsNotFoundView v-else-if="!resolved" />
        <article v-else data-testid="docs-article">
          <div class="docs-breadcrumb"><RouterLink :to="basePath">文档总览</RouterLink> <span>/</span> {{ resolved.group.title }} <span>/</span> {{ resolved.document.title }}</div>
          <header class="docs-article-header">
            <div class="docs-article-kicker">MODEL-GATE GUIDE</div>
            <h1 data-testid="docs-article-title">{{ resolved.document.title }}</h1>
            <p>{{ resolved.document.description }}</p>
            <div class="docs-article-meta">
              <span>最后更新 {{ resolved.document.updatedAt }}</span>
              <span class="docs-article-meta__dot"></span>
              <span>{{ resolved.document.status === 'upcoming' ? '玩法预告' : '接入与使用指南' }}</span>
            </div>
          </header>

          <div v-if="resolved.document.status === 'upcoming'" class="docs-coming-soon" data-testid="docs-coming-soon">
            <Icon name="clock" size="md" /><div><strong>敬请期待</strong><p>此玩法尚未上线，具体门槛、奖励与开放时间以正式公告为准。</p></div>
          </div>

          <div class="docs-mobile-toc" v-if="rendered.headings.length">
            <details>
              <summary>本页目录 · {{ rendered.headings.length }} 个章节</summary>
              <DocsTableOfContents :headings="rendered.headings" />
            </details>
          </div>

          <div class="docs-prose" v-html="rendered.html" @click="handleArticleClick"></div>
          <DocsArticleNavigation :previous="adjacent.previous" :next="adjacent.next" />
        </article>
      </main>

      <div class="docs-layout__toc">
        <DocsTableOfContents v-if="isOverview" :headings="overviewHeadings" />
        <DocsTableOfContents v-else-if="resolved" :headings="rendered.headings" />
      </div>
    </div>

    <Teleport to="body">
      <div v-if="mobileMenuOpen" class="docs-drawer" :class="{ 'is-dark': isDark }" data-testid="docs-mobile-drawer">
        <button class="docs-drawer__backdrop" aria-label="关闭文档目录" @click="mobileMenuOpen = false"></button>
        <aside class="docs-drawer__panel">
          <div class="docs-drawer__header">
            <strong>文档目录</strong>
            <button type="button" data-testid="docs-drawer-close" aria-label="关闭" @click="mobileMenuOpen = false"><Icon name="x" size="md" /></button>
          </div>
          <DocsSidebar
            :navigation="docsNavigation"
            :overview="isOverview"
            :current-category="isOverview ? undefined : resolved?.location.category"
            :current-page="isOverview ? undefined : resolved?.location.page"
            @navigate="mobileMenuOpen = false"
          />
        </aside>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import DocsArticleNavigation from '@/components/docs/DocsArticleNavigation.vue'
import DocsHeader from '@/components/docs/DocsHeader.vue'
import DocsSidebar from '@/components/docs/DocsSidebar.vue'
import DocsTableOfContents from '@/components/docs/DocsTableOfContents.vue'
import { docsNavigation, getAdjacentDocuments, resolveDocument } from '@/docs/registry'
import { renderMarkdown } from '@/docs/markdown'
import DocsNotFoundView from './DocsNotFoundView.vue'
import Icon from '@/components/icons/Icon.vue'
import DocsOverview from '@/components/docs/DocsOverview.vue'
import type { DocHeading } from '@/docs/types'
import { scrollToDocHeading } from '@/docs/navigation'
import { useDocumentationPath } from '@/docs/workspace'

const props = withDefaults(defineProps<{ embedded?: boolean }>(), { embedded: false })
const basePath = useDocumentationPath()
const shell = ref<HTMLElement>()
const route = useRoute()
const router = useRouter()
const mobileMenuOpen = ref(false)
const isDark = ref(props.embedded ? document.documentElement.classList.contains('dark') : resolveInitialTheme())
let themeObserver: MutationObserver | undefined
onMounted(() => {
  if (!props.embedded) return
  themeObserver = new MutationObserver(() => { isDark.value = document.documentElement.classList.contains('dark') })
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
})
onUnmounted(() => themeObserver?.disconnect())

const queryValue = (value: unknown): string | undefined => typeof value === 'string' ? value : undefined
const isOverview = computed(() => !route.query.cat && !route.query.page)
const overviewHeadings = computed<DocHeading[]>(() => docsNavigation.map(group => ({ id: `overview-${group.slug}`, text: group.title, level: 2 })))
const resolved = computed(() => resolveDocument(queryValue(route.query.cat), queryValue(route.query.page)))
const rendered = computed(() => resolved.value ? renderMarkdown(resolved.value.document.source, { collapsedHeadings: resolved.value.document.collapsedHeadings, documentationPath: basePath }) : { html: '', headings: [] })
const adjacent = computed(() => resolved.value
  ? getAdjacentDocuments(resolved.value.location.category, resolved.value.location.page)
  : { previous: null, next: null })

watch(() => route.fullPath, async () => {
  mobileMenuOpen.value = false
  await nextTick()
  if (route.hash) {
    try { if (scrollToDocHeading(decodeURIComponent(route.hash.slice(1)))) return } catch { /* Invalid fragments fall back to the article top. */ }
  }
  if (props.embedded) shell.value?.scrollIntoView?.({ block: 'start' })
  else window.scrollTo({ top: 0, behavior: 'smooth' })
}, { immediate: true })

function resolveInitialTheme(): boolean {
  const saved = localStorage.getItem('theme')
  return saved ? saved === 'dark' : window.matchMedia('(prefers-color-scheme: dark)').matches
}

function toggleTheme(): void {
  isDark.value = !isDark.value
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
  document.documentElement.classList.toggle('dark', isDark.value)
}

function handleArticleClick(event: MouseEvent): void {
  const target = event.target as HTMLElement
  const copyButton = target.closest<HTMLButtonElement>('[data-copy-code]')
  if (copyButton) {
    const code = copyButton.closest('.docs-code-block')?.querySelector('code')?.textContent || ''
    void copyCode(copyButton, code)
    return
  }

  const anchor = target.closest<HTMLAnchorElement>('a[href^="#"]')
  if (anchor) {
    event.preventDefault()
    try { scrollToDocHeading(decodeURIComponent((anchor.getAttribute('href') || '').slice(1))) } catch { /* Ignore malformed fragments. */ }
    return
  }
  const link = target.closest<HTMLAnchorElement>('a[href]')
  if (!link) return
  const url = new URL(link.getAttribute('href') || '/docs', window.location.origin)
  if (url.origin !== window.location.origin || url.pathname !== basePath) return
  if (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey || event.button !== 0) return
  event.preventDefault()
  void router.push(basePath + url.search + url.hash)
}

async function copyCode(button: HTMLButtonElement, code: string): Promise<void> {
  const previousLabel = button.textContent
  try { await navigator.clipboard.writeText(code); button.textContent = '已复制' }
  catch { button.textContent = '复制失败' }
  window.setTimeout(() => {
    button.textContent = previousLabel
  }, 1600)
}
</script>

<style scoped>
.docs-page, .docs-drawer {
  --docs-bg: #fbfbfa; --docs-surface: #fff; --docs-text: #252628; --docs-muted: #646463;
  --docs-faint: #85837c; --docs-border: #e7e3da; --docs-accent: #947538; --docs-accent-soft: #c9b477;
  --docs-accent-border: #d7c89f; --docs-accent-bg: #f6f2e8;
  min-height: 100vh; color: var(--docs-text); background: var(--docs-bg); font-family: 'Noto Sans SC', 'Avenir Next', 'Segoe UI', 'Microsoft YaHei', sans-serif;
}
.docs-page.is-dark, .docs-drawer.is-dark {
  --docs-bg: #121315; --docs-surface: #1b1c1f; --docs-text: #f0eeea; --docs-muted: #b7b5af;
  --docs-faint: #918d83; --docs-border: #34332f; --docs-accent: #ceb778; --docs-accent-soft: #c9b477;
  --docs-accent-border: #665832; --docs-accent-bg: #25231e;
}
.docs-page :deep(*), .docs-drawer :deep(*) { letter-spacing: 0; }
.docs-layout { position: relative; max-width: 1480px; margin: 0 auto; padding: 0 28px; display: grid; grid-template-columns: 244px minmax(0, 780px) 200px; justify-content: center; gap: 42px; }
.docs-layout__sidebar, .docs-layout__toc { position: sticky; top: 68px; align-self: start; max-height: calc(100vh - 68px); overflow-y: auto; }
.docs-main { min-width: 0; padding: 32px 0 72px; }
.docs-breadcrumb { margin-bottom: 20px; color: var(--docs-faint); font: 400 11px/1.5 'DM Mono', Consolas, monospace; }
.docs-breadcrumb span { margin: 0 8px; color: var(--docs-accent); }
.docs-breadcrumb a { color: var(--docs-muted); text-decoration: none; }
.docs-breadcrumb a:hover { color: var(--docs-accent); }
.docs-article-header { padding-bottom: 22px; border-bottom: 1px solid var(--docs-border); }
.docs-article-kicker { color: var(--docs-accent); font: 500 10px/1.2 'DM Mono', Consolas, monospace; }
.docs-article-header h1 { margin: 14px 0 10px; font-family: 'Noto Serif SC', 'Songti SC', SimSun, Georgia, serif; font-size: 36px; font-weight: 500; line-height: 1.3; overflow-wrap: anywhere; }
.docs-article-header p { max-width: 640px; margin: 0; color: var(--docs-muted); font-size: 16px; line-height: 1.75; }
.docs-article-meta { margin-top: 14px; display: flex; align-items: center; gap: 10px; color: var(--docs-faint); font-size: 11px; }
.docs-article-meta__dot { width: 3px; height: 3px; border-radius: 50%; background: var(--docs-accent); }
.docs-prose { padding-top: 22px; color: var(--docs-muted); font-size: 14px; line-height: 1.75; }
.docs-prose :deep(h1:first-child) { display: none; }
.docs-prose :deep(h2), .docs-prose :deep(h3) { scroll-margin-top: 92px; color: var(--docs-text); font-family: 'Noto Serif SC', 'Songti SC', SimSun, Georgia, serif; font-weight: 500; }
.docs-prose :deep(h2) { margin: 26px 0 12px; padding-top: 4px; font-size: 22px; }
.docs-prose :deep(h3) { margin: 20px 0 10px; font-size: 18px; }
.docs-prose :deep(p) { margin: 0 0 12px; }
.docs-prose :deep(a) { color: var(--docs-accent); text-decoration-color: var(--docs-accent-border); text-underline-offset: 4px; }
.docs-prose :deep(strong) { color: var(--docs-text); }
.docs-prose :deep(ul), .docs-prose :deep(ol) { margin: 0 0 14px; padding-left: 24px; }
.docs-prose :deep(li) { margin: 4px 0; padding-left: 4px; }
.docs-prose :deep(blockquote) { margin: 18px 0; padding: 12px 16px; color: var(--docs-text); border: 1px solid var(--docs-border); border-left: 2px solid var(--docs-accent); border-radius: 0; background: var(--docs-surface); font-size: 13px; }
.docs-prose :deep(blockquote p) { margin: 0; }
.docs-prose :deep(code) { padding: 2px 6px; color: var(--docs-accent); border: 1px solid var(--docs-border); border-radius: 3px; background: var(--docs-surface); font: 12px/1.7 'DM Mono', Consolas, monospace; overflow-wrap: anywhere; }
.docs-prose :deep(.docs-code-block) { margin: 18px 0; overflow: hidden; border: 1px solid #34332f; border-radius: 6px; background: #18191c; }
.docs-prose :deep(.docs-code-block__toolbar) { min-height: 38px; padding: 0 11px 0 16px; display: flex; align-items: center; justify-content: space-between; color: #c9b477; border-bottom: 1px solid #34332f; background: #202124; font: 500 10px/1 'DM Mono', Consolas, monospace; }
.docs-prose :deep(.docs-code-block__toolbar button) { padding: 6px 9px; color: #d6d1c4; border: 1px solid #514936; border-radius: 4px; background: transparent; cursor: pointer; font: 500 11px/1 'DM Mono', Consolas, monospace; }
.docs-prose :deep(.docs-code-block__toolbar button:hover) { color: #e6cf93; border-color: #c9b477; }
.docs-prose :deep(pre) { margin: 0; padding: 19px 21px; overflow-x: auto; color: #e5e4df; background: transparent; }
.docs-prose :deep(pre code) { padding: 0; color: inherit; border: 0; background: transparent; font-size: 12px; }
.docs-mobile-toc { display: none; margin-top: 24px; }
.docs-mobile-toc details { border: 1px solid var(--docs-border); border-radius: 4px; background: var(--docs-surface); }
.docs-mobile-toc summary { padding: 14px 16px; color: var(--docs-text); cursor: pointer; font-size: 13px; font-weight: 700; }
.docs-mobile-toc :deep(.docs-toc) { padding: 0 16px 16px; }
.docs-drawer { position: fixed; inset: 0; z-index: 100; display: flex; }
.docs-drawer__backdrop { position: absolute; inset: 0; border: 0; background: rgba(15,15,15,.65); }
.docs-drawer__panel { position: relative; width: min(86vw, 330px); height: 100%; overflow-y: auto; color: var(--docs-text); background: var(--docs-bg); box-shadow: 20px 0 70px rgba(0,0,0,.35); }
.docs-drawer__header { height: 68px; padding: 0 18px; display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid var(--docs-border); }
.docs-drawer__header button { width: 36px; height: 36px; display: grid; place-items: center; color: var(--docs-text); border: 1px solid var(--docs-border); border-radius: 4px; background: var(--docs-surface); cursor: pointer; }
.docs-coming-soon { display: flex; align-items: flex-start; gap: 12px; margin-top: 24px; padding: 18px 0; border-block: 1px solid var(--docs-accent-border); color: var(--docs-accent); }
.docs-coming-soon svg { flex-shrink: 0; margin-top: 2px; }
.docs-coming-soon strong { font-size: 15px; font-weight: 500; }
.docs-coming-soon p { margin: 6px 0 0; font-size: 12px; line-height: 1.7; color: var(--docs-muted); }
.docs-prose :deep(.docs-table-wrap) { max-width: 100%; overflow-x: auto; margin: 18px 0; border: 1px solid var(--docs-border); border-radius: 4px; background: var(--docs-surface); }
.docs-prose :deep(table) { width: 100%; font-size: 12px; text-align: left; border-collapse: collapse; }
.docs-prose :deep(th) { color: var(--docs-text); font-weight: 500; background: var(--docs-accent-bg); white-space: nowrap; }
.docs-prose :deep(th), .docs-prose :deep(td) { padding: 12px 14px; border-bottom: 1px solid var(--docs-border); min-width: 90px; }
.docs-prose :deep(tr:last-child td) { border-bottom: 0; }
.docs-prose :deep(td code) { white-space: nowrap; }
.docs-prose :deep(.docs-disclosure) { margin: 8px 0; border-block: 1px solid var(--docs-border); }
.docs-prose :deep(.docs-disclosure summary) { padding: 14px 0; cursor: pointer; color: var(--docs-accent); }
.docs-prose :deep(.docs-disclosure summary h2) { display: inline; margin: 0; padding: 0; font-family: inherit; font-size: 14px; color: var(--docs-text); }
.docs-prose :deep(.docs-disclosure summary:hover h2) { color: var(--docs-accent); }
.docs-prose :deep(.docs-disclosure-content) { padding: 0 0 12px 18px; }
.docs-prose :deep(.docs-disclosure summary:focus-visible) { outline: 2px solid var(--docs-accent); outline-offset: 2px; }
.docs-page :deep(button:focus-visible), .docs-page :deep(a:focus-visible) { outline: 2px solid var(--docs-accent); outline-offset: 3px; }
.docs-page--embedded { min-height: 100%; scroll-margin-top: 88px; border: 1px solid var(--docs-border); border-radius: 12px; }
.docs-page--embedded .docs-layout { grid-template-columns: 220px minmax(0, 780px); gap: 28px; }
.docs-page--embedded .docs-layout__toc { display: none; }
.docs-page--embedded .docs-layout__sidebar { top: 88px; max-height: calc(100vh - 100px); }
.docs-page--embedded .docs-mobile-toc { display: block; }
.docs-workspace-menu { display: none; align-items: center; gap: 8px; margin: 18px 22px 0; padding: 10px 14px; border: 1px solid var(--docs-border); border-radius: 6px; color: var(--docs-text); background: var(--docs-surface); font-size: 13px; }
@media (max-width: 1180px) { .docs-layout { grid-template-columns: 220px minmax(0, 780px); } .docs-layout__toc { display: none; } .docs-mobile-toc { display: block; } }
@media (max-width: 1180px) { .docs-page--embedded .docs-layout { display: block; } .docs-page--embedded .docs-layout__sidebar { display: none; } .docs-page--embedded .docs-workspace-menu { display: inline-flex; } }
@media (max-width: 960px) { .docs-layout { display: block; padding: 0 22px; } .docs-layout__sidebar { display: none; } .docs-main { max-width: 780px; margin: 0 auto; padding-top: 30px; } .docs-workspace-menu { display: inline-flex; } }
@media (max-width: 600px) { .docs-layout { padding: 0 17px; } .docs-breadcrumb { margin-bottom: 28px; } .docs-article-header h1 { font-size: 30px; } .docs-prose { font-size: 14px; } .docs-prose :deep(h2) { font-size: 21px; } .docs-article-header p { font-size: 14px; } }
</style>
