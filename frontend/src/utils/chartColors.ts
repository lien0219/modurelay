const lightPalette = [
  '#4f46e5',
  '#0891b2',
  '#64748b',
  '#818cf8',
  '#0e7490',
  '#94a3b8',
  '#3730a3',
  '#475569',
  '#6366f1',
  '#a5b4fc',
]

const darkPalette = [
  '#818cf8',
  '#22d3ee',
  '#94a3b8',
  '#a5b4fc',
  '#67e8f9',
  '#64748b',
  '#c7d2fe',
  '#475569',
  '#6366f1',
  '#cbd5e1',
]

export function getChartPalette(): string[] {
  const isDark = typeof document !== 'undefined' && document.documentElement.classList.contains('dark')
  return (isDark ? darkPalette : lightPalette).slice()
}

export function getChartNeutral(): string {
  return typeof document !== 'undefined' && document.documentElement.classList.contains('dark')
    ? '#94a3b8'
    : '#64748b'
}

export function getChartThemeColors() {
  const isDark = typeof document !== 'undefined' && document.documentElement.classList.contains('dark')
  return isDark
    ? {
        text: '#c3c9d4',
        grid: '#3a4250',
        surface: '#131720',
        surfaceRaised: '#191e28',
        primary: '#818cf8',
        secondary: '#22d3ee',
        neutral: '#94a3b8',
        success: '#4ade80',
        warning: '#fbbf24',
        danger: '#f87171',
        info: '#60a5fa',
      }
    : {
        text: '#344054',
        grid: '#e4e7ec',
        surface: '#ffffff',
        surfaceRaised: '#ffffff',
        primary: '#4f46e5',
        secondary: '#0891b2',
        neutral: '#64748b',
        success: '#15803d',
        warning: '#b45309',
        danger: '#b42318',
        info: '#2563eb',
      }
}
