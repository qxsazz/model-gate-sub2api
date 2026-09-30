import { onScopeDispose, watch, type Ref } from 'vue'

// Layouts can briefly overlap during navigation. One layout's cleanup must not
// remove the theme owned by its replacement. The body scope includes Teleports.
const activeLayouts = new Set<symbol>()

export function useAdminConsoleTheme(isAdminConsole: Readonly<Ref<boolean>>) {
  const owner = Symbol('admin-console-theme')
  const sync = () => {
    document.body.classList.toggle('admin-console-active', activeLayouts.size > 0)
  }

  watch(isAdminConsole, active => {
    if (active) activeLayouts.add(owner)
    else activeLayouts.delete(owner)
    sync()
  }, { immediate: true, flush: 'sync' })

  onScopeDispose(() => {
    activeLayouts.delete(owner)
    sync()
  })
}
