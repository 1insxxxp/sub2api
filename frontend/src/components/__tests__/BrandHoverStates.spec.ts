import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

const root = resolve(__dirname, '../..')

const themedInteractionFiles = [
  'components/common/DateRangePicker.vue',
  'components/common/ProxySelector.vue',
  'components/common/Select.vue',
  'components/payment/AmountInput.vue',
  'components/payment/PaymentMethodSelector.vue',
  'components/payment/PaymentProviderDialog.vue',
  'components/user/monitor/MonitorCard.vue',
  'composables/useChannelMonitorFormat.ts',
]

const neutralHoverBorderPatterns = [
  /hover:border-gray-\d+/,
  /hover:border-slate-\d+/,
  /dark:hover:border-dark-\d+/,
  /dark:hover:border-slate-\d+/,
  /dark:hover:border-gray-\d+/,
]

describe('brand hover states', () => {
  it('does not use neutral gray hover borders on themed interactive surfaces', () => {
    for (const file of themedInteractionFiles) {
      const source = readFileSync(resolve(root, file), 'utf8')

      for (const pattern of neutralHoverBorderPatterns) {
        expect(source, `${file} contains ${pattern}`).not.toMatch(pattern)
      }
    }
  })
})
