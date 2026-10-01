<template>
  <article class="vip-membership-card" :class="'palette-' + level" :data-material="material" :aria-label="'VIP ' + level + ' ' + memberTitle">
    <img class="vip-card-artwork" src="/mg-vip-b-engraving.png" alt="" width="1644" height="956" />
    <div class="vip-card-brand"><span>MODEL-GATE</span><small>PRIVATE MEMBERSHIP</small></div>
    <span v-if="current" class="vip-card-current">当前等级</span>
    <div class="vip-card-rule" />
    <h3 class="vip-card-heading"><span class="vip-card-latin">VIP <span class="vip-card-number">{{ level }}</span></span><span class="vip-card-divider">·</span>{{ memberTitle }}</h3>
    <div class="vip-card-description"><p>{{ thresholdLabel }}</p><p v-for="(benefit, index) in benefits" :key="index">{{ benefit }}</p></div>
    <footer v-if="ownerName" class="vip-card-footer member-owner"><strong :title="ownerName">{{ ownerName }}</strong><span v-if="ownerId != null">UID {{ ownerId }}</span></footer>
    <footer v-else class="vip-card-footer"><span>MODEL-GATE MEMBERSHIP</span><span>VIP {{ level }}</span></footer>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
const props = defineProps<{
  level: number
  name: string
  thresholdLabel: string
  benefits: string[]
  ownerName?: string
  ownerId?: number
  current?: boolean
}>()
const material = computed(() => ['silver', 'bronze', 'pearl', 'gold', 'platinum', 'black-diamond'][props.level] || 'silver')
const memberTitle = computed(() => props.name.endsWith('会员') ? props.name : props.name + '会员')
</script>

<style scoped>
.vip-membership-card{position:relative;width:100%;min-width:0;aspect-ratio:var(--vip-aspect-ratio,1.72);overflow:hidden;isolation:isolate;container-type:inline-size;border-radius:6px;background:var(--vip-surface);color:var(--vip-ink);letter-spacing:0}
.palette-0{--vip-surface:#eaf2f8;--vip-ink:#344650;--vip-accent:#a5aeb4}
.palette-1{--vip-surface:#edd0b8;--vip-ink:#613d29;--vip-accent:#ab7951}
.palette-2{--vip-surface:#fff;--vip-ink:#424b47;--vip-accent:#b7bcb9}
.palette-3{--vip-surface:#f5e2ae;--vip-ink:#624c20;--vip-accent:#b19551}
.palette-4{--vip-surface:#c7dada;--vip-ink:#2c4b4f;--vip-accent:#809d9d}
.palette-5{--vip-surface:#101216;--vip-ink:#e4d0a3;--vip-accent:#a18e66}
.vip-card-artwork{position:absolute;inset:0;width:100%;height:100%;object-fit:fill;filter:grayscale(1);mix-blend-mode:multiply;pointer-events:none}
.palette-2 .vip-card-artwork{filter:grayscale(1) brightness(1.12)}
.palette-5 .vip-card-artwork{filter:grayscale(1) invert(1);mix-blend-mode:screen;opacity:.7}
.vip-card-brand{position:absolute;left:6%;top:12%;display:flex;flex-direction:column;gap:6px}
.vip-card-brand>span{font:400 clamp(18px,4.4cqi,30px)/1 'Bodoni MT','Bodoni Moda','Times New Roman',serif}
.vip-card-brand>small{font:400 clamp(8px,1.6cqi,11px)/1.4 'Bodoni MT','Bodoni Moda','Times New Roman',serif}
.vip-card-current{position:absolute;top:8%;right:6%;font:500 clamp(8px,1.6cqi,10px)/1.4 'Noto Sans SC',sans-serif}
.vip-card-rule{position:absolute;left:6%;top:27%;width:15%;height:1px;background:var(--vip-accent)}
.vip-card-heading{position:absolute;left:6%;top:31%;max-width:54%;margin:0;color:inherit;font:400 clamp(17px,4.8cqi,30px)/1.2 'Noto Serif SC',SimSun,serif;white-space:nowrap}
.vip-card-latin{font-family:'Bodoni MT','Bodoni Moda','Times New Roman',serif;font-weight:400;font-variant-numeric:lining-nums tabular-nums;font-feature-settings:'lnum' 1,'tnum' 1}
.vip-card-divider{display:inline-block;margin:0 .2em}
.vip-card-description{position:absolute;left:6%;top:44%;width:53%;color:inherit;font:400 clamp(10px,2.12cqi,14px)/1.65 'Noto Serif SC',SimSun,serif}
.vip-card-description p{margin:0 0 3px}
.vip-card-footer{position:absolute;left:6%;top:77%;width:49%;border-top:1px solid var(--vip-accent);padding-top:8px;display:flex;flex-direction:column;gap:3px;color:inherit;font:400 clamp(8px,1.6cqi,11px)/1.4 'Bodoni MT','Bodoni Moda','Times New Roman',serif}
.vip-card-footer strong{font-weight:400;display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-family:'Noto Serif SC',SimSun,serif}
@container(max-width:420px){
 .vip-card-brand{top:10%;gap:3px}
 .vip-card-brand>span{font-size:16px}
 .vip-card-brand>small{font-size:7px;line-height:1.4}
 .vip-card-rule{top:28%}
 .vip-card-heading{top:32%}
 .vip-card-description{top:46%;font-size:10px;line-height:1.35}
 .vip-card-description p{margin-bottom:1px}
 .vip-card-footer{top:82%;padding-top:3px;gap:1px;font-size:8px;line-height:1.2}
}
@media(prefers-reduced-motion:no-preference){.vip-membership-card{transition:box-shadow .18s ease}}
</style>
