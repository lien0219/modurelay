import { describe, expect, it, vi } from 'vitest'
import type { Router } from 'vue-router'
import {
  navigateWithWorkspaceModeTransition,
  registerWorkspaceModeTransitionRunner,
  workspaceModeTransitioning,
} from '../workspaceModeTransition'

function createRouter() {
  return {
    currentRoute: { value: { fullPath: '/keys' } },
    resolve: vi.fn(() => ({ fullPath: '/canvas' })),
    push: vi.fn(() => Promise.resolve()),
  } as unknown as Router
}

describe('workspaceModeTransition', () => {
  it('runs one transition at a time and ignores repeated activation', async () => {
    const router = createRouter()
    let release!: () => void
    const pending = new Promise<void>((resolve) => { release = resolve })
    const runner = vi.fn(async (request: { navigate: () => Promise<unknown> }) => {
      await pending
      await request.navigate()
    })
    const unregister = registerWorkspaceModeTransitionRunner(runner)

    const first = navigateWithWorkspaceModeTransition(router, { name: 'CanvasHome' }, 'to-canvas')
    const repeated = await navigateWithWorkspaceModeTransition(router, { name: 'CanvasHome' }, 'to-canvas')

    expect(repeated).toBe(false)
    expect(workspaceModeTransitioning.value).toBe(true)
    expect(runner).toHaveBeenCalledOnce()

    release()
    await expect(first).resolves.toBe(true)
    expect(router.push).toHaveBeenCalledOnce()
    expect(workspaceModeTransitioning.value).toBe(false)
    unregister()
  })

  it('falls back to direct router navigation when the visual layer is unavailable', async () => {
    const router = createRouter()

    await expect(navigateWithWorkspaceModeTransition(router, { name: 'CanvasHome' }, 'to-canvas')).resolves.toBe(true)

    expect(router.push).toHaveBeenCalledWith({ name: 'CanvasHome' })
  })
})
