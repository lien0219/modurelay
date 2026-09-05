import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'

function getTooltipElement(): HTMLDivElement {
  const tooltip = document.body.querySelector('[role="tooltip"]')
  if (!(tooltip instanceof HTMLDivElement)) {
    throw new Error('tooltip element not found')
  }
  return tooltip
}

describe('HelpTooltip', () => {
  afterEach(() => {
    vi.restoreAllMocks()
    document.body.innerHTML = ''
  })

  it('keeps the existing hover interaction by default', async () => {
    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: {
        content: 'hover details',
      },
    })

    const trigger = wrapper.get('.group')
    const triggerButton = wrapper.get('button[aria-label="hover details"]')
    const tooltip = getTooltipElement()

    expect(tooltip.style.display).toBe('none')
    expect(triggerButton.attributes('type')).toBe('button')

    await trigger.trigger('mouseenter')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    await trigger.trigger('mouseleave')
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    await triggerButton.trigger('focusin')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    wrapper.unmount()
  })

  it('keeps a hover tooltip open while the pointer moves between the trigger and the tooltip', async () => {
    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: {
        content: 'copyable details',
      },
    })

    const trigger = wrapper.get('.group')
    const tooltip = getTooltipElement()

    await trigger.trigger('mouseenter')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    await trigger.trigger('mouseleave', { relatedTarget: tooltip })
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    tooltip.dispatchEvent(new MouseEvent('mouseleave', { relatedTarget: trigger.element }))
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    tooltip.dispatchEvent(new MouseEvent('mouseleave', { relatedTarget: null }))
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    wrapper.unmount()
  })

  it('supports click-to-toggle details and closes on outside click', async () => {
    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: {
        content: 'click details',
        trigger: 'click',
      },
    })

    const trigger = wrapper.get('.group')
    const tooltip = getTooltipElement()

    expect(tooltip.style.display).toBe('none')

    await trigger.trigger('click')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')
    expect(tooltip.textContent).toContain('click details')

    const closeButton = tooltip.querySelector('button[aria-label="Close"]')
    if (!(closeButton instanceof HTMLButtonElement)) {
      throw new Error('close button not found')
    }
    closeButton.click()
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    await trigger.trigger('click')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    wrapper.unmount()
  })

  it('opens hover details when a keyboard trigger receives focus', async () => {
    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: {
        content: 'keyboard details',
      },
      slots: {
        trigger: '<button type="button">Balance</button>',
      },
    })

    const trigger = wrapper.get('button')
    const tooltip = getTooltipElement()

    await trigger.trigger('focusin')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    await trigger.trigger('focusout')
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    wrapper.unmount()
  })

  it('positions the fixed tooltip in viewport coordinates after page scrolling', async () => {
    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: { content: 'positioned details' },
    })
    vi.spyOn(window, 'scrollX', 'get').mockReturnValue(300)
    vi.spyOn(window, 'scrollY', 'get').mockReturnValue(500)
    vi.spyOn(wrapper.get('.group').element, 'getBoundingClientRect').mockReturnValue({
      x: 100,
      y: 200,
      top: 200,
      left: 100,
      right: 140,
      bottom: 220,
      width: 40,
      height: 20,
      toJSON: () => ({}),
    })

    await wrapper.get('.group').trigger('mouseenter')
    await nextTick()

    const tooltip = getTooltipElement()
    expect(tooltip.style.left).toBe('120px')
    expect(tooltip.style.top).toBe('calc(192px)')
    wrapper.unmount()
  })
})
