<template>
  <section class="docs-not-found" data-testid="docs-not-found">
    <div class="docs-not-found__code">DOCS / 404</div>
    <h1>没有找到这篇文档</h1>
    <p>链接可能已经变更，或者查询参数不在 Model-Gate 文档注册表中。</p>
    <RouterLink :to="basePath" data-testid="docs-default-link">返回快速开始</RouterLink>
  </section>
</template>

<script setup lang="ts">
import { useDocumentationPath } from '@/docs/workspace'
const basePath = useDocumentationPath()
</script>
<style scoped>
.docs-not-found { max-width: 620px; margin: 70px auto; padding: 48px; text-align: center; border: 1px solid var(--docs-border); border-radius: 20px; background: var(--docs-surface); }
.docs-not-found__code { color: var(--docs-accent); font: 700 11px/1.2 ui-monospace, SFMono-Regular, Consolas, monospace; letter-spacing: .18em; }
h1 { margin: 16px 0 10px; color: var(--docs-text); font-size: clamp(28px, 4vw, 42px); }
p { margin: 0 auto 28px; color: var(--docs-muted); line-height: 1.75; }
a { display: inline-flex; padding: 11px 18px; color: #041018; border-radius: 10px; background: linear-gradient(135deg, #65e6ff, #56a8ff); text-decoration: none; font-weight: 700; }
</style>
