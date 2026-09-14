import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import RechargeTiersEditor from '../RechargeTiersEditor.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

describe('RechargeTiersEditor', () => {
  it('prepares three empty multiplier rows and emits edits and deletion', async () => {
    const wrapper = mount(RechargeTiersEditor, { props: { modelValue: [] } })
    expect(wrapper.findAll('input')).toHaveLength(6)
    const rows = [{ amount: 10, multiplier: 0 }, { amount: 18, multiplier: 0 }, { amount: 80, multiplier: 0 }]
    await wrapper.findAll('input')[1].setValue('2')
    const emittedRows = wrapper.emitted('update:modelValue')![0][0] as typeof rows
    expect(emittedRows).toEqual([{ ...rows[0], multiplier: 2 }, rows[1], rows[2]])
    await wrapper.setProps({ modelValue: emittedRows })
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    await wrapper.findAll('input')[1].setValue('2.5')
    expect((wrapper.emitted('update:modelValue')!.at(-1)![0] as typeof rows)[0].multiplier).toBe(2.5)
    await wrapper.findAll('button')[1].trigger('click')
    expect(wrapper.emitted('update:modelValue')!.at(-1)![0]).toHaveLength(2)
  })
})
