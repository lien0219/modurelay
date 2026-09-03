import { createHash } from 'node:crypto'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const vendorRoot = resolve(dir, '../../../../public/threeui/sylva')

const registeredFiles = [
  ['inner-green-3d.html', '69c3694bd63f44ef9f007ebe4dac57a83e4402e0cdf6b54dd10b96dd4f05e197'],
  ['inner-green-assets/three.min.js', '8a5f7249903b54d30f79f708699d2fed2d6a1d0741a4cd41377d1f01bb5a2271'],
  ['inner-green-assets/card-ecostove.jpg', '70ce084084902bc502f00c366405b661ecdff90dee95d363b36a6e146829e433'],
  ['inner-green-assets/card-ethos.jpg', '337627390f499b3ae272cec9e2f83c817694a82f42e1aa10a7b26a2c7d679dff'],
  ['inner-green-assets/lexend-latin.woff2', '1ec8f6ee2750554b4bc59ff0b507d316a82a7ba37e0e5bebc41d3bd9b9faad46'],
] as const

describe('vendored Sylva source integrity', () => {
  it.each(registeredFiles)('keeps %s byte-exact', (path, expectedHash) => {
    const contents = readFileSync(resolve(vendorRoot, path))
    expect(createHash('sha256').update(contents).digest('hex')).toBe(expectedHash)
  })
})
