<template>
  <section data-testid="docs-overview" class="docs-overview">
    <header class="overview-header">
      <div class="overview-kicker">MODEL-GATE DOCUMENTATION</div>
      <h1><span class="overview-emblem" data-overview-emblem data-icon="grid" aria-hidden="true"><Icon name="grid" size="xl" /></span><span>文档总览</span></h1>
      <p>从首次接入到会员成长，找到你的下一步。</p>
    </header>
    <div class="overview-grid">
      <section v-for="(group, index) in navigation" :id="`overview-${group.slug}`" :key="group.slug" class="overview-category">
        <div class="overview-category-meta"><span>{{ String(index + 1).padStart(2, '0') }}</span><span v-if="group.articles.every(article => article.status === 'upcoming')">敬请期待</span><span v-else>{{ group.articles.length }} 篇文档</span></div>
        <div class="overview-category-heading">
          <div class="category-art" data-category-art :data-icon="categoryIcon(group.slug)" aria-hidden="true"><Icon :name="categoryIcon(group.slug)" size="xl" /></div>
          <div class="overview-category-copy"><h2><RouterLink :data-overview-category="group.slug" :to="target(group.slug, group.articles[0].slug)">{{ group.title }}<Icon name="arrowRight" size="sm" /></RouterLink></h2><p>{{ group.description }}</p></div>
        </div>
        <ul>
          <li v-for="article in group.articles" :key="article.slug"><RouterLink :to="target(group.slug, article.slug)">{{ article.title }}<Icon name="chevronRight" size="xs" /></RouterLink></li>
        </ul>
      </section>
    </div>
  </section>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'
import type { DocGroup } from '@/docs/types'
import { getDocCategoryIcon as categoryIcon } from '@/docs/categories'
defineProps<{ navigation: DocGroup[] }>()
const target = (category: string, page: string) => ({ path: '/docs', query: { cat: category, page } })
</script>

<style scoped>
.overview-header { padding: 18px 0 24px; border-bottom: 1px solid var(--docs-border); }
.overview-kicker { color: var(--docs-accent); font: 500 10px 'DM Mono', Consolas, monospace; }
.overview-header h1 { display: flex; align-items: center; gap: 14px; margin: 18px 0 14px; color: var(--docs-text); font: 500 40px/1.3 'Noto Serif SC', 'Songti SC', SimSun, Georgia, serif; }
.overview-emblem { display: grid; place-items: center; flex-shrink: 0; width: 52px; height: 52px; color: #ebd79e; border: 1px solid #c9b477; border-radius: 6px; background: #1e2023; }
.overview-emblem svg { width: 28px; height: 28px; }
.overview-header p { color: var(--docs-muted); font-size: 15px; line-height: 1.7; }
.overview-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; padding-top: 22px; }
.overview-category { min-width: 0; padding: 18px; border: 1px solid var(--docs-border); border-radius: 6px; background: var(--docs-surface); scroll-margin-top: 92px; transition: border-color .2s ease, box-shadow .2s ease; }
.overview-category:hover, .overview-category:focus-within { border-color: var(--docs-accent-border); box-shadow: 0 5px 18px rgb(25 25 25 / 5%); }
.overview-category-heading { display: flex; align-items: center; gap: 14px; margin: 16px 0; }
.overview-category-copy { flex: 1; min-width: 0; }
.category-art { display: grid; place-items: center; flex-shrink: 0; width: 52px; height: 52px; color: #827c70; border: 1px solid #454239; border-radius: 6px; background: #1e2023; transition: color .22s ease, border-color .22s ease, box-shadow .22s ease, transform .22s ease; }
.category-art svg { width: 28px; height: 28px; }
.overview-category:hover .category-art, .overview-category:focus-within .category-art { color: #ebd79e; border-color: #c9b477; box-shadow: inset 0 0 12px rgb(201 180 119 / 12%), 0 0 14px rgb(201 180 119 / 20%); transform: translateY(-2px); }
.overview-category-meta { display: flex; justify-content: space-between; gap: 12px; color: var(--docs-faint); font-size: 10px; }
.overview-category-meta span:first-child { color: var(--docs-accent); font-family: 'DM Mono', Consolas, monospace; }
.overview-category h2 { margin: 0 0 6px; color: var(--docs-text); font: 500 20px/1.4 'Noto Serif SC', 'Songti SC', SimSun, Georgia, serif; }
.overview-category h2 a { display: flex; align-items: center; justify-content: space-between; gap: 12px; text-decoration: none; }
.overview-category h2 svg { flex-shrink: 0; color: var(--docs-accent); transition: transform .2s ease; }
.overview-category:hover h2 svg { transform: translateX(3px); }
.overview-category p { margin: 0; color: var(--docs-muted); font-size: 12px; line-height: 1.6; }
.overview-category ul { list-style: none; margin: 0; padding: 12px 0 0; border-top: 1px solid var(--docs-border); }
.overview-category li a { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 5px 0; color: var(--docs-muted); text-decoration: none; font-size: 12px; line-height: 1.6; }
.overview-category a:hover { color: var(--docs-accent); }
.overview-category li svg { flex-shrink: 0; opacity: .5; }
@media (max-width: 600px) { .overview-header { padding-top: 18px; } .overview-header h1 { font-size: 30px; } .overview-grid { grid-template-columns: minmax(0, 1fr); } .overview-category p { min-height: 0; } }
@media (prefers-reduced-motion: reduce) { .overview-category, .category-art, .overview-category h2 svg { transition: none; } .overview-category:hover .category-art, .overview-category:focus-within .category-art, .overview-category:hover h2 svg { transform: none; } }
</style>
