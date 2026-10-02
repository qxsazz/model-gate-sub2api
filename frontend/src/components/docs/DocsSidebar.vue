<template>
  <nav class="docs-sidebar" aria-label="文档目录">
    <div class="docs-sidebar__eyebrow">DOCUMENTATION</div>
    <label class="docs-search"><Icon name="search" size="sm" /><input v-model="query" aria-label="搜索文档" placeholder="搜索文档" /><button v-if="query" type="button" aria-label="清除搜索" @click="query = ''"><Icon name="x" size="sm" /></button></label>
    <RouterLink to="/docs" class="docs-sidebar__link docs-overview-link" :class="{ 'is-active': overview }" :aria-current="overview ? 'page' : undefined" @click="$emit('navigate')"><span class="docs-category-label"><span class="docs-nav-art" aria-hidden="true"><Icon name="grid" size="sm" /></span>文档总览</span><Icon name="chevronRight" size="xs" /></RouterLink>
    <p v-if="!filteredNavigation.length" role="status" class="docs-search-empty">没有找到相关文档</p>
    <div v-for="group in filteredNavigation" :key="group.slug" class="docs-sidebar__group">
      <button type="button" class="docs-sidebar__group-title" :aria-label="group.title" :aria-expanded="expanded.has(group.slug)" :aria-controls="panelId(group.slug)" @click="toggleGroup(group.slug)">
        <span class="docs-category-label"><span class="docs-nav-art" data-nav-art :data-icon="getDocCategoryIcon(group.slug)" aria-hidden="true"><Icon :name="getDocCategoryIcon(group.slug)" size="sm" /></span>{{ group.title }}</span><Icon name="chevronDown" size="xs" class="docs-category-chevron" :class="{ 'is-expanded': expanded.has(group.slug) }" />
      </button>
      <div :id="panelId(group.slug)"><Transition name="docs-group"><div v-if="expanded.has(group.slug)" class="docs-group-links">
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
        <span v-if="article.status === 'upcoming'" class="docs-upcoming-label">敬请期待</span>
        <Icon v-else name="chevronRight" size="xs" />
        </RouterLink>
      </div></Transition></div>
    </div>
  </nav>
</template>

<script setup lang="ts">
import type { DocGroup } from '@/docs/types'
import { computed, getCurrentInstance, ref, watch } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { getDocCategoryIcon } from '@/docs/categories'

const props = defineProps<{
  navigation: DocGroup[]
  currentCategory?: string
  currentPage?: string
  overview?: boolean
}>()
defineEmits<{ navigate: [] }>()
const query = ref('')
const instanceId = getCurrentInstance()!.uid
const expanded = ref(new Set(['tutorial']))
const panelId = (slug: string) => `docs-group-${instanceId}-${slug}`
function toggleGroup(slug: string) {
  if (expanded.value.has(slug)) expanded.value.delete(slug)
  else expanded.value.add(slug)
}
watch(() => props.currentCategory, category => { if (category) expanded.value.add(category) }, { immediate: true })
const filteredNavigation = computed(() => {
  const needle = query.value.trim().toLowerCase()
  return props.navigation.map(group => ({ ...group, articles: group.articles.filter(article => !needle || `${article.title} ${article.description} ${group.title} ${article.source}`.toLowerCase().includes(needle)) })).filter(group => group.articles.length)
})
watch(query, value => {
  if (value.trim()) for (const group of filteredNavigation.value) expanded.value.add(group.slug)
})
</script>

<style scoped>
.docs-sidebar { padding: 28px 16px 50px 0; }
.docs-sidebar__eyebrow { margin: 0 12px 28px; color: var(--docs-faint); font: 700 10px/1.2 ui-monospace, SFMono-Regular, Consolas, monospace; letter-spacing: .2em; }
.docs-sidebar__group { margin-bottom: 8px; }
.docs-overview-link { margin-bottom: 16px; font-weight: 500; }
.docs-overview-link svg { opacity: 1; }
.docs-overview-link .docs-nav-art svg { width: 16px; height: 16px; }
.docs-overview-link.is-active .docs-nav-art, .docs-overview-link:hover .docs-nav-art { color: #ebd79e; border-color: #c9b477; }
.docs-sidebar__group-title { display: flex; align-items: center; justify-content: space-between; gap: 10px; width: 100%; padding: 10px 12px; color: var(--docs-text); font-size: 12px; font-weight: 500; text-align: left; border-radius: 4px; }
.docs-sidebar__group-title:hover { color: var(--docs-accent); background: var(--docs-accent-bg); }
.docs-category-chevron { flex-shrink: 0; color: var(--docs-faint); transform: rotate(-90deg); transition: transform .18s ease, color .18s ease; }
.docs-category-chevron.is-expanded { transform: rotate(0); color: var(--docs-accent); }
.docs-category-label { display: flex; align-items: center; gap: 9px; }
.docs-nav-art { display: grid; place-items: center; width: 26px; height: 26px; flex-shrink: 0; color: #827c70; background: #1e2023; border: 1px solid #454239; border-radius: 4px; transition: color .2s ease, border-color .2s ease; }
.docs-sidebar__group-title:hover .docs-nav-art, .docs-sidebar__group-title[aria-expanded='true'] .docs-nav-art { color: #ebd79e; border-color: #c9b477; }
.docs-group-links { padding: 2px 0 4px 28px; }
.docs-group-enter-active, .docs-group-leave-active { transition: opacity .16s ease, transform .16s ease; }
.docs-group-enter-from, .docs-group-leave-to { opacity: 0; transform: translateY(-4px); }
.docs-sidebar__link { min-height: 32px; padding: 6px 11px; display: flex; align-items: center; justify-content: space-between; gap: 8px; color: var(--docs-muted); border: 1px solid transparent; border-radius: 4px; text-decoration: none; font-size: 12px; transition: .18s ease; }
.docs-sidebar__link:hover { color: var(--docs-text); background: var(--docs-surface); }
.docs-sidebar__link > svg { width: 14px; height: 14px; opacity: 0; fill: none; stroke: currentColor; stroke-width: 1.6; }
.docs-sidebar__link.is-active { color: var(--docs-text); border-color: var(--docs-accent-border); background: var(--docs-accent-bg); box-shadow: inset 2px 0 var(--docs-accent); }
.docs-sidebar__link.is-active svg { opacity: 1; }
.docs-search { display: flex; align-items: center; gap: 8px; margin: 0 0 28px; padding: 9px 10px; color: var(--docs-muted); border: 1px solid var(--docs-border); background: var(--docs-surface); border-radius: 4px; }
.docs-search input { width: 100%; min-width: 0; background: transparent; border: 0; outline: 0; font: inherit; font-size: 12px; color: var(--docs-text); }
.docs-search:focus-within { outline: 2px solid var(--docs-accent); outline-offset: 2px; }
.docs-search button { display: grid; place-items: center; flex-shrink: 0; }
.docs-search-empty { padding: 12px; font-size: 12px; color: var(--docs-muted); }
.docs-upcoming-label { flex-shrink: 0; font-size: 9px; color: var(--docs-faint); }
@media (prefers-reduced-motion: reduce) { .docs-sidebar * { transition: none !important; } }
</style>
