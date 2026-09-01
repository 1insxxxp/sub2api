import { beforeEach, describe, expect, it, vi } from 'vitest'

const { del, get, post } = vi.hoisted(() => ({
  del: vi.fn(),
  get: vi.fn(),
  post: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    delete: del,
    get,
    post
  }
}))

import {
  deleteGenerated,
  deleteGeneratedBatch,
  generateBalanceTransferCodes,
  getGenerated,
  type GenerateBalanceTransferCodeRequest
} from '@/api/redeem'
import type { GeneratedRedeemCode } from '@/api/redeem'
import type { PaginatedResponse } from '@/types'

describe('admin workbench balance redeem codes api', () => {
  beforeEach(() => {
    del.mockReset()
    get.mockReset()
    post.mockReset()
  })

  it('generates balance redeem codes through the workbench route', async () => {
    const request: GenerateBalanceTransferCodeRequest = {
      amount: 12.5,
      count: 2,
      expires_in_days: 14,
      notes: 'team drop',
      single_use_per_user: true
    }
    const response: GeneratedRedeemCode[] = [
      {
        id: 18,
        code: 'ABCD-EFGH',
        type: 'balance',
        value: 12.5,
        status: 'unused',
        used_by: null,
        used_at: null,
        created_at: '2026-08-20T12:00:00Z',
        created_by: 7,
        source: 'user_balance_transfer',
        single_use_per_user: true
      }
    ]
    post.mockResolvedValue({ data: response })

    const result = await generateBalanceTransferCodes(request)

    expect(post).toHaveBeenCalledWith('/admin/workbench/redeem/generated', request)
    expect(result).toEqual(response)
  })

  it('loads generated balance redeem codes through the workbench route', async () => {
    const response: PaginatedResponse<GeneratedRedeemCode> = {
      items: [],
      total: 0,
      page: 1,
      page_size: 10,
      pages: 1
    }
    get.mockResolvedValue({ data: response })

    const result = await getGenerated({ page: 1, page_size: 10 })

    expect(get).toHaveBeenCalledWith('/admin/workbench/redeem/generated', {
      params: { page: 1, page_size: 10 }
    })
    expect(result).toEqual(response)
  })

  it('deletes one generated balance redeem code through the workbench route', async () => {
    const response = { id: 18, code: 'ABCD-EFGH' } as GeneratedRedeemCode
    del.mockResolvedValue({ data: response })

    const result = await deleteGenerated(18)

    expect(del).toHaveBeenCalledWith('/admin/workbench/redeem/generated/18')
    expect(result).toEqual(response)
  })

  it('batch deletes generated balance redeem codes through the workbench route', async () => {
    const response = [] as GeneratedRedeemCode[]
    post.mockResolvedValue({ data: response })

    const result = await deleteGeneratedBatch([18, 19])

    expect(post).toHaveBeenCalledWith('/admin/workbench/redeem/generated/batch-delete', {
      ids: [18, 19]
    })
    expect(result).toEqual(response)
  })
})
