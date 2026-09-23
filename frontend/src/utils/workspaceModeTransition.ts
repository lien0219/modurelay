import { readonly, ref } from 'vue'
import type { RouteLocationRaw, Router } from 'vue-router'

export type WorkspaceModeTransitionDirection = 'to-canvas' | 'to-relay'

interface WorkspaceModeTransitionRequest {
  direction: WorkspaceModeTransitionDirection
  navigate: () => Promise<unknown>
  keepOverlay?: boolean
}

type WorkspaceModeTransitionRunner = (request: WorkspaceModeTransitionRequest) => Promise<void>

const transitioning = ref(false)
let activeRunner: WorkspaceModeTransitionRunner | null = null

const WORKSPACE_DOOR_SESSION_KEY = 'modurelay-workspace-door'

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

export async function navigateToWorkspaceUrlWithTransition(
  href: string,
  direction: WorkspaceModeTransitionDirection,
  options: { replace?: boolean } = {},
) {
  if (transitioning.value) return false

  transitioning.value = true
  try {
    const navigate = async () => {
      sessionStorage.setItem(WORKSPACE_DOOR_SESSION_KEY, direction)
      if (options.replace) window.location.replace(href)
      else window.location.assign(href)
    }

    if (activeRunner) {
      await activeRunner({ direction, navigate, keepOverlay: true })
    } else {
      await navigate()
    }
    return true
  } finally {
    transitioning.value = false
  }
}
