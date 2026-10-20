<template>
  <AppLayout>
    <div class="space-y-5">
      <div><h1 class="text-2xl font-semibold">画布内容审核</h1><p class="mt-2 text-sm opacity-70">事后审核用户生成和上传的图片、视频。素材保存 7 天，到期自动删除；审核不延长保存期限。</p></div>
      <div class="flex flex-wrap gap-3"><input v-model="userId" class="input w-40" placeholder="用户 ID" type="number" /><select v-model="reviewStatus" class="input w-40"><option value="">所有审核状态</option><option value="unreviewed">未审核</option><option value="approved">正常</option><option value="blocked">已屏蔽</option></select><button class="btn btn-primary" @click="page = 1; load()">查询</button></div>
      <p v-if="error" class="text-red-600" role="alert">{{ error }}</p>
      <p v-if="loading">正在加载…</p>
      <p v-else-if="!items.length" class="py-16 text-center opacity-60">暂无符合条件的素材</p>
      <div class="grid gap-5 md:grid-cols-2 xl:grid-cols-3">
        <article v-for="item in items" :key="item.id" class="review-card">
          <div class="review-preview">
            <img v-if="urls[item.id] && item.mime_type.startsWith('image/')" :src="urls[item.id]" alt="用户画布素材" />
            <video v-else-if="urls[item.id] && item.mime_type.startsWith('video/')" :src="urls[item.id]" controls preload="metadata" />
            <audio v-else-if="urls[item.id]" :src="urls[item.id]" controls />
            <button v-else-if="item.available" class="btn btn-secondary" @click="preview(item)">查看内容（记录访问）</button>
            <span v-else>素材已删除或过期</span>
          </div>
          <div class="space-y-2 p-4 text-sm">
            <strong>用户 #{{ item.user_id }} · {{ item.email }}</strong>
            <p>{{ item.source === 'generation' ? '生成内容' : '上传素材' }} · {{ item.model || item.mime_type }} · {{ labels[item.review_status] }}</p>
            <p class="break-words whitespace-pre-wrap">{{ item.prompt || '未提供提示词' }}</p>
            <p class="text-xs opacity-60">创建：{{ new Date(item.created_at).toLocaleString() }}<br />到期：{{ new Date(item.expires_at).toLocaleString() }}</p>
            <input v-model="notes[item.id]" class="input" placeholder="审核备注（屏蔽时必填）" />
            <div v-if="item.available" class="flex flex-wrap gap-2"><button class="btn btn-secondary" :disabled="reviewing === item.id" @click="review(item, 'approved')">标记正常</button><button class="btn btn-secondary text-red-600" :disabled="reviewing === item.id" @click="review(item, 'blocked')">屏蔽内容</button><button class="btn btn-secondary" :disabled="reviewing === item.id" @click="review(item, 'unreviewed')">待复核</button><button class="btn btn-secondary text-red-600" :disabled="reviewing === item.id" @click="deleting = item; deleteReason = ''; error = ''">删除素材</button></div>
          </div>
        </article>
      </div>
      <div class="flex items-center justify-end gap-4"><button class="btn btn-secondary" :disabled="page <= 1 || loading" @click="page--; load()">上一页</button><span>第 {{ page }} 页</span><button class="btn btn-secondary" :disabled="!hasMore || loading" @click="page++; load()">下一页</button></div>
    </div>
    <ConfirmDialog :show="!!deleting" title="删除素材" message="删除后画布、历史记录和素材库中的对应内容都将无法访问，文件将从服务器清理。此操作不可恢复。" confirm-text="确认删除" danger @cancel="deleting = null" @confirm="deleteAsset">
      <label class="block text-sm">删除原因<textarea v-model="deleteReason" class="input mt-2" maxlength="2000" placeholder="请填写删除原因" /></label>
      <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
    </ConfirmDialog>
  </AppLayout>
</template>
<script setup lang="ts">
import { ref, reactive, onMounted, onBeforeUnmount } from 'vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { apiClient } from '@/api/client'
interface Asset { id: string; user_id: number; email: string; mime_type: string; model: string; prompt: string; source: string; review_status: string; review_note: string; created_at: string; expires_at: string; available: boolean }
const deleting = ref<Asset | null>(null), deleteReason = ref('')
const labels: Record<string, string> = { unreviewed: '未审核', approved: '正常', blocked: '已屏蔽' }
const items = ref<Asset[]>([]), userId = ref(''), reviewStatus = ref(''), page = ref(1), hasMore = ref(false), loading = ref(false), error = ref(''), reviewing = ref('')
const urls = reactive<Record<string, string>>({}), notes = reactive<Record<string, string>>({})
function clearUrls() { Object.keys(urls).forEach((id) => { URL.revokeObjectURL(urls[id]); delete urls[id] }) }
async function load() { loading.value = true; error.value = ''; clearUrls(); try { const { data } = await apiClient.get('/admin/canvas/assets', { params: { user_id: userId.value, status: reviewStatus.value, page: page.value } }); items.value = data.items; hasMore.value = data.has_more; items.value.forEach((a) => { notes[a.id] = a.review_note }) } catch (e) { error.value = (e as Error).message } finally { loading.value = false } }
async function preview(item: Asset) { try { const { data } = await apiClient.get<Blob>(`/admin/canvas/assets/${item.id}/content`, { responseType: 'blob' }); urls[item.id] = URL.createObjectURL(data) } catch (e) { error.value = (e as Error).message } }
async function review(item: Asset, status: string) { reviewing.value = item.id; error.value = ''; try { await apiClient.put(`/admin/canvas/assets/${item.id}/review`, { status, note: notes[item.id] || '' }); item.review_status = status } catch (e) { error.value = (e as Error).message } finally { reviewing.value = '' } }
async function deleteAsset() {
  if (!deleting.value || reviewing.value) return
  if (!deleteReason.value.trim()) { error.value = '请填写删除原因'; return }
  const item = deleting.value
  reviewing.value = item.id; error.value = ''
  try {
    await apiClient.delete(`/admin/canvas/assets/${item.id}`, { data: { reason: deleteReason.value.trim() } })
    item.available = false
    if (urls[item.id]) { URL.revokeObjectURL(urls[item.id]); delete urls[item.id] }
    deleting.value = null
  } catch (e) { error.value = (e as Error).message } finally { reviewing.value = '' }
}
onMounted(load); onBeforeUnmount(clearUrls)
</script>
<style scoped>
.review-card { overflow: hidden; border: 1px solid var(--mg-line-warm, #e7e1d4); background: var(--mg-surface, white); border-radius: 12px; }
.review-preview { height: 240px; display: flex; align-items: center; justify-content: center; background: var(--mg-champagne-50, #f5f1e7); }
.review-preview img, .review-preview video { max-width: 100%; max-height: 100%; object-fit: contain; }
</style>
