/*
 * Pure, browser-local implementations used by Toolbox views.
 *
 * None of the functions in this module send input over the network, persist
 * input, or log secrets.  Async cryptographic functions use Web Crypto and
 * deliberately fail when a secure primitive is not available rather than
 * silently falling back to Math.random or a home-grown hash.
 */

import DOMPurify from 'dompurify'
import { marked } from 'marked'
import { CronExpressionParser } from 'cron-parser'
export { executeRegex, executeRegexInWorker, isPotentiallyUnsafeRegex } from './regex'
export type { RegexExecutionResult, RegexMatch } from './regex'

const textEncoder = new TextEncoder()
const textDecoder = new TextDecoder('utf-8', { fatal: false })
const MAX_TEXT_INPUT = 5_000_000
const MAX_DIFF_INPUT = 1_000_000

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null
}

function bytesToHex(bytes: Uint8Array): string {
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')
}

function bytesToBase64(bytes: Uint8Array): string {
  let binary = ''
  const chunkSize = 0x8000
  for (let offset = 0; offset < bytes.length; offset += chunkSize) {
    binary += String.fromCharCode(...bytes.subarray(offset, offset + chunkSize))
  }
  return btoa(binary)
}

function base64ToBytes(value: string): Uint8Array {
  const binary = atob(value)
  return Uint8Array.from(binary, (character) => character.charCodeAt(0))
}

function normalizeBase64(value: string, urlSafe: boolean): string {
  const normalized = urlSafe ? value.replace(/-/g, '+').replace(/_/g, '/') : value
  if (!/^[A-Za-z0-9+/]*={0,2}$/.test(normalized) || normalized.length % 4 === 1) {
    throw new Error('Invalid Base64 input')
  }
  return normalized.padEnd(Math.ceil(normalized.length / 4) * 4, '=')
}

function bigintToIPv4(value: bigint): string {
  return [24n, 16n, 8n, 0n].map((shift) => Number((value >> shift) & 255n)).join('.')
}

function bigintToIPv6(value: bigint): string {
  const groups = Array.from({ length: 8 }, (_, index) => Number((value >> BigInt((7 - index) * 16)) & 0xffffn))
  let bestStart = -1
  let bestLength = 0
  let start = -1
  for (let index = 0; index <= groups.length; index += 1) {
    if (index < groups.length && groups[index] === 0) {
      if (start < 0) start = index
      continue
    }
    if (start >= 0 && index - start > bestLength) {
      bestStart = start
      bestLength = index - start
    }
    start = -1
  }
  if (bestLength < 2) {
    bestStart = -1
  }
  if (bestStart >= 0) {
    const before = groups.slice(0, bestStart).map((group) => group.toString(16)).join(':')
    const after = groups.slice(bestStart + bestLength).map((group) => group.toString(16)).join(':')
    return `${before}::${after}`
  }
  return groups.map((group) => group.toString(16)).join(':')
}

function parseIPv4(value: string): bigint | null {
  const parts = value.split('.')
  if (parts.length !== 4) return null
  let result = 0n
  for (const part of parts) {
    if (!/^\d{1,3}$/.test(part)) return null
    const octet = Number(part)
    if (octet > 255) return null
    result = (result << 8n) | BigInt(octet)
  }
  return result
}

function parseIPv6(value: string): bigint | null {
  if (!/^[0-9a-fA-F:.]+$/.test(value)) return null
  const halves = value.split('::')
  if (halves.length > 2) return null
  const parseGroups = (part: string): number[] | null => {
    if (!part) return []
    const groups = part.split(':')
    const result: number[] = []
    for (const group of groups) {
      if (group.includes('.')) {
        const ipv4 = parseIPv4(group)
        if (ipv4 === null || result.length > 6) return null
        result.push(Number((ipv4 >> 16n) & 0xffffn), Number(ipv4 & 0xffffn))
      } else {
        if (!/^[0-9a-fA-F]{1,4}$/.test(group)) return null
        result.push(parseInt(group, 16))
      }
    }
    return result
  }
  const left = parseGroups(halves[0])
  const right = halves.length === 2 ? parseGroups(halves[1]) : []
  if (!left || !right || (halves.length === 1 && left.length !== 8) || (halves.length === 2 && left.length + right.length >= 8)) return null
  const groups = [...left, ...Array(8 - left.length - right.length).fill(0), ...right]
  return groups.reduce((result, group) => (result << 16n) | BigInt(group), 0n)
}

function parseIp(value: string): { version: 4 | 6; number: bigint; bits: number } | null {
  const trimmed = value.trim()
  const ipv4 = parseIPv4(trimmed)
  if (ipv4 !== null) return { version: 4, number: ipv4, bits: 32 }
  const ipv6 = parseIPv6(trimmed)
  if (ipv6 !== null) return { version: 6, number: ipv6, bits: 128 }
  return null
}

function prefixMask(bits: number, prefix: number): bigint {
  if (prefix === 0) return 0n
  return ((1n << BigInt(prefix)) - 1n) << BigInt(bits - prefix)
}

function quoteShell(value: string): string {
  return `'${value.replace(/'/g, `'"'"'`)}'`
}

function quoteJs(value: string): string {
  return JSON.stringify(value)
}

function quotePython(value: string): string {
  return JSON.stringify(value)
}

function quoteGo(value: string): string {
  return JSON.stringify(value)
}

function titleCase(value: string): string {
  return value.replace(/(^|[-_\s]+)([a-zA-Z0-9])/g, (_, prefix: string, character: string) => `${prefix}${character.toUpperCase()}`)
}

function identifier(value: string, fallback: string): string {
  const clean = value.replace(/[^A-Za-z0-9_]+/g, ' ').trim()
  const parts = clean.split(/\s+/).filter(Boolean)
  const candidate = parts.map((part, index) => index === 0 ? part : titleCase(part)).join('')
  if (!candidate) return fallback
  return /^[A-Za-z_]/.test(candidate) ? candidate : `_${candidate}`
}

const PYTHON_KEYWORDS = new Set([
  'and', 'as', 'assert', 'async', 'await', 'break', 'case', 'class', 'continue', 'def', 'del',
  'elif', 'else', 'except', 'finally', 'for', 'from', 'global', 'if', 'import', 'in', 'is',
  'lambda', 'match', 'nonlocal', 'not', 'or', 'pass', 'raise', 'return', 'try', 'while', 'with',
  'yield', 'False', 'None', 'True',
])

function pythonIdentifier(value: string, fallback: string): string {
  const candidate = identifier(value, fallback)
  return PYTHON_KEYWORDS.has(candidate) ? `${candidate}_` : candidate
}

/** Base32 decoding used for TOTP. Whitespace and separators are tolerated. */
export function decodeBase32(input: string): Uint8Array {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'
  const normalized = input.toUpperCase().replace(/[\s=-]/g, '')
  if (!normalized || /[^A-Z2-7]/.test(normalized)) throw new Error('Invalid Base32 secret')
  let buffer = 0
  let bits = 0
  const output: number[] = []
  for (const character of normalized) {
    buffer = (buffer << 5) | alphabet.indexOf(character)
    bits += 5
    if (bits >= 8) {
      bits -= 8
      output.push((buffer >> bits) & 0xff)
      // Keep only the not-yet-emitted bits. Without this bound a long
      // SHA-512 TOTP secret overflows JavaScript's 32-bit bitwise operators.
      buffer &= (1 << bits) - 1
    }
  }
  return Uint8Array.from(output)
}

export interface TotpConfig {
  readonly secret: string
  readonly issuer?: string
  readonly account?: string
  readonly algorithm: 'SHA-1' | 'SHA-256' | 'SHA-512'
  readonly digits: 6 | 8
  readonly period: number
}

export interface TotpResult {
  readonly code: string
  readonly counter: number
  readonly remainingSeconds: number
  readonly period: number
  readonly digits: 6 | 8
  readonly algorithm: TotpConfig['algorithm']
}

export function parseOtpAuthUri(input: string): TotpConfig {
  const url = new URL(input.trim())
  if (url.protocol.toLowerCase() !== 'otpauth:' || url.hostname.toLowerCase() !== 'totp') {
    throw new Error('Expected an otpauth://totp URI')
  }
  const secret = url.searchParams.get('secret')
  if (!secret) throw new Error('TOTP URI is missing a secret')
  const rawAlgorithm = (url.searchParams.get('algorithm') || 'SHA1').toUpperCase().replace('-', '')
  const algorithm = rawAlgorithm === 'SHA256' ? 'SHA-256' : rawAlgorithm === 'SHA512' ? 'SHA-512' : rawAlgorithm === 'SHA1' ? 'SHA-1' : null
  if (!algorithm) throw new Error(`Unsupported TOTP algorithm: ${rawAlgorithm}`)
  const rawDigits = Number(url.searchParams.get('digits') || 6)
  if (rawDigits !== 6 && rawDigits !== 8) throw new Error('TOTP digits must be 6 or 8')
  const period = Number(url.searchParams.get('period') || 30)
  if (!Number.isInteger(period) || period < 1 || period > 3600) throw new Error('TOTP period must be between 1 and 3600 seconds')
  const label = decodeURIComponent(url.pathname.replace(/^\//, ''))
  const issuer = url.searchParams.get('issuer') || (label.includes(':') ? label.slice(0, label.indexOf(':')) : undefined)
  const account = label.includes(':') ? label.slice(label.indexOf(':') + 1) : label || undefined
  decodeBase32(secret)
  return { secret, issuer: issuer || undefined, account, algorithm, digits: rawDigits, period }
}

export function normalizeTotpConfig(config: Partial<TotpConfig> & Pick<TotpConfig, 'secret'>): TotpConfig {
  const parsed = config.secret.trim().toLowerCase().startsWith('otpauth://') ? parseOtpAuthUri(config.secret) : undefined
  const merged: TotpConfig = {
    secret: parsed?.secret || config.secret,
    issuer: config.issuer ?? parsed?.issuer,
    account: config.account ?? parsed?.account,
    algorithm: config.algorithm ?? parsed?.algorithm ?? 'SHA-1',
    digits: config.digits ?? parsed?.digits ?? 6,
    period: config.period ?? parsed?.period ?? 30,
  }
  if (merged.digits !== 6 && merged.digits !== 8) throw new Error('TOTP digits must be 6 or 8')
  if (!Number.isInteger(merged.period) || merged.period < 1) throw new Error('TOTP period must be a positive integer')
  decodeBase32(merged.secret)
  return merged
}

export async function generateTotp(config: TotpConfig, timestamp = Date.now()): Promise<TotpResult> {
  const normalized = normalizeTotpConfig(config)
  const keyBytes = decodeBase32(normalized.secret)
  const counter = Math.floor(timestamp / 1000 / normalized.period)
  const message = new Uint8Array(8)
  let value = counter
  for (let index = 7; index >= 0; index -= 1) {
    message[index] = value & 0xff
    value = Math.floor(value / 256)
  }
  if (!globalThis.crypto?.subtle) throw new Error('Web Crypto is unavailable in this browser')
  const key = await crypto.subtle.importKey('raw', keyBytes, { name: 'HMAC', hash: normalized.algorithm }, false, ['sign'])
  const digest = new Uint8Array(await crypto.subtle.sign('HMAC', key, message))
  const offset = digest[digest.length - 1] & 0x0f
  const binary = ((digest[offset] & 0x7f) << 24) | (digest[offset + 1] << 16) | (digest[offset + 2] << 8) | digest[offset + 3]
  const code = String(binary % (10 ** normalized.digits)).padStart(normalized.digits, '0')
  const elapsed = Math.floor(timestamp / 1000) % normalized.period
  return { code, counter, remainingSeconds: normalized.period - elapsed, period: normalized.period, digits: normalized.digits, algorithm: normalized.algorithm }
}

export interface RandomStringOptions {
  readonly length: number
  readonly uppercase?: boolean
  readonly lowercase?: boolean
  readonly numbers?: boolean
  readonly symbols?: boolean
  readonly customCharacters?: string
  readonly excludeCharacters?: string
}

const RANDOM_UPPERCASE = 'ABCDEFGHJKLMNPQRSTUVWXYZ'
const RANDOM_LOWERCASE = 'abcdefghijkmnopqrstuvwxyz'
const RANDOM_NUMBERS = '23456789'
const RANDOM_SYMBOLS = '!@#$%^&*()-_=+[]{}:,.?'

function randomAlphabet(options: RandomStringOptions): string[] {
  let characters = options.customCharacters || ''
  if (!options.customCharacters) {
    if (options.uppercase) characters += RANDOM_UPPERCASE
    if (options.lowercase) characters += RANDOM_LOWERCASE
    if (options.numbers) characters += RANDOM_NUMBERS
    if (options.symbols) characters += RANDOM_SYMBOLS
  }
  return Array.from(new Set(Array.from(characters).filter((character) => !options.excludeCharacters?.includes(character))))
}

export function getRandomAlphabetSize(options: RandomStringOptions): number {
  return randomAlphabet(options).length
}

export function secureRandomBytes(length: number): Uint8Array {
  if (!Number.isSafeInteger(length) || length < 0 || length > 1_000_000) throw new Error('Random byte length is out of range')
  if (!globalThis.crypto?.getRandomValues) throw new Error('Secure random generation is unavailable in this browser')
  const result = new Uint8Array(length)
  const maxChunk = 65_536
  for (let offset = 0; offset < result.length; offset += maxChunk) {
    crypto.getRandomValues(result.subarray(offset, Math.min(offset + maxChunk, result.length)))
  }
  return result
}

export function generateRandomString(options: RandomStringOptions): string {
  if (!Number.isSafeInteger(options.length) || options.length < 1 || options.length > 100_000) throw new Error('String length must be between 1 and 100000')
  const alphabet = randomAlphabet(options)
  if (!alphabet.length) throw new Error('Select at least one character set')
  const bytesPerSample = alphabet.length <= 256 ? 1 : 4
  let random = secureRandomBytes(Math.max(options.length * bytesPerSample * 2, bytesPerSample))
  const output: string[] = []
  let offset = 0
  while (output.length < options.length) {
    if (offset + bytesPerSample > random.length) {
      random = secureRandomBytes(Math.max((options.length - output.length) * bytesPerSample * 2, bytesPerSample))
      offset = 0
    }
    let sample: number
    let limit: number
    if (bytesPerSample === 1) {
      sample = random[offset++]
      limit = 256 - (256 % alphabet.length)
    } else {
      sample = random[offset++] * 0x1000000 + random[offset++] * 0x10000 + random[offset++] * 0x100 + random[offset++]
      const range = 0x100000000
      limit = range - (range % alphabet.length)
    }
    if (sample < limit) {
      output.push(alphabet[sample % alphabet.length])
    }
  }
  return output.join('')
}

export function estimateEntropyBits(length: number, alphabetSize: number): number {
  if (length < 0 || alphabetSize < 1) return 0
  return length * Math.log2(alphabetSize)
}

export function generatePassword(options: RandomStringOptions): string {
  return generateRandomString(options)
}

export function generateUuidV4(): string {
  const bytes = secureRandomBytes(16)
  bytes[6] = (bytes[6] & 0x0f) | 0x40
  bytes[8] = (bytes[8] & 0x3f) | 0x80
  const hex = bytesToHex(bytes)
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}

export function generateUuidV7(timestamp = Date.now()): string {
  if (!Number.isSafeInteger(timestamp) || timestamp < 0 || timestamp > 0xffffffffffff) throw new Error('Invalid UUID timestamp')
  const bytes = secureRandomBytes(16)
  let time = timestamp
  for (let index = 5; index >= 0; index -= 1) {
    bytes[index] = time & 0xff
    time = Math.floor(time / 256)
  }
  bytes[6] = (bytes[6] & 0x0f) | 0x70
  bytes[8] = (bytes[8] & 0x3f) | 0x80
  const hex = bytesToHex(bytes)
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}

export interface JsonValidationResult {
  readonly valid: boolean
  readonly value?: unknown
  readonly error?: string
  readonly position?: number
}

function jsonErrorPosition(message: string): number | undefined {
  const match = message.match(/(?:position|column)\s+(\d+)/i)
  return match ? Number(match[1]) : undefined
}

export function validateJson(input: string): JsonValidationResult {
  if (input.length > MAX_TEXT_INPUT) return { valid: false, error: `JSON input exceeds the ${MAX_TEXT_INPUT.toLocaleString()} character limit` }
  try {
    const value: unknown = JSON.parse(input)
    return { valid: true, value }
  } catch (error) {
    const message = error instanceof Error ? error.message : 'Invalid JSON'
    return { valid: false, error: message, position: jsonErrorPosition(message) }
  }
}

export function formatJson(input: string, spaces = 2): string {
  if (input.length > MAX_TEXT_INPUT) throw new Error(`JSON input exceeds the ${MAX_TEXT_INPUT.toLocaleString()} character limit`)
  const parsed = JSON.parse(input) as unknown
  return JSON.stringify(parsed, null, Math.max(0, Math.min(10, Math.floor(spaces))))
}

export function minifyJson(input: string): string {
  if (input.length > MAX_TEXT_INPUT) throw new Error(`JSON input exceeds the ${MAX_TEXT_INPUT.toLocaleString()} character limit`)
  return JSON.stringify(JSON.parse(input) as unknown)
}

export function encodeBase64Utf8(input: string, urlSafe = false): string {
  const encoded = bytesToBase64(textEncoder.encode(input))
  return urlSafe ? encoded.replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '') : encoded
}

export function decodeBase64Utf8(input: string, urlSafe = false): string {
  return textDecoder.decode(base64ToBytes(normalizeBase64(input.trim(), urlSafe)))
}

export const base64Encode = encodeBase64Utf8
export const base64Decode = decodeBase64Utf8

export function encodeUrlComponent(input: string): string {
  return encodeURIComponent(input)
}

export function decodeUrlComponent(input: string): string {
  return decodeURIComponent(input)
}

export interface TimestampResult {
  readonly milliseconds: number
  readonly seconds: number
  readonly date: Date
  readonly local: string
  readonly utc: string
  readonly iso: string
}

export function timestampToDate(value: number | string, unit: 'seconds' | 'milliseconds' | 'auto' = 'auto'): TimestampResult {
  const numeric = typeof value === 'number' ? value : Number(value.trim())
  if (!Number.isFinite(numeric)) throw new Error('Timestamp must be a finite number')
  const milliseconds = unit === 'seconds' || (unit === 'auto' && Math.abs(numeric) < 100_000_000_000) ? numeric * 1000 : numeric
  const date = new Date(milliseconds)
  if (Number.isNaN(date.getTime())) throw new Error('Timestamp is outside the supported date range')
  return { milliseconds, seconds: milliseconds / 1000, date, local: date.toString(), utc: date.toUTCString(), iso: date.toISOString() }
}

export function dateToTimestamp(value: Date | string | number): TimestampResult {
  const date = value instanceof Date ? new Date(value.getTime()) : new Date(value)
  if (Number.isNaN(date.getTime())) throw new Error('Invalid date/time')
  return { milliseconds: date.getTime(), seconds: date.getTime() / 1000, date, local: date.toString(), utc: date.toUTCString(), iso: date.toISOString() }
}

export function currentTimestamp(): TimestampResult {
  return timestampToDate(Date.now(), 'milliseconds')
}

export interface JwtDecoded {
  readonly header: Record<string, unknown>
  readonly payload: Record<string, unknown>
  readonly signature: string
  readonly claims: Readonly<Record<string, unknown>>
  readonly claimDates: Readonly<Record<string, string | undefined>>
}

function decodeBase64Json(value: string): Record<string, unknown> {
  const decoded = decodeBase64Utf8(value, true)
  const parsed: unknown = JSON.parse(decoded)
  const record = asRecord(parsed)
  if (!record) throw new Error('JWT segment must contain a JSON object')
  return record
}

export function decodeJwt(token: string): JwtDecoded {
  const parts = token.trim().split('.')
  if (parts.length !== 3) throw new Error('JWT must contain header, payload and signature segments')
  const header = decodeBase64Json(parts[0])
  const payload = decodeBase64Json(parts[1])
  const claimDates: Record<string, string | undefined> = {}
  for (const claim of ['exp', 'nbf', 'iat']) {
    const value = payload[claim]
    if (typeof value !== 'number' || !Number.isFinite(value)) {
      claimDates[claim] = undefined
      continue
    }
    const date = new Date(value * 1000)
    claimDates[claim] = Number.isNaN(date.getTime()) ? undefined : date.toISOString()
  }
  return { header, payload, signature: parts[2], claims: payload, claimDates }
}

export type HashAlgorithm = 'SHA-1' | 'SHA-256' | 'SHA-384' | 'SHA-512'

export interface DigestResult {
  readonly algorithm: HashAlgorithm
  readonly hex: string
  readonly base64: string
}

async function digestBytes(algorithm: HashAlgorithm, bytes: Uint8Array): Promise<DigestResult> {
  if (!globalThis.crypto?.subtle) throw new Error('Web Crypto is unavailable in this browser')
  const digest = new Uint8Array(await crypto.subtle.digest(algorithm, bytes))
  return { algorithm, hex: bytesToHex(digest), base64: bytesToBase64(digest) }
}

export function hashText(input: string, algorithm: HashAlgorithm = 'SHA-256'): Promise<DigestResult> {
  return digestBytes(algorithm, textEncoder.encode(input))
}

export async function hmacText(input: string, secret: string, algorithm: Exclude<HashAlgorithm, 'SHA-1'> = 'SHA-256'): Promise<DigestResult> {
  if (!globalThis.crypto?.subtle) throw new Error('Web Crypto is unavailable in this browser')
  const key = await crypto.subtle.importKey('raw', textEncoder.encode(secret), { name: 'HMAC', hash: algorithm }, false, ['sign'])
  const signature = new Uint8Array(await crypto.subtle.sign('HMAC', key, textEncoder.encode(input)))
  return { algorithm, hex: bytesToHex(signature), base64: bytesToBase64(signature) }
}

export interface HttpStatusInfo {
  readonly code: number
  readonly name: string
  readonly meaning: string
  readonly commonUse: string
}

export const HTTP_STATUS_CODES: readonly HttpStatusInfo[] = [
  { code: 100, name: 'Continue', meaning: 'Request headers received; continue sending the request body.', commonUse: 'Expect: 100-continue uploads' },
  { code: 101, name: 'Switching Protocols', meaning: 'The server is switching protocols.', commonUse: 'WebSocket upgrade' },
  { code: 200, name: 'OK', meaning: 'The request succeeded.', commonUse: 'Successful reads and writes' },
  { code: 201, name: 'Created', meaning: 'A resource was created.', commonUse: 'POST resource creation' },
  { code: 202, name: 'Accepted', meaning: 'The request was accepted for processing.', commonUse: 'Asynchronous jobs' },
  { code: 204, name: 'No Content', meaning: 'The request succeeded without a response body.', commonUse: 'DELETE or idempotent updates' },
  { code: 206, name: 'Partial Content', meaning: 'The server returned a requested range.', commonUse: 'Media and resumable downloads' },
  { code: 301, name: 'Moved Permanently', meaning: 'The resource has a permanent URL.', commonUse: 'Canonical redirects' },
  { code: 302, name: 'Found', meaning: 'The resource is temporarily available at another URL.', commonUse: 'Temporary redirects' },
  { code: 304, name: 'Not Modified', meaning: 'The cached representation remains valid.', commonUse: 'Conditional GET' },
  { code: 307, name: 'Temporary Redirect', meaning: 'Temporarily redirect without changing the method.', commonUse: 'Method-preserving redirects' },
  { code: 308, name: 'Permanent Redirect', meaning: 'Permanently redirect without changing the method.', commonUse: 'Method-preserving canonical redirects' },
  { code: 400, name: 'Bad Request', meaning: 'The request is malformed or invalid.', commonUse: 'Validation errors' },
  { code: 401, name: 'Unauthorized', meaning: 'Authentication is required or invalid.', commonUse: 'Missing/expired credentials' },
  { code: 403, name: 'Forbidden', meaning: 'The server refuses the request.', commonUse: 'Authorization failures' },
  { code: 404, name: 'Not Found', meaning: 'The resource does not exist.', commonUse: 'Unknown route or identifier' },
  { code: 405, name: 'Method Not Allowed', meaning: 'The method is not supported for this resource.', commonUse: 'Wrong HTTP verb' },
  { code: 408, name: 'Request Timeout', meaning: 'The server timed out waiting for the request.', commonUse: 'Slow clients' },
  { code: 409, name: 'Conflict', meaning: 'The request conflicts with current state.', commonUse: 'Optimistic concurrency' },
  { code: 410, name: 'Gone', meaning: 'The resource is permanently unavailable.', commonUse: 'Retired URLs' },
  { code: 413, name: 'Content Too Large', meaning: 'The request body exceeds limits.', commonUse: 'Upload limits' },
  { code: 415, name: 'Unsupported Media Type', meaning: 'The body format is not supported.', commonUse: 'Content-Type validation' },
  { code: 422, name: 'Unprocessable Content', meaning: 'The syntax is valid but semantic validation failed.', commonUse: 'API field validation' },
  { code: 429, name: 'Too Many Requests', meaning: 'The client exceeded a rate limit.', commonUse: 'Throttling' },
  { code: 500, name: 'Internal Server Error', meaning: 'The server encountered an unexpected error.', commonUse: 'Unhandled server failure' },
  { code: 501, name: 'Not Implemented', meaning: 'The server does not support the requested capability.', commonUse: 'Unsupported method' },
  { code: 502, name: 'Bad Gateway', meaning: 'An upstream server returned an invalid response.', commonUse: 'Reverse proxy upstream failure' },
  { code: 503, name: 'Service Unavailable', meaning: 'The service cannot handle the request now.', commonUse: 'Maintenance or overload' },
  { code: 504, name: 'Gateway Timeout', meaning: 'An upstream service timed out.', commonUse: 'Proxy timeout' },
]

export function searchHttpStatuses(query: string): readonly HttpStatusInfo[] {
  const normalized = query.trim().toLocaleLowerCase()
  return HTTP_STATUS_CODES.filter((item) => !normalized || String(item.code).includes(normalized) || item.name.toLocaleLowerCase().includes(normalized) || item.meaning.toLocaleLowerCase().includes(normalized))
}

export interface UserAgentInfo {
  readonly browser: string
  readonly browserVersion?: string
  readonly os: string
  readonly osVersion?: string
  readonly device: 'mobile' | 'tablet' | 'desktop' | 'bot' | 'unknown'
  readonly engine: string
  readonly architecture?: string
}

export function parseUserAgent(userAgent: string): UserAgentInfo {
  const ua = userAgent.trim()
  const edge = /Edg\/([\d.]+)/.exec(ua)
  const opera = /OPR\/([\d.]+)/.exec(ua)
  const chrome = /Chrome\/([\d.]+)/.exec(ua)
  const firefox = /Firefox\/([\d.]+)/.exec(ua)
  const safari = /Version\/([\d.]+).*Safari\//.exec(ua)
  const curl = /curl\/([\d.]+)/i.exec(ua)
  const browser: [string, string?] = edge ? ['Edge', edge[1]] : opera ? ['Opera', opera[1]] : chrome && !/Chromium/.test(ua) ? ['Chrome', chrome[1]] : firefox ? ['Firefox', firefox[1]] : safari ? ['Safari', safari[1]] : curl ? ['curl', curl[1]] : ['Unknown']
  const windows = /Windows NT ([\d.]+)/.exec(ua)
  const android = /Android ([\d.]+)/.exec(ua)
  const ios = /(iPhone|CPU OS|iPad).*?([\d_]+)/.exec(ua)
  const mac = /Mac OS X ([\d_]+)/.exec(ua)
  const os: [string, string?] = windows ? ['Windows', windows[1]] : android ? ['Android', android[1]] : ios ? ['iOS', ios[2].replace(/_/g, '.')] : mac ? ['macOS', mac[1].replace(/_/g, '.')] : /Linux/.test(ua) ? ['Linux'] : ['Unknown']
  const mobile = /Mobile|Android|iPhone|Windows Phone/i.test(ua)
  const tablet = /iPad|Tablet|Android(?!.*Mobile)/i.test(ua)
  const bot = /bot|crawler|spider|slurp|headless/i.test(ua)
  const engine = /AppleWebKit\//.test(ua) ? 'WebKit' : /Gecko\//.test(ua) ? 'Gecko' : /Trident\//.test(ua) ? 'Trident' : /Presto\//.test(ua) ? 'Presto' : 'Unknown'
  const architecture = /(?:x86_64|Win64|x64|amd64)/i.test(ua) ? 'x86_64' : /(?:aarch64|arm64)/i.test(ua) ? 'arm64' : /(?:i[3-6]86|x86)/i.test(ua) ? 'x86' : undefined
  return { browser: browser[0], browserVersion: browser[1], os: os[0], osVersion: os[1], device: bot ? 'bot' : tablet ? 'tablet' : mobile ? 'mobile' : ua ? 'desktop' : 'unknown', engine, architecture }
}

export interface CidrResult {
  readonly version: 4 | 6
  readonly input: string
  readonly networkAddress: string
  readonly broadcastAddress?: string
  readonly subnetMask?: string
  readonly wildcardMask?: string
  readonly prefix: number
  readonly firstHost?: string
  readonly lastHost?: string
  readonly totalAddresses: string
  readonly usableHosts?: string
  readonly addressRange: readonly [string, string]
}

export function calculateCidr(input: string): CidrResult {
  const [address, prefixValue, ...rest] = input.trim().split('/')
  if (rest.length || prefixValue === undefined) throw new Error('CIDR must use address/prefix notation')
  const parsed = parseIp(address)
  if (!parsed) throw new Error('Invalid IP address')
  const prefix = Number(prefixValue)
  if (!Number.isInteger(prefix) || prefix < 0 || prefix > parsed.bits) throw new Error(`Prefix must be between 0 and ${parsed.bits}`)
  const mask = prefixMask(parsed.bits, prefix)
  const network = parsed.number & mask
  const hostMask = (1n << BigInt(parsed.bits)) - 1n ^ mask
  const last = network | hostMask
  const total = 1n << BigInt(parsed.bits - prefix)
  if (parsed.version === 4) {
    const subnetMask = bigintToIPv4(mask)
    const wildcardMask = bigintToIPv4(hostMask)
    const first = prefix <= 30 ? network + 1n : network
    const lastHost = prefix <= 30 ? last - 1n : last
    const usable = prefix <= 30 ? total - 2n : prefix === 31 ? 2n : 1n
    return { version: 4, input, networkAddress: bigintToIPv4(network), broadcastAddress: bigintToIPv4(last), subnetMask, wildcardMask, prefix, firstHost: bigintToIPv4(first), lastHost: bigintToIPv4(lastHost), totalAddresses: total.toString(), usableHosts: usable.toString(), addressRange: [bigintToIPv4(network), bigintToIPv4(last)] }
  }
  return { version: 6, input, networkAddress: bigintToIPv6(network), prefix, totalAddresses: total.toString(), addressRange: [bigintToIPv6(network), bigintToIPv6(last)] }
}

export type HttpMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE' | 'HEAD' | 'OPTIONS'
export interface ApiRequestSpec {
  readonly method: HttpMethod
  readonly url: string
  readonly query?: Readonly<Record<string, string>>
  readonly headers?: Readonly<Record<string, string>>
  readonly body?: string
  readonly bodyType?: 'json' | 'form' | 'text'
  readonly authorization?: { readonly type: 'bearer' | 'basic' | 'api-key'; readonly value: string; readonly username?: string; readonly headerName?: string }
}

/** Validate the URL before emitting snippets. No request is made. */
export function validateApiRequestUrl(value: string): string {
  const trimmed = value.trim()
  if (!trimmed) throw new Error('API URL is required')
  let parsed: URL
  try {
    parsed = new URL(trimmed)
  } catch {
    throw new Error('API URL must be an absolute HTTP(S) URL')
  }
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    throw new Error('API URL must use HTTP or HTTPS')
  }
  return trimmed
}

function requestUrl(spec: ApiRequestSpec): string {
  const base = validateApiRequestUrl(spec.url)
  const query = Object.entries(spec.query || {}).filter(([, value]) => value !== '').map(([key, value]) => `${encodeURIComponent(key)}=${encodeURIComponent(value)}`)
  if (!query.length) return base
  const parsed = new URL(base)
  for (const [key, value] of Object.entries(spec.query || {})) {
    if (value !== '') parsed.searchParams.append(key, value)
  }
  return parsed.toString()
}

function authHeaders(spec: ApiRequestSpec): Record<string, string> {
  const headers = { ...(spec.headers || {}) }
  const auth = spec.authorization
  if (!auth) return headers
  if (auth.type === 'bearer') headers.Authorization = `Bearer ${auth.value}`
  else if (auth.type === 'basic') headers.Authorization = `Basic ${encodeBase64Utf8(`${auth.username || ''}:${auth.value}`)}`
  else headers[auth.headerName || 'X-API-Key'] = auth.value
  return headers
}

function parseBasicAuthorization(value: string): ApiRequestSpec['authorization'] | undefined {
  let decoded: string
  try {
    decoded = decodeBase64Utf8(value.trim())
  } catch {
    throw new Error('Invalid Basic authorization value')
  }
  const separator = decoded.indexOf(':')
  if (separator < 0) throw new Error('Basic authorization must contain username and password')
  return {
    type: 'basic',
    username: decoded.slice(0, separator),
    value: decoded.slice(separator + 1),
  }
}

export interface ApiSnippets {
  readonly curl: string
  readonly fetch: string
  readonly python: string
  readonly go: string
}

export function generateApiSnippets(spec: ApiRequestSpec): ApiSnippets {
  const url = requestUrl(spec)
  const headers = authHeaders(spec)
  let body = spec.body
  if (spec.bodyType === 'json' && spec.body) {
    body = (() => { try { return JSON.stringify(JSON.parse(spec.body as string) as unknown) } catch { return spec.body } })()
    if (!Object.keys(headers).some((key) => key.toLowerCase() === 'content-type')) {
      headers['Content-Type'] = 'application/json'
    }
  } else if (spec.bodyType === 'form' && spec.body && !Object.keys(headers).some((key) => key.toLowerCase() === 'content-type')) {
    headers['Content-Type'] = 'application/x-www-form-urlencoded'
  }
  const curlParts = [`curl -X ${spec.method}`, quoteShell(url)]
  for (const [key, value] of Object.entries(headers)) curlParts.push(`-H ${quoteShell(`${key}: ${value}`)}`)
  if (body !== undefined && !['GET', 'HEAD'].includes(spec.method)) curlParts.push(`--data ${quoteShell(body)}`)
  const fetchOptions: string[] = [`method: ${quoteJs(spec.method)}`]
  if (Object.keys(headers).length) fetchOptions.push(`headers: ${JSON.stringify(headers, null, 2)}`)
  if (body !== undefined && !['GET', 'HEAD'].includes(spec.method)) fetchOptions.push(`body: ${quoteJs(body)}`)
  const fetch = `const response = await fetch(${quoteJs(url)}, {\n  ${fetchOptions.join(',\n  ')}\n})`
  const pythonHeaders = JSON.stringify(headers, null, 2)
  const pythonBody = body === undefined ? '' : `, data=${quotePython(body)}`
  const python = `import requests\n\nresponse = requests.request(${quotePython(spec.method)}, ${quotePython(url)}, headers=${pythonHeaders}${pythonBody})`
  const goHeaders = Object.entries(headers).map(([key, value]) => `request.Header.Set(${quoteGo(key)}, ${quoteGo(value)})`).join('\n')
  const goBody = body === undefined ? 'nil' : `strings.NewReader(${quoteGo(body)})`
  const go = `request, err := http.NewRequest(${quoteGo(spec.method)}, ${quoteGo(url)}, ${goBody})\nif err != nil { panic(err) }\n${goHeaders}\nresponse, err := http.DefaultClient.Do(request)`
  return { curl: curlParts.join(' '), fetch, python, go }
}

export interface CurlRequest extends ApiRequestSpec {
  readonly form?: Readonly<Record<string, string>>
}

function shellTokens(command: string): string[] {
  const tokens: string[] = []
  let current = ''
  let quote: 'single' | 'double' | null = null
  let escaping = false
  for (const character of command.trim()) {
    if (escaping) { current += character; escaping = false; continue }
    if (character === '\\' && quote !== 'single') { escaping = true; continue }
    if (character === "'" && quote !== 'double') { quote = quote === 'single' ? null : 'single'; continue }
    if (character === '"' && quote !== 'single') { quote = quote === 'double' ? null : 'double'; continue }
    if (/\s/.test(character) && !quote) { if (current) { tokens.push(current); current = '' } continue }
    current += character
  }
  if (escaping || quote) throw new Error('Unterminated quote or escape in cURL command')
  if (current) tokens.push(current)
  return tokens
}

export function parseCurl(command: string): CurlRequest {
  const tokens = shellTokens(command)
  if (tokens[0]?.toLowerCase() !== 'curl') throw new Error('Input must be a cURL command')
  let method: HttpMethod = 'GET'
  let url = ''
  const headers: Record<string, string> = {}
  const form: Record<string, string> = {}
  let body: string | undefined
  let bodyType: ApiRequestSpec['bodyType']
  let authorization: ApiRequestSpec['authorization']
  for (let index = 1; index < tokens.length; index += 1) {
    const token = tokens[index]
    if (token === '-X' || token === '--request') { const candidate = tokens[++index]?.toUpperCase() as HttpMethod; if (!candidate || !['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS'].includes(candidate)) throw new Error('Unsupported cURL method'); method = candidate; continue }
    if (token === '-H' || token === '--header') { const value = tokens[++index]; const separator = value?.indexOf(':'); if (!value || separator < 1) throw new Error('Invalid cURL header'); const key = value.slice(0, separator).trim(); const headerValue = value.slice(separator + 1).trim(); headers[key] = headerValue; if (key.toLowerCase() === 'authorization') { const bearer = /^Bearer\s+(.+)$/i.exec(headerValue); const basic = /^Basic\s+(.+)$/i.exec(headerValue); if (bearer) authorization = { type: 'bearer', value: bearer[1] }; else if (basic) authorization = parseBasicAuthorization(basic[1]) } continue }
    if (token === '-u' || token === '--user') { const credentials = tokens[++index]; if (!credentials || !credentials.includes(':')) throw new Error('Basic authentication credentials must use username:password'); const separator = credentials.indexOf(':'); authorization = { type: 'basic', username: credentials.slice(0, separator), value: credentials.slice(separator + 1) }; continue }
    if (token === '-d' || token === '--data' || token === '--data-raw' || token === '--data-binary') { body = tokens[++index]; bodyType = 'text'; if (method === 'GET') method = 'POST'; continue }
    if (token === '--data-urlencode') { const value = tokens[++index]; const separator = value?.indexOf('='); if (!value || separator < 1) throw new Error('Invalid --data-urlencode value'); form[value.slice(0, separator)] = decodeURIComponent(value.slice(separator + 1)); bodyType = 'form'; if (method === 'GET') method = 'POST'; continue }
    if (token === '-F' || token === '--form') { const value = tokens[++index]; const separator = value?.indexOf('='); if (!value || separator < 1) throw new Error('Invalid form value'); form[value.slice(0, separator)] = value.slice(separator + 1); bodyType = 'form'; if (method === 'GET') method = 'POST'; continue }
    if (token === '-G' || token === '--get') { method = 'GET'; continue }
    if (token.startsWith('-')) continue
    if (!url) url = token
  }
  if (!url) throw new Error('cURL command is missing a URL')
  if (bodyType === 'form') body = new URLSearchParams(form).toString()
  else if (bodyType === 'text' && body) {
    const contentType = Object.entries(headers).find(([key]) => key.toLowerCase() === 'content-type')?.[1].toLowerCase() || ''
    if (contentType.includes('application/json')) bodyType = 'json'
    else if (contentType.includes('application/x-www-form-urlencoded')) {
      bodyType = 'form'
      for (const [key, value] of new URLSearchParams(body)) form[key] = value
      body = new URLSearchParams(form).toString()
    }
  }
  return { method, url, headers, body, bodyType, authorization, form: Object.keys(form).length ? form : undefined }
}

export function curlToSnippets(command: string): ApiSnippets {
  return generateApiSnippets(parseCurl(command))
}

export interface StructOptions { readonly rootName?: string }

interface StructField {
  values: unknown[]
  present: number
}

interface StructDefinition {
  name: string
  total: number
  fields: Map<string, StructField>
}

function collectStructObjects(value: unknown, name: string, definitions: Map<string, StructDefinition>): void {
  if (Array.isArray(value)) {
    for (const item of value) collectStructObjects(item, name, definitions)
    return
  }
  const record = asRecord(value)
  if (!record) return
  let definition = definitions.get(name)
  if (!definition) {
    definition = { name, total: 0, fields: new Map() }
    definitions.set(name, definition)
  }
  definition.total += 1
  for (const [key, child] of Object.entries(record)) {
    const field = definition.fields.get(key) || { values: [], present: 0 }
    field.values.push(child)
    field.present += 1
    definition.fields.set(key, field)
    const nestedObject = asRecord(child)
    if (nestedObject) collectStructObjects(nestedObject, `${name}${titleCase(identifier(key, 'Value'))}`, definitions)
    else if (Array.isArray(child)) collectStructObjects(child, `${name}${titleCase(identifier(key, 'Item'))}`, definitions)
  }
}

function uniqueStrings(values: readonly string[]): string[] {
  return Array.from(new Set(values))
}

function nonNullValues(values: readonly unknown[]): unknown[] {
  return values.filter((value) => value !== null && value !== undefined)
}

function arrayType(type: string): string {
  return type.includes(' | ') ? `(${type})[]` : `${type}[]`
}

function tsTypeForValues(values: readonly unknown[], parentName: string, key: string): string {
  const nullable = values.some((value) => value === null || value === undefined)
  const present = nonNullValues(values)
  if (!present.length) return 'unknown'
  const types: string[] = []
  const arrays = present.filter(Array.isArray) as unknown[][]
  if (arrays.length) types.push(arrayType(tsTypeForValues(arrays.flat(), parentName, key)))
  const records = present.filter((value) => asRecord(value))
  if (records.length) types.push(`${parentName}${titleCase(identifier(key, 'Value'))}`)
  const scalars = present.filter((value) => !Array.isArray(value) && !asRecord(value))
  for (const value of scalars) {
    if (typeof value === 'string') types.push('string')
    else if (typeof value === 'number') types.push('number')
    else if (typeof value === 'boolean') types.push('boolean')
    else types.push('unknown')
  }
  const result = uniqueStrings(types).join(' | ') || 'unknown'
  return nullable ? `${result} | null` : result
}

function goTypeForValues(values: readonly unknown[], parentName: string, key: string, optional = false): string {
  const nullable = optional || values.some((value) => value === null || value === undefined)
  const present = nonNullValues(values)
  if (!present.length) return 'any'
  const arrays = present.filter(Array.isArray) as unknown[][]
  if (arrays.length) {
    const itemType = goTypeForValues(arrays.flat(), parentName, key)
    return `[]${itemType}`
  }
  const records = present.filter((value) => asRecord(value))
  if (records.length) return nullable ? `*${parentName}${titleCase(identifier(key, 'Value'))}` : `${parentName}${titleCase(identifier(key, 'Value'))}`
  const scalarTypes = uniqueStrings(present.map((value) => {
    if (typeof value === 'string') return 'string'
    if (typeof value === 'boolean') return 'bool'
    if (typeof value === 'number') return Number.isInteger(value) ? 'int64' : 'float64'
    return 'any'
  }))
  const scalarType = scalarTypes.length === 1 ? scalarTypes[0] : 'any'
  return nullable && scalarType !== 'any' ? `*${scalarType}` : scalarType
}

function pythonTypeForValues(values: readonly unknown[], parentName: string, key: string, optional = false): string {
  const nullable = optional || values.some((value) => value === null || value === undefined)
  const present = nonNullValues(values)
  if (!present.length) return 'Any'
  const arrays = present.filter(Array.isArray) as unknown[][]
  if (arrays.length) return `list[${pythonTypeForValues(arrays.flat(), parentName, key)}]`
  const records = present.filter((value) => asRecord(value))
  if (records.length) {
    const type = `${parentName}${titleCase(identifier(key, 'Value'))}`
    return nullable ? `Optional[${type}]` : type
  }
  const scalarTypes = uniqueStrings(present.map((value) => {
    if (typeof value === 'string') return 'str'
    if (typeof value === 'boolean') return 'bool'
    if (typeof value === 'number') return Number.isInteger(value) ? 'int' : 'float'
    return 'Any'
  }))
  const scalarType = scalarTypes.length === 1 ? scalarTypes[0] : 'Any'
  return nullable && scalarType !== 'Any' ? `Optional[${scalarType}]` : scalarType
}

function fieldIdentifiers(definition: StructDefinition): Map<string, string> {
  const used = new Set<string>()
  const result = new Map<string, string>()
  for (const key of definition.fields.keys()) {
    const base = identifier(key, 'value')
    let candidate = base
    let suffix = 2
    while (used.has(candidate)) candidate = `${base}${suffix++}`
    used.add(candidate)
    result.set(key, candidate)
  }
  return result
}

function pythonFieldIdentifiers(definition: StructDefinition): Map<string, string> {
  const used = new Set<string>()
  const result = new Map<string, string>()
  for (const key of definition.fields.keys()) {
    const base = pythonIdentifier(key, 'value')
    let candidate = base
    let suffix = 2
    while (used.has(candidate)) candidate = `${base}${suffix++}`
    used.add(candidate)
    result.set(key, candidate)
  }
  return result
}

function collectStructDefinitions(value: unknown, rootName: string): Map<string, StructDefinition> {
  const definitions = new Map<string, StructDefinition>()
  collectStructObjects(value, rootName, definitions)
  return definitions
}

export function jsonToTypeScript(input: string, options: StructOptions = {}): string {
  const value = JSON.parse(input) as unknown
  const rootName = identifier(options.rootName || 'Root', 'Root')
  const definitionRootName = Array.isArray(value) ? `${rootName}Item` : rootName
  const definitions = collectStructDefinitions(value, definitionRootName)
  if (!definitions.size) return `export type ${rootName} = ${tsTypeForValues([value], rootName, rootName)}`
  const declarations = Array.from(definitions.values()).map((definition) => `export interface ${definition.name} {\n${Array.from(definition.fields.entries()).map(([key, field]) => `  ${JSON.stringify(key)}${field.present < definition.total ? '?' : ''}: ${tsTypeForValues(field.values, definition.name, key)};`).join('\n')}\n}`).join('\n\n')
  return Array.isArray(value) ? `${declarations}\n\nexport type ${rootName} = ${definitionRootName}[]` : declarations
}

export function jsonToGo(input: string, options: StructOptions = {}): string {
  const value = JSON.parse(input) as unknown
  const rootName = titleCase(identifier(options.rootName || 'Root', 'Root'))
  const definitionRootName = Array.isArray(value) ? `${rootName}Item` : rootName
  const definitions = collectStructDefinitions(value, definitionRootName)
  if (!definitions.size) return `type ${rootName} ${goTypeForValues([value], rootName, rootName)}`
  const declarations = Array.from(definitions.values()).map((definition) => {
    const names = fieldIdentifiers(definition)
    const fields = Array.from(definition.fields.entries()).map(([key, field]) => `\t${titleCase(names.get(key) || 'Field')} ${goTypeForValues(field.values, definition.name, key, field.present < definition.total)} \`json:${JSON.stringify(key)}\``)
    return `type ${definition.name} struct {\n${fields.join('\n')}\n}`
  }).join('\n\n')
  return Array.isArray(value) ? `${declarations}\n\ntype ${rootName} []${definitionRootName}` : declarations
}

export function jsonToPython(input: string, options: StructOptions = {}): string {
  const value = JSON.parse(input) as unknown
  const rootName = titleCase(pythonIdentifier(options.rootName || 'Root', 'Root'))
  const definitionRootName = Array.isArray(value) ? `${rootName}Item` : rootName
  const definitions = collectStructDefinitions(value, definitionRootName)
  if (!definitions.size) return `from typing import Any\n\n${rootName} = ${pythonTypeForValues([value], rootName, rootName)}`
  const declarations = Array.from(definitions.values()).map((definition) => {
    const names = pythonFieldIdentifiers(definition)
    // Dataclasses require fields with defaults to follow all required fields.
    // Heterogeneous JSON arrays can otherwise put an optional field before a
    // required one and generate code that fails at class definition time.
    const entries = Array.from(definition.fields.entries()).sort(([, left], [, right]) => {
      const leftHasDefault = left.present < definition.total || left.values.some((value) => value === null)
      const rightHasDefault = right.present < definition.total || right.values.some((value) => value === null)
      return Number(leftHasDefault) - Number(rightHasDefault)
    })
    const fields = entries.map(([key, field]) => `    ${names.get(key) || 'value'}: ${pythonTypeForValues(field.values, definition.name, key, field.present < definition.total)}${field.present < definition.total || field.values.some((value) => value === null) ? ' = None' : ''}`)
    return `@dataclass\nclass ${definition.name}:\n${fields.join('\n') || '    pass'}`
  }).join('\n\n')
  // Definitions are emitted in discovery order, so a parent may refer to a
  // nested dataclass declared later. Postponed annotations keep the generated
  // module importable without forcing an arbitrary declaration sort.
  return `from __future__ import annotations\n\nfrom dataclasses import dataclass\nfrom typing import Any, Optional\n\n${declarations}${Array.isArray(value) ? `\n\n${rootName} = list[${definitionRootName}]` : ''}`
}

export type CronFieldName = 'minute' | 'hour' | 'dayOfMonth' | 'month' | 'dayOfWeek'
export interface CronExpression {
  readonly minute: readonly number[]
  readonly hour: readonly number[]
  readonly dayOfMonth: readonly number[]
  readonly month: readonly number[]
  readonly dayOfWeek: readonly number[]
  readonly expression: string
  readonly description: string
}

const CRON_NAMES: Readonly<Record<CronFieldName, readonly string[]>> = { minute: [], hour: [], dayOfMonth: [], month: ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'], dayOfWeek: ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'] }

export function parseCron(expression: string): CronExpression {
  const fields = expression.trim().split(/\s+/)
  if (fields.length !== 5) throw new Error('Cron must contain exactly 5 fields (minute hour day-of-month month day-of-week)')
  const parsedExpression = CronExpressionParser.parse(`0 ${fields.join(' ')}`, { strict: true })
  const serialized = parsedExpression.fields.serialize()
  const parsed = [serialized.minute.values, serialized.hour.values, serialized.dayOfMonth.values, serialized.month.values, serialized.dayOfWeek.values]
    .map((values) => values.filter((value): value is number => typeof value === 'number'))
  const [minute, hour, dayOfMonth, month, dayOfWeek] = parsed
  const description = `At ${minute.length === 60 ? 'every minute' : minute.length === 1 ? `minute ${minute[0]}` : `${minute.length} selected minutes`} ${hour.length === 24 ? 'of every hour' : hour.length === 1 ? `at hour ${hour[0]}` : `at ${hour.length} selected hours`}, on ${dayOfMonth.length === 31 ? 'every day' : `${dayOfMonth.length} day(s) of month`}, in ${month.length === 12 ? 'every month' : month.map((number) => CRON_NAMES.month[number - 1] || number).join(', ')}, weekday ${dayOfWeek.length >= 7 ? 'any' : dayOfWeek.map((number) => CRON_NAMES.dayOfWeek[number % 7]).join(', ')}`
  return { minute, hour, dayOfMonth, month, dayOfWeek, expression: fields.join(' '), description }
}

export function nextCronRuns(expression: string | CronExpression, count = 5, from = new Date()): Date[] {
  if (!Number.isInteger(count) || count < 1 || count > 100) throw new Error('Run count must be between 1 and 100')
  const cronExpression = typeof expression === 'string' ? expression : expression.expression
  const fields = cronExpression.trim().split(/\s+/)
  if (fields.length !== 5) throw new Error('Cron must contain exactly 5 fields (minute hour day-of-month month day-of-week)')
  const parser = CronExpressionParser.parse(`0 ${fields.join(' ')}`, { currentDate: from, strict: true })
  return parser.take(count).map((cronDate) => cronDate.toDate())
}

export interface DiffLine { readonly type: 'added' | 'removed' | 'unchanged'; readonly value: string; readonly oldLine?: number; readonly newLine?: number }

export function diffLines(original: string, modified: string, options: { readonly ignoreCase?: boolean; readonly ignoreWhitespace?: boolean } = {}): DiffLine[] {
  if (original.length > MAX_DIFF_INPUT || modified.length > MAX_DIFF_INPUT) throw new Error(`Diff input exceeds the ${MAX_DIFF_INPUT.toLocaleString()} character limit`)
  const left = original.split(/\r?\n/)
  const right = modified.split(/\r?\n/)
  const normalize = (line: string): string => { let value = options.ignoreWhitespace ? line.replace(/\s+/g, ' ').trim() : line; if (options.ignoreCase) value = value.toLocaleLowerCase(); return value }
  const rows: DiffLine[] = []
  let oldIndex = 0
  let newIndex = 0
  while (oldIndex < left.length || newIndex < right.length) {
    if (oldIndex < left.length && newIndex < right.length && normalize(left[oldIndex]) === normalize(right[newIndex])) rows.push({ type: 'unchanged', value: right[newIndex], oldLine: oldIndex + 1, newLine: newIndex + 1 }), oldIndex += 1, newIndex += 1
    else if (newIndex + 1 < right.length && oldIndex < left.length && normalize(left[oldIndex]) === normalize(right[newIndex + 1])) rows.push({ type: 'added', value: right[newIndex], newLine: newIndex + 1 }), newIndex += 1
    else if (oldIndex + 1 < left.length && newIndex < right.length && normalize(left[oldIndex + 1]) === normalize(right[newIndex])) rows.push({ type: 'removed', value: left[oldIndex], oldLine: oldIndex + 1 }), oldIndex += 1
    else { if (oldIndex < left.length) rows.push({ type: 'removed', value: left[oldIndex], oldLine: oldIndex + 1 }), oldIndex += 1; if (newIndex < right.length) rows.push({ type: 'added', value: right[newIndex], newLine: newIndex + 1 }), newIndex += 1 }
  }
  return rows
}

export interface CharacterDiff { readonly type: 'added' | 'removed' | 'unchanged'; readonly value: string }

export function diffCharacters(original: string, modified: string): CharacterDiff[] {
  if (original.length > MAX_DIFF_INPUT || modified.length > MAX_DIFF_INPUT) throw new Error(`Diff input exceeds the ${MAX_DIFF_INPUT.toLocaleString()} character limit`)
  const left = Array.from(original)
  const right = Array.from(modified)
  const rows: CharacterDiff[] = []
  let leftIndex = 0
  let rightIndex = 0
  while (leftIndex < left.length || rightIndex < right.length) {
    if (left[leftIndex] === right[rightIndex]) rows.push({ type: 'unchanged', value: left[leftIndex] }), leftIndex += 1, rightIndex += 1
    else if (rightIndex + 1 < right.length && left[leftIndex] === right[rightIndex + 1]) rows.push({ type: 'added', value: right[rightIndex++] })
    else { if (leftIndex < left.length) rows.push({ type: 'removed', value: left[leftIndex++] }); if (rightIndex < right.length) rows.push({ type: 'added', value: right[rightIndex++] }) }
  }
  return rows
}

export function markdownToSafeHtml(markdown: string): string {
  if (markdown.length > MAX_TEXT_INPUT) throw new Error(`Markdown input exceeds the ${MAX_TEXT_INPUT.toLocaleString()} character limit`)
  const rendered = marked.parse(markdown, { gfm: true, breaks: true })
  if (typeof rendered !== 'string') throw new Error('Markdown renderer returned an asynchronous result')
  const sanitized = DOMPurify.sanitize(rendered, { USE_PROFILES: { html: true }, FORBID_TAGS: ['style', 'script', 'iframe', 'object', 'embed'], FORBID_ATTR: ['style', 'onerror', 'onclick', 'onload'] })
  return sanitized.replace(/<a\b(?![^>]*\brel=)/gi, '<a rel="noopener noreferrer"')
}

export function sanitizeMarkdownHtml(html: string): string {
  const sanitized = DOMPurify.sanitize(html, { USE_PROFILES: { html: true }, FORBID_TAGS: ['style', 'script', 'iframe', 'object', 'embed'], FORBID_ATTR: ['style', 'onerror', 'onclick', 'onload'] })
  return sanitized.replace(/<a\b(?![^>]*\brel=)/gi, '<a rel="noopener noreferrer"')
}

export interface QrOptions { readonly size?: number; readonly errorCorrection?: 'L' | 'M' | 'Q' | 'H' }

export function validateQrInput(input: string, options: QrOptions = {}): { readonly value: string; readonly size: number; readonly errorCorrection: NonNullable<QrOptions['errorCorrection']> } {
  if (!input.trim()) throw new Error('QR content cannot be empty')
  const size = options.size ?? 256
  if (!Number.isInteger(size) || size < 64 || size > 2048) throw new Error('QR size must be between 64 and 2048 pixels')
  const errorCorrection = options.errorCorrection ?? 'M'
  return { value: input, size, errorCorrection }
}
