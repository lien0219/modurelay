import { describe, expect, it } from 'vitest'
import {
  ACTIVITY_PROBABILITY_SCALE,
  isActivityLimit,
  isActivityMoney,
  probabilityToPPM,
  probabilityTotalPPM,
} from '../validation'

describe('activity financial validation', () => {
  it('accepts at most eight decimal places and bounded amounts', () => {
    expect(isActivityMoney('0.00000001', false)).toBe(true)
    expect(isActivityMoney('0', true)).toBe(true)
    expect(isActivityMoney('0', false)).toBe(false)
    expect(isActivityMoney('0.000000001', true)).toBe(false)
    expect(isActivityMoney('1000001', true)).toBe(false)
    expect(isActivityMoney('1e2', true)).toBe(false)
  })

  it('requires bounded integer limits', () => {
    expect(isActivityLimit(0, true)).toBe(true)
    expect(isActivityLimit(1, false)).toBe(true)
    expect(isActivityLimit(1.5, false)).toBe(false)
    expect(isActivityLimit(1_000_001, true)).toBe(false)
  })

  it('converts displayed percentages to exact PPM values', () => {
    expect(probabilityToPPM(12.3456)).toBe(123_456)
    expect(probabilityToPPM(0)).toBeNull()
    expect(probabilityToPPM(12.34567)).toBeNull()
    expect(probabilityTotalPPM([{ probability: 12.3456 }, { probability: 87.6544 }])).toBe(ACTIVITY_PROBABILITY_SCALE)
  })
})

