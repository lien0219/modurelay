import { describe, expect, it } from 'vitest'

import { getDefaultToolOperation } from '../defaults'

describe('toolbox defaults', () => {
  it('starts each operation-based tool in a valid primary mode', () => {
    expect(getDefaultToolOperation('json')).toBe('format')
    expect(getDefaultToolOperation('base64')).toBe('encode')
    expect(getDefaultToolOperation('url')).toBe('encode')
    expect(getDefaultToolOperation('timestamp')).toBe('timestampToDate')
  })
})
