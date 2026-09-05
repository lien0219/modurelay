import { computed, type ComputedRef } from 'vue'
import { useThemeMode } from '@/composables/useThemeMode'

export interface ChartThemeColors {
  text: string
  grid: string
  surface: string
  surfaceRaised: string
  primary: string
  secondary: string
  neutral: string
  success: string
  warning: string
  danger: string
  info: string
}

const lightFallbacks = {
  text: '#344054',
  grid: '#d0d5dd',
  surface: '#ffffff',
  surfaceRaised: '#ffffff',
  success: '#15803d',
  warning: '#b45309',
  danger: '#b42318',
  info: '#2563eb',
  palette: [
    '#4f46e5',
    '#0e7490',
    '#be185d',
    '#b45309',
    '#0f766e',
    '#7e22ce',
    '#c2410c',
    '#1d4ed8',
    '#be123c',
    '#64748b'
  ]
}

const darkFallbacks = {
  text: '#c3c9d4',
  grid: '#414a59',
  surface: '#141821',
  surfaceRaised: '#1b202b',
  success: '#4ade80',
  warning: '#fbbf24',
  danger: '#f87171',
  info: '#60a5fa',
  palette: [
    '#818cf8',
    '#22d3ee',
    '#f472b6',
    '#fbbf24',
    '#2dd4bf',
    '#c084fc',
    '#fb923c',
    '#60a5fa',
    '#fb7185',
    '#cbd5e1'
  ]
}

const lineStyles = [
  { borderDash: [] as number[], pointStyle: 'circle' as const },
  { borderDash: [] as number[], pointStyle: 'triangle' as const },
  { borderDash: [8, 4], pointStyle: 'rect' as const },
  { borderDash: [3, 3], pointStyle: 'rectRot' as const },
  { borderDash: [10, 3, 2, 3], pointStyle: 'crossRot' as const },
  { borderDash: [2, 4], pointStyle: 'star' as const },
  { borderDash: [6, 3], pointStyle: 'cross' as const },
  { borderDash: [12, 4], pointStyle: 'dash' as const }
]

function isDocumentDark(): boolean {
  return typeof document !== 'undefined' && document.documentElement.classList.contains('dark')
}

function readToken(name: string, fallback: string): string {
  if (typeof document === 'undefined' || typeof getComputedStyle === 'undefined') return fallback
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback
}

function readPalette(fallbacks: readonly string[]): string[] {
  return fallbacks.map((fallback, index) => readToken(`--chart-series-${index + 1}`, fallback))
}

export function getChartPalette(isDark = isDocumentDark()): string[] {
  return readPalette((isDark ? darkFallbacks : lightFallbacks).palette)
}

export function getChartThemeColors(isDark = isDocumentDark()): ChartThemeColors {
  const fallback = isDark ? darkFallbacks : lightFallbacks
  const palette = readPalette(fallback.palette)
  return {
    text: readToken('--chart-text', fallback.text),
    grid: readToken('--chart-grid', fallback.grid),
    surface: readToken('--color-surface', fallback.surface),
    surfaceRaised: readToken('--color-surface-raised', fallback.surfaceRaised),
    primary: palette[0],
    secondary: palette[1],
    neutral: palette[9],
    success: readToken('--color-success', fallback.success),
    warning: readToken('--color-warning', fallback.warning),
    danger: readToken('--color-danger', fallback.danger),
    info: readToken('--color-info', fallback.info)
  }
}

export function useChartPalette(): ComputedRef<string[]> {
  const isDark = useThemeMode()
  return computed(() => getChartPalette(isDark.value))
}

export function useChartThemeColors(): ComputedRef<ChartThemeColors> {
  const isDark = useThemeMode()
  return computed(() => getChartThemeColors(isDark.value))
}

export function getChartSeriesStyle(index: number) {
  const style = lineStyles[index % lineStyles.length]
  return {
    borderDash: [...style.borderDash],
    pointStyle: style.pointStyle
  }
}

export function expandChartPalette(palette: readonly string[], count: number): string[] {
  if (palette.length === 0 || count <= 0) return []
  return Array.from({ length: count }, (_, index) => palette[index % palette.length])
}

export function withChartAlpha(color: string, alpha: number): string {
  const normalized = color.trim().replace('#', '')
  if (/^[0-9a-f]{6}$/i.test(normalized)) {
    const value = Number.parseInt(normalized, 16)
    const red = (value >> 16) & 255
    const green = (value >> 8) & 255
    const blue = value & 255
    return `rgba(${red}, ${green}, ${blue}, ${alpha})`
  }
  return color
}
