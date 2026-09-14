import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount, RouterLinkStub } from '@vue/test-utils'
import { defineComponent, h, type Component } from 'vue'

import HomeView from '../HomeView.vue'

const { appStore, authStore } = vi.hoisted(() => ({
  appStore: {
    cachedPublicSettings: {} as Record<string, unknown>,
    siteName: 'Fallback site',
    siteLogo: '',
    docUrl: '',
    publicSettingsLoaded: true,
    fetchPublicSettings: vi.fn(),
  },
  authStore: {
    isAuthenticated: false,
    isAdmin: false,
    user: null as { email?: string } | null,
    checkAuth: vi.fn(),
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
  useAuthStore: () => authStore,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key, locale: { value: 'en' } }),
  }
})

const defaultHomeHeroSceneStub = { template: '<div data-testid="home-hero-scene" />' }

enableAutoUnmount(afterEach)

function mockMatchMedia(matches = false) {
  vi.spyOn(window, 'matchMedia').mockReturnValue({
    matches,
    media: '(prefers-reduced-motion: reduce)',
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(() => true),
  } as unknown as MediaQueryList)
}

function mountHome(
  settings: Record<string, unknown> = {},
  homeHeroSceneStub: Component = defaultHomeHeroSceneStub,
) {
  appStore.cachedPublicSettings = {
    site_name: 'Test site',
    site_subtitle: 'Test subtitle',
    ...settings,
  }

  return mount(HomeView, {
    global: {
      stubs: {
        RouterLink: RouterLinkStub,
        LocaleSwitcher: {
          props: ['placement'],
          template: '<div data-testid="locale-switcher" :data-placement="placement" />'
        },
        Icon: { template: '<span data-testid="icon" />' },
        HomeHeroScene: homeHeroSceneStub,
        HomeAmbientEffects: {
          props: ['enabled', 'progress'],
          template: '<div data-testid="home-ambient-effects" :data-enabled="String(enabled)" />'
        },
      },
    },
  })
}

function compactDestination(wrapper: ReturnType<typeof mountHome>) {
  return wrapper.get('[data-testid="compact-home"]').findComponent(RouterLinkStub).props('to')
}

function modelPlazaDestination(wrapper: ReturnType<typeof mountHome>) {
  return wrapper
    .findAllComponents(RouterLinkStub)
    .find((link) => link.props('to') === '/model-plaza')
    ?.props('to')
}

describe('HomeView compact mode', () => {
  beforeEach(() => {
    authStore.isAuthenticated = false
    authStore.isAdmin = false
    authStore.user = null
    authStore.checkAuth.mockClear()
    appStore.fetchPublicSettings.mockClear()
    localStorage.clear()
    window.history.replaceState({}, '', '/home')
    mockMatchMedia()
    vi.spyOn(window, 'scrollTo').mockImplementation(() => undefined)
  })

  it('renders custom HTML ahead of compact mode', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      home_content: '<section id="custom-home">Custom home</section>',
    })

    expect(wrapper.get('#custom-home').text()).toBe('Custom home')
    expect(wrapper.find('[data-testid="compact-home"]').exists()).toBe(false)
  })

  it('renders custom URL content ahead of compact mode', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      home_content: ' https://example.com/home ',
    })

    expect(wrapper.get('iframe').attributes('src')).toBe('https://example.com/home')
    expect(wrapper.find('[data-testid="compact-home"]').exists()).toBe(false)
  })

  it('treats whitespace-only custom content as empty and selects compact mode', () => {
    const wrapper = mountHome({ compact_home_enabled: true, home_content: ' \n\t ' })

    expect(wrapper.get('[data-testid="compact-home"]').text()).toContain('Test site')
  })

  it.each([undefined, false])('selects the default home when compact mode is %s', (enabled) => {
    const settings = enabled === undefined ? {} : { compact_home_enabled: enabled }
    const wrapper = mountHome(settings)

    expect(wrapper.find('[data-testid="compact-home"]').exists()).toBe(false)
    expect(wrapper.find('.home-page').exists()).toBe(true)
  })

  it('links unauthenticated visitors to login', () => {
    expect(compactDestination(mountHome({ compact_home_enabled: true }))).toBe('/login')
  })

  it('links authenticated users to their dashboard', () => {
    authStore.isAuthenticated = true

    expect(compactDestination(mountHome({ compact_home_enabled: true }))).toBe('/dashboard')
  })

  it('links administrators to the admin dashboard', () => {
    authStore.isAuthenticated = true
    authStore.isAdmin = true

    const wrapper = mountHome({ compact_home_enabled: true })
    expect(compactDestination(wrapper)).toBe('/admin/dashboard')
    expect(authStore.checkAuth).toHaveBeenCalledOnce()
    expect(appStore.fetchPublicSettings).not.toHaveBeenCalled()
  })

  it('shows the model plaza link to anonymous visitors when public access is enabled', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      model_plaza_enabled: true,
      model_plaza_require_auth: false,
    })

    expect(modelPlazaDestination(wrapper)).toBe('/model-plaza')
  })

  it('hides the model plaza link from anonymous visitors when sign-in is required', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      model_plaza_enabled: true,
      model_plaza_require_auth: true,
    })

    expect(modelPlazaDestination(wrapper)).toBeUndefined()
  })

  it('shows the model plaza link to authenticated visitors when sign-in is required', () => {
    authStore.isAuthenticated = true

    const wrapper = mountHome({
      compact_home_enabled: true,
      model_plaza_enabled: true,
      model_plaza_require_auth: true,
    })

    expect(modelPlazaDestination(wrapper)).toBe('/model-plaza')
  })

  it('shows the model plaza link in the default home header', () => {
    const wrapper = mountHome({
      model_plaza_enabled: true,
      model_plaza_require_auth: false,
    })

    expect(modelPlazaDestination(wrapper)).toBe('/model-plaza')
  })

  it('hides the model plaza link when the feature is disabled', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      model_plaza_enabled: false,
      model_plaza_require_auth: false,
    })

    expect(modelPlazaDestination(wrapper)).toBeUndefined()
  })

  it('renders the kinetic official home as five fixed narrative states', () => {
    const wrapper = mountHome({ canvas_enabled: true })

    expect(wrapper.get('.home-page.kinetic-home').exists()).toBe(true)
    expect(wrapper.get('[data-testid="home-hero-scene"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="home-ambient-effects"]').exists()).toBe(true)
    expect(wrapper.findAll('[data-home-section]').map(section => section.attributes('id'))).toEqual([
      'home',
      'about',
      'canvas',
      'relay',
      'contact',
    ])
    expect(wrapper.findAll('.kinetic-card')).toHaveLength(0)
    expect(wrapper.findAll('.kinetic-section-index button')).toHaveLength(5)
    expect(wrapper.findAll('.kinetic-top-nav button')).toHaveLength(3)
    expect(wrapper.findComponent('.kinetic-login-link').props('to')).toBe('/login')
    expect(wrapper.get('.kinetic-about').text()).toContain('From AI capability to digital products')
    expect(wrapper.get('.kinetic-about').text()).toContain('AI distillation resources')
    expect(wrapper.get('.kinetic-about').text()).toContain('Custom enterprise SaaS')
    expect(wrapper.get('.kinetic-about').text()).toContain('Cross-border')
    expect(wrapper.get('.kinetic-canvas').text()).toContain('Infinite canvas')
    expect(wrapper.findComponent('.kinetic-canvas-cta').props('to')).toBe('/canvas')
    expect(wrapper.get('.kinetic-footer').text()).not.toContain('Dark mode')
    expect(wrapper.get('.kinetic-footer').text()).not.toContain('Light mode')
    expect(wrapper.get('.kinetic-footer [data-testid="locale-switcher"]').attributes('data-placement')).toBe('top-end')
  })

  it('does not restart the loader when the scene reports an immediate fallback', async () => {
    const immediateReadyScene = defineComponent({
      emits: ['ready'],
      setup(_, { emit }) {
        emit('ready')
        return () => h('div', { 'data-testid': 'home-hero-scene' })
      },
    })
    const wrapper = mountHome({}, immediateReadyScene)

    await wrapper.vm.$nextTick()

    expect(wrapper.classes()).toContain('kinetic-home-ready')
    expect(wrapper.get('[data-testid="home-ambient-effects"]').attributes('data-enabled')).toBe('true')
  })

  it('uses the dashboard destination in the official home navigation after sign-in', () => {
    authStore.isAuthenticated = true
    const wrapper = mountHome()

    expect(wrapper.findComponent('.kinetic-login-link').props('to')).toBe('/dashboard')
    expect(wrapper.findComponent('.kinetic-login-link').text()).toBe('Dashboard')
  })

  it('keeps the official hero connected to the configured product and API surface', () => {
    const wrapper = mountHome({
      api_base_url: 'https://relay.example.com/',
      model_plaza_enabled: true,
      model_plaza_require_auth: false,
    })

    expect(wrapper.get('.kinetic-hero-copy h1').text()).toBe('Test site')
    expect(wrapper.get('.kinetic-hero-eyebrow').text()).toBe('01 / Enterprise AI aggregation and procurement')
    expect(wrapper.get('.kinetic-hero-tagline').text()).toContain('enterprise AI')
    expect(wrapper.get('.kinetic-section-description').text()).toContain('10+ enterprises')
    expect(wrapper.get('.kinetic-hero-endpoint').text()).toContain('https://relay.example.com/v1')
    const links = wrapper.findAllComponents(RouterLinkStub)
    expect(links.find(link => link.classes().includes('kinetic-hero-primary'))?.props('to')).toBe('/login')
    expect(links.find(link => link.classes().includes('kinetic-hero-secondary'))?.props('to')).toBe('/model-plaza')
  })

  it('switches one fixed state for a downward wheel gesture', async () => {
    mockMatchMedia(true)
    const immediateReadyScene = defineComponent({
      emits: ['ready'],
      setup(_, { emit }) {
        emit('ready')
        return () => h('div', { 'data-testid': 'home-hero-scene' })
      },
    })
    const wrapper = mountHome({ canvas_enabled: true }, immediateReadyScene)
    await wrapper.vm.$nextTick()

    expect(wrapper.get('#home').classes()).toContain('is-active')
    await wrapper.get('.kinetic-home').trigger('wheel', { deltaX: 0, deltaY: 100 })

    expect(wrapper.get('#about').classes()).toContain('is-active')
    expect(wrapper.get('#home').attributes('aria-hidden')).toBe('true')
    expect(window.location.hash).toBe('#about')
    expect(window.scrollY).toBe(0)
  })

  it('opens the about state directly from its public hash link', async () => {
    window.history.replaceState({}, '', '/home#about')
    mockMatchMedia(true)
    const immediateReadyScene = defineComponent({
      emits: ['ready'],
      setup(_, { emit }) {
        emit('ready')
        return () => h('div', { 'data-testid': 'home-hero-scene' })
      },
    })

    const wrapper = mountHome({}, immediateReadyScene)
    await wrapper.vm.$nextTick()

    expect(wrapper.get('#about').classes()).toContain('is-active')
    expect(wrapper.get('#home').attributes('aria-hidden')).toBe('true')
    expect(window.location.hash).toBe('#about')
  })

  it('returns to the first state when the public section hash is cleared', async () => {
    window.history.replaceState({}, '', '/home#about')
    mockMatchMedia(true)
    const immediateReadyScene = defineComponent({
      emits: ['ready'],
      setup(_, { emit }) {
        emit('ready')
        return () => h('div', { 'data-testid': 'home-hero-scene' })
      },
    })

    const wrapper = mountHome({}, immediateReadyScene)
    await wrapper.vm.$nextTick()
    window.history.replaceState({}, '', '/home')
    window.dispatchEvent(new HashChangeEvent('hashchange'))
    await wrapper.vm.$nextTick()

    expect(wrapper.get('#home').classes()).toContain('is-active')
    expect(window.location.hash).toBe('')
  })

  it('supports keyboard navigation without moving the document', async () => {
    mockMatchMedia(true)
    const immediateReadyScene = defineComponent({
      emits: ['ready'],
      setup(_, { emit }) {
        emit('ready')
        return () => h('div', { 'data-testid': 'home-hero-scene' })
      },
    })
    const wrapper = mountHome({}, immediateReadyScene)
    await wrapper.vm.$nextTick()

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'End', cancelable: true }))
    await wrapper.vm.$nextTick()

    expect(wrapper.get('#contact').classes()).toContain('is-active')
    expect(document.documentElement.style.overflow).toBe('hidden')
  })

  it('keeps the primary kinetic CTA connected to the existing login route', () => {
    const wrapper = mountHome()
    const primaryCta = wrapper
      .findAllComponents(RouterLinkStub)
      .find(link => link.classes().includes('kinetic-primary-cta'))

    expect(primaryCta?.props('to')).toBe('/login')
  })
})
