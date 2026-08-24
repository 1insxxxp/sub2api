import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../SubscriptionProgressMini.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')

const { fetchActiveSubscriptions, subscriptionStoreState } = vi.hoisted(() => {
  const fetchActiveSubscriptions = vi.fn()
  return {
    fetchActiveSubscriptions,
    subscriptionStoreState: {
      activeSubscriptions: [{
        id: 1,
        group_id: 10,
        daily_usage_usd: 2,
        weekly_usage_usd: 2,
        monthly_usage_usd: 2,
        group: {
          name: 'Pro',
          daily_limit_usd: 10,
          weekly_limit_usd: null,
          monthly_limit_usd: null
        }
      }],
      hasActiveSubscriptions: true,
      fetchActiveSubscriptions
    }
  }
})

vi.mock('@/stores', () => ({
  useSubscriptionStore: () => subscriptionStoreState
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

const mountSubscriptionProgress = async () => {
  const SubscriptionProgressMini = (await import('../SubscriptionProgressMini.vue')).default
  const wrapper = mount(SubscriptionProgressMini, {
    global: {
      stubs: {
        Icon: {
          props: ['name'],
          template: '<span :data-icon="name" />'
        },
        RouterLink: {
          template: '<a><slot /></a>'
        }
      }
    }
  })
  await flushPromises()
  return wrapper
}

describe('SubscriptionProgressMini mobile layout', () => {
  beforeEach(() => {
    fetchActiveSubscriptions.mockReset().mockResolvedValue(undefined)
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 1024 })
  })

  it('preserves the desktop pill classes and progress content in mounted output', async () => {
    const wrapper = await mountSubscriptionProgress()
    const trigger = wrapper.get('[data-test="subscription-progress-trigger"]')

    expect(trigger.classes()).toEqual(expect.arrayContaining(['gap-2', 'rounded-xl', 'bg-purple-50', 'px-3', 'py-1.5']))
    expect(trigger.classes()).not.toContain('border-blue-200/80')
    expect(trigger.classes()).not.toContain('shadow-blue-950/5')
    expect(wrapper.get('.subscription-progress-trigger-icon').classes()).toContain('text-purple-600')
    expect(wrapper.get('[data-test="subscription-progress-count"]').text()).toBe('1')
    expect(wrapper.findAll('[data-test="subscription-progress-dot"]')).toHaveLength(1)
    wrapper.unmount()
  })

  it('links the trigger to the panel and reflects its open state', async () => {
    const wrapper = await mountSubscriptionProgress()
    const trigger = wrapper.get('[data-test="subscription-progress-trigger"]')

    expect(trigger.attributes('aria-label')).toBe('subscriptionProgress.viewDetails')
    expect(trigger.attributes('aria-controls')).toBe('subscription-progress-panel')
    expect(trigger.attributes('aria-expanded')).toBe('false')
    expect(wrapper.find('#subscription-progress-panel').exists()).toBe(false)

    await trigger.trigger('click')

    expect(trigger.attributes('aria-expanded')).toBe('true')
    expect(wrapper.get('#subscription-progress-panel').attributes('data-test')).toBe('subscription-progress-sheet')
    wrapper.unmount()
  })

  it('supplies the 36px blue-cyan trigger theme only from mobile CSS', () => {
    const mobileHeaderCss = styleSource.match(/@media \(max-width: 639px\) \{([\s\S]*?)\n {2}\}\n\n {2}\.app-header-balance-pill/)?.[1]
    const subscriptionSelectors = [...(mobileHeaderCss?.matchAll(/^\s*([^{}\n]*\.subscription-progress-[^{\n]+)\s*\{/gm) ?? [])]
      .map((match) => match[1].trim())
    const triggerRule = mobileHeaderCss?.match(/\.app-header-actions \.subscription-progress-trigger\s*\{([^}]*)\}/)?.[1]
    const iconRule = mobileHeaderCss?.match(/\.app-header-actions \.subscription-progress-trigger-icon\s*\{([^}]*)\}/)?.[1]
    const countRule = mobileHeaderCss?.match(/\.app-header-actions \.subscription-progress-trigger-count\s*\{([^}]*)\}/)?.[1]

    expect(subscriptionSelectors.length).toBeGreaterThan(0)
    subscriptionSelectors.forEach((selector) => expect(selector).toContain('.app-header-actions '))
    expect(triggerRule).toContain('width: var(--app-header-mobile-subscription-width)')
    expect(triggerRule).toContain('height: var(--app-header-mobile-control-size)')
    expect(triggerRule).toContain('var(--brand-rgb)')
    expect(triggerRule).toContain('var(--brand-cyan-rgb)')
    expect(iconRule).toContain('var(--brand-600)')
    expect(countRule).toContain('var(--brand-700)')
  })

  it('uses a teleported, dismissible centered modal and coordinates with other header panels', () => {
    expect(componentSource).toContain('subscription-progress-backdrop')
    expect(componentSource).toContain('subscription-progress-sheet')
    expect(componentSource).toContain(':disabled="!isMobileTooltip"')
    expect(componentSource).toContain('left-1/2 top-1/2')
    expect(componentSource).toContain('-translate-x-1/2 -translate-y-1/2')
    expect(componentSource).not.toContain('inset-x-2 bottom-2')
    expect(componentSource).toContain('app-header-floating-panel-open')
  })
})
