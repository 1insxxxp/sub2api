import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import ModelAliasesField from '../ModelAliasesField.vue'
import { createModelAliasFormRow, modelAliasesToAPI } from '@/utils/groupModelAliases'

describe('group alias anti-truncation compatibility', () => {
  it('keeps existing group policy read-only without dropping it on alias save', () => {
    const row = createModelAliasFormRow({ alias: 'long-public', upstream_model: 'provider', anti_truncation: { enabled: true, max_rounds: 2 } })
    const wrapper = mount(ModelAliasesField, {
      props: { modelValue: [row], idPrefix: 'test-alias' },
      global: { plugins: [createI18n({ legacy: false, locale: 'en', missingWarn: false, fallbackWarn: false, messages: {} })] },
    })
    expect(wrapper.find('input[type="checkbox"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('admin.accounts.antiTruncation.groupLegacyHint')
    expect(modelAliasesToAPI([row])[0]?.anti_truncation).toEqual({ enabled: true, max_rounds: 2 })
  })
})
