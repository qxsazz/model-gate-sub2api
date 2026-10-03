<template>
  <AppLayout><DocsView embedded /></AppLayout>
</template>
<script setup lang="ts">
import { provide } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import DocsView from './DocsView.vue'
import { documentationPathKey } from '@/docs/workspace'
provide(documentationPathKey, '/guide')
</script>
