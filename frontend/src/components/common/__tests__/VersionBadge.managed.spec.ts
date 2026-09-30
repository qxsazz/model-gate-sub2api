import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import VersionBadge from '../VersionBadge.vue'

const { fetchVersion, performUpdate, rollback, upstream } = vi.hoisted(() => ({ fetchVersion: vi.fn(), performUpdate: vi.fn(), rollback: vi.fn(), upstream: { baseline_version: '0.2.10', latest_version: '0.2.11', has_update: true, status: 'ok', release_url: 'https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.11', checked_at: '2026-09-30T00:00:00Z' } }))
vi.mock('@/stores', () => ({
  useAuthStore: () => ({ isAdmin: true }),
  useAppStore: () => ({ currentVersion: '0.2.10-mg.1', latestVersion: '9.0.0', hasUpdate: true, versionLoading: false, buildType: 'release', upstreamInfo: upstream, fetchVersion }),
}))
vi.mock('@/api/admin/system', () => ({ performUpdate, rollback, restartService: vi.fn(), getRollbackVersions: vi.fn() }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copied: false, copyToClipboard: vi.fn() }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const mounted: ReturnType<typeof mount>[] = []
afterEach(() => { mounted.splice(0).forEach(wrapper => wrapper.unmount()); upstream.status = 'ok'; upstream.release_url = 'https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.11' })

describe('MG managed release entry', () => {
  it.each(['unavailable', 'stale'])('makes %s checks explicit and never reports no update', async status => {
    upstream.status = status
    upstream.release_url = 'javascript:alert(1)'
    const wrapper = mount(VersionBadge)
    mounted.push(wrapper)
    await wrapper.get('button').trigger('click')
    expect(wrapper.text()).toContain(status === 'stale' ? 'version.upstreamStale' : 'version.upstreamUnavailable')
    expect(wrapper.text()).not.toContain('version.upstreamCurrent')
    expect(wrapper.find('a[href="javascript:alert(1)"]').exists()).toBe(false)
  })

  it('offers our deployment workflow and no binary replacement despite stale update state', async () => {
    const wrapper = mount(VersionBadge, { global: { stubs: { Icon: { template: '<span />' } } } })
    mounted.push(wrapper)
    await wrapper.get('button').trigger('click')
    expect(wrapper.find('a[href="https://github.com/qxsazz/model-gate-sub2api/actions/workflows/deploy-production.yml"]').exists()).toBe(true)
    expect(wrapper.findAll('button').some(button => /version.updateNow|version.rollback/.test(button.text()))).toBe(false)
    expect(wrapper.text()).not.toContain('v9.0.0')
    expect(wrapper.text()).toContain('0.2.11')
    expect(wrapper.text()).toContain('version.upstreamAvailable')
    expect(wrapper.find('a[href="https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.11"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('weishaw/sub2api')
    expect(performUpdate).not.toHaveBeenCalled()
    expect(rollback).not.toHaveBeenCalled()
  })
})
