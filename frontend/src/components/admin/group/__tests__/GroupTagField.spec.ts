import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import GroupTagField from '../GroupTagField.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

describe('GroupTagField', () => {
  it('edits custom text and color and previews both', async () => {
    const wrapper = mount(GroupTagField, {
      props: {
        modelValue: '高速线路', color: '#2563EB', name: 'test-tag',
        'onUpdate:modelValue': (value: string) => wrapper.setProps({ modelValue: value }),
        'onUpdate:color': (color: string) => wrapper.setProps({ color }),
      },
    })
    await wrapper.get('input[data-test="group-tag-text"]').setValue('专属高速')
    await wrapper.get('input[type="color"]').setValue('#16a34a')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual(['专属高速'])
    expect(wrapper.emitted('update:color')?.at(-1)).toEqual(['#16A34A'])
    const preview = wrapper.get('[data-test="group-tag-preview"]')
    expect(preview.text()).toContain('专属高速')
    expect(preview.get('[data-test="group-tag"]').attributes('style')).toContain('22, 163, 74')
  })

  it('validates custom hex colors and supports preset selection and clearing', async () => {
    const wrapper = mount(GroupTagField, {
      props: {
        modelValue: '自定义', color: '#2563EB', name: 'test-tag',
        'onUpdate:modelValue': (value: string) => wrapper.setProps({ modelValue: value }),
        'onUpdate:color': (color: string) => wrapper.setProps({ color }),
      },
    })
    const hex = wrapper.get('input[data-test="group-tag-color-hex"]')
    await hex.setValue('red; color: black')
    expect((hex.element as HTMLInputElement).checkValidity()).toBe(false)
    await wrapper.get('input[type="radio"][value="image"]').setValue(true)
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual(['image'])
    expect(wrapper.emitted('update:color')?.at(-1)).toEqual([''])
    await wrapper.get('input[type="radio"][value=""]').setValue(true)
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([''])
    expect(wrapper.emitted('update:color')?.at(-1)).toEqual([''])
    expect(wrapper.find('[data-test="group-tag-preview"] [data-test="group-tag"]').exists()).toBe(false)
  })
})
