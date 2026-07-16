import {
  onUnmounted,
  ref,
  watch,
  type MaybeRefOrGetter,
  type Ref,
  toValue
} from 'vue'
import gsap from 'gsap'

export type GsapCounterFormat =
  | 'integer'
  | 'decimal'
  | 'currency'
  | 'percent'
  | 'compact'
  | ((value: number) => string)

export interface UseGsapCounterOptions {
  /** Tween duration in seconds (0.35–0.65). Default 0.48 */
  duration?: number
  /** Decimal places for `decimal` / used by currency when not custom */
  decimals?: number
  /** Currency / percent / integer formatting locale */
  locale?: string
  /** Prefix before formatted number (e.g. `$`, `+`) */
  prefix?: string
  /** Suffix after formatted number (e.g. `%`, `ms`) */
  suffix?: string
  format?: GsapCounterFormat
  /** Shown when value is null / undefined / NaN / non-finite */
  fallback?: string
  /** First valid value animates from 0. Default true */
  fromZeroOnFirst?: boolean
  /** Ease. Default power2.out */
  ease?: string
}

const DEFAULT_DURATION = 0.48
const MIN_DURATION = 0.35
const MAX_DURATION = 0.65

function prefersReducedMotion(): boolean {
  if (typeof window === 'undefined' || !window.matchMedia) return false
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

function clampDuration(duration: number): number {
  return Math.min(MAX_DURATION, Math.max(MIN_DURATION, duration))
}

function isValidNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value)
}

function formatCounterValue(
  value: number,
  options: UseGsapCounterOptions
): string {
  const {
    format = 'integer',
    decimals = 0,
    locale = 'en-US',
    prefix = '',
    suffix = ''
  } = options

  let body: string
  if (typeof format === 'function') {
    body = format(value)
  } else {
    switch (format) {
      case 'currency': {
        const digits = decimals > 0 ? decimals : 2
        body = new Intl.NumberFormat(locale, {
          minimumFractionDigits: digits,
          maximumFractionDigits: digits
        }).format(value)
        break
      }
      case 'percent': {
        const digits = decimals
        body = new Intl.NumberFormat(locale, {
          minimumFractionDigits: digits,
          maximumFractionDigits: digits
        }).format(value)
        break
      }
      case 'compact': {
        const abs = Math.abs(value)
        if (abs >= 1_000_000_000) {
          body = `${(value / 1_000_000_000).toFixed(2)}B`
        } else if (abs >= 1_000_000) {
          body = `${(value / 1_000_000).toFixed(abs >= 10_000_000 ? 1 : 2)}M`
        } else if (abs >= 1_000) {
          body = `${(value / 1_000).toFixed(abs >= 10_000 ? 1 : 2)}K`
        } else {
          body = new Intl.NumberFormat(locale, {
            maximumFractionDigits: 0
          }).format(value)
        }
        break
      }
      case 'decimal': {
        body = new Intl.NumberFormat(locale, {
          minimumFractionDigits: decimals,
          maximumFractionDigits: decimals
        }).format(value)
        break
      }
      case 'integer':
      default: {
        body = new Intl.NumberFormat(locale, {
          maximumFractionDigits: 0
        }).format(Math.round(value))
        break
      }
    }
  }

  return `${prefix}${body}${suffix}`
}

/**
 * GSAP-driven number counter: first load from 0, later updates from previous value.
 * Updates a controlled display string (does not mutate Vue-owned DOM trees).
 */
export function useGsapCounter(
  source: MaybeRefOrGetter<number | null | undefined>,
  options: UseGsapCounterOptions = {}
): {
  displayText: Ref<string>
  currentValue: Ref<number>
} {
  const fallback = options.fallback ?? '—'
  const displayText = ref(fallback)
  const currentValue = ref(0)
  const hasAnimatedOnce = ref(false)

  const proxy = { value: 0 }
  let tween: gsap.core.Tween | null = null

  const killTween = () => {
    if (tween) {
      tween.kill()
      tween = null
    }
  }

  const applyImmediate = (value: number) => {
    killTween()
    proxy.value = value
    currentValue.value = value
    displayText.value = formatCounterValue(value, options)
    hasAnimatedOnce.value = true
  }

  const applyFallback = () => {
    killTween()
    displayText.value = fallback
  }

  const animateTo = (next: number) => {
    if (prefersReducedMotion()) {
      applyImmediate(next)
      return
    }

    const fromZero = options.fromZeroOnFirst !== false && !hasAnimatedOnce.value
    const from = fromZero ? 0 : proxy.value

    killTween()
    proxy.value = from
    currentValue.value = from
    displayText.value = formatCounterValue(from, options)

    const duration = clampDuration(options.duration ?? DEFAULT_DURATION)

    tween = gsap.to(proxy, {
      value: next,
      duration,
      ease: options.ease ?? 'power2.out',
      overwrite: true,
      onUpdate: () => {
        currentValue.value = proxy.value
        displayText.value = formatCounterValue(proxy.value, options)
      },
      onComplete: () => {
        proxy.value = next
        currentValue.value = next
        displayText.value = formatCounterValue(next, options)
        tween = null
      }
    })

    hasAnimatedOnce.value = true
  }

  watch(
    () => toValue(source),
    (raw) => {
      if (!isValidNumber(raw)) {
        applyFallback()
        return
      }
      if (hasAnimatedOnce.value && raw === proxy.value) {
        displayText.value = formatCounterValue(raw, options)
        return
      }
      animateTo(raw)
    },
    { immediate: true }
  )

  onUnmounted(() => {
    killTween()
  })

  return { displayText, currentValue }
}
