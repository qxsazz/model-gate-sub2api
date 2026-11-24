import type { ChatAttachmentUpload } from '@/api/webChat'

const imageTypes = new Map([['png', 'image/png'], ['jpg', 'image/jpeg'], ['jpeg', 'image/jpeg'], ['webp', 'image/webp']])
const imageMimes = new Set(imageTypes.values())
export const chatFileAccept = [...Array.from(imageTypes.keys(), ext => '.' + ext), ...imageMimes].join(',')
export const maxChatFiles = 3
export const maxChatFileBytes = 4 * 1024 * 1024

export interface DraftChatFile extends ChatAttachmentUpload {
  size: number
  imageSrc?: string
}

function imageMime(file: File) {
  const dot = file.name.lastIndexOf('.')
  return dot < 0 ? undefined : imageTypes.get(file.name.slice(dot + 1).toLowerCase())
}

export function validateChatFiles(existing: DraftChatFile[], files: File[]) {
  if (existing.length + files.length > maxChatFiles) return 'count'
  if ([...existing, ...files].reduce((n, f) => n + f.size, 0) > maxChatFileBytes) return 'total'
  for (const file of files) {
    if (!file.size) return 'empty'
    const mime = imageMime(file)
    if (!mime || (file.type && file.type !== 'application/octet-stream' && file.type !== mime)) return 'type'
  }
  return ''
}

// Check the header before previewing. The server fully decodes the image and enforces its dimensions.
function matchesImageHeader(data: string, mime: string) {
  const header = atob(data.slice(0, 16))
  if (mime === 'image/png') return header.startsWith('\x89PNG\r\n\x1a\n')
  if (mime === 'image/jpeg') return header.startsWith('\xff\xd8\xff')
  return mime === 'image/webp' && header.startsWith('RIFF') && header.slice(8, 12) === 'WEBP'
}

export function readChatFile(file: File, signal: AbortSignal): Promise<DraftChatFile> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    const abort = () => reader.abort()
    if (signal.aborted) { reject(new Error('cancelled')); return }
    signal.addEventListener('abort', abort, { once: true })
    reader.onloadend = () => signal.removeEventListener('abort', abort)
    reader.onabort = reader.onerror = () => reject(new Error('read'))
    reader.onload = () => {
      const url = String(reader.result)
      const data = url.slice(url.indexOf(',') + 1)
      const mime = imageMime(file)
      if (!mime || !matchesImageHeader(data, mime)) { reject(new Error('type')); return }
      resolve({ name: file.name, mime_type: file.type, data, size: file.size, imageSrc: 'data:' + mime + ';base64,' + data })
    }
    reader.readAsDataURL(file)
  })
}

export function isChatImage(type: string) {
  return imageMimes.has(type)
}

export function formatChatFileSize(size: number) {
  if (size < 1024) return size + ' B'
  if (size < 1024 * 1024) return (size / 1024).toFixed(1) + ' KiB'
  return (size / (1024 * 1024)).toFixed(1) + ' MiB'
}
