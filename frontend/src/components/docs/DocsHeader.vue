<template>
  <header class="docs-header">
    <div class="docs-header__inner">
      <div class="docs-header__left">
        <button
          type="button"
          class="docs-icon-button docs-menu-button"
          data-testid="docs-menu-button"
          aria-label="打开文档目录"
          @click="$emit('open-menu')"
        >
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 7h16M4 12h16M4 17h16" /></svg>
        </button>
        <RouterLink to="/home" class="docs-brand" aria-label="返回 Model-Gate 首页">
          <img :src="MG_BRAND.logo" alt="" />
          <span>MODEL-GATE <b>DOCS</b></span>
        </RouterLink>
      </div>

      <div class="docs-header__actions">
        <RouterLink to="/home" class="docs-header-link">首页</RouterLink>
        <RouterLink to="/dashboard" class="docs-header-link docs-header-link--primary">控制台</RouterLink>
        <button
          type="button"
          class="docs-icon-button"
          :aria-label="isDark ? '切换到浅色主题' : '切换到深色主题'"
          @click="$emit('toggle-theme')"
        >
          <svg v-if="isDark" viewBox="0 0 24 24" aria-hidden="true">
            <circle cx="12" cy="12" r="4" /><path d="M12 2v2M12 20v2M4.93 4.93l1.42 1.42M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.42-1.42M17.66 6.34l1.41-1.41" />
          </svg>
          <svg v-else viewBox="0 0 24 24" aria-hidden="true"><path d="M21 12.8A9 9 0 1 1 11.2 3 7 7 0 0 0 21 12.8Z" /></svg>
        </button>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { MG_BRAND } from '@/brand/config'
defineProps<{ isDark: boolean }>()
defineEmits<{ 'open-menu': []; 'toggle-theme': [] }>()
</script>

<style scoped>
.docs-header { position: sticky; top: 0; z-index: 40; border-bottom: 1px solid var(--docs-border); background: color-mix(in srgb, var(--docs-bg) 88%, transparent); backdrop-filter: blur(18px); }
.docs-header__inner { max-width: 1520px; height: 68px; margin: 0 auto; padding: 0 28px; display: flex; align-items: center; justify-content: space-between; gap: 20px; }
.docs-header__left, .docs-header__actions { display: flex; align-items: center; gap: 12px; }
.docs-brand { display: flex; align-items: center; gap: 11px; color: var(--docs-text); text-decoration: none; font-size: 13px; font-weight: 700; letter-spacing: .11em; white-space: nowrap; }
.docs-brand img { width: 31px; height: 31px; object-fit: contain; filter: drop-shadow(0 0 12px rgba(47, 197, 255, .35)); }
.docs-brand b { color: var(--docs-accent); font-weight: 800; }
.docs-header-link { color: var(--docs-muted); padding: 8px 12px; border-radius: 9px; text-decoration: none; font-size: 13px; transition: .2s ease; }
.docs-header-link:hover { color: var(--docs-text); background: var(--docs-surface); }
.docs-header-link--primary { color: #041018; background: linear-gradient(135deg, #65e6ff, #56a8ff); font-weight: 700; }
.docs-header-link--primary:hover { color: #041018; box-shadow: 0 0 24px rgba(67, 197, 255, .28); }
.docs-icon-button { width: 38px; height: 38px; padding: 0; display: grid; place-items: center; color: var(--docs-muted); border: 1px solid var(--docs-border); border-radius: 10px; background: var(--docs-surface); cursor: pointer; }
.docs-icon-button:hover { color: var(--docs-text); border-color: var(--docs-accent-soft); }
.docs-icon-button svg { width: 18px; height: 18px; fill: none; stroke: currentColor; stroke-width: 1.8; stroke-linecap: round; stroke-linejoin: round; }
.docs-menu-button { display: none; }
@media (max-width: 960px) { .docs-header__inner { padding: 0 18px; } .docs-menu-button { display: grid; } }
@media (max-width: 560px) { .docs-header-link { display: none; } .docs-brand span { font-size: 11px; letter-spacing: .08em; } }
</style>
