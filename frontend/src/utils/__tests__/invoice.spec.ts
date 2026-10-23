import { describe, expect, it } from 'vitest'
import { formatInvoiceAmount, parseInvoiceAmount } from '../invoice'

describe('invoice CNY amounts', () => {
  it.each([['300', 30000], ['300.01', 30001], ['300.1', 30010], [' 1000.29 ', 100029]])('parses %s into integer cents', (value, cents) => {
    expect(parseInvoiceAmount(value)).toBe(cents)
  })
  it.each(['', '-300', '300.001', '3e2', 'NaN', 'Infinity', '300,00', '9007199254740992'])('rejects malformed or unsafe amount %s', value => {
    expect(parseInvoiceAmount(value)).toBeNull()
  })
  it('formats amounts in CNY rather than account-balance USD', () => {
    expect(formatInvoiceAmount(30001)).toContain('300.01')
    expect(formatInvoiceAmount(30001)).toMatch(/[¥￥]/)
    expect(formatInvoiceAmount(30001)).not.toContain('$')
  })
})
