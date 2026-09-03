import { beforeEach, describe, expect, it, vi } from 'vitest'

import { disposeOnboardingTour, removeOnboardingArtifacts } from '@/utils/onboardingCleanup'

function addDriverArtifacts() {
  document.body.classList.add('driver-active', 'driver-fade')
  document.body.insertAdjacentHTML(
    'beforeend',
    '<svg class="driver-overlay"></svg><div class="driver-popover"></div><div id="driver-dummy-element"></div>'
  )
  const activeElement = document.createElement('button')
  activeElement.className = 'driver-active-element driver-no-interaction'
  activeElement.setAttribute('aria-haspopup', 'dialog')
  activeElement.setAttribute('aria-expanded', 'true')
  activeElement.setAttribute('aria-controls', 'driver-popover-content')
  document.body.appendChild(activeElement)
  return activeElement
}

describe('onboarding cleanup', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    document.body.className = ''
  })

  it('destroys the active Driver instance and removes all detached overlay state', () => {
    const activeElement = addDriverArtifacts()
    const destroy = vi.fn()
    const setDriverInstance = vi.fn()

    disposeOnboardingTour({
      getDriverInstance: () => ({ destroy }),
      setDriverInstance
    })

    expect(destroy).toHaveBeenCalledOnce()
    expect(setDriverInstance).toHaveBeenCalledWith(null)
    expect(document.querySelector('.driver-overlay')).toBeNull()
    expect(document.querySelector('.driver-popover')).toBeNull()
    expect(document.body.classList.contains('driver-active')).toBe(false)
    expect(activeElement.classList.contains('driver-active-element')).toBe(false)
    expect(activeElement.hasAttribute('aria-controls')).toBe(false)
  })

  it('removes stale overlay nodes even when no Driver instance is available', () => {
    addDriverArtifacts()

    removeOnboardingArtifacts()

    expect(document.querySelector('.driver-overlay')).toBeNull()
    expect(document.querySelector('#driver-dummy-element')).toBeNull()
    expect(document.body.classList.contains('driver-fade')).toBe(false)
  })
})
