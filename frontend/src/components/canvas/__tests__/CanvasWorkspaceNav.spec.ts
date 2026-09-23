import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { mount } from '@vue/test-utils'
import CanvasWorkspaceNav from '../CanvasWorkspaceNav.vue'

const route = vi.hoisted(() => ({
  name: 'CanvasHome',
  hash: '',
  fullPath: '/canvas',
}))

const router = vi.hoisted(() => ({
  push: vi.fn(),
}))

const transition = vi.hoisted(() => ({
  navigate: vi.fn(),
  active: { __v_isRef: true, value: false },
}))

vi.mock('vue-router', async (importOriginal) => {
  const original = await importOriginal<typeof import('vue-router')>()
  return {
    ...original,
    useRoute: () => route,
    useRouter: () => router,
  }
})

vi.mock('vue-i18n', async (importOriginal) => {
  const original = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...original,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

vi.mock('@/utils/workspaceModeTransition', () => ({
  navigateWithWorkspaceModeTransition: transition.navigate,
  workspaceModeTransitioning: transition.active,
}))

const RouterLinkStub = {
  props: ['to'],
  emits: ['click'],
  template: '<a href="#" @click.prevent="$emit(\'click\', $event)"><slot /></a>',
}

function mountNav() {
  const pinia = createPinia()
  setActivePinia(pinia)
  return mount(CanvasWorkspaceNav, {
    global: {
      plugins: [pinia],
      stubs: {
        Icon: true,
        RouterLink: RouterLinkStub,
      },
    },
    slots: {
      default: '<main id="canvas-workspace-content">Canvas content</main>',
    },
  })
}

describe('CanvasWorkspaceNav', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    route.name = 'CanvasHome'
    route.hash = ''
    route.fullPath = '/canvas'
    transition.active.value = false
  })

  it('renders an independent workspace shell around the canvas content', () => {
    const wrapper = mountNav()

    expect(wrapper.get('.canvas-workspace-nav').exists()).toBe(true)
    expect(wrapper.get('#canvas-workspace-content').text()).toBe('Canvas content')
    expect(wrapper.get('[role="tablist"]').attributes('aria-label')).toBe('canvas.workspaceNav.modeSwitcherAria')
  })

  it('returns to the API key workspace through the shared transition', async () => {
    const wrapper = mountNav()

    await wrapper.get('[data-testid="workspace-mode-relay"]').trigger('click')

    expect(transition.navigate).toHaveBeenCalledOnce()
    expect(transition.navigate).toHaveBeenCalledWith(router, { name: 'Keys' }, 'to-relay')
  })
})
