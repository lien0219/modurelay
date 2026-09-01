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
  >
    <a class="kinetic-skip-link" href="#kinetic-main">{{ copy.chrome.skip }}</a>

    <div class="kinetic-world" aria-hidden="true">
      <HomeHeroScene :progress="sceneProgress" @ready="handleSceneReady" />
    </div>
    <HomeAmbientEffects class="kinetic-ambient" :enabled="ambientEnabled" :progress="sceneProgress" />

    <Transition name="kinetic-loader">
      <div v-if="loaderVisible" class="kinetic-loader" role="status" :aria-label="copy.chrome.loading">
        <div class="kinetic-loader-shell">
          <span class="kinetic-loader-mark" aria-hidden="true">
            <img v-if="siteLogo" :src="siteLogo" alt="" />
            <span v-else>M</span>
          </span>
          <div class="kinetic-loader-status">
            <span>{{ copy.chrome.loading }}</span>
            <strong>{{ String(Math.round(loaderProgress)).padStart(2, '0') }}</strong>
          </div>
          <i class="kinetic-loader-track" aria-hidden="true">
            <b :style="{ transform: 'scaleX(' + loaderProgress / 100 + ')' }"></b>
          </i>
        </div>
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
        <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="kinetic-login-link">
          {{ isAuthenticated ? copy.chrome.dashboard : copy.chrome.login }}
        </router-link>
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
        <div class="kinetic-hero-copy kinetic-reveal">
          <span class="kinetic-hero-eyebrow">{{ copy.hero.eyebrow }}</span>
          <h1>{{ siteName }}</h1>
          <p class="kinetic-hero-tagline">{{ copy.hero.tagline }}</p>
          <p class="kinetic-hero-description">{{ copy.hero.description }}</p>
          <div class="kinetic-hero-actions">
            <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="kinetic-hero-primary">
              {{ isAuthenticated ? copy.contact.dashboardCta : copy.hero.primaryCta }}
              <Icon name="arrowRight" size="sm" />
            </router-link>
            <router-link v-if="showModelPlazaEntry" to="/model-plaza" class="kinetic-hero-secondary">
              {{ copy.hero.secondaryCta }}
            </router-link>
            <a v-else-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="kinetic-hero-secondary">
              {{ copy.chrome.docs }}
            </a>
          </div>
          <code class="kinetic-hero-endpoint">
            <span>{{ copy.hero.endpoint }}</span>
            {{ apiBaseUrl }}/v1/chat/completions
          </code>
        </div>
        <div class="kinetic-hero-meta kinetic-reveal">
          <span>MODURELAY / SIGNAL 01</span>
          <p>{{ copy.hero.signal }}</p>
        </div>
      </section>

      <section id="manifesto" data-home-section class="kinetic-section kinetic-manifesto" aria-labelledby="kinetic-manifesto-title">
        <div class="kinetic-manifesto-sticky">
          <div class="kinetic-manifesto-wash" aria-hidden="true"></div>
          <div class="kinetic-manifesto-layout">
            <div class="kinetic-manifesto-heading kinetic-reveal">
              <span class="kinetic-section-label">01 / {{ copy.nav.manifesto }}</span>
              <h1 id="kinetic-manifesto-title">
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
              :data-card-index="index"
              :style="cardStyle(index)"
              :aria-hidden="Math.abs(index - deckPosition) > 1.45 ? 'true' : 'false'"
              @pointerenter="animateCardParticles"
              @pointerleave="settleCardParticles"
              @focusin="animateCardParticles"
              @focusout="settleCardParticles"
            >
              <div class="kinetic-card-visual" aria-hidden="true">
                <span class="kinetic-card-plane kinetic-card-plane-one"></span>
                <span class="kinetic-card-plane kinetic-card-plane-two"></span>
                <span class="kinetic-card-plane kinetic-card-plane-three"></span>
                <span class="kinetic-card-orbit"></span>
                <span class="kinetic-card-scan"></span>
                <span class="kinetic-card-particles">
                  <i
                    v-for="particleIndex in 12"
                    :key="particleIndex"
                    class="kinetic-card-particle"
                    :style="cardParticleStyle(index, particleIndex - 1)"
                  ></i>
                </span>
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
          <div class="kinetic-lab-shell">
            <div class="kinetic-lab-heading kinetic-reveal">
              <span class="kinetic-section-label">03 / {{ copy.nav.lab }}</span>
              <div class="kinetic-lab-title-row">
                <div class="kinetic-lab-mark" aria-hidden="true"><span>M</span></div>
                <h2 id="kinetic-lab-title">{{ copy.lab.title }}</h2>
              </div>
            </div>
            <div class="kinetic-lab-route kinetic-reveal" aria-hidden="true">
              <span class="kinetic-lab-route-line"></span>
              <i class="kinetic-lab-route-node kinetic-lab-route-node-start"></i>
              <i class="kinetic-lab-route-node kinetic-lab-route-node-middle"></i>
              <i class="kinetic-lab-route-node kinetic-lab-route-node-end"></i>
              <strong>M</strong>
            </div>
            <div class="kinetic-lab-copy kinetic-reveal">
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
            <LocaleSwitcher placement="top-end" />
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
import { ScrollTrigger } from 'gsap/ScrollTrigger'
import { brand } from '@/config/brand'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import HomeHeroScene from '@/components/home/HomeHeroScene.vue'
import HomeAmbientEffects from '@/components/home/HomeAmbientEffects.vue'
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import { toggleThemeWithTransition } from '@/utils/themeTransition'

type SectionId = 'home' | 'manifesto' | 'work' | 'lab' | 'contact'

const zhCopy = {
  chrome: {
    loading: '正在加载中继生态场', navigation: '首页导航', work: '作品', contact: '联系',
    docs: '文档', usage: '用量查询', dashboard: '控制台', login: '登录',
    light: '浅色模式', dark: '深色模式', skip: '跳到主要内容'
  },
  nav: { home: '首页', manifesto: '系统宣言', work: '核心能力', lab: '中继实验室', contact: '开始使用' },
  hero: {
    eyebrow: '企业级的 AI API 网关',
    tagline: '一个入口，连接每个模型。',
    description: '用兼容接口统一模型接入，把策略路由、故障切换与用量记录留在同一条可观测链路中。',
    primaryCta: '开始使用',
    secondaryCta: '查看模型',
    endpoint: '兼容端点',
    signal: '策略路由 / 故障切换 / 用量可见'
  },
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
    loading: 'Loading the relay landscape', navigation: 'Homepage navigation', work: 'Work', contact: 'Contact',
    docs: 'Docs', usage: 'Usage', dashboard: 'Dashboard', login: 'Sign in',
    light: 'Light mode', dark: 'Dark mode', skip: 'Skip to main content'
  },
  nav: { home: 'Home', manifesto: 'System manifesto', work: 'Core systems', lab: 'Relay lab', contact: 'Get started' },
  hero: {
    eyebrow: 'An enterprise-grade AI API gateway',
    tagline: 'One entry. Every model.',
    description: 'Unify model access through a compatible API while policy routing, failover, and usage records stay on one observable path.',
    primaryCta: 'Get started',
    secondaryCta: 'Browse models',
    endpoint: 'Compatible endpoint',
    signal: 'Policy routing / failover / visible usage'
  },
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
const sceneReady = ref(false)
const ambientEnabled = ref(false)

let sectionObserver: IntersectionObserver | null = null
let homeMatchMedia: ReturnType<typeof gsap.matchMedia> | null = null
let heroIntroTimeline: gsap.core.Timeline | null = null
let cardTransitionTimeline: gsap.core.Timeline | null = null
let cardTransitionTarget: HTMLElement | null = null
let cardTransitionToken = 0
let scrollFrame = 0
let deckAnimationFrame = 0
let loaderTimer = 0
let loaderSafetyTimer = 0
let loaderHideTimer = 0
let homeMounted = false
let targetDeckPosition = 0
const cardParticleTimelines = new Set<gsap.core.Timeline>()
const cardParticleTimelineByCard = new WeakMap<HTMLElement, gsap.core.Timeline>()

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

function cardStyle(index: number): CSSProperties {
  const delta = index - deckPosition.value
  const distance = Math.abs(delta)
  const isMobileViewport = window.innerWidth < 700
  const x = delta * (isMobileViewport ? 78 : 34)
  const y = distance * (isMobileViewport ? 1.2 : 1.8)
  const z = -distance * 260
  const rotateY = delta * -6
  const rotateZ = delta * 0.45
  const scale = Math.max(0.76, 1 - distance * 0.11)
  const opacity = Math.max(0, 1 - Math.max(0, distance - 0.3) * 0.88)
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

function cardParticleStyle(cardIndex: number, particleIndex: number): CSSProperties {
  const horizontal = ((cardIndex * 37 + particleIndex * 61) % 89) / 88
  const vertical = ((cardIndex * 53 + particleIndex * 29) % 83) / 82
  const size = 1.2 + ((cardIndex + particleIndex * 3) % 5) * 0.55
  return {
    '--particle-left': `${7 + horizontal * 86}%`,
    '--particle-top': `${8 + vertical * 84}%`,
    '--particle-size': `${size.toFixed(2)}px`,
    '--particle-alpha': String(0.16 + ((particleIndex * 7) % 6) * 0.045),
  } as CSSProperties
}

function animateCardParticles(event: Event) {
  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return
  const card = event.currentTarget as HTMLElement | null
  if (!card) return
  const particles = Array.from(card.querySelectorAll<HTMLElement>('.kinetic-card-particle'))
  const planes = Array.from(card.querySelectorAll<HTMLElement>('.kinetic-card-plane'))
  const orbit = card.querySelector<HTMLElement>('.kinetic-card-orbit')
  if (!particles.length) return

  const previousTimeline = cardParticleTimelineByCard.get(card)
  previousTimeline?.kill()
  if (previousTimeline) cardParticleTimelines.delete(previousTimeline)
  const timeline = gsap.timeline({
    defaults: { overwrite: 'auto' },
    onComplete: () => {
      cardParticleTimelines.delete(timeline)
      if (cardParticleTimelineByCard.get(card) === timeline) cardParticleTimelineByCard.delete(card)
    },
  })
  cardParticleTimelines.add(timeline)
  cardParticleTimelineByCard.set(card, timeline)
  timeline
    .to(particles, {
      x: index => ((index * 47) % 58) - 29,
      y: index => ((index * 31) % 44) - 22,
      scale: index => 1.15 + (index % 4) * 0.18,
      autoAlpha: 0.72,
      duration: 0.28,
      ease: 'power2.out',
      stagger: { amount: 0.14, from: 'random' },
    }, 0)
    .to(planes, {
      x: index => (index - 1) * 7,
      y: index => (1 - index) * 5,
      duration: 0.42,
      ease: 'power2.out',
    }, 0)
    .to(orbit, { rotation: 18, duration: 0.48, ease: 'power2.out' }, 0)
    .to(particles, {
      x: 0,
      y: 0,
      scale: 1,
      autoAlpha: index => 0.16 + ((index * 7) % 6) * 0.045,
      duration: 0.58,
      ease: 'elastic.out(1, 0.42)',
      stagger: { amount: 0.18, from: 'random' },
    }, '>-0.06')
}

function settleCardParticles(event: Event) {
  const card = event.currentTarget as HTMLElement | null
  const relatedTarget = 'relatedTarget' in event ? event.relatedTarget as Node | null : null
  if (!card || (relatedTarget && card.contains(relatedTarget))) return
  const particles = card.querySelectorAll<HTMLElement>('.kinetic-card-particle')
  const planes = card.querySelectorAll<HTMLElement>('.kinetic-card-plane')
  const orbit = card.querySelector<HTMLElement>('.kinetic-card-orbit')
  const timeline = cardParticleTimelineByCard.get(card)
  timeline?.kill()
  if (timeline) cardParticleTimelines.delete(timeline)
  cardParticleTimelineByCard.delete(card)
  gsap.to(particles, {
    x: 0,
    y: 0,
    scale: 1,
    autoAlpha: index => 0.16 + ((index * 7) % 6) * 0.045,
    duration: 0.34,
    ease: 'power2.out',
    overwrite: true,
  })
  gsap.to(planes, { x: 0, y: 0, duration: 0.34, ease: 'power2.out', overwrite: true })
  gsap.to(orbit, { rotation: 0, duration: 0.34, ease: 'power2.out', overwrite: true })
}

async function animateActiveCard(index: number) {
  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return
  const transitionToken = ++cardTransitionToken
  await nextTick()
  if (transitionToken !== cardTransitionToken) return
  const card = pageRef.value?.querySelector<HTMLElement>(`.kinetic-card[data-card-index="${index}"]`)
  if (!card) return
  const topline = card.querySelector<HTMLElement>('.kinetic-card-topline')
  const content = Array.from(card.querySelectorAll<HTMLElement>('.kinetic-card-content > *'))
  const visual = card.querySelector<HTMLElement>('.kinetic-card-visual')

  cardTransitionTimeline?.kill()
  if (cardTransitionTarget) {
    gsap.set(cardTransitionTarget.querySelectorAll('.kinetic-card-topline, .kinetic-card-content > *, .kinetic-card-visual'), {
      clearProps: 'transform,opacity,visibility',
    })
  }
  cardTransitionTarget = card
  cardTransitionTimeline = gsap.timeline({ defaults: { ease: 'power3.out', overwrite: 'auto' } })
    .fromTo(visual, { scale: 1.035, autoAlpha: 0.76 }, {
      scale: 1,
      autoAlpha: 1,
      duration: 0.48,
      clearProps: 'transform,opacity,visibility',
    }, 0)
    .fromTo(topline, { y: -4, autoAlpha: 0.45 }, {
      y: 0,
      autoAlpha: 1,
      duration: 0.3,
      clearProps: 'transform,opacity,visibility',
    }, 0.06)
    .fromTo(content, { y: 8, autoAlpha: 0.38 }, {
      y: 0,
      autoAlpha: 1,
      duration: 0.34,
      stagger: 0.04,
      clearProps: 'transform,opacity,visibility',
    }, 0.1)
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
  targetDeckPosition = index
  if (!work) return
  const scrollSpan = Math.max(1, work.offsetHeight - window.innerHeight)
  const destination = work.offsetTop + (index / Math.max(1, workCards.value.length - 1)) * scrollSpan
  window.scrollTo({
    top: destination,
    behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth'
  })
}

function scheduleDeckMotion() {
  if (deckAnimationFrame) return
  const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  const update = () => {
    const delta = targetDeckPosition - deckPosition.value
    if (reduceMotion || Math.abs(delta) < 0.001) {
      deckPosition.value = targetDeckPosition
      activeCardIndex.value = Math.round(deckPosition.value)
      deckAnimationFrame = 0
      return
    }
    deckPosition.value += delta * 0.16
    activeCardIndex.value = Math.round(deckPosition.value)
    deckAnimationFrame = requestAnimationFrame(update)
  }
  deckAnimationFrame = requestAnimationFrame(update)
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
      targetDeckPosition = local * Math.max(0, workCards.value.length - 1)
      scheduleDeckMotion()
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
  ambientEnabled.value = false
  if (sceneReady.value) {
    loaderProgress.value = 100
    loaderVisible.value = false
    ambientEnabled.value = true
    return
  }
  loaderVisible.value = true
  loaderProgress.value = 4
  loaderTimer = window.setInterval(() => {
    loaderProgress.value = Math.min(94, loaderProgress.value + Math.max(1, (96 - loaderProgress.value) * 0.07))
  }, 72)
  loaderSafetyTimer = window.setTimeout(handleSceneReady, 17_500)
}

function handleSceneReady() {
  sceneReady.value = true
  window.clearInterval(loaderTimer)
  window.clearTimeout(loaderSafetyTimer)
  loaderTimer = 0
  loaderSafetyTimer = 0
  loaderProgress.value = 100
  loaderHideTimer = window.setTimeout(() => {
    loaderVisible.value = false
    ambientEnabled.value = true
    heroIntroTimeline?.play(0)
    if (homeMatchMedia) window.requestAnimationFrame(() => ScrollTrigger.refresh())
    loaderHideTimer = 0
  }, window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 0 : 280)
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
  gsap.registerPlugin(ScrollTrigger)
  homeMatchMedia = gsap.matchMedia()
  homeMatchMedia.add('(prefers-reduced-motion: no-preference)', () => {
    const hero = Array.from(root.querySelectorAll<HTMLElement>('.kinetic-hero .kinetic-reveal'))
    heroIntroTimeline = gsap.timeline({ paused: true, defaults: { ease: 'power3.out' } })
      .fromTo(hero, { autoAlpha: 0, y: 18 }, {
        autoAlpha: 1,
        y: 0,
        duration: 0.72,
        stagger: 0.075,
      })
    if (!loaderVisible.value) heroIntroTimeline.play(0)

    sections.filter(section => section.id !== 'home').forEach((section) => {
      const sectionReveals = Array.from(section.querySelectorAll<HTMLElement>('.kinetic-reveal'))
      if (!sectionReveals.length) return
      const timeline = gsap.timeline({ paused: true, defaults: { ease: 'power3.out' } })
        .fromTo(sectionReveals, { autoAlpha: 0, y: 24 }, {
          autoAlpha: 1,
          y: 0,
          duration: 0.58,
          stagger: 0.07,
        })
      ScrollTrigger.create({
        trigger: section,
        start: 'top 82%',
        end: 'bottom 16%',
        animation: timeline,
        toggleActions: 'play reverse play reverse',
        fastScrollEnd: true,
      })
    })
  }, root)
}

function cleanupMotion() {
  cancelAnimationFrame(scrollFrame)
  cancelAnimationFrame(deckAnimationFrame)
  deckAnimationFrame = 0
  window.removeEventListener('scroll', updateScrollState)
  window.removeEventListener('resize', updateScrollState)
  sectionObserver?.disconnect()
  sectionObserver = null
  homeMatchMedia?.revert()
  homeMatchMedia = null
  heroIntroTimeline?.kill()
  heroIntroTimeline = null
  cardTransitionToken += 1
  cardTransitionTimeline?.kill()
  cardTransitionTimeline = null
  if (cardTransitionTarget) {
    gsap.set(cardTransitionTarget.querySelectorAll('.kinetic-card-topline, .kinetic-card-content > *, .kinetic-card-visual'), {
      clearProps: 'transform,opacity,visibility',
    })
  }
  cardTransitionTarget = null
  cardParticleTimelines.forEach(timeline => timeline.kill())
  cardParticleTimelines.clear()
}

async function syncHomeMode() {
  cleanupMotion()
  activeSection.value = 'home'
  scrollProgress.value = 0
  deckPosition.value = 0
  targetDeckPosition = 0
  activeCardIndex.value = 0
  if (hasHomeContent.value || compactHomeEnabled.value) {
    sceneReady.value = false
    ambientEnabled.value = false
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

watch(activeCardIndex, (index, previousIndex) => {
  if (index !== previousIndex) void animateActiveCard(index)
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
  ambientEnabled.value = false
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
  --kinetic-ink: #f5f7fb;
  --kinetic-muted: rgba(226, 232, 240, 0.74);
  --kinetic-dim: rgba(203, 213, 225, 0.48);
  --kinetic-line: rgba(199, 210, 254, 0.2);
  --kinetic-line-strong: rgba(165, 180, 252, 0.46);
  --kinetic-teal: #22d3ee;
  --kinetic-violet: #818cf8;
  position: relative;
  width: 100%;
  max-width: 100vw;
  min-height: 100vh;
  overflow: clip;
  color: var(--kinetic-ink);
  background: #4a4d44;
  color-scheme: dark;
  font-family: "Noto Sans SC", ui-sans-serif, system-ui, sans-serif;
  isolation: isolate;
}
.kinetic-world {
  position: fixed;
  inset: 0;
  z-index: 0;
  contain: layout paint;
  pointer-events: none;
}
.kinetic-ambient {
  position: fixed;
  inset: 0;
  z-index: 2;
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
.kinetic-top-nav a,
.kinetic-top-nav button {
  position: relative;
  z-index: 1;
  display: inline-flex;
  min-height: 28px;
  align-items: center;
  color: var(--kinetic-muted);
  font: 560 9px/1 ui-monospace, SFMono-Regular, Menlo, monospace;
  letter-spacing: 0.07em;
  text-transform: uppercase;
  transition: color var(--motion-fast) var(--ease-standard);
}
.kinetic-top-nav a:hover,
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
.kinetic-header a:focus-visible,
.kinetic-hero-actions a:focus-visible,
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
.kinetic-hero-copy {
  position: absolute;
  top: 22vh;
  left: max(24px, 6vw);
  width: min(620px, calc(100% - 48px));
}
.kinetic-hero-eyebrow {
  display: block;
  margin-bottom: 20px;
  color: var(--kinetic-teal);
  font: 620 11px/1.4 ui-monospace, SFMono-Regular, Menlo, monospace;
  letter-spacing: 0;
  text-transform: uppercase;
}
.kinetic-hero-copy h1 {
  max-width: 100%;
  margin: 0;
  color: var(--kinetic-ink);
  font-size: 108px;
  font-weight: 620;
  line-height: 0.92;
  letter-spacing: 0;
  text-wrap: balance;
}
.kinetic-hero-tagline {
  margin: 26px 0 0;
  color: var(--kinetic-ink);
  font-size: 32px;
  font-weight: 440;
  line-height: 1.2;
}
.kinetic-hero-description {
  max-width: 560px;
  margin: 16px 0 0;
  color: var(--kinetic-muted);
  font-size: 16px;
  line-height: 1.7;
}
.kinetic-hero-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 28px;
}
.kinetic-hero-primary,
.kinetic-hero-secondary {
  display: inline-flex;
  min-height: 44px;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 0 18px;
  border: 1px solid transparent;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 650;
  transition: color var(--motion-fast) var(--ease-standard), background-color var(--motion-fast) var(--ease-standard), border-color var(--motion-fast) var(--ease-standard), transform var(--motion-fast) var(--ease-standard);
}
.kinetic-hero-primary {
  color: #fff;
  background: var(--color-primary, #6366f1);
}
.kinetic-hero-primary:hover {
  background: var(--color-primary-hover, #818cf8);
  transform: translateY(-1px);
}
.kinetic-hero-secondary {
  border-color: var(--kinetic-line-strong);
  color: var(--kinetic-ink);
  background: rgba(19, 23, 32, 0.72);
}
.kinetic-hero-secondary:hover {
  border-color: var(--kinetic-teal);
  background: rgba(25, 30, 40, 0.9);
  transform: translateY(-1px);
}
.kinetic-hero-endpoint {
  display: flex;
  width: fit-content;
  max-width: 100%;
  flex-wrap: wrap;
  gap: 8px 12px;
  margin-top: 20px;
  color: var(--kinetic-muted);
  font: 520 11px/1.5 ui-monospace, SFMono-Regular, Menlo, monospace;
  overflow-wrap: anywhere;
}
.kinetic-hero-endpoint span {
  color: var(--kinetic-dim);
}
.kinetic-hero-meta {
  position: absolute;
  bottom: 6vh;
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
  background: transparent;
  box-shadow: none;
  pointer-events: none;
}
.kinetic-manifesto-layout {
  position: absolute;
  right: clamp(48px, 8vw, 138px);
  bottom: clamp(54px, 8vh, 100px);
  left: clamp(38px, 8vw, 138px);
  display: grid;
  grid-template-columns: minmax(0, 1.24fr) minmax(270px, 0.56fr);
  align-items: end;
  gap: clamp(44px, 7vw, 112px);
}
.kinetic-manifesto-heading h1 {
  position: relative;
  max-width: 820px;
  margin: 24px 0 0;
  color: #eff6f1;
  font-family: "Noto Sans SC", ui-sans-serif, system-ui, sans-serif;
  font-size: 96px;
  font-weight: 560;
  line-height: 0.98;
  letter-spacing: 0;
  text-wrap: balance;
  text-shadow: 0 14px 44px rgba(0, 0, 0, 0.24);
}
.kinetic-manifesto-heading h1 span {
  display: block;
  max-width: 100%;
  overflow-wrap: anywhere;
}
.kinetic-manifesto-heading h1 span:nth-child(2) {
  color: #b9f3da;
}
.kinetic-manifesto-copy {
  max-width: 380px;
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
  letter-spacing: 0;
  text-transform: uppercase;
}
.kinetic-deck {
  position: absolute;
  inset: 0;
  perspective: 1300px;
  transform-style: preserve-3d;
}
.kinetic-card {
  --card-accent: 129, 140, 248;
  --card-secondary: 34, 211, 238;
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
  width: min(46vw, 640px);
  height: min(54vh, 530px);
  min-height: 370px;
  flex-direction: column;
  justify-content: space-between;
  overflow: hidden;
  padding: clamp(20px, 2.4vw, 34px);
  border: 1px solid rgba(199, 210, 254, 0.28);
  border-radius: 16px;
  color: #f5fbf8;
  background: rgba(11, 13, 18, 0.9);
  box-shadow: inset 0 1px rgba(255, 255, 255, 0.08), 0 24px 58px rgba(0, 0, 0, 0.28);
  opacity: var(--deck-opacity);
  transform: translate3d(calc(-50% + var(--deck-x)), calc(-50% + var(--deck-y)), var(--deck-z)) rotateY(var(--deck-ry)) rotateZ(var(--deck-rz)) scale(var(--deck-scale));
  transform-style: preserve-3d;
  transition: border-color var(--motion-base) var(--ease-standard), box-shadow var(--motion-base) var(--ease-standard);
  will-change: transform, opacity;
}
.kinetic-card::before {
  position: absolute;
  inset: 34% 0 0;
  z-index: 1;
  background: linear-gradient(180deg, transparent, rgba(7, 9, 13, 0.54) 34%, rgba(7, 9, 13, 0.96));
  content: '';
  pointer-events: none;
}
.kinetic-card::after {
  position: absolute;
  inset: 8px;
  border: 1px solid rgba(255, 255, 255, 0.045);
  border-radius: 10px;
  content: '';
  pointer-events: none;
}
.kinetic-card.is-active {
  border-color: rgba(165, 180, 252, 0.62);
  box-shadow: inset 0 1px rgba(255, 255, 255, 0.12), 0 28px 70px rgba(0, 0, 0, 0.34);
}
.kinetic-card-visual {
  position: absolute;
  inset: 0;
  overflow: hidden;
  background:
    radial-gradient(circle at 53% 36%, rgba(var(--card-accent), 0.38), transparent 24%),
    radial-gradient(circle at 16% 84%, rgba(var(--card-secondary), 0.3), transparent 38%),
    radial-gradient(circle at 88% 72%, rgba(var(--card-accent), 0.14), transparent 30%),
    #0b0d12;
}
.kinetic-card-routing { --card-accent: 99, 102, 241; --card-secondary: 34, 211, 238; }
.kinetic-card-signals { --card-accent: 34, 211, 238; --card-secondary: 129, 140, 248; }
.kinetic-card-learning { --card-accent: 74, 222, 128; --card-secondary: 129, 140, 248; }
.kinetic-card-control { --card-accent: 129, 140, 248; --card-secondary: 94, 234, 212; }
.kinetic-card-plane {
  position: absolute;
  top: 50%;
  left: 50%;
  display: block;
  border: 1px solid rgba(220, 250, 240, 0.38);
  background: rgba(var(--card-accent), 0.07);
  box-shadow: inset 0 0 24px rgba(var(--card-secondary), 0.06);
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
.kinetic-card-particles {
  position: absolute;
  inset: 0;
  display: block;
  pointer-events: none;
}
.kinetic-card-particle {
  position: absolute;
  top: var(--particle-top);
  left: var(--particle-left);
  display: block;
  width: var(--particle-size);
  height: var(--particle-size);
  border-radius: 50%;
  opacity: var(--particle-alpha);
  background: rgb(var(--card-accent));
  box-shadow: 0 0 8px rgba(var(--card-accent), 0.7);
  will-change: transform, opacity;
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
  max-width: 82%;
  flex-direction: column;
  align-items: flex-start;
  padding: 0;
}
.kinetic-card-content h3 {
  margin: 0;
  font: 540 clamp(30px, 3.7vw, 54px)/0.96 ui-monospace, SFMono-Regular, Menlo, monospace;
  letter-spacing: 0;
  text-transform: uppercase;
}
.kinetic-card-content p {
  margin: 15px 0 0;
  color: rgba(226, 232, 240, 0.76);
  font-size: 12px;
  line-height: 1.62;
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
.kinetic-work-filter button.is-active { color: #f4fbf7; transform: translateX(2px); }
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
  min-height: 146svh;
}
.kinetic-lab-sticky::before {
  position: absolute;
  inset: 0;
  background: transparent;
  box-shadow: none;
  content: '';
  pointer-events: none;
}
.kinetic-lab-shell {
  position: absolute;
  top: 51%;
  right: 0;
  left: 0;
  display: grid;
  width: 100%;
  min-height: 430px;
  grid-template-columns: minmax(0, 1.18fr) minmax(150px, 0.44fr) minmax(260px, 0.72fr);
  align-items: center;
  gap: clamp(32px, 5vw, 78px);
  padding: clamp(42px, 5vw, 72px) max(56px, calc((100vw - 1180px) / 2 + 56px));
  color: #eff8f3;
  background: transparent;
  transform: translateY(-50%);
  isolation: isolate;
}
.kinetic-lab-heading {
  position: relative;
  display: flex;
  min-width: 0;
  min-height: 150px;
  flex-direction: column;
  justify-content: center;
  gap: 0;
}
.kinetic-lab-heading > .kinetic-section-label {
  position: absolute;
  top: 0;
  left: 0;
}
.kinetic-lab-title-row {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: clamp(18px, 2.6vw, 36px);
}
.kinetic-lab-mark {
  display: grid;
  width: 76px;
  flex: 0 0 auto;
  aspect-ratio: 1;
  place-items: center;
  border: 1px solid rgba(239, 255, 247, 0.56);
  border-radius: 50%;
  box-shadow: 0 0 0 8px rgba(151, 247, 222, 0.04);
  font: 560 24px/1 ui-monospace, SFMono-Regular, Menlo, monospace;
}
.kinetic-lab-shell h2 {
  min-width: 0;
  margin: 0;
  font-family: "Noto Sans SC", ui-sans-serif, system-ui, sans-serif;
  font-size: 58px;
  font-weight: 560;
  line-height: 1;
  letter-spacing: 0;
  overflow-wrap: anywhere;
}
.kinetic-lab-route {
  position: relative;
  width: 100%;
  height: 150px;
}
.kinetic-lab-route::before,
.kinetic-lab-route::after {
  position: absolute;
  left: 50%;
  width: 1px;
  height: 34%;
  background: rgba(197, 241, 224, 0.18);
  content: '';
  transform: translateX(-50%);
}
.kinetic-lab-route::before { top: 0; }
.kinetic-lab-route::after { bottom: 0; }
.kinetic-lab-route-line {
  position: absolute;
  top: 50%;
  right: 0;
  left: 0;
  height: 1px;
  background: rgba(209, 249, 233, 0.38);
}
.kinetic-lab-route-node {
  position: absolute;
  top: 50%;
  width: 9px;
  height: 9px;
  border: 1px solid rgba(218, 255, 239, 0.68);
  border-radius: 50%;
  background: #12372d;
  transform: translate(-50%, -50%);
}
.kinetic-lab-route-node-start { left: 0; }
.kinetic-lab-route-node-middle { left: 28%; }
.kinetic-lab-route-node-end { left: 100%; }
.kinetic-lab-route strong {
  position: absolute;
  top: 50%;
  left: 58%;
  display: grid;
  width: 58px;
  aspect-ratio: 1;
  place-items: center;
  border: 1px solid rgba(229, 255, 242, 0.58);
  border-radius: 50%;
  color: #e9fff4;
  background: rgba(16, 56, 45, 0.74);
  box-shadow: 0 0 0 8px rgba(112, 230, 190, 0.05);
  font: 600 17px/1 ui-monospace, SFMono-Regular, Menlo, monospace;
  transform: translate(-50%, -50%);
}
.kinetic-lab-copy { max-width: 340px; }
.kinetic-lab-copy p {
  margin: 15px 0 0;
  color: var(--kinetic-muted);
  font-size: 13px;
  line-height: 1.68;
}
.kinetic-lab-copy a {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  margin-top: 18px;
  padding-bottom: 4px;
  border-bottom: 1px solid var(--kinetic-line-strong);
  color: var(--kinetic-ink);
  font: 560 9px/1 ui-monospace, SFMono-Regular, Menlo, monospace;
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
  background: transparent;
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
  letter-spacing: 0;
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
  z-index: 30;
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
  color: #eef7f2;
  background: #26332d;
  contain: layout paint style;
  pointer-events: none;
}
.kinetic-loader-shell {
  display: grid;
  width: min(330px, calc(100vw - 48px));
  grid-template-columns: 52px minmax(0, 1fr);
  align-items: center;
  gap: 17px;
}
.kinetic-loader-mark {
  display: grid;
  width: 52px;
  height: 52px;
  place-items: center;
  overflow: hidden;
  border: 1px solid rgba(226, 232, 240, 0.36);
  border-radius: 50%;
  background: rgba(11, 13, 18, 0.62);
  font: 650 15px/1 ui-monospace, SFMono-Regular, Menlo, monospace;
}
.kinetic-loader-mark img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}
.kinetic-loader-status {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  color: rgba(226, 232, 240, 0.68);
  font: 560 10px/1.2 ui-monospace, SFMono-Regular, Menlo, monospace;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}
.kinetic-loader-status > span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.kinetic-loader-status strong {
  color: #f5f7fb;
  font-size: 13px;
  font-weight: 620;
}
.kinetic-loader-track {
  position: relative;
  display: block;
  height: 3px;
  grid-column: 2;
  overflow: hidden;
  background: rgba(226, 232, 240, 0.16);
}
.kinetic-loader-track b {
  position: absolute;
  inset: 0;
  background: #a5b4fc;
  transform-origin: left;
  transition: transform 100ms linear;
}
.kinetic-loader-enter-active,
.kinetic-loader-leave-active { transition: opacity 280ms var(--ease-standard); }
.kinetic-loader-enter-from,
.kinetic-loader-leave-to { opacity: 0; }

@media (max-width: 1120px) {
  .kinetic-lab-shell {
    grid-template-columns: minmax(0, 1fr) minmax(280px, 0.62fr);
    padding-right: 34px;
    padding-left: 34px;
  }
  .kinetic-lab-route { display: none; }
}

@media (max-width: 920px) {
  .kinetic-wordmark-copy { display: none; }
  .kinetic-section-index { display: none; }
  .kinetic-hero-copy h1 { font-size: 82px; }
  .kinetic-manifesto-layout {
    right: 34px;
    bottom: 72px;
    left: 34px;
    grid-template-columns: 1fr;
    gap: 40px;
  }
  .kinetic-manifesto-copy { max-width: 440px; margin-left: auto; }
  .kinetic-card { width: min(64vw, 600px); }
  .kinetic-lab-shell {
    width: 100%;
    grid-template-columns: minmax(0, 1fr) minmax(180px, 0.58fr);
  }
}

@media (max-width: 640px) {
  .kinetic-header { padding: 16px; }
  .kinetic-top-nav { min-width: 164px; height: 36px; padding: 0 13px; }
  .kinetic-wordmark-mark { width: 34px; height: 34px; }
  .kinetic-hero { min-height: 104svh; }
  .kinetic-hero-copy { top: 112px; right: 18px; left: 18px; width: auto; }
  .kinetic-hero-eyebrow { margin-bottom: 14px; font-size: 9px; }
  .kinetic-hero-copy h1 { font-size: 50px; line-height: 0.96; }
  .kinetic-hero-tagline { margin-top: 18px; font-size: 26px; }
  .kinetic-hero-description { max-width: 430px; margin-top: 12px; font-size: 14px; line-height: 1.6; }
  .kinetic-hero-actions { margin-top: 20px; }
  .kinetic-hero-primary, .kinetic-hero-secondary { min-height: 42px; padding: 0 14px; }
  .kinetic-hero-endpoint { margin-top: 16px; font-size: 9px; }
  .kinetic-hero-meta { bottom: 34px; left: 18px; width: min(280px, 72vw); }
  .kinetic-manifesto { min-height: 128svh; }
  .kinetic-manifesto-layout { right: 18px; bottom: 72px; left: 18px; gap: 32px; }
  .kinetic-manifesto-heading h1 { max-width: 100%; font-size: 48px; line-height: 1; }
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
  .kinetic-card-content { max-width: 92%; padding: 0; }
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
  .kinetic-lab-shell {
    width: 100%;
    min-height: 0;
    grid-template-columns: 1fr;
    justify-items: start;
    gap: 32px;
    padding: 38px 20px;
  }
  .kinetic-lab-heading {
    min-height: 0;
    justify-content: flex-start;
    gap: 22px;
  }
  .kinetic-lab-heading > .kinetic-section-label { position: static; }
  .kinetic-lab-title-row { gap: 18px; }
  .kinetic-lab-mark { width: 56px; font-size: 18px; }
  .kinetic-lab-shell h2 { font-size: 42px; }
  .kinetic-lab-copy { max-width: 100%; }
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
  .kinetic-loader-track b { transition-duration: 1ms !important; }
  .kinetic-card { will-change: auto; }
  .kinetic-card-particle { will-change: auto; }
  .kinetic-reveal { opacity: 1 !important; visibility: visible !important; transform: none !important; }
  .kinetic-loader { display: none; }
}
</style>
