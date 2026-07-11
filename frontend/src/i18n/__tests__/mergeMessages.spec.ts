import { describe, expect, it } from 'vitest'

import { mergeMessages } from '../locales/mergeMessages'

describe('mergeMessages', () => {
  it('fills missing nested messages without replacing current module values', () => {
    expect(
      mergeMessages(
        {
          section: {
            current: 'current translation',
          },
        },
        {
          section: {
            current: 'restored translation',
            missing: 'restored translation',
          },
        },
      ),
    ).toEqual({
      section: {
        current: 'current translation',
        missing: 'restored translation',
      },
    })
  })
})
