<template>
  <nav class="docs-sidebar" aria-label="文档目录">
    <div class="docs-sidebar__eyebrow">DOCUMENTATION</div>
    <div v-for="group in navigation" :key="group.slug" class="docs-sidebar__group">
      <div class="docs-sidebar__group-title">{{ group.title }}</div>
      <RouterLink
        v-for="article in group.articles"
        :key="article.slug"
        :to="{ path: '/docs', query: { cat: group.slug, page: article.slug } }"
        class="docs-sidebar__link"
        :class="{ 'is-active': currentCategory === group.slug && currentPage === article.slug }"
        :aria-current="currentCategory === group.slug && currentPage === article.slug ? 'page' : undefined"
        @click="$emit('navigate')"
      >
        <span>{{ article.title }}</span>
        <svg viewBox="0 0 20 20" aria-hidden="true"><path d="m7 4 6 6-6 6" /></svg>
      </RouterLink>
    </div>
  </nav>
</template>

<script setup lang="ts">
import type { DocGroup } from '@/docs/types'

defineProps<{
  navigation: DocGroup[]
  currentCategory?: string
  currentPage?: string
}>()
defineEmits<{ navigate: [] }>()
</script>

<style scoped>
.docs-sidebar { padding: 34px 24px 60px 6px; }
.docs-sidebar__eyebrow { margin: 0 12px 28px; color: var(--docs-faint); font: 700 10px/1.2 ui-monospace, SFMono-Regular, Consolas, monospace; letter-spacing: .2em; }
.docs-sidebar__group { margin-bottom: 29px; }
.docs-sidebar__group-title { margin: 0 12px 9px; color: var(--docs-text); font-size: 12px; font-weight: 700; letter-spacing: .04em; }
.docs-sidebar__link { min-height: 37px; padding: 8px 11px; display: flex; align-items: center; justify-content: space-between; gap: 8px; color: var(--docs-muted); border: 1px solid transparent; border-radius: 9px; text-decoration: none; font-size: 13px; transition: .18s ease; }
.docs-sidebar__link:hover { color: var(--docs-text); background: var(--docs-surface); }
.docs-sidebar__link svg { width: 14px; height: 14px; opacity: 0; fill: none; stroke: currentColor; stroke-width: 1.6; }
.docs-sidebar__link.is-active { color: var(--docs-accent); border-color: var(--docs-accent-border); background: linear-gradient(90deg, var(--docs-accent-bg), transparent); box-shadow: inset 2px 0 var(--docs-accent); }
.docs-sidebar__link.is-active svg { opacity: 1; }
</style>
