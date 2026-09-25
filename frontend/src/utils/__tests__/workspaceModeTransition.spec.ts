import { describe, expect, it, vi } from 'vitest'
import type { Router } from 'vue-router'
import {
  consumeWorkspaceBackReturnPending,
  markWorkspaceBackReturnPending,
  navigateToWorkspaceUrlWithTransition,
  navigateWithWorkspaceModeTransition,
  registerWorkspaceModeTransitionRunner,
  resetWorkspaceModeTransitionState,
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

  it('tracks one browser-back return from Infinite Canvas', () => {
    markWorkspaceBackReturnPending()

    expect(consumeWorkspaceBackReturnPending()).toBe(true)
    expect(consumeWorkspaceBackReturnPending()).toBe(false)
  })

  it('can recover a cross-document transition lock after BFCache restoration', async () => {
    let release!: () => void
    const pending = new Promise<void>((resolve) => { release = resolve })
    const runner = vi.fn(async () => {
      await pending
    })
    const unregister = registerWorkspaceModeTransitionRunner(runner)

    const navigation = navigateToWorkspaceUrlWithTransition('/infinite-canvas/canvas', 'to-canvas')
    await Promise.resolve()

    expect(workspaceModeTransitioning.value).toBe(true)
    resetWorkspaceModeTransitionState()
    expect(workspaceModeTransitioning.value).toBe(false)

    release()
    await expect(navigation).resolves.toBe(true)
    unregister()
  })

  it('falls back to direct router navigation when the visual layer is unavailable', async () => {
    const router = createRouter()

    await expect(navigateWithWorkspaceModeTransition(router, { name: 'CanvasHome' }, 'to-canvas')).resolves.toBe(true)

    expect(router.push).toHaveBeenCalledWith({ name: 'CanvasHome' })
  })

  it('keeps the transition overlay closed during a cross-document handoff', async () => {
    const runner = vi.fn(async () => undefined)
    const unregister = registerWorkspaceModeTransitionRunner(runner)

    await expect(
      navigateToWorkspaceUrlWithTransition('/infinite-canvas/canvas', 'to-canvas'),
    ).resolves.toBe(true)

    expect(runner).toHaveBeenCalledWith(expect.objectContaining({
      direction: 'to-canvas',
      keepOverlay: true,
    }))
    expect(workspaceModeTransitioning.value).toBe(false)
    unregister()
  })
})
