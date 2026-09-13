import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import type { ApiKey } from '@/types'
import CanvasVideoView from '../CanvasVideoView.vue'

const keysAPI = vi.hoisted(() => ({ list: vi.fn() }))
const videoAPI = vi.hoisted(() => ({
  submit: vi.fn(),
  status: vi.fn(),
  content: vi.fn(),
}))

vi.mock('@/api/keys', () => ({ keysAPI }))
vi.mock('@/api/video', () => ({ videoAPI }))
vi.mock('vue-i18n', async (importOriginal) => {
  const original = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...original,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => params?.id ? `${key}:${String(params.id)}` : key,
    }),
  }
})

const CanvasWorkspaceNavStub = { template: '<div><slot /></div>' }

const grokKey = {
  id: 4,
  key: 'sk-video-test',
  name: 'Video key',
  status: 'active',
  group: {
    id: 8,
    name: 'Grok media',
    status: 'active',
    platform: 'grok',
    allow_image_generation: true,
  },
} as unknown as ApiKey

function mountView() {
  return mount(CanvasVideoView, {
    global: {
      stubs: {
        CanvasWorkspaceNav: CanvasWorkspaceNavStub,
        Icon: true,
        RouterLink: { template: '<a><slot /></a>' },
      },
    },
  })
}

describe('CanvasVideoView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    keysAPI.list.mockResolvedValue({ items: [grokKey] })
    videoAPI.submit.mockResolvedValue({ request_id: 'video-42', status: 'pending' })
    videoAPI.status.mockResolvedValue({ request_id: 'video-42', status: 'done', video: { url: '/content' } })
    videoAPI.content.mockResolvedValue(new Blob(['video'], { type: 'video/mp4' }))
    vi.stubGlobal('URL', {
      ...URL,
      createObjectURL: vi.fn(() => 'blob:video-result'),
      revokeObjectURL: vi.fn(),
    })
  })

  it('submits, polls, and renders a downloadable authenticated video result', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('#canvas-video-prompt').setValue('A slow tracking shot across a quiet city')
    await wrapper.get('[data-testid="canvas-video-submit"]').trigger('submit')
    await flushPromises()

    expect(videoAPI.submit).toHaveBeenCalledWith(
      'sk-video-test',
      expect.objectContaining({
        model: 'grok-imagine-video-1.5',
        prompt: 'A slow tracking shot across a quiet city',
        duration: 6,
        aspect_ratio: '16:9',
        resolution: '480p',
      }),
      expect.any(AbortSignal),
    )
    expect(videoAPI.status).toHaveBeenCalledWith('sk-video-test', 'video-42', expect.any(AbortSignal))
    expect(videoAPI.content).toHaveBeenCalledWith('sk-video-test', 'video-42', expect.any(AbortSignal))
    expect(wrapper.get('video').attributes('src')).toBe('blob:video-result')
    expect(wrapper.find('[data-testid="canvas-video-download"]').exists()).toBe(true)
  })
})
