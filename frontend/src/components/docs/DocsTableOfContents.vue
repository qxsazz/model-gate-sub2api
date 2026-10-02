<template>
  <aside v-if="headings.length" class="docs-toc" aria-label="本页目录">
    <div class="docs-toc__title">本页目录</div>
    <a
      v-for="heading in headings"
      :key="heading.id"
      :href="`#${encodeURIComponent(heading.id)}`"
      class="docs-toc__link"
      :class="{ 'is-child': heading.level === 3 }"
      @click="scrollToHeading($event, heading.id)"
    >{{ heading.text }}</a>
  </aside>
</template>

<script setup lang="ts">
import type { DocHeading } from '@/docs/types'
import { scrollToDocHeading } from '@/docs/navigation'

defineProps<{ headings: DocHeading[] }>()

function scrollToHeading(event: MouseEvent, id: string): void {
  event.preventDefault()
  scrollToDocHeading(id)
}
</script>

<style scoped>
.docs-toc { padding: 36px 0 56px 24px; }
.docs-toc__title { margin-bottom: 14px; color: var(--docs-text); font-size: 11px; font-weight: 800; letter-spacing: .08em; }
.docs-toc__link { display: block; padding: 5px 0 5px 13px; color: var(--docs-faint); border-left: 1px solid var(--docs-border); text-decoration: none; font-size: 12px; line-height: 1.45; transition: .18s ease; }
.docs-toc__link:hover { color: var(--docs-accent); border-color: var(--docs-accent); }
.docs-toc__link.is-child { padding-left: 24px; font-size: 11px; }
</style>
