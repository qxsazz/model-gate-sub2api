import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import DocsTableOfContents from '../DocsTableOfContents.vue'

describe('documentation heading navigation', () => {
  it('opens a folded section before scrolling to its nested heading', async () => {
    const detail = document.createElement('details')
    detail.innerHTML = '<summary>排查</summary><h3 id="docs-nested-test">网络</h3>'
    document.body.append(detail)
    const scroll = vi.fn()
    detail.querySelector('h3')!.scrollIntoView = scroll
    const wrapper = mount(DocsTableOfContents, { props: { headings: [{ id: 'docs-nested-test', text: '网络', level: 3 }] } })
    try {
      await wrapper.get('a').trigger('click')
      expect(detail.open).toBe(true)
      expect(scroll).toHaveBeenCalledOnce()
    } finally { detail.remove(); wrapper.unmount(); history.replaceState(null, '', '/') }
  })
})
