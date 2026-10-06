<template>
  <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
    <path :d="paths[name] || paths.book" />
  </svg>
</template>
<script setup lang="ts">
defineProps<{ name: string }>()
const paths: Record<string, string> = {
  gift: 'M12 8v13M3 12h18M5 12v9h14v-9M3 8h18v4H3zM12 8H7a3 3 0 113-3l2 3zm0 0h5a3 3 0 10-3-3l-2 3z',
  key: 'M15 3a6 6 0 00-5.8 7.5L3 17.7V21h3.3l1.5-1.5H10v-2.2l3.3-3.3A6 6 0 1015 3z',
  grid: 'M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h7v7h-7z',
  card: 'M3 5h18v14H3zM3 10h18M6 15h4',
  chart: 'M4 21V11h4v10M10 21V6h4v15M16 21V2h4v19',
  user: 'M16 7a4 4 0 11-8 0 4 4 0 018 0zM5 21v-2a7 7 0 0114 0v2',
  users:
    'M16 20h5v-2a5 5 0 00-4-5M8 10a4 4 0 110-8 4 4 0 010 8zM2 21v-2a6 6 0 0112 0v2M16 2a4 4 0 010 8',
  document: 'M14 2H5v20h14V7l-5-5zM14 2v5h5M8 12h8M8 16h6',
  book: 'M12 5c-3-2-6-2-9-1v15c3-1 6-1 9 1m0-15c3-2 6-2 9-1v15c-3-1-6-1-9 1V5z',
  terminal: 'M3 4h18v16H3zM6 8l4 4-4 4M13 16h5',
  shield: 'M12 2l8 3v6c0 5-4 8-8 11-4-3-8-6-8-11V5l8-3zM8 12l3 3 5-6',
  coin: 'M12 22a10 10 0 100-20 10 10 0 000 20zM8 6l4 6 4-6M7 12h10M7 16h10M12 12v7',
  ticket:
    'M3 5h18v5a2 2 0 000 4v5H3v-5a2 2 0 000-4V5zM15 5v2m0 3v2m0 3v2m0 2v1',
  arrow: 'M4 12h16M14 6l6 6-6 6',
  check: 'M5 12l4 4L19 6',
  close: 'M5 5l14 14M19 5L5 19',
  moon: 'M21 13a9 9 0 01-10-10 9 9 0 1010 10z',
  sun: 'M12 3V1m0 22v-2M3 12H1m22 0h-2M5.6 5.6L4 4m16 16l-1.6-1.6M18.4 5.6L20 4M4 20l1.6-1.6M16 12a4 4 0 11-8 0 4 4 0 018 0z',
  menu: 'M3 6h18M3 12h18M3 18h18',
  info: 'M12 22a10 10 0 100-20 10 10 0 000 20zM12 10v7M12 6v1',
  bell: 'M4 18h16l-2-3V9a6 6 0 00-12 0v6l-2 3zM10 21h4',
  help: 'M9 8a3 3 0 116 0c0 2-3 2-3 5M12 17v1M22 12a10 10 0 11-20 0 10 10 0 0120 0z',
  copy: 'M8 16H5V3h11v3M8 7h11v14H8V7z',
}
</script>
