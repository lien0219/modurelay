export const ACTIVITY_LIMIT_MAX = 1_000_000
export const ACTIVITY_PROBABILITY_SCALE = 1_000_000

const MONEY_PATTERN = /^\d+(?:\.\d{1,8})?$/

export function isActivityMoney(value: string, allowZero: boolean): boolean {
  const normalized = String(value).trim()
  if (!MONEY_PATTERN.test(normalized)) return false
  const amount = Number(normalized)
  if (!Number.isFinite(amount) || amount > ACTIVITY_LIMIT_MAX) return false
  return allowZero ? amount >= 0 : amount > 0
}

export function isActivityLimit(value: number, allowZero: boolean): boolean {
  return Number.isInteger(value) && value <= ACTIVITY_LIMIT_MAX && (allowZero ? value >= 0 : value > 0)
}

export function probabilityToPPM(value: number): number | null {
  if (!Number.isFinite(value)) return null
  const scaled = value * 10_000
  const rounded = Math.round(scaled)
  if (Math.abs(scaled - rounded) > 1e-7 || rounded <= 0 || rounded > ACTIVITY_PROBABILITY_SCALE) return null
  return rounded
}

export function probabilityTotalPPM(prizes: Array<{ probability: number }>): number | null {
  let total = 0
  for (const prize of prizes) {
    const ppm = probabilityToPPM(Number(prize.probability))
    if (ppm === null) return null
    total += ppm
  }
  return total
}

