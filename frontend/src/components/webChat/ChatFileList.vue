<template>
  <ul class="file-list" :aria-label="t('webChat.files.title')">
    <li v-for="(item, i) in files" :key="i" class="file-chip">
      <img v-if="item.imageSrc" :src="item.imageSrc" alt="" class="file-thumb" />
      <Icon v-else name="document" size="sm" aria-hidden="true" />
      <div class="file-info">
        <button v-if="item.id" type="button" class="file-action" :title="item.name"
          :aria-label="t(isChatImage(item.mime_type) ? 'webChat.files.view' : 'webChat.files.download', { name: item.name })" @click="$emit('open', { ...item, id: item.id })">
          {{ item.name }}
        </button>
        <span v-else class="file-name" :title="item.name">{{ item.name }}</span>
        <small>{{ formatChatFileSize(item.size) }}</small>
      </div>
      <button v-if="removable" type="button" class="file-remove" data-testid="chat-remove-attachment" :disabled="disabled"
        :aria-label="t('webChat.files.remove', { name: item.name })" @click="$emit('remove', i)">
        <Icon name="x" size="xs" />
      </button>
    </li>
  </ul>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { ChatAttachment } from '@/api/webChat'
import { formatChatFileSize, isChatImage } from '@/utils/webChatAttachments'

interface FileLabel { id?: string; name: string; mime_type: string; size: number; imageSrc?: string }
defineProps<{ files: FileLabel[]; removable?: boolean; disabled?: boolean }>()
defineEmits<{ open: [file: ChatAttachment]; remove: [index: number] }>()
const { t } = useI18n()
</script>

<style scoped>
.file-list { display: flex; flex-wrap: wrap; gap: 6px; min-width: 0; list-style: none; margin: 0; padding: 0; }
.file-chip { display: flex; align-items: center; gap: 7px; max-width: 100%; min-width: 0; padding: 5px 8px; border: 1px solid var(--mg-line-warm); border-radius: 7px; background: var(--mg-surface); }
.file-thumb { width: 34px; height: 34px; object-fit: cover; border-radius: 4px; }
.file-chip > svg, .file-thumb, .file-remove { flex: none; }
.file-info { min-width: 0; max-width: 14rem; line-height: 1.3; }
.file-name, .file-action { display: block; max-width: 100%; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; text-align: start; }
.file-name, .file-action { font-size: 12px; font-weight: 600; }
.file-action { text-decoration: underline; text-underline-offset: 2px; }
.file-info small { color: var(--mg-muted); font-size: 11px; }
.file-remove { display: grid; place-items: center; width: 28px; height: 28px; }
</style>
