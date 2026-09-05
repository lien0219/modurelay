import { readonly, ref, type Ref } from 'vue'

const isDarkTheme = ref(false)
let themeObserver: MutationObserver | null = null

function readDocumentTheme(): boolean {
  return typeof document !== 'undefined' && document.documentElement.classList.contains('dark')
}

function ensureThemeObserver() {
  if (
    themeObserver ||
    typeof document === 'undefined' ||
    typeof MutationObserver === 'undefined'
  ) {
    return
  }

  themeObserver = new MutationObserver(() => {
    isDarkTheme.value = readDocumentTheme()
  })
  themeObserver.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['class']
  })
}

export function syncThemeMode(isDark = readDocumentTheme()): boolean {
  isDarkTheme.value = isDark
  return isDark
}

export function useThemeMode(): Readonly<Ref<boolean>> {
  syncThemeMode()
  ensureThemeObserver()
  return readonly(isDarkTheme)
}
