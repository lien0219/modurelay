import { describe, expect, it, vi } from 'vitest'
import type { Router } from 'vue-router'
import {
  consumeCanvasDocumentDeparture,
  markWorkspaceDocumentDeparture,
  navigateToWorkspaceUrlWithTransition,
  navigateWithWorkspaceModeTransition,
  registerWorkspaceModeTransitionRunner,
  resetWorkspaceModeTransitionState,
  workspaceModeTransitioning,
  workspaceRouteTransitionDirection,
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

  it('tracks cross-document arrivals without treating reloads as workspace changes', () => {
    markWorkspaceDocumentDeparture()
    expect(consumeCanvasDocumentDeparture()).toBe(false)
    sessionStorage.setItem('modurelay-workspace-document-departure', 'canvas')
    expect(consumeCanvasDocumentDeparture()).toBe(true)
    expect(consumeCanvasDocumentDeparture()).toBe(false)
  })

  it('selects the door direction only when routes cross workspace modes', () => {
    expect(workspaceRouteTransitionDirection('/dashboard', '/token-market')).toBe('to-market')
    expect(workspaceRouteTransitionDirection('/token-market/cart', '/dashboard')).toBe('to-relay')
    expect(workspaceRouteTransitionDirection('/token-market', '/token-market/wallet')).toBeNull()
    expect(workspaceRouteTransitionDirection('/dashboard', '/canvas/editor')).toBe('to-canvas')
    expect(workspaceRouteTransitionDirection('/dashboard', '/canvas')).toBeNull()
    expect(workspaceRouteTransitionDirection('/keys', '/dashboard')).toBeNull()
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

  it('hands market navigation to the same visual runner and routes to the market', async () => {
    const router = createRouter()
    const runner = vi.fn(async (request: { navigate: () => Promise<unknown> }) => request.navigate())
    const unregister = registerWorkspaceModeTransitionRunner(runner)

    await expect(navigateWithWorkspaceModeTransition(router, { name: 'TokenMarket' }, 'to-market')).resolves.toBe(true)

    expect(runner).toHaveBeenCalledWith(expect.objectContaining({ direction: 'to-market' }))
    expect(router.push).toHaveBeenCalledWith({ name: 'TokenMarket' })
    unregister()
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
