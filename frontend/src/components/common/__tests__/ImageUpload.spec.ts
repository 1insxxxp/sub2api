import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import ImageUpload from '@/components/common/ImageUpload.vue'

const originalFileReader = globalThis.FileReader

function installFileReaderMock(result: string, options: { includeEventTarget?: boolean } = {}) {
  const includeEventTarget = options.includeEventTarget ?? true

  class MockFileReader {
    result: string | ArrayBuffer | null = null
    onload: ((this: FileReader, ev: ProgressEvent<FileReader>) => unknown) | null = null
    onerror: ((this: FileReader, ev: ProgressEvent<FileReader>) => unknown) | null = null
    error: DOMException | null = null

    readAsDataURL() {
      this.result = result
      this.onload?.call(this as unknown as FileReader, makeLoadEvent(this, includeEventTarget))
    }

    readAsText() {
      this.result = result
      this.onload?.call(this as unknown as FileReader, makeLoadEvent(this, includeEventTarget))
    }
  }

  globalThis.FileReader = MockFileReader as unknown as typeof FileReader
}

function makeLoadEvent(reader: unknown, includeEventTarget: boolean) {
  return (includeEventTarget
    ? { target: reader }
    : new ProgressEvent('load')) as ProgressEvent<FileReader>
}

describe('ImageUpload', () => {
  afterEach(() => {
    globalThis.FileReader = originalFileReader
  })

  it('previews the uploaded image immediately after model update', async () => {
    const dataUrl = 'data:image/png;base64,dGVzdC1sb2dv'
    installFileReaderMock(dataUrl)

    const wrapper = mount(ImageUpload, {
      props: {
        modelValue: '',
        mode: 'image',
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    const file = new File(['test-logo'], 'logo.png', { type: 'image/png' })
    const input = wrapper.get<HTMLInputElement>('input[type="file"]')
    Object.defineProperty(input.element, 'files', {
      configurable: true,
      value: [file],
    })
    await input.trigger('change')

    const emitted = wrapper.emitted<'update:modelValue'>('update:modelValue')
    expect(emitted?.[0]).toEqual([dataUrl])

    await wrapper.setProps({ modelValue: dataUrl })

    expect(wrapper.get('img').attributes('src')).toBe(dataUrl)
  })

  it('emits the uploaded image result when the load event target is unavailable', async () => {
    const dataUrl = 'data:image/png;base64,bm8tdGFyZ2V0'
    installFileReaderMock(dataUrl, { includeEventTarget: false })

    const wrapper = mount(ImageUpload, {
      props: {
        modelValue: '',
        mode: 'image',
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    const file = new File(['test-logo'], 'logo.png', { type: 'image/png' })
    const input = wrapper.get<HTMLInputElement>('input[type="file"]')
    Object.defineProperty(input.element, 'files', {
      configurable: true,
      value: [file],
    })
    await input.trigger('change')

    expect(wrapper.emitted<'update:modelValue'>('update:modelValue')?.[0]).toEqual([dataUrl])
  })
})
