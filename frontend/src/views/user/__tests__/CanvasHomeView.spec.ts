import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import type { CanvasDocument, CanvasProject } from '@/api/canvas'
import CanvasHomeView from '../CanvasHomeView.vue'

const api = vi.hoisted(() => ({
  listProjects: vi.fn(),
  createProject: vi.fn(),
  getProject: vi.fn(),
  saveProject: vi.fn(),
  deleteProject: vi.fn(),
  assetURL: vi.fn(),
}))

const router = vi.hoisted(() => ({
  push: vi.fn(),
  resolve: vi.fn(() => ({ href: '/canvas/editor?project=7' })),
}))
const route = vi.hoisted(() => ({
  name: 'CanvasHome',
  hash: '',
  fullPath: '/canvas',
}))

const translation = vi.hoisted(() => ({
  locale: { value: 'en' },
  t: (key: string) => ({
    'canvas.untitled': 'Untitled canvas',
    'canvas.home.noResultsTitle': 'No matching canvases',
  })[key] || key,
}))

vi.mock('@/api/canvas', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/api/canvas')>()
  return { ...original, canvasAPI: api }
})

vi.mock('vue-router', async (importOriginal) => {
  const original = await importOriginal<typeof import('vue-router')>()
  return { ...original, useRoute: () => route, useRouter: () => router }
})

vi.mock('vue-i18n', async (importOriginal) => {
  const original = await importOriginal<typeof import('vue-i18n')>()
  return { ...original, useI18n: () => translation }
})

const emptyDocument = (): CanvasDocument => ({
  schema_version: 1,
  nodes: [],
  edges: [],
  viewport: { x: 0, y: 0, zoom: 1 },
})

function project(id: number, title: string, document = emptyDocument()): CanvasProject {
  return {
    id,
    title,
    document,
    schema_version: 1,
    revision: 1,
    document_bytes: 100,
    created_at: `2026-09-${String(id).padStart(2, '0')}T08:00:00Z`,
    updated_at: `2026-09-${String(id).padStart(2, '0')}T09:00:00Z`,
  }
}

const CanvasWorkspaceNavStub = { template: '<div><slot /></div>' }
const ConfirmDialogStub = {
  props: ['show'],
  emits: ['confirm', 'cancel'],
  template: '<button v-if="show" data-testid="confirm-delete" @click="$emit(\'confirm\')">confirm</button>',
}

async function mountView(projects: CanvasProject[] = [], attachTo?: Element) {
  const pinia = createPinia()
  setActivePinia(pinia)
  api.listProjects.mockResolvedValue(projects)
  const wrapper = mount(CanvasHomeView, {
    attachTo,
    global: {
      plugins: [pinia],
      stubs: {
        CanvasWorkspaceNav: CanvasWorkspaceNavStub,
        ConfirmDialog: ConfirmDialogStub,
        Icon: true,
        RouterLink: { template: '<a><slot /></a>' },
      },
    },
  })
  await flushPromises()
  return wrapper
}

describe('CanvasHomeView', () => {
  beforeEach(async () => {
    vi.clearAllMocks()
    route.hash = ''
    route.fullPath = '/canvas'
    api.listProjects.mockResolvedValue([])
    api.deleteProject.mockResolvedValue(undefined)
    api.assetURL.mockResolvedValue('/api/canvas/assets/9/content')
    router.resolve.mockImplementation(location => ({
      href: `/canvas/editor?project=${String(location.query.project)}`,
    }))
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('loads and renders the project library', async () => {
    const wrapper = await mountView([
      project(4, 'Campaign concepts'),
      project(7, 'Product launch'),
    ])

    expect(api.listProjects).toHaveBeenCalledOnce()
    expect(wrapper.findAll('.canvas-home__project-card')).toHaveLength(2)
    expect(wrapper.text()).toContain('Product launch')
  })

  it('creates and persists the connected starter workflow before opening it', async () => {
    const created = project(9, 'Untitled canvas')
    api.createProject.mockResolvedValue(created)
    api.saveProject.mockImplementation((id, _revision, title, document) => Promise.resolve({
      ...created,
      id,
      title,
      document,
      revision: 2,
    }))
    const wrapper = await mountView()

    await wrapper.get('[data-testid="canvas-start"]').trigger('click')
    await flushPromises()

    expect(api.createProject).toHaveBeenCalledWith('Untitled canvas')
    expect(api.saveProject).toHaveBeenCalledOnce()
    const savedDocument = api.saveProject.mock.calls[0][3] as CanvasDocument
    expect(savedDocument.nodes).toHaveLength(3)
    expect(savedDocument.edges).toHaveLength(2)
    expect(router.push).toHaveBeenCalledWith({
      name: 'CanvasEditor',
      query: { project: '9' },
    })
  })

  it('opens an existing project in the editor or a new tab', async () => {
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)
    const wrapper = await mountView([project(7, 'Product launch')])

    await wrapper.get('.canvas-home__project-open').trigger('click')
    expect(router.push).toHaveBeenCalledWith({
      name: 'CanvasEditor',
      query: { project: '7' },
    })

    await wrapper.get('.canvas-home__project-actions button').trigger('click')
    expect(router.resolve).toHaveBeenCalledWith({
      name: 'CanvasEditor',
      query: { project: '7' },
    })
    expect(openSpy).toHaveBeenCalledWith(
      '/canvas/editor?project=7',
      '_blank',
      'noopener,noreferrer',
    )
  })

  it('deletes a project only after confirmation', async () => {
    const wrapper = await mountView([project(7, 'Product launch')])

    await wrapper.get('.canvas-home__project-actions button.is-danger').trigger('click')
    await wrapper.get('[data-testid="confirm-delete"]').trigger('click')
    await flushPromises()

    expect(api.deleteProject).toHaveBeenCalledWith(7)
    expect(wrapper.findAll('.canvas-home__project-card')).toHaveLength(0)
    expect(router.push).not.toHaveBeenCalled()
  })

  it('filters projects and recovers from the no-results state', async () => {
    const wrapper = await mountView([
      project(4, 'Campaign concepts'),
      project(7, 'Product launch'),
    ])

    await wrapper.get('input[type="search"]').setValue('missing')
    expect(wrapper.findAll('.canvas-home__project-card')).toHaveLength(0)
    expect(wrapper.text()).toContain('No matching canvases')

    await wrapper.get('.canvas-home__empty-state button').trigger('click')
    expect(wrapper.findAll('.canvas-home__project-card')).toHaveLength(2)
  })

  it('collects prompts and saved assets from real canvas documents', async () => {
    const document: CanvasDocument = {
      ...emptyDocument(),
      nodes: [
        { id: 'prompt-1', type: 'prompt', position: { x: 0, y: 0 }, data: { label: 'Prompt', prompt: 'A precise studio portrait' } },
        { id: 'image-1', type: 'image', position: { x: 320, y: 0 }, data: { label: 'Result', assetId: 9, fileName: 'portrait.png' } },
      ],
    }
    const wrapper = await mountView([project(7, 'Product launch', document)])
    await flushPromises()

    expect(wrapper.get('.canvas-home__prompt-card').text()).toContain('A precise studio portrait')
    expect(wrapper.get('.canvas-home__asset-card').text()).toContain('portrait.png')
    expect(api.assetURL).toHaveBeenCalledWith(9)
  })

  it('scrolls hash navigation after the router has restored its scroll position', async () => {
    const scrollIntoView = vi.fn()
    const originalScrollIntoView = Element.prototype.scrollIntoView
    const host = document.createElement('div')
    document.body.appendChild(host)
    Object.defineProperty(Element.prototype, 'scrollIntoView', {
      configurable: true,
      value: scrollIntoView,
    })
    vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
      callback(0)
      return 1
    })
    route.hash = '#canvas-prompts'
    route.fullPath = '/canvas#canvas-prompts'

    const wrapper = await mountView([], host)
    try {
      await flushPromises()
      expect(scrollIntoView).toHaveBeenCalledWith({ block: 'start' })
    } finally {
      wrapper.unmount()
      host.remove()
      if (originalScrollIntoView) {
        Object.defineProperty(Element.prototype, 'scrollIntoView', {
          configurable: true,
          value: originalScrollIntoView,
        })
      } else {
        delete (Element.prototype as { scrollIntoView?: unknown }).scrollIntoView
      }
    }
  })
})
