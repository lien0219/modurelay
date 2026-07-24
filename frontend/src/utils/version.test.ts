import { describe, expect, it } from 'vitest'
import { extractSemanticVersion } from './version'

describe('extractSemanticVersion', () => {
  it.each([
    ['main-v0.0.1-81a2f23b25fb', '0.0.1'],
    ['v0.0.1', '0.0.1'],
    ['0.0.1', '0.0.1'],
    ['release/v12.34.56+build.7', '12.34.56']
  ])('extracts %s as %s', (input, expected) => {
    expect(extractSemanticVersion(input)).toBe(expected)
  })

  it('keeps a non-semantic fallback readable', () => {
    expect(extractSemanticVersion('vdev')).toBe('dev')
  })

  it('returns an empty string for missing values', () => {
    expect(extractSemanticVersion(undefined)).toBe('')
  })
})
