import { readonly, ref } from 'vue'
import type { RouteLocationRaw, Router } from 'vue-router'

export type WorkspaceModeTransitionDirection = 'to-canvas' | 'to-relay'

interface WorkspaceModeTransitionRequest {
  direction: WorkspaceModeTransitionDirection
  navigate: () => Promise<unknown>
}

type WorkspaceModeTransitionRunner = (request: WorkspaceModeTransitionRequest) => Promise<void>

const transitioning = ref(false)
let activeRunner: WorkspaceModeTransitionRunner | null = null

export const workspaceModeTransitioning = readonly(transitioning)

export function registerWorkspaceModeTransitionRunner(runner: WorkspaceModeTransitionRunner) {
  activeRunner = runner
  return () => {
    if (activeRunner === runner) activeRunner = null
  }
}

export async function navigateWithWorkspaceModeTransition(
  router: Router,
  to: RouteLocationRaw,
  direction: WorkspaceModeTransitionDirection,
) {
  if (transitioning.value) return false

  const target = router.resolve(to)
  if (target.fullPath === router.currentRoute.value.fullPath) return false

  transitioning.value = true
  try {
    const navigate = () => router.push(to)
    if (activeRunner) {
      await activeRunner({ direction, navigate })
    } else {
      await navigate()
    }
    return true
  } finally {
    transitioning.value = false
  }
}
