import { onMounted, onBeforeUnmount } from 'vue'

const dayDuration = 24 * 60 * 60 * 1000
const shanghaiOffset = 8 * 60 * 60 * 1000

export function useCheckinDayRefresh(refresh: (dayChanged: boolean) => void) {
  let observedDay: number
  let timer: number | undefined

  function schedule(now: number) {
    window.clearTimeout(timer)
    const nextMidnight = (observedDay + 1) * dayDuration - shanghaiOffset
    timer = window.setTimeout(refreshWhenVisible, nextMidnight - now + 250)
  }

  function refreshWhenVisible() {
    if (document.hidden) return
    const now = Date.now()
    const day = Math.floor((now + shanghaiOffset) / dayDuration)
    const dayChanged = day !== observedDay
    observedDay = day
    schedule(now)
    refresh(dayChanged)
  }

  function refreshFromPageCache(event: PageTransitionEvent) {
    if (event.persisted) refreshWhenVisible()
  }

  onMounted(() => {
    const now = Date.now()
    observedDay = Math.floor((now + shanghaiOffset) / dayDuration)
    schedule(now)
    window.addEventListener('focus', refreshWhenVisible)
    window.addEventListener('pageshow', refreshFromPageCache)
    document.addEventListener('visibilitychange', refreshWhenVisible)
  })

  onBeforeUnmount(() => {
    window.clearTimeout(timer)
    window.removeEventListener('focus', refreshWhenVisible)
    window.removeEventListener('pageshow', refreshFromPageCache)
    document.removeEventListener('visibilitychange', refreshWhenVisible)
  })
}
