import { readFileSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { mergePromptSources as migrateSources, type PromptSource } from '../../../../../canvas/web/src/services/api/prompt-source-migration'

const DEFAULT_PROMPT_SOURCES: PromptSource[] = ['products', 'photography', 'design', 'cinema-portraits', 'cinema-scenes', 'cinema-worlds'].map(collection => ({
  id: `model-gate-curated-${collection}`, name: collection, url: new URL(`/canvas-app/prompt-library/v2/${collection}.json`, window.location.origin).href, homepage: '', enabled: true, builtIn: true
}))
const createPromptSource = (source: Partial<PromptSource>): PromptSource => ({ id: 'test', name: '', url: '', homepage: '', enabled: true, builtIn: false, ...source })
const mergePromptSources = (saved: PromptSource[]) => migrateSources(saved, DEFAULT_PROMPT_SOURCES)

describe('curated canvas prompt sources', () => {
  it('replaces retired built-ins while preserving custom sources and their settings', () => {
    const old = createPromptSource({ id: 'banana-prompt-quicker', name: 'Old source', builtIn: true })
    const custom = createPromptSource({ id: 'my-prompts', name: '我的提示词', url: 'https://example.com/prompts.json', enabled: false })
    const sources = mergePromptSources([old, custom])
    expect(sources.filter(source => source.builtIn)).toHaveLength(6)
    expect(sources.some(source => source.id === old.id)).toBe(false)
    expect(sources.find(source => source.id === custom.id)).toEqual(custom)
    expect(sources.filter(source => source.builtIn).every(source => source.enabled)).toBe(true)
  })

  it('retains curated source switches and restores the current bundled source URL', () => {
    const saved = { ...DEFAULT_PROMPT_SOURCES[0], enabled: false, url: 'https://example.com/old.json' }
    const sources = mergePromptSources([saved, { ...DEFAULT_PROMPT_SOURCES[1], enabled: true }])
    expect(sources[0].enabled).toBe(false)
    expect(sources[0].url).toBe(DEFAULT_PROMPT_SOURCES[0].url)
    expect(sources.slice(1).every(source => source.enabled)).toBe(true)
    for (const source of sources) {
      expect(new URL(source.url).origin).toBe(window.location.origin)
      expect(new URL(source.url).pathname).toMatch(/^\/canvas-app\/prompt-library\/v2\//)
    }
  })

  it('keeps all built-ins disabled when the user previously disabled the entire library', () => {
    const saved = createPromptSource({ id: 'youmind-gpt-image-2', builtIn: true, enabled: false })
    expect(mergePromptSources([saved]).every(source => !source.enabled)).toBe(true)
    expect(mergePromptSources([]).every(source => source.enabled)).toBe(true)
  })

  it('ships 72 unique usable prompts with real local covers and matching provenance', () => {
    const directory = resolve(__dirname, '../../../../../canvas/web/public/prompt-library/v2')
    const entries = ['products', 'photography', 'design', 'cinema-portraits', 'cinema-scenes', 'cinema-worlds'].flatMap(collection => {
      const items = JSON.parse(readFileSync(join(directory, `${collection}.json`), 'utf8'))
      expect(items).toHaveLength(12)
      return items
    })
    expect(new Set(entries.map(item => item.id)).size).toBe(72)
    let coverBytes = 0
    for (const item of entries) {
      expect(item.title).toMatch(/[\u4e00-\u9fff]/)
      expect(item.description.length).toBeGreaterThan(10)
      expect(item.prompt.length).toBeGreaterThan(100)
      expect(item.prompt).not.toMatch(/--ar |--v |\{\{/)
      expect(item.tags.length).toBeGreaterThan(1)
      expect(item.referenceImageUrls).toEqual([])
      expect(item.imageModel).toBeUndefined()
      expect(item.sourceUrl).toMatch(/^https:\/\/github\.com\//)
      const cover = readFileSync(join(directory, item.coverUrl.replace('/canvas-app/prompt-library/v2/', '')))
      expect(cover.subarray(0, 4).toString()).toBe('RIFF')
      expect(cover.subarray(8, 12).toString()).toBe('WEBP')
      coverBytes += cover.length
    }
    expect(coverBytes).toBeLessThan(6 * 1024 * 1024)
    const manifest = JSON.parse(readFileSync(join(directory, 'manifest.json'), 'utf8'))
    expect(manifest.items.map((item: { id: string }) => item.id).sort()).toEqual(entries.map(item => item.id).sort())
    expect(readFileSync(join(directory, 'licenses/MIT.txt'), 'utf8')).toContain('Copyright')
    expect(readFileSync(join(directory, 'licenses/CC0-1.0.txt'), 'utf8')).toContain('CC0')
  })
})
