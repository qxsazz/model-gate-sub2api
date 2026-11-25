import type { EventBusKey } from '@vueuse/core'

export const achievementCheckinEvent: EventBusKey<{ userId: number; date: string }> = Symbol('achievement-checkin')
