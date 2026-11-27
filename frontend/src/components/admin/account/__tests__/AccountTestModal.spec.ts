import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AccountTestModal from '../AccountTestModal.vue'

const { getAvailableModels, copyToClipboard } = vi.hoisted(() => ({
  getAvailableModels: vi.fn(),
  copyToClipboard: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      getAvailableModels
    }
  }
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'admin.accounts.imagePromptDefault': 'Generate a cute orange cat astronaut sticker on a clean pastel background.'
  }
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => {
        if (params?.prompt) return `${key} ${params.prompt}`
        if (key === 'admin.accounts.imageReceived' && params?.count) {
          return `received-${params.count}`
        }
        if (key === 'admin.accounts.imagePreviewAlt' && params?.index) {
          return `test-image-${params.index}`
        }
        return messages[key] || key
      }
    })
  }
})

function createStreamResponse(lines: string[]) {
  const encoder = new TextEncoder()
  const chunks = lines.map((line) => encoder.encode(line))
  let index = 0

  return {
    ok: true,
    body: {
      getReader: () => ({
        read: vi.fn().mockImplementation(async () => {
          if (index < chunks.length) {
            return { done: false, value: chunks[index++] }
          }
          return { done: true, value: undefined }
        })
      })
    }
  } as Response
}

function mountModal(props: Record<string, unknown> = {}) {
  return mount(AccountTestModal, {
    props: {
      show: false,
      account: {
        id: 42,
        name: 'Gemini Image Test',
        platform: 'gemini',
        type: 'apikey',
        status: 'active'
      },
      ...props
    } as any,
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        Select: {
          props: ['modelValue', 'options', 'valueKey', 'labelKey'],
          emits: ['update:modelValue'],
          template: `
            <select :value="modelValue" @change="$emit('update:modelValue', $event.target.value)">
              <option
                v-for="option in options"
                :key="option[valueKey || 'value']"
                :value="option[valueKey || 'value']"
              >
                {{ option[labelKey || 'label'] }}
              </option>
            </select>
          `
        },
        TextArea: {
          props: ['modelValue'],
          emits: ['update:modelValue'],
          template: '<textarea class="textarea-stub" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
        },
        Icon: true
      }
    }
  })
}

describe('AccountTestModal', () => {
  beforeEach(() => {
    getAvailableModels.mockResolvedValue([
      { id: 'gemini-2.0-flash', display_name: 'Gemini 2.0 Flash' },
      { id: 'gemini-2.5-flash-image', display_name: 'Gemini 2.5 Flash Image' },
      { id: 'gemini-3.1-flash-image', display_name: 'Gemini 3.1 Flash Image' }
    ])
    copyToClipboard.mockReset()
    Object.defineProperty(globalThis, 'localStorage', {
      value: {
        getItem: vi.fn((key: string) => (key === 'auth_token' ? 'test-token' : null)),
        setItem: vi.fn(),
        removeItem: vi.fn(),
        clear: vi.fn()
      },
      configurable: true
    })
    global.fetch = vi.fn().mockResolvedValue(
      createStreamResponse([
        'data: {"type":"test_start","model":"gemini-2.5-flash-image"}\n',
        'data: {"type":"image","image_url":"data:image/png;base64,QUJD","mime_type":"image/png"}\n',
        'data: {"type":"test_complete","success":true}\n'
      ])
    ) as any
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('loads available models when mounted already open', async () => {
    mountModal({ show: true })
    await flushPromises()

    expect(getAvailableModels).toHaveBeenCalledWith(42)
  })

  it('普通文本模型测试默认使用 hi 提示词', async () => {
    getAvailableModels.mockResolvedValue([
      { id: 'gpt-5.4', display_name: 'GPT-5.4' }
    ])
    const wrapper = mountModal({
      show: true,
      account: {
        id: 31,
        name: 'OpenAI Test',
        platform: 'openai',
        type: 'apikey',
        status: 'active'
      }
    })
    await flushPromises()

    const promptSelect = wrapper.findAll('select').at(-1)
    expect(promptSelect).toBeTruthy()
    expect(promptSelect!.text()).toContain('admin.accounts.textPromptPresetBasic')
    expect(promptSelect!.element.value).toBe('basic')
    expect(wrapper.text()).not.toContain('admin.accounts.channelIdentityHint')

    const startButton = wrapper.findAll('button').find((button) =>
      button.text().includes('admin.accounts.startTest')
    )
    await startButton!.trigger('click')
    await flushPromises()

    const [, request] = (global.fetch as any).mock.calls[0]
    expect(JSON.parse(request.body).prompt).toBe('hi')
  })

  it('文本模型测试可选择渠道身份与能力自检提示词', async () => {
    getAvailableModels.mockResolvedValue([
      { id: 'gpt-5.4', display_name: 'GPT-5.4' }
    ])
    const wrapper = mountModal({
      show: true,
      account: {
        id: 31,
        name: 'OpenAI Test',
        platform: 'openai',
        type: 'apikey',
        status: 'active'
      }
    })
    await flushPromises()

    const promptSelect = wrapper.findAll('select').at(-1)
    expect(promptSelect).toBeTruthy()
    expect(promptSelect!.text()).toContain('admin.accounts.textPromptPresetChannelIdentity')
    await promptSelect!.setValue('channel_identity')
    expect(wrapper.text()).toContain('admin.accounts.channelIdentityHint')

    const startButton = wrapper.findAll('button').find((button) =>
      button.text().includes('admin.accounts.startTest')
    )
    await startButton!.trigger('click')
    await flushPromises()

    const [, request] = (global.fetch as any).mock.calls[0]
    const prompt = JSON.parse(request.body).prompt
    expect(prompt).toContain('请对你当前所处的服务环境进行一次客观的身份与能力自检。')
    expect(prompt).toContain('模型名称：\\\n所属产品：\\\n')
    expect(prompt).toContain('判断依据：\\\n无法确认的信息：\n\n')
    expect(prompt.endsWith('其他内部机密信息')).toBe(true)
  })

  it.each([
    { platform: 'typesafe', extra: {} },
    { platform: 'anthropic', extra: { synthetic_ui_test: true } }
  ])('$platform 专用连接测试使用原请求，不显示聊天提示词选项', async ({ platform, extra }) => {
    getAvailableModels.mockResolvedValue([{ id: 'probe', display_name: 'Probe' }])
    const wrapper = mountModal({
      show: true,
      account: {
        id: 32,
        name: 'TypeSafe Test',
        platform,
        extra,
        type: 'apikey',
        status: 'active'
      }
    })
    await flushPromises()

    expect(wrapper.text()).not.toContain('admin.accounts.textPromptPresetLabel')
    await wrapper.get('.batch-test-actions button:last-child').trigger('click')
    await flushPromises()
    const [, request] = vi.mocked(fetch).mock.calls[0]
    expect(JSON.parse(request!.body as string)).toEqual({ model_id: 'probe', prompt: '' })
  })

  it.each([
    { platform: 'gemini', image: 'gemini-3.1-flash-image', text: 'gemini-4-argon' },
    { platform: 'openai', image: 'gpt-image-2', text: 'gpt-5.4' },
    { platform: 'openai', image: 'public-art', text: 'public-chat', mapping: { 'public-*': 'gpt-image-2', 'public-chat': 'gpt-5.4' } }
  ])('$platform 混合批量测试按每个模型发提示词，并显示对应的自检回复（$image）', async ({ platform, image, text, mapping }) => {
    getAvailableModels.mockResolvedValue([image, text].map(id => ({ id, display_name: id })))
    vi.mocked(fetch).mockImplementation(async (_url, request) => {
      const { model_id } = JSON.parse(request!.body as string)
      return createStreamResponse([
        `data: ${JSON.stringify({ type: 'content', text: model_id })}\n`,
        'data: {"type":"content","text":" reply"}\n',
        'data: {"type":"test_complete","success":true}\n'
      ])
    })
    const wrapper = mountModal({ show: true, account: { id: 42, platform, type: 'apikey', credentials: { model_mapping: mapping } } })
    await flushPromises()
    await wrapper.get('textarea').setValue('draw a cloud')
    await wrapper.get('select').setValue(text)
    const preset = wrapper.findAll('select').find(select => select.find('option[value="channel_identity"]').exists())
    await preset!.setValue('channel_identity')
    const batchButton = wrapper.findAll('button').find(button => button.text().includes('admin.accounts.testAllModels'))
    await batchButton!.trigger('click')
    await flushPromises()
    const requests = vi.mocked(fetch).mock.calls.map(([, request]) => JSON.parse(request!.body as string))
    expect(requests).toEqual(expect.arrayContaining([
      expect.objectContaining({ model_id: image, prompt: 'draw a cloud' }),
      expect.objectContaining({ model_id: text, prompt: expect.stringContaining('身份与能力自检') })
    ]))
    const rows = wrapper.findAll('.batch-test-result')
    expect(rows.find(row => row.text().includes(text))!.get('details').text()).toContain(`${text} reply`)
    expect(rows.find(row => row.text().includes(image))!.find('details').exists()).toBe(false)
  })

  it('keeps adaptive protocol status boundaries in batch self-check replies', async () => {
    getAvailableModels.mockResolvedValue([
      { id: 'model-a', display_name: 'Model A' },
      { id: 'model-b', display_name: 'Model B' }
    ])
    vi.mocked(fetch).mockImplementation(async () => createStreamResponse([
      'data: {"type":"content","text":"Chat reply"}\n',
      'data: {"type":"status","text":"Testing /v1/messages"}\n',
      'data: {"type":"content","text":"Anthropic reply"}\n',
      'data: {"type":"status","text":"Passed /v1/messages"}\n',
      'data: {"type":"status","text":"Testing /responses"}\n',
      'data: {"type":"content","text":"Responses reply"}\n',
      'data: {"type":"test_complete","success":true}\n'
    ]))
    const wrapper = mountModal({ show: true, account: { id: 42, platform: 'deepseek', type: 'apikey' } })
    await flushPromises()
    const preset = wrapper.findAll('select').find(select => select.find('option[value="channel_identity"]').exists())
    await preset!.setValue('channel_identity')
    const batchButton = wrapper.findAll('button').find(button => button.text().includes('admin.accounts.testAllModels'))
    await batchButton!.trigger('click')
    await flushPromises()

    for (const row of wrapper.findAll('.batch-test-result')) {
      expect(row.get('details div').text()).toBe(
        'Chat reply\n\n[Testing /v1/messages]\nAnthropic reply\n\n[Passed /v1/messages]\n\n[Testing /responses]\nResponses reply'
      )
    }
    expect(wrapper.findAll('.batch-test-result')).toHaveLength(2)
  })

  it('gemini 图片模型测试会携带提示词并渲染图片预览', async () => {
    const wrapper = mountModal()
    await wrapper.setProps({ show: true })
    await flushPromises()

    const promptInput = wrapper.find('textarea.textarea-stub')
    expect(promptInput.exists()).toBe(true)
    await promptInput.setValue('draw a tiny orange cat astronaut')

    const buttons = wrapper.findAll('button')
    const startButton = buttons.find((button) => button.text().includes('admin.accounts.startTest'))
    expect(startButton).toBeTruthy()

    await startButton!.trigger('click')
    await flushPromises()
    await flushPromises()

    expect(global.fetch).toHaveBeenCalledTimes(1)
    const [, request] = (global.fetch as any).mock.calls[0]
    expect(JSON.parse(request.body)).toEqual({
      model_id: 'gemini-3.1-flash-image',
      prompt: 'draw a tiny orange cat astronaut'
    })

    const preview = wrapper.find('img[alt="test-image-1"]')
    expect(preview.exists()).toBe(true)
    expect(preview.attributes('src')).toBe('data:image/png;base64,QUJD')
    expect(wrapper.find('.batch-test-copy').exists()).toBe(true)
  })

  it('grok 账号测试默认选择 Grok 模型', async () => {
    getAvailableModels.mockResolvedValue([
      { id: 'grok-4.3', display_name: 'Grok 4.3' },
      { id: 'grok-build-0.1', display_name: 'Grok Build 0.1' }
    ])
    global.fetch = vi.fn().mockResolvedValue(
      createStreamResponse([
        'data: {"type":"test_start","model":"grok-4.3"}\n',
        'data: {"type":"content","text":"ok"}\n',
        'data: {"type":"test_complete","success":true}\n'
      ])
    ) as any

    const wrapper = mountModal({
      account: {
        id: 13,
        name: 'Grok Account',
        platform: 'grok',
        type: 'oauth',
        status: 'active'
      }
    })
    await wrapper.setProps({ show: true })
    await flushPromises()

    const buttons = wrapper.findAll('button')
    const startButton = buttons.find((button) => button.text().includes('admin.accounts.startTest'))
    expect(startButton).toBeTruthy()

    await startButton!.trigger('click')
    await flushPromises()

    expect(global.fetch).toHaveBeenCalledTimes(1)
    const [, request] = (global.fetch as any).mock.calls[0]
    expect(JSON.parse(request.body)).toEqual({
      model_id: 'grok-4.3',
      prompt: 'hi',
      mode: 'text'
    })
  })

  it('OpenAI Compact 探测会携带 compact 测试模式', async () => {
    getAvailableModels.mockResolvedValue([
      { id: 'gpt-5.4', display_name: 'GPT-5.4' }
    ])
    global.fetch = vi.fn().mockResolvedValue(
      createStreamResponse([
        'data: {"type":"test_complete","success":true}\n'
      ])
    ) as any

    const wrapper = mountModal({
      account: {
        id: 42,
        name: 'OpenAI OAuth',
        platform: 'openai',
        type: 'oauth',
        status: 'active'
      }
    })
    await wrapper.setProps({ show: true })
    await flushPromises()

    ;(wrapper.vm as any).selectedModelId = 'gpt-5.4'
    ;(wrapper.vm as any).testMode = 'compact'
    await (wrapper.vm as any).startTest()
    await flushPromises()

    expect(wrapper.text()).not.toContain('admin.accounts.textPromptPresetLabel')
    expect(global.fetch).toHaveBeenCalledTimes(1)
    const [, request] = (global.fetch as any).mock.calls[0]
    expect(JSON.parse(request.body)).toMatchObject({
      model_id: 'gpt-5.4',
      prompt: '',
      mode: 'compact'
    })
  })

  it('Grok 文本测试摘要与发送日志标明所选自检', async () => {
    getAvailableModels.mockResolvedValue([{ id: 'grok-4.3', display_name: 'Grok' }])
    vi.mocked(fetch).mockResolvedValue(createStreamResponse([
      'data: {"type":"test_start","model":"grok-4.3"}\n',
      'data: {"type":"test_complete","success":true}\n'
    ]))
    const wrapper = mountModal({ show: true, account: { id: 13, platform: 'grok', type: 'oauth' } })
    await flushPromises()
    const preset = wrapper.findAll('select').find(select => select.find('option[value="channel_identity"]').exists())
    await preset!.setValue('channel_identity')
    expect(wrapper.text()).toContain('admin.accounts.selectedTestPromptSummary admin.accounts.textPromptPresetChannelIdentity')
    await wrapper.get('.batch-test-actions button:last-child').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('admin.accounts.sendingSelectedTestMessage admin.accounts.textPromptPresetChannelIdentity')
    const [, request] = vi.mocked(fetch).mock.calls[0]
    expect(JSON.parse(request!.body as string)).toMatchObject({ mode: 'text', prompt: expect.stringContaining('身份与能力自检') })
  })

  it('可以按模型顺序批量测试，并在失败后继续测试后续模型', async () => {
    getAvailableModels.mockResolvedValue([
      { id: 'gpt-first', display_name: 'GPT First' },
      { id: 'gpt-second', display_name: 'GPT Second' },
      { id: 'gpt-third', display_name: 'GPT Third' }
    ])
    let responseIndex = 0
    global.fetch = vi.fn().mockImplementation(() => {
      const index = responseIndex++
      const model = ['gpt-first', 'gpt-second', 'gpt-third'][index]
      const lines = index === 1
        ? [`data: {"type":"error","error":"upstream denied"}\n`]
        : [
            `data: {"type":"test_start","model":"${model}"}\n`,
            'data: {"type":"test_complete","success":true}\n'
          ]
      return Promise.resolve(createStreamResponse(lines))
    }) as any

    const wrapper = mountModal({
      show: true,
      account: {
        id: 42,
        name: 'OpenAI Account',
        platform: 'openai',
        type: 'apikey',
        status: 'active'
      }
    })
    await flushPromises()

    const batchButton = wrapper.findAll('button').find((button) => button.text().includes('admin.accounts.testAllModels'))
    expect(batchButton).toBeTruthy()

    await (wrapper.vm as any).startBatchTest()

    expect(global.fetch).toHaveBeenCalledTimes(3)
    expect((global.fetch as any).mock.calls.map(([, request]: [string, RequestInit]) => JSON.parse(request.body as string).model_id)).toEqual([
      'gpt-first',
      'gpt-second',
      'gpt-third'
    ])
    expect((wrapper.vm as any).batchResults).toMatchObject([
      { modelId: 'gpt-first', status: 'success', durationMs: expect.any(Number) },
      { modelId: 'gpt-second', status: 'failed', error: 'upstream denied', durationMs: expect.any(Number) },
      { modelId: 'gpt-third', status: 'success', durationMs: expect.any(Number) }
    ])
    for (const result of (wrapper.vm as any).batchResults) {
      expect(result.durationMs).toBeGreaterThanOrEqual(0)
    }
    expect(wrapper.text()).toContain('admin.accounts.batchSuccessCount')
    expect(wrapper.text()).toContain('admin.accounts.batchFailedCount')
    expect(wrapper.text()).toContain('admin.accounts.batchLatency')
    expect(wrapper.find('.batch-test-actions').classes()).toContain('sm:flex-row')
    expect(wrapper.find('.batch-test-result > div').classes()).toEqual(expect.arrayContaining([
      'w-full',
      'justify-between'
    ]))
    expect(wrapper.find('.batch-test-error').classes()).toEqual(expect.arrayContaining([
      'w-full',
      'min-w-0',
      'max-h-24',
      'overflow-y-auto'
    ]))
  })

  it('一键测试会在首个模型完成前同时发起所有模型请求', async () => {
    getAvailableModels.mockResolvedValue([
      { id: 'gpt-first', display_name: 'GPT First' },
      { id: 'gpt-second', display_name: 'GPT Second' },
      { id: 'gpt-third', display_name: 'GPT Third' }
    ])
    const resolvers: Array<(response: Response) => void> = []
    global.fetch = vi.fn().mockImplementation(() => new Promise<Response>((resolve) => {
      resolvers.push(resolve)
    })) as any

    const wrapper = mountModal({
      show: true,
      account: {
        id: 42,
        name: 'OpenAI Account',
        platform: 'openai',
        type: 'apikey',
        status: 'active'
      }
    })
    await flushPromises()

    const batchPromise = (wrapper.vm as any).startBatchTest()
    await vi.waitFor(() => expect(global.fetch).toHaveBeenCalledTimes(3))
    expect((wrapper.vm as any).batchResults.every((result: { status: string }) => result.status === 'testing')).toBe(true)

    for (const resolve of resolvers) {
      resolve(createStreamResponse([
        'data: {"type":"test_complete","success":true}\n'
      ]))
    }
    await batchPromise

    expect((wrapper.vm as any).batchResults).toMatchObject([
      { modelId: 'gpt-first', status: 'success' },
      { modelId: 'gpt-second', status: 'success' },
      { modelId: 'gpt-third', status: 'success' }
    ])
  })

  it('停止批量测试会取消当前请求并标记未开始的模型', async () => {
    getAvailableModels.mockResolvedValue([
      { id: 'gpt-first', display_name: 'GPT First' },
      { id: 'gpt-second', display_name: 'GPT Second' },
      { id: 'gpt-third', display_name: 'GPT Third' }
    ])
    const resolvers: Array<(response: Response) => void> = []
    global.fetch = vi.fn().mockImplementation(() => new Promise<Response>((resolve) => {
      resolvers.push(resolve)
    })) as any

    const wrapper = mountModal({
      show: true,
      account: {
        id: 42,
        name: 'OpenAI Account',
        platform: 'openai',
        type: 'apikey',
        status: 'active'
      }
    })
    await flushPromises()

    const batchPromise = (wrapper.vm as any).startBatchTest()
    await vi.waitFor(() => expect(global.fetch).toHaveBeenCalledTimes(3))
    ;(wrapper.vm as any).stopBatchTest()
    for (const resolve of resolvers) {
      resolve(createStreamResponse([]))
    }
    await batchPromise

    expect((wrapper.vm as any).batchResults).toMatchObject([
      { modelId: 'gpt-first', status: 'cancelled' },
      { modelId: 'gpt-second', status: 'cancelled' },
      { modelId: 'gpt-third', status: 'cancelled' }
    ])
    expect((wrapper.vm as any).batchResults.every((result: { durationMs?: number }) => result.durationMs === undefined)).toBe(true)
  })
})
