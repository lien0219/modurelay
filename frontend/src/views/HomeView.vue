<template>
  <div v-if="hasHomeContent" class="min-h-screen bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white">
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <div v-else v-html="homeContent"></div>
  </div>

  <div v-else-if="compactHomeEnabled" data-testid="compact-home" class="compact-home flex min-h-screen flex-col">
    <header class="compact-home-header">
      <nav class="compact-home-nav" aria-label="Home navigation">
        <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="compact-home-brand">
          <span class="brand-mark"><img :src="siteLogo || brand.logo" :alt="siteName" /></span>
          <span>{{ siteName }}</span>
        </router-link>
        <div class="compact-home-actions">
          <LocaleSwitcher />
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="home-icon-button" :title="copy.chrome.docs">
            <Icon name="book" size="sm" />
          </a>
          <router-link v-if="showModelPlazaEntry" to="/model-plaza" class="home-quiet-link">
            <Icon name="grid" size="sm" />
            <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
          </router-link>
          <button type="button" class="home-icon-button" :title="isDark ? copy.chrome.light : copy.chrome.dark" @click="toggleTheme">
            <Icon v-if="isDark" name="sun" size="sm" />
            <Icon v-else name="moon" size="sm" />
          </button>
          <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="home-solid-button">
            {{ isAuthenticated ? copy.chrome.dashboard : copy.chrome.login }}
          </router-link>
        </div>
      </nav>
    </header>
    <main class="compact-home-main">
      <div class="compact-home-content">
        <span class="home-overline">MODURELAY</span>
        <div class="compact-home-logo brand-mark brand-mark-large">
          <img :src="siteLogo || brand.logo" :alt="siteName" />
        </div>
        <h1>{{ siteName }}</h1>
        <p>{{ siteSubtitle }}</p>
        <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="home-primary-button">
          {{ isAuthenticated ? copy.contact.dashboardCta : copy.contact.primaryCta }}
          <Icon name="arrowRight" size="sm" />
        </router-link>
      </div>
    </main>
    <footer class="compact-home-footer">&copy; {{ currentYear }} {{ siteName }}</footer>
  </div>

  <div
    v-else
    ref="pageRef"
    class="home-page kinetic-home"
    :class="{ 'kinetic-home-ready': !loaderVisible, 'kinetic-home-scrolled': scrollProgress > 2 }"
    @pointermove="updatePointerVars"
  >
    <a class="kinetic-skip-link" href="#kinetic-main">{{ copy.chrome.skip }}</a>

    <div class="kinetic-world" aria-hidden="true">
      <HomeHeroScene :progress="sceneProgress" @ready="handleSceneReady" />
    </div>

    <Transition name="kinetic-loader">
      <div v-if="loaderVisible" class="kinetic-loader" role="status" :aria-label="copy.chrome.loading">
        <div class="kinetic-loader-shell">
          <i
            v-for="tick in 25"
            :key="tick"
            class="kinetic-loader-tick"
            :style="loaderTickStyle(tick - 1)"
          ></i>
          <span>{{ String(Math.round(loaderProgress)).padStart(2, '0') }}</span>
        </div>
        <p>{{ copy.chrome.loading }}</p>
      </div>
    </Transition>

    <header class="kinetic-header">
      <button type="button" class="kinetic-wordmark" :aria-label="copy.nav.home" @click="scrollToSection('home')">
        <span class="kinetic-wordmark-mark">
          <img v-if="siteLogo" :src="siteLogo" :alt="siteName" />
          <span v-else>M</span>
        </span>
        <span class="kinetic-wordmark-copy">
          <strong>{{ siteName }}</strong>
          <small>RELAY SYSTEMS</small>
        </span>
      </button>

      <nav class="kinetic-top-nav" :aria-label="copy.chrome.navigation">
        <button
          type="button"
          :class="{ 'is-current': activeSection === 'work' }"
          :aria-current="activeSection === 'work' ? 'page' : undefined"
          @click="scrollToSection('work')"
        >
          {{ copy.chrome.work }}
        </button>
        <span class="kinetic-nav-signal" aria-hidden="true"><i></i></span>
        <button
          type="button"
          :class="{ 'is-current': activeSection === 'contact' }"
          :aria-current="activeSection === 'contact' ? 'page' : undefined"
          @click="scrollToSection('contact')"
        >
          {{ copy.chrome.contact }}
        </button>
      </nav>
    </header>

    <aside class="kinetic-section-index" aria-hidden="true">
      <span>{{ String(activeSectionIndex + 1).padStart(2, '0') }}</span>
      <i><b :style="{ transform: 'scaleY(' + sectionRailProgress + ')' }"></b></i>
      <span>{{ String(navigationItems.length).padStart(2, '0') }}</span>
    </aside>

    <main id="kinetic-main">
      <section id="home" data-home-section class="kinetic-section kinetic-hero" aria-label="ModuRelay">
        <div class="kinetic-hero-meta kinetic-reveal">
          <span>MODURELAY / SIGNAL 01</span>
          <p>{{ copy.hero.signal }}</p>
        </div>
        <div class="kinetic-hero-instruction kinetic-reveal">
          <span>{{ copy.chrome.move }}</span>
          <i></i>
          <span>{{ copy.chrome.scroll }}</span>
        </div>
        <span class="kinetic-hero-coordinate kinetic-reveal" aria-hidden="true">30.2° N / 120.2° E</span>
      </section>

      <section id="manifesto" data-home-section class="kinetic-section kinetic-manifesto" aria-labelledby="kinetic-manifesto-title">
        <div class="kinetic-manifesto-sticky">
          <div class="kinetic-manifesto-wash" aria-hidden="true"></div>
          <div class="kinetic-manifesto-layout">
            <div class="kinetic-manifesto-heading kinetic-reveal">
              <span class="kinetic-section-label">01 / {{ copy.nav.manifesto }}</span>
              <h1 id="kinetic-manifesto-title" :data-text="copy.manifesto.ariaTitle">
                <span>{{ copy.manifesto.lineOne }}</span>
                <span>{{ copy.manifesto.lineTwo }}</span>
                <span>{{ copy.manifesto.lineThree }}</span>
              </h1>
            </div>
            <div class="kinetic-manifesto-copy kinetic-reveal">
              <span>{{ copy.manifesto.eyebrow }}</span>
              <p>{{ copy.manifesto.description }}</p>
              <p>{{ copy.manifesto.detail }}</p>
              <code>{{ apiBaseUrl }}/v1/chat/completions</code>
            </div>
          </div>
        </div>
      </section>

      <section id="work" ref="workSectionRef" data-home-section class="kinetic-section kinetic-work" aria-labelledby="kinetic-work-title">
        <div class="kinetic-work-sticky">
          <div class="kinetic-work-heading kinetic-reveal">
            <span class="kinetic-section-label">02 / {{ copy.nav.work }}</span>
            <h2 id="kinetic-work-title">{{ copy.work.title }}</h2>
          </div>

          <div class="kinetic-deck" data-testid="kinetic-work-deck" :aria-label="copy.work.deckLabel">
            <article
              v-for="(card, index) in workCards"
              :key="card.key"
              class="kinetic-card"
              :class="['kinetic-card-' + card.key, { 'is-active': index === activeCardIndex }]"
              :style="cardStyle(index)"
              :aria-hidden="Math.abs(index - deckPosition) > 1.45 ? 'true' : 'false'"
            >
              <div class="kinetic-card-visual" aria-hidden="true">
                <span class="kinetic-card-plane kinetic-card-plane-one"></span>
                <span class="kinetic-card-plane kinetic-card-plane-two"></span>
                <span class="kinetic-card-plane kinetic-card-plane-three"></span>
                <span class="kinetic-card-orbit"></span>
                <span class="kinetic-card-scan"></span>
              </div>
              <div class="kinetic-card-topline">
                <span>{{ String(index + 1).padStart(2, '0') }}</span>
                <span>{{ card.eyebrow }}</span>
              </div>
              <div class="kinetic-card-content">
                <h3>{{ card.title }}</h3>
                <p>{{ card.description }}</p>
                <router-link :to="card.to" :tabindex="index === activeCardIndex ? 0 : -1">
                  {{ card.action }}<Icon name="arrowRight" size="sm" />
                </router-link>
              </div>
            </article>
          </div>

          <div class="kinetic-work-filter">
            <span>{{ copy.work.question }}</span>
            <button
              v-for="(card, index) in workCards"
              :key="card.key"
              type="button"
              :class="{ 'is-active': index === activeCardIndex }"
              @click="selectCard(index)"
            >
              <i></i>{{ card.short }}
            </button>
          </div>

          <div class="kinetic-deck-progress" aria-hidden="true">
            <span>{{ String(activeCardIndex + 1).padStart(2, '0') }}</span>
            <i><b :style="{ transform: 'scaleX(' + deckRailProgress + ')' }"></b></i>
            <span>{{ String(workCards.length).padStart(2, '0') }}</span>
          </div>
        </div>
      </section>

      <section id="lab" data-home-section class="kinetic-section kinetic-lab" aria-labelledby="kinetic-lab-title">
        <div class="kinetic-lab-sticky">
          <div class="kinetic-lab-water" aria-hidden="true"></div>
          <div class="kinetic-lab-shell kinetic-reveal">
            <div class="kinetic-lab-mark" aria-hidden="true"><span>M</span></div>
            <h2 id="kinetic-lab-title">{{ copy.lab.title }}</h2>
            <i class="kinetic-lab-arrow" aria-hidden="true"></i>
            <div class="kinetic-lab-copy">
              <span>{{ copy.lab.eyebrow }}</span>
              <p>{{ copy.lab.description }}</p>
              <router-link :to="learningEntry">{{ copy.lab.action }}<Icon name="arrowRight" size="sm" /></router-link>
            </div>
          </div>
        </div>
      </section>

      <section id="contact" data-home-section class="kinetic-section kinetic-contact" aria-labelledby="kinetic-contact-title">
        <div class="kinetic-contact-content kinetic-reveal">
          <span class="kinetic-section-label">04 / {{ copy.nav.contact }}</span>
          <h2 id="kinetic-contact-title">
            <span>{{ copy.contact.lineOne }}</span>
            <span>{{ copy.contact.lineTwo }}</span>
          </h2>
          <p>{{ copy.contact.description }}</p>
          <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="kinetic-primary-cta">
            {{ isAuthenticated ? copy.contact.dashboardCta : copy.contact.primaryCta }}
            <Icon name="arrowRight" size="sm" />
          </router-link>
        </div>

        <footer class="kinetic-footer">
          <span>&copy; {{ currentYear }} {{ siteName }}</span>
          <div class="kinetic-footer-links">
            <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">{{ copy.chrome.docs }}</a>
            <router-link v-if="showModelPlazaEntry" to="/model-plaza">{{ t('nav.modelPlaza') }}</router-link>
            <router-link to="/key-usage">{{ copy.chrome.usage }}</router-link>
            <LocaleSwitcher />
            <button type="button" @click="toggleTheme">{{ isDark ? copy.chrome.light : copy.chrome.dark }}</button>
          </div>
          <span>MODURELAY / SYSTEM 01</span>
        </footer>
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch, type CSSProperties } from 'vue'
import { useI18n } from 'vue-i18n'
import { gsap } from 'gsap'
import { brand } from '@/config/brand'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import HomeHeroScene from '@/components/home/HomeHeroScene.vue'
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import { toggleThemeWithTransition } from '@/utils/themeTransition'

type SectionId = 'home' | 'manifesto' | 'work' | 'lab' | 'contact'

const zhCopy = {
  chrome: {
    loading: '正在构建信号场', navigation: '首页导航', work: '作品', contact: '联系',
    docs: '文档', usage: '用量查询', dashboard: '控制台', login: '登录',
    light: '浅色模式', dark: '深色模式', skip: '跳到主要内容',
    move: '移动指针扰动信号', scroll: '滚动进入系统'
  },
  nav: { home: '首页', manifesto: '系统宣言', work: '核心能力', lab: '中继实验室', contact: '开始使用' },
  hero: { signal: '一个持续响应指针与请求流的中继生命体。' },
  manifesto: {
    ariaTitle: '可靠的 AI 接口体验',
    lineOne: '可靠的',
    lineTwo: 'AI 接口',
    lineThree: '体验',
    eyebrow: '一个入口 / 多条路径',
    description: 'ModuRelay 把兼容接口、模型发现与供应商路由汇入同一条调用路径。',
    detail: '策略、故障切换、用量和请求上下文保持可见，让应用不必感知底层线路变化。'
  },
  work: {
    title: '运行中的系统', deckLabel: 'ModuRelay 核心能力空间卡片', question: '你要进入哪一层？',
    cards: [
      { title: '统一入口', short: 'API', eyebrow: 'OPENAI COMPATIBLE', description: '保留熟悉的 SDK 与请求结构，把模型接入集中到一个 base URL。', action: '开始接入' },
      { title: '策略路由', short: 'ROUTING', eyebrow: 'POLICY ENGINE', description: '依据模型、分组和线路健康选择可用路径，并遵循已配置的故障切换。', action: '查看模型' },
      { title: '请求信号', short: 'SIGNALS', eyebrow: 'USAGE + TRACE', description: '把密钥、模型、Token 用量与线路结果放回同一套查询上下文。', action: '查询用量' },
      { title: 'AI 实践', short: 'LEARNING', eyebrow: 'AGENT + WORKFLOW', description: '围绕 Agent、工作流、记忆与具身智能整理案例、练习和学习路径。', action: '进入学习' },
      { title: '运行控制', short: 'CONTROL', eyebrow: 'OPERATIONS', description: '在控制台维护账户池、策略与系统配置，让转发链路持续可控。', action: '打开控制台' }
    ]
  },
  lab: {
    title: '中继实验室', eyebrow: '从原型进入实践',
    description: '探索 Agent、工作流和 AI 系统能力，并把实验带回可运行的应用链路。',
    action: '进入 AI 学习'
  },
  contact: {
    lineOne: '一个入口',
    lineTwo: '连接每个模型',
    description: '从兼容端点开始，把模型、路由、账户和用量带回同一条可靠链路。',
    primaryCta: '开始使用',
    dashboardCta: '进入控制台'
  }
}

const enCopy = {
  chrome: {
    loading: 'Building the signal field', navigation: 'Homepage navigation', work: 'Work', contact: 'Contact',
    docs: 'Docs', usage: 'Usage', dashboard: 'Dashboard', login: 'Sign in',
    light: 'Light mode', dark: 'Dark mode', skip: 'Skip to main content',
    move: 'Move to disturb the signal', scroll: 'Scroll into the system'
  },
  nav: { home: 'Home', manifesto: 'System manifesto', work: 'Core systems', lab: 'Relay lab', contact: 'Get started' },
  hero: { signal: 'A relay organism that responds to pointers and request flow.' },
  manifesto: {
    ariaTitle: 'Reliable AI API experiences',
    lineOne: 'Reliable',
    lineTwo: 'AI API',
    lineThree: 'Experiences',
    eyebrow: 'One entry / many paths',
    description: 'ModuRelay brings compatible APIs, model discovery and provider routing into one request path.',
    detail: 'Policy, failover, usage and request context stay visible while applications remain independent from the underlying route.'
  },
  work: {
    title: 'Systems in motion', deckLabel: 'ModuRelay core system cards', question: 'Which layer are you looking for?',
    cards: [
      { title: 'Unified entry', short: 'API', eyebrow: 'OPENAI COMPATIBLE', description: 'Keep familiar SDKs and request shapes while models converge on one base URL.', action: 'Start integrating' },
      { title: 'Policy routing', short: 'ROUTING', eyebrow: 'POLICY ENGINE', description: 'Choose an available path from model, group and route health, then follow configured failover.', action: 'Browse models' },
      { title: 'Request signals', short: 'SIGNALS', eyebrow: 'USAGE + TRACE', description: 'Keep key, model, token usage and route outcome in one queryable operating context.', action: 'Inspect usage' },
      { title: 'AI practice', short: 'LEARNING', eyebrow: 'AGENT + WORKFLOW', description: 'Turn agents, workflows, memory and embodied AI into cases, exercises and learning paths.', action: 'Enter learning' },
      { title: 'Operational control', short: 'CONTROL', eyebrow: 'OPERATIONS', description: 'Maintain account pools, policy and system settings from one operational surface.', action: 'Open dashboard' }
    ]
  },
  lab: {
    title: 'The Relay Lab', eyebrow: 'From prototype to practice',
    description: 'Explore agents, workflows and AI system capability, then bring experiments back to a runnable application path.',
    action: 'Enter AI Learning'
  },
  contact: {
    lineOne: 'One entry',
    lineTwo: 'Every model',
    description: 'Start with a compatible endpoint and bring models, routing, accounts and usage into one reliable path.',
    primaryCta: 'Get started',
    dashboardCta: 'Open dashboard'
  }
}

const { t, locale } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const pageRef = ref<HTMLElement | null>(null)
const workSectionRef = ref<HTMLElement | null>(null)
const activeSection = ref<SectionId>('home')
const scrollProgress = ref(0)
const deckPosition = ref(0)
const activeCardIndex = ref(0)
const isDark = ref(document.documentElement.classList.contains('dark'))
const loaderVisible = ref(true)
const loaderProgress = ref(4)

let sectionObserver: IntersectionObserver | null = null
let revealObserver: IntersectionObserver | null = null
let homeMatchMedia: ReturnType<typeof gsap.matchMedia> | null = null
let scrollFrame = 0
let loaderTimer = 0
let loaderSafetyTimer = 0
let loaderHideTimer = 0
let homeMounted = false

const copy = computed(() => locale.value === 'zh' ? zhCopy : enCopy)
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || brand.name)
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || brand.slogan)
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)
const isHomeContentUrl = computed(() => /^https?:\/\//.test(homeContent.value.trim()))
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const currentYear = computed(() => new Date().getFullYear())
const modelPlazaEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))
const modelPlazaRequiresAuth = computed(() => appStore.cachedPublicSettings?.model_plaza_require_auth === true)
const showModelPlazaEntry = computed(() => modelPlazaEnabled.value && (isAuthenticated.value || !modelPlazaRequiresAuth.value))
const learningEntry = computed(() => isAuthenticated.value ? '/ai-learning' : { path: '/login', query: { redirect: '/ai-learning' } })
const apiBaseUrl = computed(() => {
  const configured = appStore.cachedPublicSettings?.api_base_url
  return typeof configured === 'string' && configured.trim() ? configured.trim().replace(/\/+$/, '') : window.location.origin
})
const navigationItems = computed(() => (Object.keys(copy.value.nav) as SectionId[]).map(id => ({ id, label: copy.value.nav[id] })))
const activeSectionIndex = computed(() => Math.max(0, navigationItems.value.findIndex(item => item.id === activeSection.value)))
const sceneProgress = computed(() => scrollProgress.value / 100)
const sectionRailProgress = computed(() => Math.max(0.04, Math.min(1, scrollProgress.value / 100)))
const workCards = computed(() => {
  const definitions = copy.value.work.cards
  const loginOrDashboard = isAuthenticated.value ? dashboardPath.value : '/login'
  const modelDestination = showModelPlazaEntry.value ? '/model-plaza' : loginOrDashboard
  return [
    { key: 'gateway', ...definitions[0], to: loginOrDashboard },
    { key: 'routing', ...definitions[1], to: modelDestination },
    { key: 'signals', ...definitions[2], to: '/key-usage' },
    { key: 'learning', ...definitions[3], to: learningEntry.value },
    { key: 'control', ...definitions[4], to: loginOrDashboard }
  ]
})
const deckRailProgress = computed(() => {
  if (workCards.value.length <= 1) return 1
  return Math.max(0.04, Math.min(1, deckPosition.value / (workCards.value.length - 1)))
})

function toggleTheme(event?: MouseEvent) {
  isDark.value = toggleThemeWithTransition(isDark.value, event)
}

function loaderTickStyle(index: number): CSSProperties {
  const angle = -90 + (index / 24) * 180
  return {
    transform: 'rotate(' + angle + 'deg) translateY(-42px)',
    opacity: index / 24 <= loaderProgress.value / 100 ? '1' : '0.14'
  }
}

function cardStyle(index: number): CSSProperties {
  const delta = index - deckPosition.value
  const distance = Math.abs(delta)
  const x = delta * (window.innerWidth < 700 ? 76 : 37)
  const y = Math.sin(index * 1.7) * (window.innerWidth < 700 ? 2.4 : 4.8) + distance * 1.3
  const z = -distance * 390
  const rotateY = delta * -12
  const rotateZ = delta * 1.4
  const scale = Math.max(0.7, 1 - distance * 0.1)
  const opacity = Math.max(0, 1 - Math.max(0, distance - 0.72) * 0.54)
  return {
    '--deck-x': x + 'vw',
    '--deck-y': y + 'vh',
    '--deck-z': z + 'px',
    '--deck-ry': rotateY + 'deg',
    '--deck-rz': rotateZ + 'deg',
    '--deck-scale': String(scale),
    '--deck-opacity': String(opacity),
    '--deck-order': String(100 - Math.round(distance * 10))
  } as CSSProperties
}

function scrollToSection(id: SectionId) {
  document.getElementById(id)?.scrollIntoView({
    behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth',
    block: 'start'
  })
}

function selectCard(index: number) {
  const work = workSectionRef.value
  activeCardIndex.value = index
  deckPosition.value = index
  if (!work) return
  const scrollSpan = Math.max(1, work.offsetHeight - window.innerHeight)
  const destination = work.offsetTop + (index / Math.max(1, workCards.value.length - 1)) * scrollSpan
  window.scrollTo({
    top: destination,
    behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth'
  })
}

function updatePointerVars(event: PointerEvent) {
  const root = pageRef.value
  if (!root) return
  const pointerX = event.clientX / Math.max(1, window.innerWidth) - 0.5
  const pointerY = event.clientY / Math.max(1, window.innerHeight) - 0.5
  root.style.setProperty('--pointer-shift-x', pointerX * 22 + 'px')
  root.style.setProperty('--pointer-shift-y', pointerY * 14 + 'px')
  root.style.setProperty('--pointer-tilt-x', pointerY * -6 + 'deg')
  root.style.setProperty('--pointer-tilt-y', pointerX * 8 + 'deg')
}

function updateScrollState() {
  cancelAnimationFrame(scrollFrame)
  scrollFrame = requestAnimationFrame(() => {
    const scrollTop = window.scrollY || document.documentElement.scrollTop
    const scrollable = Math.max(document.documentElement.scrollHeight - window.innerHeight, 1)
    scrollProgress.value = Math.min(100, Math.max(0, scrollTop / scrollable * 100))
    const work = workSectionRef.value
    if (work) {
      const workSpan = Math.max(1, work.offsetHeight - window.innerHeight)
      const local = Math.min(1, Math.max(0, (scrollTop - work.offsetTop) / workSpan))
      deckPosition.value = local * Math.max(0, workCards.value.length - 1)
      activeCardIndex.value = Math.round(deckPosition.value)
    }
  })
}

function clearLoaderTimers() {
  window.clearInterval(loaderTimer)
  window.clearTimeout(loaderSafetyTimer)
  window.clearTimeout(loaderHideTimer)
  loaderTimer = 0
  loaderSafetyTimer = 0
  loaderHideTimer = 0
}

function beginLoader() {
  clearLoaderTimers()
  loaderVisible.value = true
  loaderProgress.value = 4
  loaderTimer = window.setInterval(() => {
    loaderProgress.value = Math.min(94, loaderProgress.value + Math.max(1, (96 - loaderProgress.value) * 0.07))
  }, 72)
  loaderSafetyTimer = window.setTimeout(handleSceneReady, 7000)
}

function handleSceneReady() {
  window.clearInterval(loaderTimer)
  window.clearTimeout(loaderSafetyTimer)
  loaderTimer = 0
  loaderSafetyTimer = 0
  loaderProgress.value = 100
  loaderHideTimer = window.setTimeout(() => {
    loaderVisible.value = false
    loaderHideTimer = 0
  }, window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 0 : 520)
}

function initializeMotion() {
  const root = pageRef.value
  if (!root) return
  const sections = Array.from(root.querySelectorAll<HTMLElement>('[data-home-section]'))
  if ('IntersectionObserver' in window) {
    sectionObserver = new IntersectionObserver((entries) => {
      const visible = entries.filter(entry => entry.isIntersecting).sort((a, b) => b.intersectionRatio - a.intersectionRatio)
      if (visible[0]?.target.id) activeSection.value = visible[0].target.id as SectionId
    }, { rootMargin: '-28% 0px -48% 0px', threshold: [0.05, 0.18, 0.42, 0.68] })
    sections.forEach(section => sectionObserver?.observe(section))
  }

  const reveals = Array.from(root.querySelectorAll<HTMLElement>('.kinetic-reveal'))
  const motionQuery = window.matchMedia('(prefers-reduced-motion: reduce)')
  if (motionQuery.matches || typeof motionQuery.addEventListener !== 'function') {
    gsap.set(reveals, { autoAlpha: 1, y: 0 })
    return
  }
  homeMatchMedia = gsap.matchMedia()
  homeMatchMedia.add('(prefers-reduced-motion: no-preference)', () => {
    const hero = Array.from(root.querySelectorAll<HTMLElement>('.kinetic-hero .kinetic-reveal'))
    gsap.timeline({ defaults: { ease: 'power3.out' } })
      .fromTo(hero, { autoAlpha: 0, y: 18 }, { autoAlpha: 1, y: 0, duration: 0.88, stagger: 0.08 })
    if ('IntersectionObserver' in window) {
      revealObserver = new IntersectionObserver((entries, observer) => {
        entries.forEach((entry) => {
          if (!entry.isIntersecting || entry.target.closest('.kinetic-hero')) return
          gsap.fromTo(entry.target, { autoAlpha: 0, y: 34 }, {
            autoAlpha: 1, y: 0, duration: 0.82, ease: 'power3.out'
          })
          observer.unobserve(entry.target)
        })
      }, { rootMargin: '0px 0px -15% 0px', threshold: 0.12 })
      reveals.forEach(element => revealObserver?.observe(element))
    } else {
      gsap.set(reveals, { autoAlpha: 1, y: 0 })
    }
    return () => revealObserver?.disconnect()
  })
}

function cleanupMotion() {
  cancelAnimationFrame(scrollFrame)
  window.removeEventListener('scroll', updateScrollState)
  window.removeEventListener('resize', updateScrollState)
  sectionObserver?.disconnect()
  revealObserver?.disconnect()
  sectionObserver = null
  revealObserver = null
  homeMatchMedia?.revert()
  homeMatchMedia = null
}

async function syncHomeMode() {
  cleanupMotion()
  activeSection.value = 'home'
  scrollProgress.value = 0
  deckPosition.value = 0
  activeCardIndex.value = 0
  if (hasHomeContent.value || compactHomeEnabled.value) {
    clearLoaderTimers()
    loaderVisible.value = false
    return
  }
  beginLoader()
  await nextTick()
  if (hasHomeContent.value || compactHomeEnabled.value) return
  initializeMotion()
  updateScrollState()
  window.addEventListener('scroll', updateScrollState, { passive: true })
  window.addEventListener('resize', updateScrollState, { passive: true })
}

watch([hasHomeContent, compactHomeEnabled], () => {
  if (homeMounted) void syncHomeMode()
})

onMounted(() => {
  homeMounted = true
  isDark.value = document.documentElement.classList.contains('dark')
  authStore.checkAuth()
  if (!appStore.publicSettingsLoaded) appStore.fetchPublicSettings()
  void syncHomeMode()
})

onBeforeUnmount(() => {
  homeMounted = false
  cleanupMotion()
  clearLoaderTimers()
})
</script>

<style scoped>
.compact-home { background: var(--mr-canvas); color: var(--mr-text); }
.compact-home-header { border-bottom: 1px solid var(--mr-border); background: color-mix(in srgb, var(--mr-surface) 78%, transparent); backdrop-filter: blur(18px) saturate(135%); }
.compact-home-nav { display: flex; align-items: center; justify-content: space-between; gap: 16px; width: min(100% - 32px, 1180px); min-height: 64px; margin: 0 auto; }
.compact-home-brand { display: inline-flex; min-width: 0; align-items: center; gap: 10px; color: var(--mr-text); font-size: 15px; font-weight: 650; }
.compact-home-brand > span:last-child { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.compact-home-actions { display: flex; align-items: center; gap: 5px; }
.compact-home-main { display: grid; min-height: min(70vh, 720px); place-items: center; padding: 64px 16px; }
.compact-home-content { max-width: 580px; text-align: center; }
.compact-home-content h1 { margin: 18px 0 10px; font-size: clamp(30px, 6vw, 48px); font-weight: 680; letter-spacing: -0.03em; }
.compact-home-content p { margin: 0 auto 28px; max-width: 520px; color: var(--mr-text-muted); line-height: 1.7; white-space: pre-wrap; }
.compact-home-logo { margin: 16px auto 0; }
.compact-home-footer { padding: 20px 16px; border-top: 1px solid var(--mr-border); color: var(--mr-text-subtle); font-size: 12px; text-align: center; }
.brand-mark { display: grid; width: 30px; height: 30px; flex: 0 0 auto; place-items: center; overflow: hidden; border: 1px solid var(--mr-primary-border); border-radius: 8px; background: var(--mr-primary); }
.brand-mark-large { width: 64px; height: 64px; border-radius: 16px; }
.brand-mark img { width: 100%; height: 100%; object-fit: contain; }
.home-icon-button, .home-quiet-link { display: inline-flex; min-width: 36px; min-height: 36px; align-items: center; justify-content: center; gap: 6px; padding: 0 9px; border: 1px solid transparent; border-radius: 8px; color: var(--mr-text-muted); font-size: 12px; transition: color var(--motion-fast) var(--ease-standard), background-color var(--motion-fast) var(--ease-standard), border-color var(--motion-fast) var(--ease-standard), transform var(--motion-fast) var(--ease-standard); }
.home-icon-button:hover, .home-quiet-link:hover { border-color: var(--mr-border); color: var(--mr-text); background: var(--mr-surface-subtle); transform: translateY(-1px); }
.home-solid-button, .home-primary-button { display: inline-flex; min-height: 38px; align-items: center; justify-content: center; gap: 7px; padding: 0 14px; border-radius: 8px; color: #fff; background: var(--mr-primary); font-size: 12px; font-weight: 650; transition: background-color var(--motion-fast) var(--ease-standard), transform var(--motion-fast) var(--ease-standard); }
.home-solid-button:hover, .home-primary-button:hover { background: var(--mr-primary-strong); transform: translateY(-1px); }
.home-overline { color: var(--mr-primary); font-size: 11px; font-weight: 700; letter-spacing: 0.14em; }

.kinetic-home {
  --kinetic-ink: #eef6f1;
  --kinetic-muted: rgba(218, 232, 225, 0.67);
  --kinetic-dim: rgba(196, 218, 209, 0.42);
  --kinetic-line: rgba(215, 239, 229, 0.2);
  --kinetic-line-strong: rgba(229, 248, 240, 0.42);
  --kinetic-teal: #75e5d9;
  --kinetic-violet: #9b8cff;
  --pointer-shift-x: 0px;
  --pointer-shift-y: 0px;
  --pointer-tilt-x: 0deg;
  --pointer-tilt-y: 0deg;
  position: relative;
  width: 100%;
  max-width: 100vw;
  min-height: 100vh;
  overflow: clip;
  color: var(--kinetic-ink);
  background: #04090b;
  font-family: "Noto Sans SC", ui-sans-serif, system-ui, sans-serif;
}
.kinetic-world {
  position: fixed;
  inset: 0;
  z-index: 0;
  pointer-events: none;
}
.kinetic-skip-link {
  position: fixed;
  top: 12px;
  left: 12px;
  z-index: 120;
  padding: 10px 14px;
  color: #041012;
  background: #eafff7;
  font-size: 12px;
  transform: translateY(-160%);
  transition: transform var(--motion-fast) var(--ease-enter);
}
.kinetic-skip-link:focus { transform: translateY(0); }
.kinetic-header {
  position: fixed;
  top: 0;
  right: 0;
  left: 0;
  z-index: 55;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  padding: clamp(20px, 3vw, 42px) clamp(20px, 3.6vw, 58px);
  pointer-events: none;
}
.kinetic-wordmark,
.kinetic-top-nav {
  pointer-events: auto;
}
.kinetic-wordmark {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  color: var(--kinetic-ink);
  opacity: 0.72;
  text-align: left;
  transition: opacity var(--motion-fast) var(--ease-standard), transform var(--motion-fast) var(--ease-standard);
}
.kinetic-wordmark:hover { opacity: 1; transform: translateY(-1px); }
.kinetic-wordmark-mark {
  display: grid;
  width: 28px;
  height: 28px;
  place-items: center;
  overflow: hidden;
  border: 1px solid var(--kinetic-line-strong);
  border-radius: 50%;
  font: 650 10px/1 ui-monospace, SFMono-Regular, Menlo, monospace;
}
.kinetic-wordmark-mark img { width: 100%; height: 100%; object-fit: contain; }
.kinetic-wordmark-copy { display: flex; flex-direction: column; gap: 3px; }
.kinetic-wordmark-copy strong { font: 620 10px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.08em; }
.kinetic-wordmark-copy small { color: var(--kinetic-dim); font: 600 6px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.14em; }
.kinetic-top-nav {
  position: relative;
  display: flex;
  min-width: 194px;
  height: 40px;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 0 17px;
  border: 1px solid var(--kinetic-line);
  border-radius: 999px;
  background: rgba(3, 9, 11, 0.7);
  box-shadow: inset 0 1px rgba(255, 255, 255, 0.05), 0 14px 34px rgba(0, 0, 0, 0.24);
  backdrop-filter: blur(18px) saturate(120%);
}
.kinetic-top-nav::after {
  position: absolute;
  right: 14%;
  bottom: -11px;
  left: 14%;
  height: 20px;
  border-radius: 50%;
  background: rgba(101, 220, 190, 0.15);
  filter: blur(10px);
  content: '';
  pointer-events: none;
}
.kinetic-top-nav button {
  position: relative;
  z-index: 1;
  color: var(--kinetic-muted);
  font: 560 9px/1 ui-monospace, SFMono-Regular, Menlo, monospace;
  letter-spacing: 0.07em;
  text-transform: uppercase;
  transition: color var(--motion-fast) var(--ease-standard);
}
.kinetic-top-nav button:hover,
.kinetic-top-nav button.is-current { color: #fff; }
.kinetic-nav-signal {
  position: relative;
  z-index: 1;
  display: block;
  width: 44px;
  height: 10px;
  overflow: hidden;
}
.kinetic-nav-signal::before {
  position: absolute;
  top: 50%;
  right: 0;
  left: 0;
  height: 1px;
  background: var(--kinetic-line-strong);
  content: '';
}
.kinetic-nav-signal i {
  position: absolute;
  top: 4px;
  left: 0;
  width: 18px;
  height: 2px;
  border-radius: 50%;
  background: #dfffee;
  filter: blur(0.2px) drop-shadow(0 0 4px rgba(159, 255, 223, 0.6));
  animation: kinetic-signal 2.6s ease-in-out infinite;
}
@keyframes kinetic-signal {
  0%, 100% { transform: translateX(0) scaleX(0.35); }
  50% { transform: translateX(26px) scaleX(1); }
}
.kinetic-header button:focus-visible,
.kinetic-card a:focus-visible,
.kinetic-work-filter button:focus-visible,
.kinetic-lab-shell a:focus-visible,
.kinetic-contact a:focus-visible,
.kinetic-footer a:focus-visible,
.kinetic-footer button:focus-visible {
  outline: 2px solid var(--kinetic-teal);
  outline-offset: 4px;
}
.kinetic-section-index {
  position: fixed;
  top: 50%;
  right: clamp(16px, 2.2vw, 36px);
  z-index: 45;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  color: var(--kinetic-dim);
  font: 560 7px/1 ui-monospace, SFMono-Regular, Menlo, monospace;
  pointer-events: none;
  transform: translateY(-50%);
}
.kinetic-section-index > i {
  position: relative;
  display: block;
  width: 1px;
  height: 62px;
  background: var(--kinetic-line);
}
.kinetic-section-index b {
  position: absolute;
  inset: 0;
  background: var(--kinetic-ink);
  transform-origin: top;
}
.kinetic-section {
  position: relative;
  z-index: 4;
  min-height: 100svh;
  pointer-events: none;
}
.kinetic-section > * { pointer-events: auto; }
.kinetic-hero {
  min-height: 112svh;
}
.kinetic-hero-meta {
  position: absolute;
  bottom: 8vh;
  left: clamp(24px, 6vw, 96px);
  width: min(300px, 30vw);
}
.kinetic-hero-meta > span,
.kinetic-section-label,
.kinetic-manifesto-copy > span,
.kinetic-lab-copy > span {
  color: var(--kinetic-dim);
  font: 580 8px/1.2 ui-monospace, SFMono-Regular, Menlo, monospace;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}
.kinetic-hero-meta p {
  margin: 10px 0 0;
  color: var(--kinetic-muted);
  font-size: 11px;
  line-height: 1.65;
}
.kinetic-hero-instruction {
  position: absolute;
  right: clamp(46px, 7vw, 110px);
  bottom: 8vh;
  display: flex;
  align-items: center;
  gap: 12px;
  color: var(--kinetic-dim);
  font: 560 7px/1 ui-monospace, SFMono-Regular, Menlo, monospace;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}
.kinetic-hero-instruction i {
  display: block;
  width: 56px;
  height: 1px;
  background: var(--kinetic-line-strong);
}
.kinetic-hero-coordinate {
  position: absolute;
  top: 22%;
  left: 50%;
  color: rgba(213, 237, 226, 0.3);
  font: 540 7px/1 ui-monospace, SFMono-Regular, Menlo, monospace;
  letter-spacing: 0.12em;
  transform: translateX(-50%);
}
.kinetic-manifesto {
  min-height: 138svh;
}
.kinetic-manifesto-sticky,
.kinetic-work-sticky,
.kinetic-lab-sticky {
  position: sticky;
  top: 0;
  height: 100svh;
  overflow: hidden;
}
.kinetic-manifesto-wash {
  position: absolute;
  inset: 0;
  background: linear-gradient(180deg, transparent 12%, rgba(6, 22, 23, 0.1) 38%, rgba(4, 10, 12, 0.76) 100%);
  pointer-events: none;
}
.kinetic-manifesto-layout {
  position: absolute;
  right: clamp(48px, 8vw, 138px);
  bottom: clamp(54px, 8vh, 100px);
  left: clamp(38px, 8vw, 138px);
  display: grid;
  grid-template-columns: minmax(0, 1.55fr) minmax(250px, 0.55fr);
  align-items: end;
  gap: clamp(36px, 8vw, 140px);
}
.kinetic-manifesto-heading h1 {
  position: relative;
  margin: 20px 0 0;
  color: #eff6f1;
  font: 480 clamp(58px, 7.6vw, 126px)/0.82 ui-monospace, SFMono-Regular, Menlo, monospace;
  letter-spacing: -0.09em;
  text-transform: uppercase;
  text-shadow: -2px 0 rgba(108, 232, 220, 0.2), 2px 0 rgba(157, 129, 255, 0.18);
}
.kinetic-manifesto-heading h1 span { display: block; }
.kinetic-manifesto-heading h1 span:first-child {
  position: relative;
  transform: translateX(var(--pointer-shift-x));
}
.kinetic-manifesto-heading h1 span:nth-child(2) { transform: translateX(8%); }
.kinetic-manifesto-heading h1 span:last-child { transform: translateX(1.5%); }
.kinetic-manifesto-heading h1::after {
  position: absolute;
  top: 30%;
  right: 0;
  left: 0;
  height: 13%;
  color: rgba(228, 248, 239, 0.52);
  overflow: hidden;
  content: attr(data-text);
  filter: blur(0.4px);
  transform: translateX(var(--pointer-shift-x)) skewX(-7deg);
  white-space: nowrap;
  mix-blend-mode: screen;
}
.kinetic-manifesto-copy {
  padding-bottom: 2vh;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}
.kinetic-manifesto-copy p {
  margin: 18px 0 0;
  color: var(--kinetic-muted);
  font-size: clamp(10px, 0.9vw, 13px);
  line-height: 1.62;
}
.kinetic-manifesto-copy code {
  display: block;
  overflow: hidden;
  margin-top: 24px;
  padding-top: 14px;
  border-top: 1px solid var(--kinetic-line);
  color: rgba(151, 235, 217, 0.62);
  font-size: 8px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.kinetic-work {
  min-height: 460svh;
}
.kinetic-work-heading {
  position: absolute;
  top: clamp(88px, 12vh, 138px);
  left: clamp(28px, 4.8vw, 78px);
  z-index: 140;
}
.kinetic-work-heading h2 {
  margin: 9px 0 0;
  color: var(--kinetic-ink);
  font: 470 clamp(22px, 2.5vw, 40px)/1 ui-monospace, SFMono-Regular, Menlo, monospace;
  letter-spacing: -0.06em;
  text-transform: uppercase;
}
.kinetic-deck {
  position: absolute;
  inset: 0;
  perspective: 1300px;
  transform-style: preserve-3d;
}
.kinetic-card {
  --deck-x: 0vw;
  --deck-y: 0vh;
  --deck-z: 0px;
  --deck-ry: 0deg;
  --deck-rz: 0deg;
  --deck-scale: 1;
  --deck-opacity: 1;
  --deck-order: 100;
  position: absolute;
  top: 50%;
  left: 50%;
  z-index: var(--deck-order);
  display: flex;
  width: min(41vw, 610px);
  height: min(49vh, 490px);
  min-height: 330px;
  flex-direction: column;
  justify-content: space-between;
  overflow: hidden;
  padding: clamp(20px, 2.4vw, 34px);
  border: 1px solid rgba(232, 247, 240, 0.24);
  border-radius: clamp(18px, 2.2vw, 32px);
  color: #f5fbf8;
  background: rgba(8, 14, 18, 0.62);
  box-shadow: inset 0 1px rgba(255, 255, 255, 0.08), 0 38px 90px rgba(0, 0, 0, 0.34);
  opacity: var(--deck-opacity);
  backdrop-filter: blur(13px) saturate(134%);
  transform: translate3d(calc(-50% + var(--deck-x)), calc(-50% + var(--deck-y)), var(--deck-z)) rotateY(var(--deck-ry)) rotateZ(var(--deck-rz)) scale(var(--deck-scale));
  transform-style: preserve-3d;
  will-change: transform, opacity;
}
.kinetic-card::after {
  position: absolute;
  inset: 0;
  opacity: 0.11;
  background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 180 180' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='.76' numOctaves='2'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)'/%3E%3C/svg%3E");
  content: '';
  mix-blend-mode: overlay;
  pointer-events: none;
}
.kinetic-card.is-active { border-color: rgba(231, 250, 241, 0.48); }
.kinetic-card-visual {
  position: absolute;
  inset: 0;
  overflow: hidden;
  background:
    radial-gradient(circle at 50% 42%, rgba(103, 225, 210, 0.32), transparent 23%),
    radial-gradient(circle at 12% 88%, rgba(139, 113, 255, 0.36), transparent 34%),
    #0a1417;
}
.kinetic-card-routing .kinetic-card-visual { filter: hue-rotate(38deg); }
.kinetic-card-signals .kinetic-card-visual { filter: hue-rotate(280deg) saturate(1.25); }
.kinetic-card-learning .kinetic-card-visual { filter: hue-rotate(112deg) saturate(1.2); }
.kinetic-card-control .kinetic-card-visual { filter: hue-rotate(210deg) saturate(0.82); }
.kinetic-card-plane {
  position: absolute;
  top: 50%;
  left: 50%;
  display: block;
  border: 1px solid rgba(220, 250, 240, 0.38);
  background: rgba(116, 225, 210, 0.09);
  box-shadow: inset 0 0 30px rgba(121, 255, 227, 0.08);
  transform-style: preserve-3d;
}
.kinetic-card-plane-one {
  width: 46%;
  height: 62%;
  transform: translate(-76%, -53%) rotate(-16deg) skewY(8deg);
}
.kinetic-card-plane-two {
  width: 52%;
  height: 52%;
  transform: translate(-14%, -44%) rotate(19deg) skewX(-9deg);
}
.kinetic-card-plane-three {
  width: 20%;
  height: 86%;
  background: rgba(161, 128, 255, 0.12);
  transform: translate(-50%, -52%) rotate(42deg);
}
.kinetic-card-orbit {
  position: absolute;
  top: 50%;
  left: 50%;
  display: block;
  width: 34%;
  aspect-ratio: 1;
  border: 2px solid rgba(239, 255, 247, 0.56);
  border-radius: 50%;
  box-shadow: 0 0 0 9px rgba(111, 222, 211, 0.06), 0 0 42px rgba(151, 255, 231, 0.18);
  transform: translate(-50%, -50%);
}
.kinetic-card-scan {
  position: absolute;
  right: -20%;
  bottom: 17%;
  left: -20%;
  height: 1px;
  background: rgba(239, 255, 248, 0.68);
  box-shadow: 0 0 18px rgba(128, 255, 226, 0.46);
  transform: rotate(-9deg);
}
.kinetic-card-topline,
.kinetic-card-content {
  position: relative;
  z-index: 2;
}
.kinetic-card-topline {
  display: flex;
  justify-content: space-between;
  color: rgba(235, 248, 242, 0.62);
  font: 560 8px/1 ui-monospace, SFMono-Regular, Menlo, monospace;
  letter-spacing: 0.1em;
}
.kinetic-card-content {
  display: flex;
  max-width: 78%;
  flex-direction: column;
  align-items: flex-start;
  padding: 18px;
  background: rgba(2, 8, 10, 0.58);
  backdrop-filter: blur(12px);
}
.kinetic-card-content h3 {
  margin: 0;
  font: 500 clamp(30px, 4vw, 58px)/0.9 ui-monospace, SFMono-Regular, Menlo, monospace;
  letter-spacing: -0.075em;
  text-transform: uppercase;
}
.kinetic-card-content p {
  margin: 15px 0 0;
  color: rgba(224, 239, 232, 0.7);
  font-size: 11px;
  line-height: 1.58;
}
.kinetic-card-content a {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  margin-top: 18px;
  padding-bottom: 4px;
  border-bottom: 1px solid rgba(235, 249, 242, 0.42);
  color: #f4fbf7;
  font: 560 8px/1 ui-monospace, SFMono-Regular, Menlo, monospace;
  letter-spacing: 0.07em;
}
.kinetic-work-filter {
  position: absolute;
  bottom: clamp(28px, 5vh, 64px);
  left: clamp(28px, 4.8vw, 78px);
  z-index: 145;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 9px;
}
.kinetic-work-filter > span {
  margin-bottom: 4px;
  color: var(--kinetic-dim);
  font: 560 7px/1 ui-monospace, SFMono-Regular, Menlo, monospace;
  letter-spacing: 0.09em;
  text-transform: uppercase;
}
.kinetic-work-filter button {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: rgba(213, 231, 222, 0.46);
  font: 540 8px/1 ui-monospace, SFMono-Regular, Menlo, monospace;
  text-transform: uppercase;
  transition: color var(--motion-fast) var(--ease-standard), transform var(--motion-fast) var(--ease-standard);
}
.kinetic-work-filter button i {
  width: 10px;
  height: 1px;
  background: currentColor;
}
.kinetic-work-filter button:hover,
.kinetic-work-filter button.is-active { color: #f4fbf7; transform: translateX(5px); }
.kinetic-deck-progress {
  position: absolute;
  right: clamp(52px, 7vw, 110px);
  bottom: clamp(30px, 5vh, 66px);
  z-index: 145;
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--kinetic-dim);
  font: 560 7px/1 ui-monospace, SFMono-Regular, Menlo, monospace;
}
.kinetic-deck-progress > i {
  position: relative;
  display: block;
  width: 86px;
  height: 1px;
  background: var(--kinetic-line);
}
.kinetic-deck-progress b {
  position: absolute;
  inset: 0;
  background: #eefaf5;
  transform-origin: left;
}
.kinetic-lab {
  min-height: 154svh;
}
.kinetic-lab-water {
  position: absolute;
  top: -9%;
  right: -10%;
  left: -10%;
  height: 36%;
  opacity: 0.38;
  background:
    repeating-radial-gradient(ellipse at 50% 0%, rgba(106, 237, 204, 0.17) 0 1px, transparent 2px 14px);
  filter: blur(1px);
  transform: perspective(600px) rotateX(62deg) translateY(var(--pointer-shift-y));
}
.kinetic-lab-shell {
  position: absolute;
  top: 50%;
  left: 50%;
  display: grid;
  width: min(72vw, 1080px);
  min-height: min(48vh, 500px);
  grid-template-columns: 0.45fr 0.9fr auto 1fr;
  align-items: center;
  gap: clamp(22px, 4vw, 66px);
  padding: clamp(34px, 5vw, 76px);
  border: 1px solid rgba(229, 246, 238, 0.24);
  border-radius: 44% 38% 42% 46% / 46% 50% 42% 45%;
  color: #eff8f3;
  background:
    radial-gradient(circle, rgba(207, 239, 225, 0.26) 0 2px, transparent 2.7px) 0 0 / 13px 13px,
    rgba(7, 15, 17, 0.68);
  box-shadow: inset 0 0 90px rgba(75, 143, 126, 0.2), 0 44px 100px rgba(0, 0, 0, 0.28);
  backdrop-filter: blur(11px) saturate(128%);
  transform: translate(-50%, -50%) rotateX(var(--pointer-tilt-x)) rotateY(var(--pointer-tilt-y));
}
.kinetic-lab-mark {
  display: grid;
  width: clamp(70px, 8vw, 114px);
  aspect-ratio: 1;
  place-items: center;
  border: 1px solid rgba(239, 255, 247, 0.56);
  border-radius: 50%;
  box-shadow: 0 0 0 11px rgba(151, 247, 222, 0.05);
  font: 500 clamp(20px, 2vw, 32px)/1 ui-monospace, SFMono-Regular, Menlo, monospace;
}
.kinetic-lab-shell h2 {
  margin: 0;
  font: 480 clamp(40px, 5vw, 76px)/0.9 ui-monospace, SFMono-Regular, Menlo, monospace;
  letter-spacing: -0.08em;
  text-transform: uppercase;
}
.kinetic-lab-arrow {
  position: relative;
  width: 54px;
  height: 1px;
  background: rgba(239, 252, 246, 0.5);
}
.kinetic-lab-arrow::after {
  position: absolute;
  top: -3px;
  right: 0;
  width: 7px;
  height: 7px;
  border-top: 1px solid currentColor;
  border-right: 1px solid currentColor;
  content: '';
  transform: rotate(45deg);
}
.kinetic-lab-copy p {
  margin: 15px 0 0;
  color: var(--kinetic-muted);
  font-size: 11px;
  line-height: 1.62;
}
.kinetic-lab-copy a {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  margin-top: 18px;
  padding-bottom: 4px;
  border-bottom: 1px solid var(--kinetic-line-strong);
  color: var(--kinetic-ink);
  font: 560 8px/1 ui-monospace, SFMono-Regular, Menlo, monospace;
}
.kinetic-contact {
  display: flex;
  min-height: 112svh;
  align-items: center;
  justify-content: center;
  text-align: center;
}
.kinetic-contact::before {
  position: absolute;
  inset: 0;
  background: radial-gradient(circle at center, rgba(7, 13, 15, 0.1), rgba(3, 8, 10, 0.74) 84%);
  content: '';
  pointer-events: none;
}
.kinetic-contact-content {
  position: relative;
  z-index: 3;
  display: flex;
  max-width: 1140px;
  flex-direction: column;
  align-items: center;
  padding: 100px 28px;
}
.kinetic-contact-content h2 {
  margin: 24px 0 0;
  font: 470 clamp(62px, 9.2vw, 152px)/0.78 ui-monospace, SFMono-Regular, Menlo, monospace;
  letter-spacing: -0.095em;
  text-transform: uppercase;
}
.kinetic-contact-content h2 span { display: block; }
.kinetic-contact-content h2 span:last-child { color: transparent; -webkit-text-stroke: 1px rgba(235, 249, 242, 0.75); }
.kinetic-contact-content p {
  max-width: 520px;
  margin: 32px 0 0;
  color: var(--kinetic-muted);
  font-size: 13px;
  line-height: 1.65;
}
.kinetic-primary-cta {
  position: relative;
  display: inline-flex;
  width: 132px;
  height: 132px;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin-top: 32px;
  border: 1px solid var(--kinetic-line-strong);
  border-radius: 50%;
  color: #f3fbf7;
  background: rgba(3, 10, 12, 0.54);
  font: 570 9px/1.2 ui-monospace, SFMono-Regular, Menlo, monospace;
  backdrop-filter: blur(15px);
  transition: border-color var(--motion-base) var(--ease-standard), background-color var(--motion-base) var(--ease-standard), transform var(--motion-base) var(--ease-enter);
}
.kinetic-primary-cta::before {
  position: absolute;
  inset: 8px;
  border: 1px solid rgba(119, 231, 215, 0.28);
  border-radius: inherit;
  content: '';
  transition: transform var(--motion-slow) var(--ease-enter);
}
.kinetic-primary-cta:hover { border-color: var(--kinetic-teal); background: rgba(30, 94, 91, 0.56); transform: rotate(-4deg) scale(1.025); }
.kinetic-primary-cta:hover::before { transform: rotate(24deg); }
.kinetic-footer {
  position: absolute;
  right: clamp(24px, 3.6vw, 58px);
  bottom: 28px;
  left: clamp(24px, 3.6vw, 58px);
  z-index: 5;
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  gap: 28px;
  padding-top: 15px;
  border-top: 1px solid var(--kinetic-line);
  color: var(--kinetic-dim);
  font: 550 7px/1.3 ui-monospace, SFMono-Regular, Menlo, monospace;
  letter-spacing: 0.08em;
  text-align: left;
}
.kinetic-footer > span:last-child { text-align: right; }
.kinetic-footer-links {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 16px;
}
.kinetic-footer-links a,
.kinetic-footer-links button { color: var(--kinetic-muted); transition: color var(--motion-fast) var(--ease-standard); }
.kinetic-footer-links a:hover,
.kinetic-footer-links button:hover { color: #fff; }
.kinetic-loader {
  position: fixed;
  inset: 0;
  z-index: 100;
  display: grid;
  place-content: center;
  color: #b8fff0;
  background: #020607;
  pointer-events: none;
}
.kinetic-loader-shell {
  position: relative;
  display: grid;
  width: 118px;
  height: 70px;
  place-items: end center;
}
.kinetic-loader-shell > span {
  position: relative;
  z-index: 2;
  font: 560 11px/1 ui-monospace, SFMono-Regular, Menlo, monospace;
}
.kinetic-loader-tick {
  position: absolute;
  bottom: 0;
  left: 50%;
  width: 1px;
  height: 12px;
  background: #68d9d0;
  transform-origin: 0 42px;
  transition: opacity 100ms linear;
}
.kinetic-loader p {
  margin: 18px 0 0;
  color: rgba(187, 225, 215, 0.46);
  font: 540 7px/1 ui-monospace, SFMono-Regular, Menlo, monospace;
  letter-spacing: 0.12em;
  text-align: center;
  text-transform: uppercase;
}
.kinetic-loader-enter-active,
.kinetic-loader-leave-active { transition: opacity 520ms var(--ease-standard), filter 520ms var(--ease-standard); }
.kinetic-loader-enter-from,
.kinetic-loader-leave-to { opacity: 0; filter: blur(12px); }

@media (max-width: 920px) {
  .kinetic-wordmark-copy { display: none; }
  .kinetic-section-index { display: none; }
  .kinetic-manifesto-layout {
    right: 34px;
    bottom: 72px;
    left: 34px;
    grid-template-columns: 1fr;
    gap: 40px;
  }
  .kinetic-manifesto-copy { max-width: 440px; margin-left: auto; }
  .kinetic-card { width: min(64vw, 600px); }
  .kinetic-lab-shell { width: 82vw; grid-template-columns: auto 1fr; border-radius: 80px; }
  .kinetic-lab-arrow { display: none; }
}

@media (max-width: 640px) {
  .kinetic-header { padding: 16px; }
  .kinetic-top-nav { min-width: 164px; height: 36px; padding: 0 13px; }
  .kinetic-wordmark-mark { width: 34px; height: 34px; }
  .kinetic-hero { min-height: 104svh; }
  .kinetic-hero-meta { bottom: 86px; left: 18px; width: min(260px, 70vw); }
  .kinetic-hero-instruction { right: 18px; bottom: 32px; left: 18px; justify-content: flex-end; }
  .kinetic-hero-instruction span:first-child { display: none; }
  .kinetic-hero-coordinate { top: 18%; }
  .kinetic-manifesto { min-height: 128svh; }
  .kinetic-manifesto-layout { right: 18px; bottom: 72px; left: 18px; gap: 32px; }
  .kinetic-manifesto-heading h1 { font-size: clamp(48px, 16vw, 72px); line-height: 0.84; }
  .kinetic-manifesto-heading h1 span:nth-child(2) { transform: none; }
  .kinetic-manifesto-copy { margin: 0; }
  .kinetic-manifesto-copy p:nth-of-type(2) { display: none; }
  .kinetic-work { min-height: 410svh; }
  .kinetic-work-heading { top: 78px; left: 18px; }
  .kinetic-card {
    width: 82vw;
    height: 54vh;
    min-height: 390px;
    padding: 20px;
  }
  .kinetic-card-content { max-width: 92%; padding: 15px; }
  .kinetic-card-content h3 { font-size: clamp(31px, 10vw, 48px); }
  .kinetic-work-filter {
    right: 18px;
    bottom: 26px;
    left: 18px;
    flex-direction: row;
    flex-wrap: wrap;
    gap: 8px 14px;
  }
  .kinetic-work-filter > span { width: 100%; }
  .kinetic-work-filter button { font-size: 7px; }
  .kinetic-deck-progress { display: none; }
  .kinetic-lab { min-height: 132svh; }
  .kinetic-lab-water { height: 28%; }
  .kinetic-lab-shell {
    width: 88vw;
    min-height: 58vh;
    grid-template-columns: 1fr;
    justify-items: start;
    gap: 20px;
    padding: 32px 26px;
    border-radius: 44px;
  }
  .kinetic-lab-mark { width: 64px; }
  .kinetic-lab-shell h2 { font-size: clamp(38px, 13vw, 58px); }
  .kinetic-contact-content { padding-right: 18px; padding-left: 18px; }
  .kinetic-contact-content h2 { font-size: clamp(54px, 16vw, 76px); line-height: 0.82; }
  .kinetic-primary-cta { width: 108px; height: 108px; }
  .kinetic-footer { right: 18px; bottom: 18px; left: 18px; display: flex; flex-direction: column; align-items: flex-start; gap: 8px; }
  .kinetic-footer-links { flex-wrap: wrap; justify-content: flex-start; }
  .kinetic-footer > span:last-child { display: none; }
  .compact-home-nav { width: min(100% - 20px, 1180px); }
  .compact-home-actions :deep(.locale-switcher) { display: none; }
}

@media (prefers-reduced-motion: reduce) {
  .kinetic-nav-signal i { animation: none; }
  .kinetic-wordmark,
  .kinetic-top-nav button,
  .kinetic-work-filter button,
  .kinetic-primary-cta,
  .kinetic-primary-cta::before,
  .kinetic-footer a,
  .kinetic-footer button,
  .kinetic-loader,
  .kinetic-loader-tick { transition-duration: 1ms !important; }
  .kinetic-manifesto-heading h1::after { display: none; }
  .kinetic-card { will-change: auto; }
  .kinetic-reveal { opacity: 1 !important; visibility: visible !important; transform: none !important; }
  .kinetic-loader { display: none; }
}
</style>
