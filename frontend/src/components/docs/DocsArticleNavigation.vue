<template>
  <nav class="docs-article-nav" aria-label="上一篇和下一篇">
    <RouterLink v-if="previous && previousLocation" :to="docTarget(previousLocation)" class="docs-article-nav__card">
      <span>← 上一篇</span><strong>{{ previous.title }}</strong>
    </RouterLink>
    <span v-else></span>
    <RouterLink v-if="next && nextLocation" :to="docTarget(nextLocation)" class="docs-article-nav__card docs-article-nav__card--next">
      <span>下一篇 →</span><strong>{{ next.title }}</strong>
    </RouterLink>
  </nav>
</template>

<script setup lang="ts">
import { useDocumentationPath } from '@/docs/workspace'
import { computed } from 'vue'
import { findDocumentLocation } from '@/docs/registry'
import type { DocArticle, DocLocation } from '@/docs/types'

const basePath = useDocumentationPath()
const props = defineProps<{ previous: DocArticle | null; next: DocArticle | null }>()
const previousLocation = computed(() => props.previous ? findDocumentLocation(props.previous) : null)
const nextLocation = computed(() => props.next ? findDocumentLocation(props.next) : null)
const docTarget = (location: DocLocation) => ({ path: basePath, query: { cat: location.category, page: location.page } })
</script>

<style scoped>
.docs-article-nav { margin-top: 58px; padding-top: 24px; display: grid; grid-template-columns: 1fr 1fr; gap: 14px; border-top: 1px solid var(--docs-border); }
.docs-article-nav__card { min-height: 72px; padding: 14px 0; display: flex; flex-direction: column; justify-content: center; gap: 7px; color: var(--docs-muted); text-decoration: none; transition: .2s ease; }
.docs-article-nav__card:hover { color: var(--docs-accent); }
.docs-article-nav__card span { font-size: 11px; color: var(--docs-faint); }
.docs-article-nav__card strong { color: var(--docs-text); font-size: 14px; }
.docs-article-nav__card--next { text-align: right; }
@media (max-width: 600px) { .docs-article-nav { grid-template-columns: 1fr; } .docs-article-nav > span { display: none; } }
</style>
