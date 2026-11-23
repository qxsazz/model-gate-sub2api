import type { ChatAttachmentUpload } from '@/api/webChat'

const imageTypes = new Map([['png', 'image/png'], ['jpg', 'image/jpeg'], ['jpeg', 'image/jpeg'], ['webp', 'image/webp']])
const textExtensions = new Set('txt md markdown csv tsv json log yaml yml xml html css js ts py go sh'.split(' '))
const documentExtensions = new Set(['pdf', 'docx'])
export const chatFileAccept = [...imageTypes.keys(), ...textExtensions, ...documentExtensions].map(ext => '.' + ext).join(',')
export const maxChatFiles = 3
export const maxChatFileBytes = 4 * 1024 * 1024
const maxTextBytes = 128 * 1024

export interface DraftChatFile extends ChatAttachmentUpload {
  size: number
  imageSrc?: string
  textPreview?: string
}

function extension(file: File) {
  const dot = file.name.lastIndexOf('.')
  return dot < 0 ? '' : file.name.slice(dot + 1).toLowerCase()
}

export function validateChatFiles(existing: DraftChatFile[], files: File[]) {
  if (existing.length + files.length > maxChatFiles) return 'count'
  if ([...existing, ...files].reduce((n, f) => n + f.size, 0) > maxChatFileBytes) return 'total'
  for (const file of files) {
    const ext = extension(file)
    if (!file.size) return 'empty'
    if (!imageTypes.has(ext) && !documentExtensions.has(ext) && !textExtensions.has(ext)) return 'type'
    if (textExtensions.has(ext) && file.size > maxTextBytes) return 'textSize'
  }
  return ''
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
      const ext = extension(file)
      const item: DraftChatFile = { name: file.name, mime_type: file.type, data, size: file.size }
      if (imageTypes.has(ext)) item.imageSrc = 'data:' + imageTypes.get(ext) + ';base64,' + data
      if (textExtensions.has(ext)) {
        try {
          const bytes = Uint8Array.from(atob(data), c => c.charCodeAt(0))
          if (bytes.includes(0)) throw new Error('binary')
          item.textPreview = new TextDecoder('utf-8', { fatal: true }).decode(bytes).trim().slice(0, 120)
        } catch { reject(new Error('encoding')); return }
      }
      resolve(item)
    }
    reader.readAsDataURL(file)
  })
}

export function isChatImage(type: string) {
  return ['image/png', 'image/jpeg', 'image/webp'].includes(type)
}

export function formatChatFileSize(size: number) {
  if (size < 1024) return size + ' B'
  if (size < 1024 * 1024) return (size / 1024).toFixed(1) + ' KiB'
  return (size / (1024 * 1024)).toFixed(1) + ' MiB'
}
