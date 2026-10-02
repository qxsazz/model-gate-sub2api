<template>
  <span v-if="level > 0" class="vip-badge" :class="`vip-badge-${level}`" :title="`VIP ${level} · 查看会员权益`">
    <Icon name="badge" size="sm" aria-hidden="true" /> VIP {{ level }}
  </span>
</template>
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { getVIP } from '@/api/vip'
const level = ref(0)
onMounted(async () => { try { level.value = (await getVIP()).badge_level } catch { level.value = 0 } })
</script>
<style scoped>
.vip-badge { display: inline-flex; align-items: center; gap: 5px; height: 24px; padding: 0 8px; border: 1px solid #927c46; border-radius: 5px; color: #dfcf9f; background: #121316; font: 500 11px 'DM Mono', Consolas, monospace; white-space: nowrap; letter-spacing: 0; }
.vip-badge-3, .vip-badge-4, .vip-badge-5 { border-color: #c9b477; }
.vip-badge-5 { box-shadow: inset 0 0 0 1px rgb(201 180 119 / 15%); color: #f1dfac; }
</style>
