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
          <Icon name="menu" size="md" />
        </button>
        <RouterLink to="/home" class="docs-brand" aria-label="返回 Model-Gate 首页">
          <img :src="MG_BRAND.logo" alt="" />
          <span>MODEL-GATE <b>接入文档</b></span>
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
          <Icon :name="isDark ? 'sun' : 'moon'" size="md" />
        </button>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { MG_BRAND } from '@/brand/config'
import Icon from '@/components/icons/Icon.vue'
defineProps<{ isDark: boolean }>()
defineEmits<{ 'open-menu': []; 'toggle-theme': [] }>()
</script>

<style scoped>
.docs-header { position: sticky; top: 0; z-index: 40; border-bottom: 1px solid var(--docs-border); background: color-mix(in srgb, var(--docs-bg) 88%, transparent); backdrop-filter: blur(18px); }
.docs-header__inner { max-width: 1520px; height: 68px; margin: 0 auto; padding: 0 28px; display: flex; align-items: center; justify-content: space-between; gap: 20px; }
.docs-header__left, .docs-header__actions { display: flex; align-items: center; gap: 12px; }
.docs-brand { display: flex; align-items: center; gap: 11px; color: var(--docs-text); text-decoration: none; font-size: 18px; font-weight: 500; font-family: 'Bodoni MT', Didot, Georgia, serif; white-space: nowrap; }
.docs-brand img { width: 36px; height: 36px; object-fit: contain; }
.docs-brand b { padding-left: 12px; margin-left: 8px; border-left: 1px solid var(--docs-border); color: var(--docs-muted); font: 400 12px 'Noto Sans SC', 'Microsoft YaHei', sans-serif; }
.docs-header-link { color: var(--docs-muted); padding: 8px 12px; border-radius: 4px; text-decoration: none; font-size: 13px; transition: .2s ease; }
.docs-header-link:hover { color: var(--docs-text); background: var(--docs-surface); }
.docs-header-link--primary { color: #f5f1e7; background: #191a1d; font-weight: 500; border: 1px solid #514936; }
.docs-header-link--primary:hover { color: #e6cf93; background: #242528; }
.docs-icon-button { width: 38px; height: 38px; padding: 0; display: grid; place-items: center; color: var(--docs-muted); border: 1px solid var(--docs-border); border-radius: 4px; background: var(--docs-surface); cursor: pointer; }
.docs-icon-button:hover { color: var(--docs-text); border-color: var(--docs-accent-soft); }
.docs-icon-button svg { width: 18px; height: 18px; fill: none; stroke: currentColor; stroke-width: 1.8; stroke-linecap: round; stroke-linejoin: round; }
.docs-menu-button { display: none; }
@media (max-width: 960px) { .docs-header__inner { padding: 0 18px; } .docs-menu-button { display: grid; } }
@media (max-width: 560px) { .docs-header-link { display: none; } .docs-brand span { font-size: 15px; } .docs-brand b { display: none; } .docs-header__left { gap: 8px; } .docs-brand { gap: 8px; } }
</style>
