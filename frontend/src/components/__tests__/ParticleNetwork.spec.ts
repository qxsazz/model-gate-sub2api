import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ParticleNetwork from '../ParticleNetwork.vue'

afterEach(() => vi.restoreAllMocks())

describe('ParticleNetwork theme', () => {
  it('draws cream particles over white without additive blending and follows theme changes', async () => {
    let frame: FrameRequestCallback = () => {}
    const draws: string[] = []
    const context = {
      globalCompositeOperation: 'source-over',
      save: vi.fn(), restore: vi.fn(), setTransform: vi.fn(), scale: vi.fn(),
      fillRect: vi.fn(), beginPath: vi.fn(), arc: vi.fn(),
      fill() { draws.push(this.globalCompositeOperation) },
      moveTo: vi.fn(), lineTo: vi.fn(), stroke: vi.fn()
    }
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(context as unknown as CanvasRenderingContext2D)
    vi.spyOn(window, 'requestAnimationFrame').mockImplementation(callback => { frame = callback; return 1 })
    vi.spyOn(window, 'cancelAnimationFrame').mockImplementation(() => {})
    const wrapper = mount(ParticleNetwork, { props: { count: 2, particleColor: 'cream', bgColor: '#FFFFFF' } })
    frame(0)
    expect(draws.length).toBeGreaterThan(0)
    expect(draws.every(mode => mode === 'source-over')).toBe(true)
    draws.length = 0
    await wrapper.setProps({ particleColor: 'blue', bgColor: '#0A0B0F' })
    frame(16)
    expect(draws.every(mode => mode === 'lighter')).toBe(true)
    wrapper.unmount()
  })
})
