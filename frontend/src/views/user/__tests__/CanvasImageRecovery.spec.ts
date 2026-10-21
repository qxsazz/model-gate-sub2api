import { describe, expect, it } from 'vitest'
import { IMAGE_BATCH_WAIT_MS, recoveredImagePhase, recoveredImageSlots } from '../../../../../canvas/web/src/lib/image-task-recovery'
const now = Date.now()
function task(status = 'completed', slot = 0, size = 1, age = 417 * 60_000) {
  return { id: `task-${slot}`, status, created_at: new Date(now - age).toISOString(), metadata: { slot, batchSize: size } }
}
describe('image workbench recovery follows server state', () => {
  it('does not restore a seven-hour-old completed task as generating when image recovery fails', () => {
    expect(recoveredImagePhase(task(), false)).toBe('unavailable')
    expect(recoveredImagePhase(task(), true)).toBe('success')
    expect(recoveredImagePhase(task('failed'), false)).toBe('failed')
    expect(recoveredImagePhase(task('interrupted'), false)).toBe('failed')
  })
  it('shows generation only for actual queued and running tasks', () => {
    expect(recoveredImagePhase(task('queued'), false)).toBe('pending')
    expect(recoveredImagePhase(task('running'), false)).toBe('pending')
  })
  it('recovers legacy single-slot retry indices without inventing missing jobs', () => {
    const original = task('completed', 2)
    const result = recoveredImageSlots([original], now)
    expect(result.slots).toEqual([original])
    expect(result.expired).toBe(true)
  })
  it('bounds missing-slot waiting while allowing genuine slots to arrive', () => {
    const result = recoveredImageSlots([task('completed', 1, 3)], now)
    expect(result.slots.map(t => t?.id)).toEqual([undefined, 'task-1', undefined])
    expect(result.expired).toBe(true)
    expect(recoveredImageSlots([task('completed', 1, 3, IMAGE_BATCH_WAIT_MS - 1)], now).expired).toBe(false)
  })
})
