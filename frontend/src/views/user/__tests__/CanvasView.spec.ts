import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import type { CanvasDocument, CanvasProject } from '@/api/canvas'
import CanvasView from '../CanvasView.vue'

const api = vi.hoisted(() => ({
  listProjects: vi.fn(),
  getProject: vi.fn(),
  deleteProject: vi.fn(),
}))

const drafts = vi.hoisted(() => ({
  getCanvasDraft: vi.fn(),
  removeCanvasDraft: vi.fn(),
  saveCanvasDraft: vi.fn(),
}))

const route = vi.hoisted(() => ({
  path: '/canvas/editor',
  query: { project: '7' } as Record<string, string>,
}))

const router = vi.hoisted(() => ({
  push: vi.fn(),
  replace: vi.fn(),
  resolve: vi.fn(() => ({ href: '/canvas/editor?project=7' })),
}))

const translation = vi.hoisted(() => ({
  locale: { value: 'en' },
  t: (key: string) => key,
}))

vi.mock('@/api/canvas', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/api/canvas')>()
  return { ...original, canvasAPI: api }
})

vi.mock('@/repositories/canvasDrafts', () => drafts)

vi.mock('vue-router', async (importOriginal) => {
  const original = await importOriginal<typeof import('vue-router')>()
  return {
    ...original,
    onBeforeRouteLeave: vi.fn(),
    useRoute: () => route,
    useRouter: () => router,
  }
})

vi.mock('vue-i18n', async (importOriginal) => {
  const original = await importOriginal<typeof import('vue-i18n')>()
  return { ...original, useI18n: () => translation }
})

const document = (): CanvasDocument => ({
  schema_version: 1,
  nodes: [],
  edges: [],
  viewport: { x: 0, y: 0, zoom: 1 },
})

const project = (): CanvasProject => ({
  id: 7,
  title: 'Product launch',
  document: document(),
  schema_version: 1,
  revision: 1,
  document_bytes: 100,
  created_at: '2026-09-12T08:00:00Z',
  updated_at: '2026-09-12T09:00:00Z',
})

const CanvasWorkspaceNavStub = { template: '<div><slot /></div>' }
const VueFlowStub = { template: '<div class="vue-flow-stub"></div>' }
const ConfirmDialogStub = {
  props: ['show', 'confirming'],
  emits: ['confirm', 'cancel'],
  template: '<button v-if="show" data-testid="confirm-delete" :disabled="confirming" @click="$emit(\'confirm\')">confirm</button>',
}

async function mountView() {
  const pinia = createPinia()
  setActivePinia(pinia)
  const wrapper = mount(CanvasView, {
    global: {
      plugins: [pinia],
      stubs: {
        CanvasWorkspaceNav: CanvasWorkspaceNavStub,
        ConfirmDialog: ConfirmDialogStub,
        Icon: true,
        VueFlow: VueFlowStub,
      },
    },
  })
  await flushPromises()
  router.replace.mockClear()
  return wrapper
}

describe('CanvasView project deletion', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    const currentProject = project()
    api.listProjects.mockResolvedValue([currentProject])
    api.getProject.mockResolvedValue(currentProject)
    api.deleteProject.mockResolvedValue(undefined)
    drafts.getCanvasDraft.mockResolvedValue(undefined)
    drafts.removeCanvasDraft.mockResolvedValue(undefined)
    drafts.saveCanvasDraft.mockResolvedValue(undefined)
    router.replace.mockResolvedValue(undefined)
  })

  it('returns to the canvas home after deleting the current project', async () => {
    const wrapper = await mountView()

    await wrapper.get('.canvas-icon-button--danger').trigger('click')
    await wrapper.get('[data-testid="confirm-delete"]').trigger('click')
    await flushPromises()

    expect(api.deleteProject).toHaveBeenCalledWith(7)
    expect(drafts.removeCanvasDraft).toHaveBeenCalledWith(7)
    expect(router.replace).toHaveBeenCalledOnce()
    expect(router.replace).toHaveBeenCalledWith({ name: 'CanvasHome' })
    wrapper.unmount()
  })

  it('stays in the editor and reports the error when deletion fails', async () => {
    api.deleteProject.mockRejectedValue(new Error('delete failed'))
    const wrapper = await mountView()

    await wrapper.get('.canvas-icon-button--danger').trigger('click')
    await wrapper.get('[data-testid="confirm-delete"]').trigger('click')
    await flushPromises()

    expect(router.replace).not.toHaveBeenCalled()
    expect(drafts.removeCanvasDraft).not.toHaveBeenCalled()
    expect(wrapper.get('[role="alert"]').text()).toContain('delete failed')
    expect(wrapper.find('[data-testid="confirm-delete"]').exists()).toBe(true)
    wrapper.unmount()
  })
})
