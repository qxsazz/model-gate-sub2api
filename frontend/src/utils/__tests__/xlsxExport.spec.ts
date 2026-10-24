import { describe, expect, it } from 'vitest'
import * as XLSX from 'xlsx'

describe('patched SheetJS export compatibility', () => {
  it('writes a real workbook and preserves untrusted labels as text cells', () => {
    expect(XLSX.version).toBe('0.20.3')
    const rows = [['模型', '费用'], ['=1+1', 0.25], ['@SUM(1)', 0.5]]
    const book = XLSX.utils.book_new()
    XLSX.utils.book_append_sheet(book, XLSX.utils.aoa_to_sheet(rows), 'Usage')
    const bytes = new Uint8Array(XLSX.write(book, { bookType: 'xlsx', type: 'array' }))
    expect([...bytes.slice(0, 2)]).toEqual([0x50, 0x4b])
    // Parsing is test-only; the application continues to use the export API.
    const result = XLSX.read(bytes, { type: 'array' }).Sheets.Usage!
    expect(result.A1?.v).toBe('模型')
    expect(result.A2).toMatchObject({ t: 's', v: '=1+1' })
    expect(result.A2?.f).toBeUndefined()
    expect(result.A3?.f).toBeUndefined()
    expect(result.B3?.v).toBe(0.5)
  })
})
