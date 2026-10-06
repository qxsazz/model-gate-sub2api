<template>
  <span class="medal-art" :style="style" role="img" :aria-label="name" />
</template>
<script setup lang="ts">
import { computed } from 'vue'
const props = defineProps<{ medal: string; name: string }>()
const crops: Record<string, number[]> = {
  T01: [198, 22, 339, 324],
  T02: [598, 20, 345, 328],
  T03: [994, 19, 354, 328],
  T04: [188, 350, 355, 330],
  T05: [596, 350, 353, 330],
  T06: [995, 350, 355, 330],
  R01: [171, 674, 376, 302],
  R02: [582, 674, 366, 302],
  R03: [990, 674, 375, 302],
  S01: [35, 192, 470, 637],
  S02: [531, 192, 473, 637],
  S03: [1020, 192, 486, 637],
  'A-K01': [72, 25, 450, 321],
  'A-K02': [543, 25, 452, 321],
  'A-K03': [1006, 25, 465, 321],
  'A-X01': [119, 354, 380, 307],
  'A-X02': [579, 354, 380, 307],
  'A-X03': [1041, 354, 380, 307],
  'A-C01': [65, 655, 449, 318],
  'A-C02': [536, 655, 465, 318],
  'A-C03': [1009, 655, 470, 318],
}
const style = computed(() => {
  const [x, y, w, h] = crops[props.medal] ?? crops.T01!
  const file = props.medal.startsWith('S')
    ? 'checkin'
    : props.medal.startsWith('A')
      ? 'activity'
      : 'medal'
  return {
    aspectRatio: `${w}/${h}`,
    backgroundImage: `url(${import.meta.env.BASE_URL}achievements/${file}-virtual-v2.png)`,
    backgroundSize: `${(1536 / w) * 100}% ${(1024 / h) * 100}%`,
    backgroundPosition: `${(x / (1536 - w)) * 100}% ${(y / (1024 - h)) * 100}%`,
  }
})
</script>
<style scoped>
.medal-art {
  display: block;
  width: 100%;
  background-repeat: no-repeat;
  mix-blend-mode: normal;
}
</style>
