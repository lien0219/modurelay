import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
}))

vi.mock('../client', () => ({
  apiClient: {
    get,
    post,
  },
}))

import {
  createSystemOperationKey,
  getRollbackVersions,
  rollback,
  type RollbackVersionInfo
} from '@/api/admin/system'

describe('admin system rollback API', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    get.mockReset()
    post.mockReset()
  })

  it('getRollbackVersions fetches the rollback version list', async () => {
    const versions: RollbackVersionInfo[] = [
      {
        version: '0.1.146',
        published_at: '2026-07-07T00:00:00Z',
        html_url: 'https://github.com/lien0219/modurelay/releases/tag/v0.1.146',
        method: 'binary'
      }
    ]
    get.mockResolvedValue({ data: { versions } })

    const result = await getRollbackVersions()

    expect(get).toHaveBeenCalledWith('/admin/system/rollback-versions')
    expect(result.versions).toEqual(versions)
  })

  it('rollback posts the target version in the request body', async () => {
    post.mockResolvedValue({ data: { message: 'ok', need_restart: true } })

    const result = await rollback('0.1.146')

    expect(post).toHaveBeenCalledWith(
      '/admin/system/rollback',
      { version: '0.1.146' },
      { timeout: 15 * 60 * 1000, headers: undefined }
    )
    expect(result.need_restart).toBe(true)
  })

  it('rollback without a version posts no body (legacy backup rollback)', async () => {
    post.mockResolvedValue({ data: { message: 'ok', need_restart: true } })

    await rollback()

    expect(post).toHaveBeenCalledWith(
      '/admin/system/rollback',
      undefined,
      { timeout: 15 * 60 * 1000, headers: undefined }
    )
  })

  it('reuses an explicit idempotency key in a rollback request', async () => {
    post.mockResolvedValue({ data: { message: 'ok', need_restart: true } })

    await rollback('0.3.0', 'system-rollback-0.3.0-operation-id')

    expect(post).toHaveBeenCalledWith(
      '/admin/system/rollback',
      { version: '0.3.0' },
      {
        timeout: 15 * 60 * 1000,
        headers: { 'Idempotency-Key': 'system-rollback-0.3.0-operation-id' }
      }
    )
  })

  it('creates scoped system operation keys', () => {
    vi.spyOn(globalThis.crypto, 'randomUUID').mockReturnValue(
      '11111111-1111-4111-8111-111111111111'
    )

    expect(createSystemOperationKey('rollback', '0.3.0')).toBe(
      'system-rollback-0.3.0-11111111-1111-4111-8111-111111111111'
    )
  })
})
