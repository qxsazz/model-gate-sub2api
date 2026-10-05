<template>
  <article class="vip-membership-card" :class="'palette-' + level" :data-material="material" :aria-label="'VIP ' + level + ' ' + memberTitle" @pointermove="reflect" @pointerleave="resetReflection">
    <img class="vip-card-artwork" src="/mg-vip-b-engraving.png" alt="" width="1644" height="956" />
    <div class="vip-card-grain" aria-hidden="true" />
    <div class="vip-card-reflection" aria-hidden="true" />
    <div class="vip-card-edge" aria-hidden="true" />
    <div class="vip-card-brand"><span>MODEL-GATE</span><small>PRIVATE MEMBERSHIP</small></div>
    <span v-if="current" class="vip-card-current">当前等级</span>
    <div class="vip-card-rule" />
    <h3 class="vip-card-heading"><span class="vip-card-latin">VIP <span class="vip-card-number">{{ level }}</span></span><span class="vip-card-divider">·</span>{{ memberTitle }}</h3>
    <div class="vip-card-description"><p v-for="(line, index) in [thresholdLabel, ...benefits]" :key="index"><span v-for="(part, partIndex) in textParts(line)" :key="partIndex" :class="{ 'vip-card-numeric': part.numeric }">{{ part.value }}</span></p></div>
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
function textParts(text: string) {
  return text.split(/([$]?\d+(?:[.,]\d+)*(?:%|\s*\/\s*日)?)/g).filter(Boolean).map(value => ({ value, numeric: /^\$?\d/.test(value) }))
}
function reflect(event: PointerEvent) {
  if (event.pointerType !== 'mouse' || window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return
  const card = event.currentTarget as HTMLElement
  const rect = card.getBoundingClientRect()
  if (!rect.width || !rect.height) return
  const x = Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width))
  const y = Math.max(0, Math.min(1, (event.clientY - rect.top) / rect.height))
  card.style.setProperty('--vip-light-x', `${x * 100}%`)
  card.style.setProperty('--vip-light-y', `${y * 100}%`)
  card.style.setProperty('--vip-beam', `${20 + x * 60}%`)
}
function resetReflection(event: PointerEvent) {
  const card = event.currentTarget as HTMLElement
  for (const key of ['--vip-light-x', '--vip-light-y', '--vip-beam']) card.style.removeProperty(key)
}
</script>

<style scoped>
.vip-membership-card{position:relative;width:100%;min-width:0;aspect-ratio:var(--vip-aspect-ratio,1.72);overflow:hidden;isolation:isolate;container-type:inline-size;border-radius:6px;background:var(--vip-surface);color:var(--vip-ink);letter-spacing:0}
.palette-0{--vip-surface:#b7bfc1;--vip-ink:#283a41;--vip-accent:#7c8d95;--vip-metal:linear-gradient(115deg,#929ea3,#ced6d8 26%,#e4e8e8 37%,#a1aeb4 55%,#ccd5d8 76%,#8d9ba3)}
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
.vip-card-latin{font-family:'DM Mono','SFMono-Regular',Consolas,monospace;font-weight:400;font-variant-numeric:lining-nums tabular-nums;font-feature-settings:'lnum' 1,'tnum' 1}
.vip-card-numeric,.vip-card-footer>span{font-family:'DM Mono','SFMono-Regular',Consolas,monospace;font-variant-numeric:lining-nums tabular-nums;font-feature-settings:'lnum' 1,'tnum' 1}
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
.vip-membership-card{background:var(--vip-metal,var(--vip-surface));border:1px solid var(--vip-accent);box-shadow:inset 0 1px 0 #ffffffb0,inset 0 -1px 0 #52667370,0 10px 22px #273a4719}
.palette-1{--vip-metal:linear-gradient(115deg,#b68b6f,#efd9c7 27%,#faf0e4 37%,#bb9177 55%,#ead0b9 75%,#ac8065)}
.palette-2{--vip-metal:linear-gradient(115deg,#edf0f0,#fcfdfd 26%,#fff 37%,#e9edef 55%,#fff 76%,#e5ebed)}
.palette-3{--vip-metal:linear-gradient(115deg,#bea365,#f1e2b3 25%,#fff4d6 37%,#c9b170 55%,#f1e2b5 76%,#b59a58)}
.palette-4{--vip-metal:linear-gradient(115deg,#9cbbba,#d9e9e6 25%,#f1f8f5 38%,#abc6c3 56%,#d7e7e2 77%,#91b1b0)}
.palette-5{--vip-metal:linear-gradient(115deg,#101317,#30353b 25%,#494c4f 36%,#161a1f 54%,#2c3238 76%,#101317);box-shadow:inset 0 1px 0 #ffffff30,inset 0 -1px 0 #000,0 10px 22px #151d3429}
.vip-card-artwork{filter:grayscale(1) contrast(1.18) brightness(1.09);opacity:.87}
.palette-2 .vip-card-artwork{filter:grayscale(1) brightness(1.16);opacity:.78}
.palette-5 .vip-card-artwork{filter:grayscale(1) invert(1);opacity:.5}
.vip-card-grain,.vip-card-reflection,.vip-card-edge{position:absolute;inset:0;pointer-events:none}
.vip-card-grain{z-index:1;background:repeating-linear-gradient(0deg,#ffffff24 0px,#ffffff24 .5px,#22384214 .5px,#22384214 1px,transparent 1px,transparent 3px);opacity:.6;mix-blend-mode:soft-light}
.vip-card-reflection{z-index:2;background:radial-gradient(ellipse at var(--vip-light-x,68%) var(--vip-light-y,27%),#ffffff8a,transparent 51%),linear-gradient(112deg,transparent,#fff0 calc(var(--vip-beam,45%) - 15%),#ffffffe0 var(--vip-beam,45%),#ffffff26 calc(var(--vip-beam,45%) + 10%),transparent calc(var(--vip-beam,45%) + 24%));mix-blend-mode:soft-light;opacity:.82}
.vip-card-edge{inset:4px;border:1px solid #ffffff8c;border-bottom-color:#526d7869;border-right-color:#526d7869;border-radius:4px;box-shadow:inset 0 0 0 1px #546c7630}
.vip-card-brand,.vip-card-heading,.vip-card-description,.vip-card-footer,.vip-card-current{z-index:3}
.vip-card-brand,.vip-card-heading{text-shadow:0 1px 0 #ffffffa0}
.palette-5 .vip-card-brand,.palette-5 .vip-card-heading{text-shadow:0 1px 0 #000a}
.palette-5 .vip-card-reflection{opacity:.5}.palette-2 .vip-card-grain{opacity:.4}
.vip-card-description{width:57%;font-size:clamp(10px,2.05cqi,13px);line-height:1.45}
.vip-card-footer{top:83%;padding-top:5px}
@container(max-width:420px){.vip-card-description{top:44%;font-size:10px;line-height:1.3}.vip-card-description p{margin-bottom:1px}.vip-card-footer{top:85%;padding-top:3px}}
@container(max-width:320px){.vip-card-description{width:64%;font-size:9px;line-height:1.25}.vip-card-heading{font-size:16px}}
@media(prefers-reduced-motion:reduce){.vip-card-reflection{opacity:.4}}
@media(prefers-reduced-motion:no-preference){.vip-membership-card{transition:box-shadow .18s ease}}
</style>
