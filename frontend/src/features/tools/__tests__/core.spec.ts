import { describe, expect, it } from 'vitest'

import {
  calculateCidr,
  decodeBase64Utf8,
  decodeJwt,
  decodeUrlComponent,
  diffCharacters,
  diffLines,
  encodeBase64Utf8,
  encodeUrlComponent,
  executeRegex,
  formatJson,
  generateApiSnippets,
  generatePassword,
  generateRandomString,
  generateTotp,
  generateUuidV4,
  generateUuidV7,
  getRandomAlphabetSize,
  hashText,
  hmacText,
  jsonToGo,
  jsonToPython,
  jsonToTypeScript,
  markdownToSafeHtml,
  minifyJson,
  nextCronRuns,
  parseCron,
  parseCurl,
  parseOtpAuthUri,
  searchHttpStatuses,
  timestampToDate,
  validateApiRequestUrl,
  validateJson,
} from '../core'

describe('toolbox core: TOTP', () => {
  it('matches the RFC 6238 SHA-1 vector', async () => {
    const result = await generateTotp({
      secret: 'GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ',
      algorithm: 'SHA-1',
      digits: 8,
      period: 30,
    }, 59_000)

    expect(result.code).toBe('94287082')
    expect(result.counter).toBe(1)
    expect(result.remainingSeconds).toBe(1)
  })

  it('matches the RFC 6238 SHA-256 and SHA-512 vectors', async () => {
    const sha256Secret = 'GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZA'
    const sha512Secret = 'GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZDGNA'
    await expect(generateTotp({ secret: sha256Secret, algorithm: 'SHA-256', digits: 8, period: 30 }, 59_000)).resolves.toMatchObject({ code: '46119246' })
    await expect(generateTotp({ secret: sha512Secret, algorithm: 'SHA-512', digits: 8, period: 30 }, 59_000)).resolves.toMatchObject({ code: '90693936' })
  })

  it('parses otpauth labels and options locally', () => {
    expect(parseOtpAuthUri('otpauth://totp/Example:alice%40example.com?secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ&issuer=Example&algorithm=SHA256&digits=8&period=60')).toEqual({
      secret: 'GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ',
      issuer: 'Example',
      account: 'alice@example.com',
      algorithm: 'SHA-256',
      digits: 8,
      period: 60,
    })
  })
})

describe('toolbox core: random values and structured encodings', () => {
  it('uses secure random output and produces valid UUID variants', () => {
    const password = generatePassword({ length: 32, uppercase: true, lowercase: true, numbers: true, symbols: true })
    const random = generateRandomString({ length: 20, customCharacters: 'ab' })
    expect(password).toHaveLength(32)
    expect(random).toMatch(/^[ab]{20}$/)
    expect(generateUuidV4()).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/)
    expect(generateUuidV7(1_700_000_000_000)).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/)
  })

  it('supports large Unicode alphabets without truncating code points', () => {
    const alphabet = '😀😃😄😁😆😅😂🤣😊😇'
    const result = generateRandomString({ length: 64, customCharacters: alphabet })
    expect(getRandomAlphabetSize({ length: 1, customCharacters: alphabet })).toBe(Array.from(alphabet).length)
    expect(Array.from(result)).toHaveLength(64)
    expect(Array.from(result).every((character) => Array.from(alphabet).includes(character))).toBe(true)
  })

  it('validates, formats, minifies and round-trips UTF-8 Base64', () => {
    const input = '{"message":"你好 👋","items":[1,true]}'
    expect(validateJson(input)).toMatchObject({ valid: true, value: { message: '你好 👋', items: [1, true] } })
    expect(validateJson('{"message":}').valid).toBe(false)
    expect(formatJson(input, 2)).toContain('\n  "message"')
    expect(minifyJson(formatJson(input))).toBe(input)

    const encoded = encodeBase64Utf8('你好 👋')
    expect(decodeBase64Utf8(encoded)).toBe('你好 👋')
    const urlEncoded = encodeBase64Utf8('subjects/?', true)
    expect(urlEncoded).not.toContain('=')
    expect(decodeBase64Utf8(urlEncoded, true)).toBe('subjects/?')
    expect(() => decodeBase64Utf8('%%%')).toThrow()
  })

  it('encodes URL components and distinguishes seconds from milliseconds', () => {
    const value = 'a value/中文?x=1'
    expect(decodeUrlComponent(encodeUrlComponent(value))).toBe(value)
    expect(timestampToDate(1_700_000_000, 'seconds').milliseconds).toBe(1_700_000_000_000)
    expect(timestampToDate('1700000000000', 'auto').seconds).toBe(1_700_000_000)
  })
})

describe('toolbox core: JWT and cryptography', () => {
  it('decodes JWT payload claims without claiming signature verification', () => {
    const token = `${encodeBase64Utf8('{"alg":"none","typ":"JWT"}', true)}.${encodeBase64Utf8('{"sub":"123","iat":1700000000,"exp":1700003600}', true)}.signature`
    const result = decodeJwt(token)
    expect(result.header).toEqual({ alg: 'none', typ: 'JWT' })
    expect(result.claims.sub).toBe('123')
    expect(result.claimDates.exp).toBe('2023-11-14T23:13:20.000Z')
    expect(result.signature).toBe('signature')
  })

  it('matches standard SHA-256/SHA-512 and HMAC vectors', async () => {
    const sha256 = await hashText('abc', 'SHA-256')
    const sha512 = await hashText('abc', 'SHA-512')
    const hmac = await hmacText('The quick brown fox jumps over the lazy dog', 'key', 'SHA-256')
    expect(sha256.hex).toBe('ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad')
    expect(sha512.hex).toBe('ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f')
    expect(hmac.hex).toBe('f7bc83f430538424b13298e6aa6fb143ef4d59a14946175997479dbc2d1a3cd8')
  })
})

describe('toolbox core: regex, CIDR and API conversion', () => {
  it('returns capture and named groups and rejects unsafe expressions', () => {
    const result = executeRegex('(foo)-(?<id>\\d+)', 'g', 'foo-42 foo-7')
    expect(result.timedOut).toBe(false)
    expect(result.matches.map((match) => [match.full, match.index, match.groups, match.namedGroups])).toEqual([
      ['foo-42', 0, ['foo', '42'], { id: '42' }],
      ['foo-7', 7, ['foo', '7'], { id: '7' }],
    ])
    expect(executeRegex('(a+)+$', '', 'a'.repeat(20) + 'x').error).toContain('unsafe')
  })

  it('calculates IPv4 and IPv6 CIDR ranges', () => {
    expect(calculateCidr('192.168.1.42/24')).toMatchObject({
      version: 4,
      networkAddress: '192.168.1.0',
      broadcastAddress: '192.168.1.255',
      firstHost: '192.168.1.1',
      lastHost: '192.168.1.254',
      totalAddresses: '256',
      usableHosts: '254',
    })
    expect(calculateCidr('2001:db8::1234/64')).toMatchObject({ version: 6, networkAddress: '2001:db8::', prefix: 64, totalAddresses: '18446744073709551616' })
  })

  it('parses cURL and generates request snippets without executing requests', () => {
    const parsed = parseCurl("curl -X POST 'https://example.test/api?q=1' -H 'Content-Type: application/json' -H 'Authorization: Bearer token' --data '{\"ok\":true}'")
    expect(parsed).toMatchObject({ method: 'POST', url: 'https://example.test/api?q=1', body: '{"ok":true}', bodyType: 'json', authorization: { type: 'bearer', value: 'token' } })
    const snippets = generateApiSnippets(parsed)
    expect(snippets.curl).toContain("-X POST")
    expect(snippets.fetch).toContain('fetch(')
    expect(snippets.python).toContain('requests.request')
    expect(snippets.go).toContain('http.NewRequest')
  })

  it('recognizes URL-encoded form bodies from cURL content type headers', () => {
    const parsed = parseCurl("curl -H 'Content-Type: application/x-www-form-urlencoded' --data 'page=1&filter=active' https://example.test/search")
    expect(parsed).toMatchObject({ method: 'POST', bodyType: 'form', body: 'page=1&filter=active', form: { page: '1', filter: 'active' } })
  })

  it('validates API URLs and adds a JSON content type when absent', () => {
    expect(validateApiRequestUrl(' https://example.test/api ')).toBe('https://example.test/api')
    expect(() => validateApiRequestUrl('')).toThrow('required')
    expect(() => validateApiRequestUrl('javascript:alert(1)')).toThrow('HTTP')
    const snippets = generateApiSnippets({ method: 'POST', url: 'https://example.test/api', body: '{"ok":true}', bodyType: 'json' })
    expect(snippets.curl).toContain("Content-Type: application/json")
  })

  it('preserves Basic auth username and password when parsing cURL', () => {
    const parsed = parseCurl("curl --user 'alice:p@ss' https://example.test")
    expect(parsed.authorization).toEqual({ type: 'basic', username: 'alice', value: 'p@ss' })

    const encoded = encodeBase64Utf8('alice:p@ss')
    const fromHeader = parseCurl(`curl -H 'Authorization: Basic ${encoded}' https://example.test`)
    expect(fromHeader.authorization).toEqual({ type: 'basic', username: 'alice', value: 'p@ss' })
  })
})

describe('toolbox core: struct, cron, diff and markdown', () => {
  it('converts nested JSON to typed language definitions', () => {
    const input = '{"user":{"first_name":"Ada","active":true},"tags":["math"],"note":null}'
    expect(jsonToTypeScript(input, { rootName: 'Profile' })).toContain('export interface Profile')
    expect(jsonToTypeScript(input, { rootName: 'Profile' })).toContain('"first_name": string;')
    expect(jsonToGo(input, { rootName: 'Profile' })).toContain('type Profile struct')
    expect(jsonToGo(input, { rootName: 'Profile' })).toContain('json:"first_name"')
    const python = jsonToPython(input, { rootName: 'Profile' })
    expect(python).toContain('from __future__ import annotations')
    expect(python).toContain('class Profile:')
  })

  it('keeps Python forward references importable for nested dataclasses', () => {
    const python = jsonToPython('{"user":{"name":"Ada"}}', { rootName: 'Profile' })
    expect(python.startsWith('from __future__ import annotations\n')).toBe(true)
    expect(python.indexOf('class Profile:')).toBeLessThan(python.indexOf('class ProfileUser:'))
    expect(python).toContain('    user: ProfileUser')
  })

  it('preserves top-level JSON arrays in generated types', () => {
    const input = '[{"id":1},{"id":2,"label":"two"}]'
    expect(jsonToTypeScript(input, { rootName: 'Entry' })).toContain('export type Entry = EntryItem[]')
    expect(jsonToGo(input, { rootName: 'Entry' })).toContain('type Entry []EntryItem')
    expect(jsonToPython(input, { rootName: 'Entry' })).toContain('Entry = list[EntryItem]')
  })

  it('merges heterogeneous arrays and nullable fields in generated types', () => {
    const input = '{"items":[{"id":1,"name":"one"},{"id":2,"active":true}],"value":null}'
    const typescript = jsonToTypeScript(input, { rootName: 'Payload' })
    expect(typescript).toContain('"items": PayloadItems[]')
    expect(typescript).toContain('"active"?: boolean')
    expect(typescript).toContain('"value": unknown')
    expect(jsonToGo(input, { rootName: 'Payload' })).toContain('type PayloadItems struct')
    expect(jsonToPython(input, { rootName: 'Payload' })).toContain('class PayloadItems:')
  })

  it('emits valid Python identifiers for invalid and reserved JSON field names', () => {
    const python = jsonToPython('{"class":1,"a-b":2,"a b":3,"$value":4}', { rootName: 'Payload' })
    expect(python).toContain('class_: int')
    expect(python).toContain('aB: int')
    expect(python).toContain('aB2: int')
    expect(python).toContain('value: int')
  })

  it('orders Python dataclass defaults after required fields', () => {
    const python = jsonToPython('[{"optional":null,"required":1},{"required":2}]', { rootName: 'Entry' })
    expect(python.indexOf('required: int')).toBeLessThan(python.indexOf('optional: Any = None'))
  })

  it('parses five-field cron and returns future matching runs', () => {
    const cron = parseCron('*/15 * * * *')
    expect(cron.minute).toEqual([0, 15, 30, 45])
    const runs = nextCronRuns(cron, 2, new Date('2024-01-01T00:01:00Z'))
    expect(runs).toHaveLength(2)
    expect(runs.every((date) => date.getMinutes() % 15 === 0)).toBe(true)
    expect(runs[0].getHours()).toBe(runs[1].getHours())
    expect(runs[1].getTime()).toBeGreaterThan(runs[0].getTime())
  })

  it('reports line/character changes and sanitizes markdown XSS', () => {
    expect(diffLines('one\ntwo', 'one\nthree')).toEqual([
      { type: 'unchanged', value: 'one', oldLine: 1, newLine: 1 },
      { type: 'removed', value: 'two', oldLine: 2 },
      { type: 'added', value: 'three', newLine: 2 },
    ])
    expect(diffCharacters('cat', 'cart')).toEqual([
      { type: 'unchanged', value: 'c' },
      { type: 'unchanged', value: 'a' },
      { type: 'added', value: 'r' },
      { type: 'unchanged', value: 't' },
    ])
    const html = markdownToSafeHtml('# Hello\n\n<script>alert(1)</script>\n\n[link](https://example.com)')
    expect(html).toContain('<h1>Hello</h1>')
    expect(html).not.toContain('<script>')
    expect(html).toContain('href="https://example.com"')
  })
})

describe('toolbox core: local reference data', () => {
  it('searches HTTP statuses by code and name', () => {
    expect(searchHttpStatuses('404')[0]).toMatchObject({ code: 404, name: 'Not Found' })
    expect(searchHttpStatuses('gateway').map((item) => item.code)).toEqual([502, 504])
  })
})
