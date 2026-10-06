export function scrollToDocHeading(id: string): boolean {
  const target = document.getElementById(id)
  if (!target) return false
  let detail = target.closest('details')
  while (detail) {
    detail.open = true
    detail = detail.parentElement?.closest('details') || null
  }
  target.scrollIntoView({ behavior: 'smooth', block: 'start' })
  history.replaceState(history.state, '', `#${encodeURIComponent(id)}`)
  return true
}
