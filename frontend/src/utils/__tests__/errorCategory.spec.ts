import { describe, expect, it } from 'vitest'
import { mapErrorCategory } from '../errorCategory'

describe('canceled request categories', () => {
  it('prioritizes cancellation evidence over an earlier upstream failure', () => {
    expect(mapErrorCategory('upstream', 'upstream_error', true)).toBe('client_cancel')
    expect(mapErrorCategory('network', 'client_cancel')).toBe('client_cancel')
  })

  it('keeps upstream and input failures distinct when no cancellation is known', () => {
    expect(mapErrorCategory('upstream', 'upstream_error', false)).toBe('upstream')
    expect(mapErrorCategory('request', 'invalid_request_error')).toBe('invalid_request')
  })
})
