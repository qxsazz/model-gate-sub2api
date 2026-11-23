import { describe, expect, it } from 'vitest'
import { chatFileAccept, maxChatFileBytes, readChatFile, validateChatFiles, type DraftChatFile } from '../webChatAttachments'

const textFile = (name: string, bytes: number) => new File([new Uint8Array(bytes)], name)
const signal = () => new AbortController().signal
const existing: DraftChatFile = { name: 'first.txt', size: 1, mime_type: '', data: 'YQ==' }

describe('web chat attachments', () => {
  it('checks the cumulative byte and file counts across separate selections', () => {
    const exact = textFile('photo.jpg', maxChatFileBytes - existing.size)
    expect(validateChatFiles([existing], [exact])).toBe('')
    expect(validateChatFiles([existing], [exact, textFile('extra.md', 1)])).toBe('total')
    expect(validateChatFiles([existing], [textFile('1.pdf', 1), textFile('2.docx', 1)])).toBe('')
    expect(validateChatFiles([existing], Array(3).fill(textFile('1.md', 1)))).toBe('count')
  })

  it('accepts only the published extensions and bounds individual text files', () => {
    expect(chatFileAccept.split(',')).toContain('.webp')
    expect(validateChatFiles([], [textFile('a.Js', 128 * 1024)])).toBe('')
    expect(validateChatFiles([], [textFile('a.Js', 128 * 1024 + 1)])).toBe('textSize')
    expect(validateChatFiles([], [textFile('report.pdf', 0)])).toBe('empty')
    expect(validateChatFiles([], [textFile('photo.svg', 4)])).toBe('type')
    expect(validateChatFiles([], [textFile('no-extension', 4)])).toBe('type')
  })

  it('reads exact bytes, preserves an absent browser MIME type and decodes UTF-8 previews', async () => {
    const result = await readChatFile(new File(['模型'], 'notes.md'), signal())
    expect(result).toEqual({ name: 'notes.md', mime_type: '', data: '5qih5Z6L', size: 6, textPreview: '模型' })
    const webp = await readChatFile(new File(['a'], 'photo.WEBP'), signal())
    expect(webp.imageSrc).toBe('data:image/webp;base64,YQ==')
  })

  it('rejects invalid UTF-8, NUL-containing text and a cancelled selection', async () => {
    for (const byte of [0xff, 0]) {
      await expect(readChatFile(new File([new Uint8Array([byte])], 'notes.txt'), signal())).rejects.toThrow('encoding')
    }
    const controller = new AbortController()
    controller.abort()
    await expect(readChatFile(textFile('note.txt', 1), controller.signal)).rejects.toThrow('cancelled')
  })
})
