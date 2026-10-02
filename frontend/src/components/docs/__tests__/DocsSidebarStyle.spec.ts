import { mount, RouterLinkStub } from '@vue/test-utils'
import { parse } from 'vue/compiler-sfc'
import { expect, it } from 'vitest'
import DocsSidebar from '../DocsSidebar.vue'
import source from '../DocsSidebar.vue?raw'
import { docsNavigation } from '@/docs/registry'

it('keeps the overview emblem visible after leaving the overview', async () => {
  const style = document.createElement('style')
  style.textContent = parse(source).descriptor.styles[0].content
  document.head.append(style)
  const wrapper = mount(DocsSidebar, { attachTo: document.body, props: { navigation: docsNavigation, overview: true }, global: { stubs: { RouterLink: RouterLinkStub } } })
  try {
    const emblem = wrapper.get('.docs-overview-link .docs-nav-art svg').element
    expect(getComputedStyle(emblem).opacity).toBe('1')
    await wrapper.setProps({ overview: false, currentCategory: 'tutorial', currentPage: 'quick-start' })
    expect(getComputedStyle(emblem).opacity).toBe('1')
  } finally { wrapper.unmount(); style.remove() }
})
