import { enableAutoUnmount, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'

import { useAppStore } from '@/stores/app'
import Toast from '../Toast.vue'

enableAutoUnmount(afterEach)
vi.mock('vue-i18n', async importOriginal => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: () => '关闭' })
}))

function mountToast() {
  return mount(Toast, {
    global: {
      stubs: {
        Icon: {
          props: ['name'],
          template: '<span class="icon-stub" :data-icon="name" />',
        },
        teleport: true,
        TransitionGroup: true,
      },
    },
  })
}

describe('Toast', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    setActivePinia(createPinia())
  })

  afterEach(() => {
    vi.clearAllTimers()
    vi.useRealTimers()
  })

  it('renders every notification tone with its own status icon and readable content', () => {
    const store = useAppStore()
    store.showSuccess('已复制')
    store.showError('保存失败，请重试')
    store.showWarning('额度即将用尽')
    store.showInfo('正在同步')

    const wrapper = mountToast()
    const panels = wrapper.findAll('.toast-panel')
    expect(panels).toHaveLength(4)
    expect(panels.map(panel => panel.get('.toast-icon-shell .icon-stub').attributes('data-icon')))
      .toEqual(['checkCircle', 'xCircle', 'exclamationTriangle', 'infoCircle'])
    expect(panels.map(panel => panel.get('.toast-message').text()))
      .toEqual(['已复制', '保存失败，请重试', '额度即将用尽', '正在同步'])
    expect(wrapper.get('.toast-stack').attributes('aria-live')).toBe('polite')
    expect(wrapper.find('button').exists()).toBe(false)
  })

  it('retains a close button for persistent messages without affecting timed messages', async () => {
    const store = useAppStore()
    const first = store.showToast('info', '常驻消息')
    const second = store.showError('保存失败')
    const wrapper = mountToast()
    expect(wrapper.findAll('button')).toHaveLength(1)
    await wrapper.findAll('button[aria-label="关闭"]')[0].trigger('click')
    expect(store.toasts.map(toast => toast.id)).toEqual([second])
    expect(store.toasts.some(toast => toast.id === first)).toBe(false)
    expect(wrapper.findAll('.toast-message').map(message => message.text())).toEqual(['保存失败'])
  })

  it('keeps persistent details visible while timed messages expire', async () => {
    const store = useAppStore()
    store.showSuccess('已复制', 3000)
    const persistent = store.showToast('info', '完整的消息内容')
    store.toasts.find(toast => toast.id === persistent)!.title = '任务状态'
    const wrapper = mountToast()
    expect(wrapper.findAll('.toast-progress')).toHaveLength(1)
    expect(wrapper.get('.toast-progress').attributes('style')).toContain('3000ms')
    expect(wrapper.get('.toast-title').text()).toBe('任务状态')
    await vi.advanceTimersByTimeAsync(3000)
    await nextTick()
    expect(wrapper.findAll('.toast-panel')).toHaveLength(1)
    expect(wrapper.get('.toast-message').text()).toBe('完整的消息内容')
    expect(wrapper.find('.toast-progress').exists()).toBe(false)
  })
})
