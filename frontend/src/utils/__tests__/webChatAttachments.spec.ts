import { describe, expect, it } from 'vitest'
import { chatFileAccept, maxChatFileBytes, readChatFile, validateChatFiles, type DraftChatFile } from '../webChatAttachments'

const sizedFile = (name: string, size: number, type = '') => new File([new Uint8Array(size)], name, { type })
const signal = () => new AbortController().signal
const existing: DraftChatFile = { name: 'first.png', size: 1, mime_type: 'image/png', data: 'YQ==' }
const jpegData = '/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkSEw8UHRofHh0aHBwgJC4nICIsIxwcKDcpLDAxNDQ0Hyc5PTgyPC4zNDL/wAALCAABAAEBAREA/8QAFAABAAAAAAAAAAAAAAAAAAAACP/EABQQAQAAAAAAAAAAAAAAAAAAAAD/2gAIAQEAAD8AP7//2Q=='
const webpData = 'UklGRjgAAABXRUJQVlA4ICwAAACQAQCdASoBAAEAAgA0JaACdLoAA5gA/vmTb/+QH/+QH/+QH/8gP+IXeyAwAA=='

describe('web chat image uploads', () => {
  it('checks the cumulative byte and image counts across separate selections', () => {
    const exact = sizedFile('photo.jpg', maxChatFileBytes - existing.size)
    expect(validateChatFiles([existing], [exact])).toBe('')
    expect(validateChatFiles([existing], [exact, sizedFile('extra.png', 1)])).toBe('total')
    expect(validateChatFiles([existing], [sizedFile('1.jpg', 1), sizedFile('2.webp', 1)])).toBe('')
    expect(validateChatFiles([existing], Array(3).fill(sizedFile('1.png', 1)))).toBe('count')
  })

  it('limits the picker and the validation of new selections to PNG, JPEG and WebP', () => {
    expect(new Set(chatFileAccept.split(','))).toEqual(new Set(['.png', '.jpg', '.jpeg', '.webp', 'image/png', 'image/jpeg', 'image/webp']))
    for (const name of ['doc.pdf', 'doc.docx', 'notes.md', 'notes.txt', 'sheet.csv', 'script.js', 'photo.svg', 'photo.gif', 'no-extension']) {
      expect(validateChatFiles([], [sizedFile(name, 20, 'image/png')])).toBe('type')
    }
    expect(validateChatFiles([], [sizedFile('photo.jpg', 0)])).toBe('empty')
    expect(validateChatFiles([], [sizedFile('photo.PNG', 20, 'image/jpeg')])).toBe('type')
    expect(validateChatFiles([], [sizedFile('photo.jpg', 20, 'application/pdf')])).toBe('type')
    expect(validateChatFiles([], [sizedFile('photo.JPEG', 20, 'image/jpeg')])).toBe('')
    expect(validateChatFiles([], [sizedFile('photo.WEBP', 20, 'application/octet-stream')])).toBe('')
  })

  it.each([
    ['photo.WEBP', webpData, 'image/webp'],
    ['photo.JPEG', jpegData, 'image/jpeg']
  ])('reads and previews %s even when the browser omits its MIME type', async (name, data, mime) => {
    const bytes = Uint8Array.from(atob(data), c => c.charCodeAt(0))
    const result = await readChatFile(new File([bytes], name), signal())
    expect(result).toEqual({ name, mime_type: '', data, size: bytes.length, imageSrc: 'data:' + mime + ';base64,' + data })
  })

  it('rejects a document renamed as an image, mismatched image formats and a cancelled selection', async () => {
    const bytes = Uint8Array.from(atob(webpData), c => c.charCodeAt(0))
    await expect(readChatFile(new File(['%PDF-1.5'], 'renamed.png'), signal())).rejects.toThrow('type')
    await expect(readChatFile(new File([bytes], 'wrong.jpg'), signal())).rejects.toThrow('type')
    const controller = new AbortController()
    controller.abort()
    await expect(readChatFile(sizedFile('photo.png', 1), controller.signal)).rejects.toThrow('cancelled')
  })
})
