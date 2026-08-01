const THREE_MODULE_URLS = [
  'https://cdn.jsdelivr.net/npm/three@0.180.0/build/three.module.min.js',
  'https://unpkg.com/three@0.180.0/build/three.module.min.js'
] as const

async function importThreeFromCdn() {
  let lastError: unknown

  for (const url of THREE_MODULE_URLS) {
    try {
      return await import(/* @vite-ignore */ url)
    } catch (error) {
      lastError = error
    }
  }

  throw lastError instanceof Error
    ? lastError
    : new Error('Unable to load the pinned Three.js runtime')
}

let threeModulePromise: ReturnType<typeof importThreeFromCdn> | null = null

export function loadThree() {
  if (!threeModulePromise) {
    threeModulePromise = importThreeFromCdn().catch((error) => {
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
