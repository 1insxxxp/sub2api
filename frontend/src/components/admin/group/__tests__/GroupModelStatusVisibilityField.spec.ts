import { mount } from '@vue/test-utils'
import { defineComponent, h, ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'

import GroupModelStatusVisibilityField from '../GroupModelStatusVisibilityField.vue'
import type { ModelStatusVisibility } from '@/types'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const mountInteractive = (
  initial: ModelStatusVisibility,
  candidates: string[] = ['gpt-image-1', 'gpt-4.1', 'gemini-*'],
) => {
  const host = defineComponent({
    setup() {
      const config = ref(initial)
      return () => h(GroupModelStatusVisibilityField, {
        modelValue: config.value,
        candidates,
        'onUpdate:modelValue': (value: ModelStatusVisibility) => {
          config.value = value
        },
      })
    },
  })
  return mount(host, {
    global: {
      stubs: {
        Icon: { props: ['name'], template: '<i :data-icon="name" />' },
      },
    },
  })
}

describe('GroupModelStatusVisibilityField', () => {
  it('defaults to showing all models and does not render the selector while disabled', () => {
    const wrapper = mountInteractive({ enabled: false, models: [] })

    expect(wrapper.find('[data-testid="model-status-visibility-toggle"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="model-status-visibility-search"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('admin.groups.modelStatusVisibility.disabledHint')
  })

  it('selects, clears and manually adds models without changing unrelated configuration', async () => {
    const wrapper = mountInteractive({ enabled: true, models: ['GPT-4.1'] })

    expect(wrapper.findAll('input[type="checkbox"]')).toHaveLength(3)
    expect(wrapper.find('input[aria-label="gpt-4.1"]').element).toHaveProperty('checked', true)

    await wrapper.find('[data-testid="model-status-visibility-select-all"]').trigger('click')
    const field = wrapper.findComponent(GroupModelStatusVisibilityField)
    expect(field.emitted('update:modelValue')?.at(-1)?.[0]).toMatchObject({
      enabled: true,
      models: ['GPT-4.1', 'gpt-image-1', 'gemini-*'],
    })

    await wrapper.find('[data-testid="model-status-visibility-clear"]').trigger('click')
    expect(field.emitted('update:modelValue')?.at(-1)?.[0]).toMatchObject({ enabled: true, models: [] })

    const custom = wrapper.find('[data-testid="model-status-visibility-custom"]')
    await custom.setValue('gpt-image-2')
    await wrapper.find('[data-testid="model-status-visibility-add-custom"]').trigger('click')
    expect(wrapper.text()).toContain('gpt-image-2')
    expect(wrapper.text()).not.toContain('modelAllowlist')
  })

  it('keeps long names readable and rejects non-trailing wildcard entries', async () => {
    const wrapper = mountInteractive({ enabled: true, models: [] }, [
      'provider/very-long-model-name-that-must-wrap-on-a-narrow-screen',
    ])
    expect(wrapper.find('.break-words').exists()).toBe(true)

    const custom = wrapper.find('[data-testid="model-status-visibility-custom"]')
    await custom.setValue('bad*model')
    await wrapper.find('[data-testid="model-status-visibility-add-custom"]').trigger('click')
    expect(wrapper.find('[data-testid="model-status-visibility-custom-error"]').exists()).toBe(true)
  })
})
