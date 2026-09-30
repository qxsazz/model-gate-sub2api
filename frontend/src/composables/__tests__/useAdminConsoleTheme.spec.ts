import { mount } from '@vue/test-utils'
import { defineComponent, h, nextTick, ref, Teleport } from 'vue'
import { afterEach, describe, expect, it } from 'vitest'
import { useAdminConsoleTheme } from '../useAdminConsoleTheme'

const wrappers: ReturnType<typeof mount>[] = []

function createLayout(admin = true) {
  const isAdmin = ref(admin)
  const wrapper = mount(defineComponent({
    setup() {
      useAdminConsoleTheme(isAdmin)
      return () => h(Teleport, { to: 'body' }, h('div', { class: 'modal-content' }, 'Dialog'))
    }
  }))
  wrappers.push(wrapper)
  return { wrapper, isAdmin }
}

afterEach(() => {
  wrappers.splice(0).forEach(wrapper => wrapper.unmount())
})

describe('admin theme isolation and teleported surfaces', () => {
  it('leaves user pages and their dialogs outside the admin theme', () => {
    createLayout(false)
    expect(document.body.classList.contains('admin-console-active')).toBe(false)
    expect(document.querySelector('.admin-console-active .modal-content')).toBeNull()
  })

  it('includes a real teleported dialog in the admin theme scope', () => {
    createLayout()
    expect(document.querySelector('.admin-console-active .modal-content')?.textContent).toBe('Dialog')
  })

  it('removes the theme on navigation back to a user page and restores it on return', async () => {
    const { isAdmin } = createLayout()
    isAdmin.value = false
    await nextTick()
    expect(document.body.classList.contains('admin-console-active')).toBe(false)
    isAdmin.value = true
    await nextTick()
    expect(document.body.classList.contains('admin-console-active')).toBe(true)
  })

  it('keeps the theme while a replacement admin layout is mounted, then cleans up', () => {
    const first = createLayout()
    const second = createLayout()
    first.wrapper.unmount()
    expect(document.body.classList.contains('admin-console-active')).toBe(true)
    second.wrapper.unmount()
    expect(document.body.classList.contains('admin-console-active')).toBe(false)
  })
})
