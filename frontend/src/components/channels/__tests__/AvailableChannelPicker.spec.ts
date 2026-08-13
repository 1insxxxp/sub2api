import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { CatalogChannelEntry } from '../availableChannelCatalog'
import enDashboard from '@/i18n/locales/en/dashboard'
import zhDashboard from '@/i18n/locales/zh/dashboard'

const labels: Record<string, string> = {
  'availableChannels.catalog.selectChannel': '选择渠道', 'availableChannels.catalog.channelPickerTitle': '选择可用渠道',
  'availableChannels.catalog.channelPickerSearch': '搜索渠道名称或平台…', 'availableChannels.catalog.groupsCount': '{count} 个分组',
  'availableChannels.catalog.modelsCount': '{count} 个模型', 'common.close': '关闭',
  'availableChannels.catalog.channelPickerNoResults': '没有匹配的渠道',
}
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string, params?: Record<string, string | number>) => Object.entries(params ?? {}).reduce((result, [name, replacement]) => result.replace(`{${name}}`, String(replacement)), labels[key] ?? key) }),
}))

const channels: CatalogChannelEntry[] = [
  { key: 'alpha', name: 'Alpha route', description: '', platforms: ['openai', 'anthropic'], groups: [], groupCount: 2, modelCount: 7 },
  { key: 'beta', name: 'Beta Gemini', description: '', platforms: ['gemini'], groups: [], groupCount: 1, modelCount: 3 },
]
const mountedWrappers: VueWrapper[] = []
afterEach(() => { mountedWrappers.splice(0).forEach(wrapper => wrapper.unmount()); document.body.style.overflow = ''; document.body.innerHTML = '' })
async function mountPicker() {
  const component = (await import('../AvailableChannelPicker.vue')).default
  const wrapper = mount(component, {
    attachTo: document.body,
    props: { channels, modelValue: 'alpha' },
    global: {
      stubs: {
        AvailableChannelPlatformBadge: { props: ['platform'], template: '<span data-platform-badge :data-platform="platform">{{ platform }}</span>' },
      },
    },
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('AvailableChannelPicker', () => {
  it('shows a mobile-only 44px trigger with selected metadata', async () => {
    const wrapper = await mountPicker(); const trigger = wrapper.get('[data-testid="channel-picker-trigger"]')
    expect(trigger.element.tagName).toBe('BUTTON'); expect(trigger.classes()).toContain('min-h-11'); expect(trigger.classes()).toContain('xl:hidden')
    expect(trigger.classes()).toEqual(expect.arrayContaining(['hover:border-primary-200', 'focus-visible:ring-2', 'focus-visible:ring-offset-2']))
    expect(trigger.text()).toContain('Alpha route'); expect(trigger.text()).toContain('openai'); expect(trigger.text()).toContain('2 个分组'); expect(trigger.text()).toContain('7 个模型')
    expect(trigger.findAll('[data-platform-badge]')).toHaveLength(2)
  })
  it('opens a teleported modal sheet with local search and mobile-safe scrolling', async () => {
    const wrapper = await mountPicker(); await wrapper.get('[data-testid="channel-picker-trigger"]').trigger('click'); await flushPromises()
    const dialog = document.body.querySelector<HTMLElement>('[data-testid="channel-picker-dialog"]')!
    expect(dialog.parentElement).toBe(document.body); expect(dialog.getAttribute('role')).toBe('dialog'); expect(dialog.getAttribute('aria-modal')).toBe('true'); expect(dialog.className).toContain('z-[70]')
    const panel = dialog.querySelector<HTMLElement>('[data-testid="channel-picker-panel"]')!
    expect(panel.className).toContain('max-h-[calc(100dvh-3rem)]'); expect(panel.className).toContain('rounded-t-3xl'); expect(panel.className).toContain('pb-[max(1rem,env(safe-area-inset-bottom))]'); expect(panel.className).toContain('overscroll-contain')
    expect(dialog.querySelector('[data-testid="channel-picker-header"]')!.className).toContain('sticky'); expect(dialog.querySelector('[data-testid="channel-picker-options"]')!.className).toContain('overflow-y-auto'); expect(dialog.querySelector('[data-testid="channel-picker-options"]')!.className).toContain('overscroll-contain')
    const search = dialog.querySelector<HTMLInputElement>('[data-testid="channel-picker-search"]')!; expect(document.activeElement).toBe(search)
    expect(search.getAttribute('aria-label')).toBe('搜索渠道名称或平台…')
    expect(search.getAttribute('name')).toBe('available-channel-picker-search')
    expect(search.getAttribute('autocomplete')).toBe('off')
    expect(zhDashboard.availableChannels.catalog.channelPickerSearch).toBe('搜索渠道名称或平台…')
    expect(enDashboard.availableChannels.catalog.channelPickerSearch).toBe('Search channels or platforms…')
    const option = dialog.querySelector<HTMLElement>('[data-testid="channel-picker-option"]')!
    expect(option.className).toContain('hover:border-primary-200')
    expect(option.className).toContain('focus-visible:ring-2')
    search.value = 'gemini'; search.dispatchEvent(new Event('input')); await flushPromises()
    expect(dialog.textContent).toContain('Beta Gemini'); expect(dialog.textContent).not.toContain('Alpha route'); expect(wrapper.props('modelValue')).toBe('alpha')
    expect(dialog.querySelectorAll('[data-platform-badge]')).toHaveLength(1)
    search.value = 'missing'; search.dispatchEvent(new Event('input')); await flushPromises()
    expect(dialog.textContent).toContain('没有匹配的渠道')
    expect(dialog.querySelector('[data-testid="channel-picker-options"]')!.getAttribute('role')).toBe('listbox')
  })
  it('selects, emits, closes, restores focus, and resets search', async () => {
    const wrapper = await mountPicker(); const trigger = wrapper.get('[data-testid="channel-picker-trigger"]'); trigger.element.focus(); await trigger.trigger('click'); await flushPromises()
    const dialog = document.body.querySelector<HTMLElement>('[data-testid="channel-picker-dialog"]')!; const options = dialog.querySelectorAll<HTMLButtonElement>('[data-testid="channel-picker-option"]')
    expect(options[0].getAttribute('aria-selected')).toBe('true'); expect(options[1].textContent).toContain('gemini'); options[1].click(); await flushPromises()
    expect(wrapper.emitted('update:modelValue')).toEqual([['beta']]); expect(document.body.querySelector('[data-testid="channel-picker-dialog"]')).toBeNull(); expect(document.activeElement).toBe(trigger.element)
    await trigger.trigger('click'); await flushPromises(); expect((document.body.querySelector('[data-testid="channel-picker-search"]') as HTMLInputElement).value).toBe('')
  })
  it.each(['Escape', 'button', 'backdrop'])('closes through %s and restores body overflow', async (method) => {
    document.body.style.overflow = 'clip'; const wrapper = await mountPicker(); await wrapper.get('[data-testid="channel-picker-trigger"]').trigger('click'); await flushPromises(); expect(document.body.style.overflow).toBe('hidden')
    const dialog = document.body.querySelector<HTMLElement>('[data-testid="channel-picker-dialog"]')!
    if (method === 'Escape') window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' })); else if (method === 'button') (dialog.querySelector('[data-testid="channel-picker-close"]') as HTMLButtonElement).click(); else dialog.click()
    await flushPromises(); expect(document.body.querySelector('[data-testid="channel-picker-dialog"]')).toBeNull(); expect(document.body.style.overflow).toBe('clip'); wrapper.unmount()
  })
  it('unlocks on unmount and contains no API or price calculation', async () => {
    const wrapper = await mountPicker(); await wrapper.get('[data-testid="channel-picker-trigger"]').trigger('click'); await flushPromises(); wrapper.unmount(); expect(document.body.style.overflow).toBe('')
    const source = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), '../AvailableChannelPicker.vue'), 'utf8')
    expect(source).not.toMatch(/@\/api|fetch\(|axios|priceCnyMultiplier|buildAvailableChannelCatalog/); expect(source).toContain('motion-reduce:transition-none')
    expect(source).not.toContain("platforms.join(' · ')")
  })
})
