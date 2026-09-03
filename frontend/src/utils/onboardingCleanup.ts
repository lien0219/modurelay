const ONBOARDING_NODE_SELECTOR = [
  '.driver-overlay',
  '.driver-popover',
  '.driver-popover-container',
  '.driver-popover-arrow',
  '#driver-dummy-element'
].join(', ')

const DRIVER_BODY_CLASSES = ['driver-active', 'driver-fade', 'driver-simple'] as const

interface DriverHandle {
  destroy: () => void
}

interface OnboardingStoreHandle {
  getDriverInstance: () => DriverHandle | null
  setDriverInstance: (instance: null) => void
}

/** Remove Driver.js state that lives outside Vue's component tree. */
export function removeOnboardingArtifacts(): void {
  if (typeof document === 'undefined') return

  document.querySelectorAll(ONBOARDING_NODE_SELECTOR).forEach((element) => element.remove())
  document.querySelectorAll('.driver-active-element, .driver-no-interaction').forEach((element) => {
    element.classList.remove('driver-active-element', 'driver-no-interaction')
    element.removeAttribute('aria-haspopup')
    element.removeAttribute('aria-expanded')
    element.removeAttribute('aria-controls')
  })
  document.body?.classList.remove(...DRIVER_BODY_CLASSES)
}

/** Destroy the active tour first, then defensively clear any detached DOM residue. */
export function disposeOnboardingTour(store: OnboardingStoreHandle): void {
  try {
    store.getDriverInstance()?.destroy()
  } finally {
    store.setDriverInstance(null)
    removeOnboardingArtifacts()
  }
}
