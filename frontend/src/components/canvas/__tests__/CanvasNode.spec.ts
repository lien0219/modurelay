import { describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { shallowMount } from '@vue/test-utils'
import CanvasNode from '@/components/canvas/CanvasNode.vue'
import { CANVAS_HANDLES } from '@/utils/canvasGraph'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const HandleStub = defineComponent({
  inheritAttrs: false,
  props: ['id', 'type'],
  template: '<button class="handle-stub" :data-id="id" :data-type="type" :aria-label="$attrs[\'aria-label\']"></button>',
})

function mountNode(type: 'prompt' | 'reference' | 'generation' | 'image', data: Record<string, unknown> = {}) {
  return shallowMount(CanvasNode, {
    props: {
      id: `${type}-1`,
      type,
      selected: false,
      connectable: true,
      position: { x: 0, y: 0 },
      dimensions: { width: 288, height: 180 },
      dragging: false,
      resizing: false,
      zIndex: 1,
      data: { label: type, ...data },
      events: {},
    },
    global: {
      stubs: {
        Handle: HandleStub,
        Icon: true,
      },
    },
  })
}

describe('CanvasNode', () => {
  it('renders one accessible output handle for a prompt', () => {
    const wrapper = mountNode('prompt', { prompt: 'A precise product scene' })
    const handles = wrapper.findAll('.handle-stub')

    expect(handles).toHaveLength(1)
    expect(handles[0].attributes('data-id')).toBe(CANVAS_HANDLES.promptOutput)
    expect(handles[0].attributes('data-type')).toBe('source')
    expect(handles[0].attributes('aria-label')).toBe('canvas.handles.promptOutput')
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('A precise product scene')
  })

  it('reports inline prompt edits through the runtime callback', async () => {
    const onChange = vi.fn()
    const wrapper = mountNode('prompt', { prompt: '', onChange })

    await wrapper.get('textarea').setValue('A connected product workflow')

    expect(onChange).toHaveBeenCalledWith('prompt-1')
  })

  it('renders separate prompt/reference inputs and a generation output', () => {
    const wrapper = mountNode('generation', {
      model: 'gpt-image-1',
      status: 'processing',
      promptConnected: true,
      referenceConnected: false,
    })
    const handles = wrapper.findAll('.handle-stub')

    expect(handles.map(handle => handle.attributes('data-id'))).toEqual([
      CANVAS_HANDLES.generationPrompt,
      CANVAS_HANDLES.generationReference,
      CANVAS_HANDLES.generationOutput,
    ])
    expect(handles.map(handle => handle.attributes('data-type'))).toEqual(['target', 'target', 'source'])
    expect(wrapper.classes()).toContain('canvas-node--busy')
    expect(wrapper.text()).toContain('canvas.status.processing')
  })

  it('renders the durable asset preview for image nodes', () => {
    const wrapper = mountNode('image', { url: 'https://example.test/result.png', label: 'Result' })

    expect(wrapper.get('img').attributes('src')).toBe('https://example.test/result.png')
    expect(wrapper.get('img').attributes('alt')).toBe('Result')
    expect(wrapper.get('.handle-stub').attributes('data-id')).toBe(CANVAS_HANDLES.imageInput)
  })

  it('exposes each image editing tool through the node toolbar', async () => {
    const onEdit = vi.fn()
    const onMaskEdit = vi.fn()
    const onAngle = vi.fn()
    const wrapper = mountNode('image', {
      url: 'https://example.test/result.png',
      onEdit,
      onMaskEdit,
      onAngle,
    })

    await wrapper.get('button[title="canvas.editImage"]').trigger('click')
    await wrapper.get('button[title="canvas.editors.maskTitle"]').trigger('click')
    await wrapper.get('button[title="canvas.editors.angleTitle"]').trigger('click')

    expect(onEdit).toHaveBeenCalledWith('image-1')
    expect(onMaskEdit).toHaveBeenCalledWith('image-1')
    expect(onAngle).toHaveBeenCalledWith('image-1')
  })
})
