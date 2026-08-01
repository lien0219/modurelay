let threeModulePromise: Promise<any> | null = null

export function loadThree() {
  if (!threeModulePromise) {
    threeModulePromise = import('three').catch((error) => {
      threeModulePromise = null
      throw error
    })
  }

  return threeModulePromise
}

export function isDarkTheme(): boolean {
  return document.documentElement.classList.contains('dark')
}

export function observeTheme(callback: (dark: boolean) => void): () => void {
  callback(isDarkTheme())

  const observer = new MutationObserver(() => callback(isDarkTheme()))
  observer.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['class']
  })

  return () => observer.disconnect()
}
