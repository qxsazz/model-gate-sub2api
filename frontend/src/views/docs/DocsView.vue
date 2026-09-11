<template>
  <div class="docs-page" :class="{ 'is-dark': isDark }" data-testid="docs-shell">
    <DocsHeader :is-dark="isDark" @open-menu="mobileMenuOpen = true" @toggle-theme="toggleTheme" />

    <div class="docs-ambient" aria-hidden="true"><span></span><span></span></div>

    <div class="docs-layout">
      <aside class="docs-layout__sidebar">
        <DocsSidebar
          :navigation="docsNavigation"
          :current-category="resolved?.location.category"
          :current-page="resolved?.location.page"
        />
      </aside>

      <main class="docs-main">
        <DocsNotFoundView v-if="!resolved" />
        <article v-else data-testid="docs-article">
          <div class="docs-breadcrumb">{{ resolved.group.title }} <span>/</span> {{ resolved.document.title }}</div>
          <header class="docs-article-header">
            <div class="docs-article-kicker">MODEL-GATE GUIDE</div>
            <h1 data-testid="docs-article-title">{{ resolved.document.title }}</h1>
            <p>{{ resolved.document.description }}</p>
            <div class="docs-article-meta">
              <span>最后更新 {{ resolved.document.updatedAt }}</span>
              <span class="docs-article-meta__dot"></span>
              <span>静态文档</span>
            </div>
          </header>

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
        <DocsTableOfContents v-if="resolved" :headings="rendered.headings" />
      </div>
    </div>

    <Teleport to="body">
      <div v-if="mobileMenuOpen" class="docs-drawer" data-testid="docs-mobile-drawer">
        <button class="docs-drawer__backdrop" aria-label="关闭文档目录" @click="mobileMenuOpen = false"></button>
        <aside class="docs-drawer__panel">
          <div class="docs-drawer__header">
            <strong>文档目录</strong>
            <button type="button" data-testid="docs-drawer-close" aria-label="关闭" @click="mobileMenuOpen = false">×</button>
          </div>
          <DocsSidebar
            :navigation="docsNavigation"
            :current-category="resolved?.location.category"
            :current-page="resolved?.location.page"
            @navigate="mobileMenuOpen = false"
          />
        </aside>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import DocsArticleNavigation from '@/components/docs/DocsArticleNavigation.vue'
import DocsHeader from '@/components/docs/DocsHeader.vue'
import DocsSidebar from '@/components/docs/DocsSidebar.vue'
import DocsTableOfContents from '@/components/docs/DocsTableOfContents.vue'
import { docsNavigation, getAdjacentDocuments, resolveDocument } from '@/docs/registry'
import { renderMarkdown } from '@/docs/markdown'
import DocsNotFoundView from './DocsNotFoundView.vue'

const route = useRoute()
const router = useRouter()
const mobileMenuOpen = ref(false)
const isDark = ref(resolveInitialTheme())

const queryValue = (value: unknown): string | undefined => typeof value === 'string' ? value : undefined
const resolved = computed(() => resolveDocument(queryValue(route.query.cat), queryValue(route.query.page)))
const rendered = computed(() => resolved.value ? renderMarkdown(resolved.value.document.source) : { html: '', headings: [] })
const adjacent = computed(() => resolved.value
  ? getAdjacentDocuments(resolved.value.location.category, resolved.value.location.page)
  : { previous: null, next: null })

watch(() => route.fullPath, () => {
  mobileMenuOpen.value = false
  window.scrollTo({ top: 0, behavior: 'smooth' })
})

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

  const link = target.closest<HTMLAnchorElement>('a[href^="/docs"]')
  if (!link) return
  event.preventDefault()
  void router.push(link.getAttribute('href') || '/docs')
}

async function copyCode(button: HTMLButtonElement, code: string): Promise<void> {
  await navigator.clipboard.writeText(code)
  const previousLabel = button.textContent
  button.textContent = '已复制'
  window.setTimeout(() => {
    button.textContent = previousLabel
  }, 1600)
}
</script>

<style scoped>
.docs-page {
  --docs-bg: #f5f8fb; --docs-surface: rgba(255,255,255,.76); --docs-text: #102130; --docs-muted: #536777;
  --docs-faint: #7e909d; --docs-border: rgba(39,79,103,.13); --docs-accent: #087fa2; --docs-accent-soft: rgba(8,127,162,.35);
  --docs-accent-border: rgba(8,127,162,.24); --docs-accent-bg: rgba(8,127,162,.08);
  min-height: 100vh; color: var(--docs-text); background: var(--docs-bg); font-family: "Avenir Next", "Segoe UI", sans-serif;
}
.docs-page.is-dark {
  --docs-bg: #080c12; --docs-surface: rgba(17,25,36,.72); --docs-text: #e8f4fb; --docs-muted: #9db0bf;
  --docs-faint: #6f8494; --docs-border: rgba(143,189,219,.13); --docs-accent: #65e6ff; --docs-accent-soft: rgba(101,230,255,.42);
  --docs-accent-border: rgba(101,230,255,.2); --docs-accent-bg: rgba(45,178,213,.09);
}
.docs-ambient { position: fixed; inset: 68px 0 0; overflow: hidden; pointer-events: none; }
.docs-ambient::before { content: ''; position: absolute; inset: 0; opacity: .22; background-image: linear-gradient(var(--docs-border) 1px, transparent 1px), linear-gradient(90deg, var(--docs-border) 1px, transparent 1px); background-size: 52px 52px; mask-image: linear-gradient(to bottom, black, transparent 72%); }
.docs-ambient span { position: absolute; width: 420px; height: 420px; border-radius: 50%; filter: blur(90px); opacity: .11; background: #28c8ff; }
.docs-ambient span:first-child { top: -220px; left: 35%; } .docs-ambient span:last-child { right: -250px; top: 34%; background: #5d66ff; }
.docs-layout { position: relative; max-width: 1520px; margin: 0 auto; padding: 0 28px; display: grid; grid-template-columns: 232px minmax(0, 820px) 210px; justify-content: center; gap: 36px; }
.docs-layout__sidebar, .docs-layout__toc { position: sticky; top: 68px; align-self: start; max-height: calc(100vh - 68px); overflow-y: auto; }
.docs-main { min-width: 0; padding: 42px 0 90px; }
.docs-breadcrumb { margin-bottom: 42px; color: var(--docs-faint); font: 600 11px/1.5 ui-monospace, SFMono-Regular, Consolas, monospace; letter-spacing: .05em; }
.docs-breadcrumb span { margin: 0 8px; color: var(--docs-accent); }
.docs-article-header { padding-bottom: 34px; border-bottom: 1px solid var(--docs-border); }
.docs-article-kicker { color: var(--docs-accent); font: 800 10px/1.2 ui-monospace, SFMono-Regular, Consolas, monospace; letter-spacing: .2em; }
.docs-article-header h1 { margin: 15px 0 12px; font-family: Georgia, "Noto Serif SC", serif; font-size: clamp(38px, 6vw, 62px); line-height: 1.04; letter-spacing: -.035em; }
.docs-article-header p { max-width: 640px; margin: 0; color: var(--docs-muted); font-size: 16px; line-height: 1.75; }
.docs-article-meta { margin-top: 20px; display: flex; align-items: center; gap: 10px; color: var(--docs-faint); font-size: 11px; }
.docs-article-meta__dot { width: 3px; height: 3px; border-radius: 50%; background: var(--docs-accent); }
.docs-prose { padding-top: 30px; color: var(--docs-muted); font-size: 15px; line-height: 1.85; }
.docs-prose :deep(h1:first-child) { display: none; }
.docs-prose :deep(h2), .docs-prose :deep(h3) { scroll-margin-top: 92px; color: var(--docs-text); font-family: Georgia, "Noto Serif SC", serif; }
.docs-prose :deep(h2) { margin: 46px 0 15px; padding-top: 4px; font-size: 27px; letter-spacing: -.02em; }
.docs-prose :deep(h3) { margin: 30px 0 12px; font-size: 20px; }
.docs-prose :deep(p) { margin: 0 0 18px; }
.docs-prose :deep(a) { color: var(--docs-accent); text-decoration-color: var(--docs-accent-border); text-underline-offset: 4px; }
.docs-prose :deep(strong) { color: var(--docs-text); }
.docs-prose :deep(ul), .docs-prose :deep(ol) { margin: 0 0 20px; padding-left: 24px; }
.docs-prose :deep(li) { margin: 7px 0; padding-left: 4px; }
.docs-prose :deep(blockquote) { margin: 24px 0; padding: 16px 18px; color: var(--docs-text); border: 1px solid var(--docs-accent-border); border-left: 3px solid var(--docs-accent); border-radius: 4px 12px 12px 4px; background: var(--docs-accent-bg); }
.docs-prose :deep(blockquote p) { margin: 0; }
.docs-prose :deep(code) { padding: 2px 6px; color: var(--docs-accent); border: 1px solid var(--docs-border); border-radius: 5px; background: var(--docs-surface); font: 12px/1.6 ui-monospace, SFMono-Regular, Consolas, monospace; }
.docs-prose :deep(.docs-code-block) { margin: 24px 0; overflow: hidden; border: 1px solid rgba(115,190,224,.16); border-radius: 13px; background: #08111b; box-shadow: 0 18px 40px rgba(0,0,0,.16); }
.docs-prose :deep(.docs-code-block__toolbar) { min-height: 37px; padding: 0 11px 0 16px; display: flex; align-items: center; justify-content: space-between; color: #7893a5; border-bottom: 1px solid rgba(115,190,224,.12); background: #0b1622; font: 700 9px/1 ui-monospace, SFMono-Regular, Consolas, monospace; letter-spacing: .13em; }
.docs-prose :deep(.docs-code-block__toolbar button) { padding: 6px 9px; color: #9eb7c6; border: 1px solid rgba(115,190,224,.16); border-radius: 6px; background: transparent; cursor: pointer; font: 600 11px/1 ui-monospace, SFMono-Regular, Consolas, monospace; letter-spacing: 0; }
.docs-prose :deep(.docs-code-block__toolbar button:hover) { color: #65e6ff; border-color: rgba(101,230,255,.35); }
.docs-prose :deep(pre) { margin: 0; padding: 19px 21px; overflow-x: auto; color: #d9edf8; background: transparent; }
.docs-prose :deep(pre code) { padding: 0; color: inherit; border: 0; background: transparent; font-size: 12px; }
.docs-mobile-toc { display: none; margin-top: 24px; }
.docs-mobile-toc details { border: 1px solid var(--docs-border); border-radius: 12px; background: var(--docs-surface); }
.docs-mobile-toc summary { padding: 14px 16px; color: var(--docs-text); cursor: pointer; font-size: 13px; font-weight: 700; }
.docs-mobile-toc :deep(.docs-toc) { padding: 0 16px 16px; }
.docs-drawer { position: fixed; inset: 0; z-index: 100; display: flex; }
.docs-drawer__backdrop { position: absolute; inset: 0; border: 0; background: rgba(1,5,10,.72); backdrop-filter: blur(6px); }
.docs-drawer__panel { position: relative; width: min(86vw, 330px); height: 100%; overflow-y: auto; color: var(--docs-text); background: var(--docs-bg); box-shadow: 20px 0 70px rgba(0,0,0,.35); }
.docs-drawer__header { height: 68px; padding: 0 18px; display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid var(--docs-border); }
.docs-drawer__header button { width: 36px; height: 36px; color: var(--docs-text); border: 1px solid var(--docs-border); border-radius: 9px; background: var(--docs-surface); font-size: 22px; cursor: pointer; }
@media (max-width: 1180px) { .docs-layout { grid-template-columns: 220px minmax(0, 780px); } .docs-layout__toc { display: none; } .docs-mobile-toc { display: block; } }
@media (max-width: 960px) { .docs-layout { display: block; padding: 0 22px; } .docs-layout__sidebar { display: none; } .docs-main { max-width: 780px; margin: 0 auto; padding-top: 30px; } }
@media (max-width: 600px) { .docs-layout { padding: 0 17px; } .docs-breadcrumb { margin-bottom: 28px; } .docs-article-header h1 { font-size: 38px; } .docs-prose { font-size: 14px; } .docs-prose :deep(h2) { font-size: 24px; } }
</style>
