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
    :class="{
      'kinetic-home-ready': !loaderVisible,
      'kinetic-home-advanced': activeSectionIndex > 0,
      'is-transitioning': isTransitioning,
    }"
    @wheel.prevent="handleWheel"
    @touchstart.passive="handleTouchStart"
    @touchend.passive="handleTouchEnd"
    @touchcancel.passive="resetTouchGesture"
  >
    <a class="kinetic-skip-link" href="#kinetic-main">{{ copy.chrome.skip }}</a>

    <div class="kinetic-world" aria-hidden="true">
      <HomeHeroScene :progress="sceneProgress" @ready="handleSceneReady" />
    </div>
    <HomeAmbientEffects class="kinetic-ambient" :enabled="ambientEnabled" :progress="sceneProgress" />
    <div class="kinetic-world-shade" aria-hidden="true"></div>

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
        </div>
      </div>
    </Transition>

    <header class="kinetic-header">
      <button type="button" class="kinetic-wordmark" :aria-label="copy.nav.home" @click="goToSection('home')">
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
          :class="{ 'is-current': activeSection === 'about' }"
          :aria-current="activeSection === 'about' ? 'page' : undefined"
          @click="goToSection('about')"
        >
          {{ copy.nav.about }}
        </button>
        <button
          v-if="canvasEnabled"
          type="button"
          class="kinetic-canvas-nav"
          :class="{ 'is-current': activeSection === 'canvas' }"
          :aria-current="activeSection === 'canvas' ? 'page' : undefined"
          @click="goToSection('canvas')"
        >
          {{ copy.nav.canvas }}
        </button>
        <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="kinetic-login-link">
          {{ isAuthenticated ? copy.chrome.dashboard : copy.chrome.login }}
        </router-link>
        <span class="kinetic-nav-signal" aria-hidden="true"><i></i></span>
        <button
          type="button"
          :class="{ 'is-current': activeSection === 'contact' }"
          :aria-current="activeSection === 'contact' ? 'page' : undefined"
          @click="goToSection('contact')"
        >
          {{ copy.chrome.contact }}
        </button>
      </nav>
    </header>

    <nav class="kinetic-section-index" :aria-label="copy.chrome.sectionNavigation">
      <button
        v-for="(item, index) in navigationItems"
        :key="item.id"
        type="button"
        :class="{ 'is-active': index === activeSectionIndex }"
        :aria-label="copy.chrome.goToSection.replace('{section}', item.label)"
        :aria-current="index === activeSectionIndex ? 'step' : undefined"
        :data-label="item.label"
        @click="goToSection(item.id)"
      >
        <i aria-hidden="true"></i>
      </button>
    </nav>

    <main
      id="kinetic-main"
      ref="stageRef"
      tabindex="-1"
      :aria-busy="isTransitioning || undefined"
    >
      <p class="kinetic-sr-only" role="status" aria-live="polite">
        {{ copy.chrome.sectionStatus.replace('{current}', String(activeSectionIndex + 1)).replace('{total}', String(navigationItems.length)).replace('{section}', activeSectionLabel) }}
      </p>

      <section
        id="home"
        data-home-section
        class="kinetic-section kinetic-hero"
        :class="{ 'is-active': activeSection === 'home' }"
        :aria-hidden="activeSection === 'home' ? 'false' : 'true'"
        :inert="activeSection !== 'home'"
        aria-labelledby="kinetic-hero-title"
      >
        <div class="kinetic-section-inner">
          <div class="kinetic-section-copy kinetic-hero-copy">
            <span class="kinetic-section-label kinetic-hero-eyebrow">01 / {{ copy.hero.eyebrow }}</span>
            <h1 id="kinetic-hero-title">{{ siteName }}</h1>
            <p class="kinetic-hero-tagline">{{ copy.hero.tagline }}</p>
            <p class="kinetic-section-description">{{ copy.hero.description }}</p>
            <div class="kinetic-section-actions">
              <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="kinetic-primary-action kinetic-hero-primary">
                {{ isAuthenticated ? copy.contact.dashboardCta : copy.hero.primaryCta }}
                <Icon name="arrowRight" size="sm" aria-hidden="true" />
              </router-link>
              <router-link v-if="showModelPlazaEntry" to="/model-plaza" class="kinetic-secondary-action kinetic-hero-secondary">
                {{ copy.hero.secondaryCta }}
              </router-link>
              <button v-else-if="canvasEnabled" type="button" class="kinetic-secondary-action kinetic-hero-secondary" @click="goToSection('canvas')">
                {{ copy.canvas.secondaryCta }}
              </button>
            </div>
            <code class="kinetic-hero-endpoint">
              <span>{{ copy.hero.endpoint }}</span>
              {{ apiBaseUrl }}/v1
            </code>
          </div>
        </div>
      </section>

      <section
        id="about"
        data-home-section
        class="kinetic-section kinetic-about"
        :class="{ 'is-active': activeSection === 'about' }"
        :aria-hidden="activeSection === 'about' ? 'false' : 'true'"
        :inert="activeSection !== 'about'"
        aria-labelledby="kinetic-about-title"
      >
        <div class="kinetic-section-inner kinetic-about-layout">
          <div class="kinetic-section-copy kinetic-about-intro">
            <span class="kinetic-section-label">02 / {{ copy.about.eyebrow }}</span>
            <h2 id="kinetic-about-title">{{ copy.about.title }}</h2>
            <p class="kinetic-section-description">{{ copy.about.description }}</p>
            <div class="kinetic-section-actions">
              <button type="button" class="kinetic-primary-action" @click="goToSection('contact')">
                {{ copy.about.action }}
                <Icon name="arrowRight" size="sm" aria-hidden="true" />
              </button>
            </div>
          </div>

          <div class="kinetic-about-directory" :aria-label="copy.about.servicesLabel">
            <section class="kinetic-about-group">
              <span>01</span>
              <h3>{{ copy.about.aiTitle }}</h3>
              <ul>
                <li v-for="service in copy.about.aiServices" :key="service">{{ service }}</li>
              </ul>
            </section>
            <section class="kinetic-about-group">
              <span>02</span>
              <h3>{{ copy.about.deliveryTitle }}</h3>
              <ul>
                <li v-for="service in copy.about.deliveryServices" :key="service">{{ service }}</li>
              </ul>
            </section>
            <section class="kinetic-about-group kinetic-about-industries">
              <span>03</span>
              <h3>{{ copy.about.industriesTitle }}</h3>
              <ul>
                <li v-for="industry in copy.about.industries" :key="industry">{{ industry }}</li>
              </ul>
            </section>
          </div>
        </div>
      </section>

      <section
        id="canvas"
        data-home-section
        class="kinetic-section kinetic-canvas"
        :class="{ 'is-active': activeSection === 'canvas' }"
        :aria-hidden="activeSection === 'canvas' ? 'false' : 'true'"
        :inert="activeSection !== 'canvas'"
        aria-labelledby="kinetic-canvas-title"
      >
        <div class="kinetic-section-inner">
          <div class="kinetic-section-copy">
            <span class="kinetic-section-label">03 / {{ copy.canvas.eyebrow }}</span>
            <h2 id="kinetic-canvas-title">{{ copy.canvas.title }}</h2>
            <p class="kinetic-section-description">{{ copy.canvas.description }}</p>
            <ul class="kinetic-capability-list" :aria-label="copy.canvas.capabilitiesLabel">
              <li v-for="capability in copy.canvas.capabilities" :key="capability">{{ capability }}</li>
            </ul>
            <div v-if="canvasEnabled" class="kinetic-section-actions">
              <router-link to="/canvas" class="kinetic-primary-action kinetic-canvas-cta">
                {{ copy.canvas.action }}
                <Icon name="arrowRight" size="sm" aria-hidden="true" />
              </router-link>
            </div>
          </div>
        </div>
      </section>

      <section
        id="relay"
        data-home-section
        class="kinetic-section kinetic-relay"
        :class="{ 'is-active': activeSection === 'relay' }"
        :aria-hidden="activeSection === 'relay' ? 'false' : 'true'"
        :inert="activeSection !== 'relay'"
        aria-labelledby="kinetic-relay-title"
      >
        <div class="kinetic-section-inner">
          <div class="kinetic-section-copy">
            <span class="kinetic-section-label">04 / {{ copy.relay.eyebrow }}</span>
            <h2 id="kinetic-relay-title">{{ copy.relay.title }}</h2>
            <p class="kinetic-section-description">{{ copy.relay.description }}</p>
            <ul class="kinetic-capability-list" :aria-label="copy.relay.capabilitiesLabel">
              <li v-for="capability in copy.relay.capabilities" :key="capability">{{ capability }}</li>
            </ul>
            <div class="kinetic-section-actions">
              <router-link :to="relayDestination" class="kinetic-secondary-action kinetic-relay-cta">
                {{ showModelPlazaEntry ? copy.relay.modelsAction : copy.relay.usageAction }}
                <Icon name="arrowRight" size="sm" aria-hidden="true" />
              </router-link>
            </div>
          </div>
        </div>
      </section>

      <section
        id="contact"
        data-home-section
        class="kinetic-section kinetic-contact"
        :class="{ 'is-active': activeSection === 'contact' }"
        :aria-hidden="activeSection === 'contact' ? 'false' : 'true'"
        :inert="activeSection !== 'contact'"
        aria-labelledby="kinetic-contact-title"
      >
        <div class="kinetic-section-inner">
          <div class="kinetic-section-copy kinetic-contact-content">
            <span class="kinetic-section-label">05 / {{ copy.nav.contact }}</span>
            <h2 id="kinetic-contact-title">
              <span>{{ copy.contact.lineOne }}</span>
              <span>{{ copy.contact.lineTwo }}</span>
            </h2>
            <p class="kinetic-section-description">{{ copy.contact.description }}</p>
            <div class="kinetic-section-actions">
              <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="kinetic-primary-action kinetic-primary-cta">
                {{ isAuthenticated ? copy.contact.dashboardCta : copy.contact.primaryCta }}
                <Icon name="arrowRight" size="sm" aria-hidden="true" />
              </router-link>
              <router-link to="/quick-start" class="kinetic-secondary-action">
                {{ copy.contact.quickStartCta }}
              </router-link>
            </div>
          </div>
        </div>

        <footer class="kinetic-footer">
          <span>&copy; {{ currentYear }} {{ siteName }}</span>
          <div class="kinetic-footer-links">
            <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">{{ copy.chrome.docs }}</a>
            <router-link v-if="showModelPlazaEntry" to="/model-plaza">{{ t('nav.modelPlaza') }}</router-link>
            <router-link to="/key-usage">{{ copy.chrome.usage }}</router-link>
            <LocaleSwitcher placement="top-end" />
          </div>
        </footer>
      </section>
    </main>

    <button
      v-if="activeSectionIndex < navigationItems.length - 1"
      type="button"
      class="kinetic-next-section"
      :aria-label="copy.chrome.nextSection.replace('{section}', nextSectionLabel)"
      :title="copy.chrome.nextSection.replace('{section}', nextSectionLabel)"
      @click="stepSection(1)"
    >
      <Icon name="chevronDown" size="sm" aria-hidden="true" />
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { gsap } from 'gsap'
import { brand } from '@/config/brand'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import HomeHeroScene from '@/components/home/HomeHeroScene.vue'
import HomeAmbientEffects from '@/components/home/HomeAmbientEffects.vue'
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import { toggleThemeWithTransition } from '@/utils/themeTransition'

type SectionId = 'home' | 'about' | 'canvas' | 'relay' | 'contact'

const SECTION_ORDER: SectionId[] = ['home', 'about', 'canvas', 'relay', 'contact']
const WHEEL_THRESHOLD = 24
const WHEEL_IDLE_MS = 180
const TOUCH_THRESHOLD = 48

const zhCopy = {
  chrome: {
    loading: '正在加载中继生态场', navigation: '首页导航', contact: '开始使用',
    docs: '文档', usage: '用量查询', dashboard: '控制台', login: '登录',
    light: '浅色模式', dark: '深色模式', skip: '跳到主要内容',
    sectionNavigation: '首页内容导航', goToSection: '前往{section}',
    nextSection: '查看{section}', sectionStatus: '第 {current} 项，共 {total} 项：{section}',
  },
  nav: { home: '首页', about: '关于我们', canvas: '无限画布', relay: '统一中继', contact: '开始使用' },
  hero: {
    eyebrow: '企业级 AI 聚合与采购平台',
    tagline: '企业 AI 采购、接入与治理，由一个可靠平台统一承载。',
    description: '已为 10+ 企业提供 AI 服务与采购支持，统一完成多模型接入、智能路由、成本治理与用量审计，为关键 AI 业务构建可靠、可控、可持续扩展的服务底座。',
    primaryCta: '开始使用',
    secondaryCta: '查看模型',
    endpoint: '兼容端点',
  },
  about: {
    eyebrow: '关于我们',
    title: '从 AI 能力到数字产品，交付企业下一条增长曲线',
    description: '我们面向企业提供从产品定义、工程研发到持续运营的一体化技术服务，把复杂 AI 能力、业务自动化与数字产品真正带进生产流程。无论从 0 到 1 打造新产品，还是升级既有系统，都以可落地、可扩展、可治理为交付标准。',
    action: '开启合作',
    servicesLabel: '我们的服务与行业范围',
    aiTitle: 'AI 产品与效率引擎',
    aiServices: ['AI 产品开发', 'AI 自动化产品', 'AI 提效产品', 'AI 工作流', 'AI 蒸馏资源服务', 'Token 资产管理'],
    deliveryTitle: '企业软件与数字交付',
    deliveryServices: ['企业 SaaS 软件定制', '企业知识库搭建', '软件定制', 'App 与小程序定制', '游戏开发'],
    industriesTitle: '跨行业落地',
    industries: ['AI', '电商', '直播', '元宇宙', '互联网', '教育', '跨境', '支付'],
  },
  canvas: {
    eyebrow: '无限画布',
    title: '把创意铺在一张可运行的画布上',
    description: '组织提示词、参考素材与生成结果，让图像和视频工作流始终保留上下文。',
    capabilitiesLabel: '无限画布能力',
    capabilities: ['提示词与素材', '图像与视频', '项目持续保存'],
    action: '进入无限画布',
    secondaryCta: '了解无限画布',
  },
  relay: {
    eyebrow: '统一中继',
    title: '一次接入，路由与用量始终可见',
    description: '保留熟悉的 SDK 与请求结构，由已配置的策略选择模型线路，并记录每次调用。',
    capabilitiesLabel: '统一中继能力',
    capabilities: ['兼容 API', '策略路由', '用量记录'],
    modelsAction: '查看可用模型',
    usageAction: '查询用量',
  },
  contact: {
    lineOne: '从一个入口开始',
    lineTwo: '把 AI 连接起来',
    description: '创建密钥，选择模型，然后把创作与调用带入同一条清晰链路。',
    primaryCta: '开始使用',
    dashboardCta: '进入控制台',
    quickStartCta: '查看快速启动',
  },
}

const enCopy = {
  chrome: {
    loading: 'Loading the relay landscape', navigation: 'Homepage navigation', contact: 'Get started',
    docs: 'Docs', usage: 'Usage', dashboard: 'Dashboard', login: 'Sign in',
    light: 'Light mode', dark: 'Dark mode', skip: 'Skip to main content',
    sectionNavigation: 'Homepage sections', goToSection: 'Go to {section}',
    nextSection: 'View {section}', sectionStatus: 'Item {current} of {total}: {section}',
  },
  nav: { home: 'Home', about: 'About us', canvas: 'Infinite canvas', relay: 'Unified relay', contact: 'Get started' },
  hero: {
    eyebrow: 'Enterprise AI aggregation and procurement',
    tagline: 'Source, connect, and govern enterprise AI on one reliable platform.',
    description: 'Already supporting 10+ enterprises with AI services and procurement, ModuRelay unifies multi-model access, intelligent routing, cost governance, and usage auditing for reliable, controlled operations at scale.',
    primaryCta: 'Get started',
    secondaryCta: 'Browse models',
    endpoint: 'Compatible endpoint',
  },
  about: {
    eyebrow: 'About us',
    title: 'From AI capability to digital products, we build the enterprise\'s next growth engine',
    description: 'We help enterprises move from product definition and engineering to continuous operation, turning complex AI capabilities, business automation, and digital ideas into production-ready systems. From new ventures to core-system upgrades, every engagement is built to be deployable, scalable, and governable.',
    action: 'Start a partnership',
    servicesLabel: 'Our services and industry reach',
    aiTitle: 'AI products and efficiency',
    aiServices: ['AI product development', 'AI automation products', 'AI productivity products', 'AI workflows', 'AI distillation resources', 'Token asset management'],
    deliveryTitle: 'Enterprise digital delivery',
    deliveryServices: ['Custom enterprise SaaS', 'Enterprise knowledge bases', 'Custom software', 'Apps and mini programs', 'Game development'],
    industriesTitle: 'Industry reach',
    industries: ['AI', 'E-commerce', 'Live streaming', 'Metaverse', 'Internet', 'Education', 'Cross-border', 'Payments'],
  },
  canvas: {
    eyebrow: 'Infinite canvas',
    title: 'Give every idea room to become a workflow',
    description: 'Arrange prompts, references, and generated results while image and video work keep their context.',
    capabilitiesLabel: 'Infinite canvas capabilities',
    capabilities: ['Prompts and assets', 'Image and video', 'Persistent projects'],
    action: 'Open infinite canvas',
    secondaryCta: 'Explore the canvas',
  },
  relay: {
    eyebrow: 'Unified relay',
    title: 'Connect once. Keep routing and usage visible.',
    description: 'Keep familiar SDKs and request shapes while configured policy chooses a model route and records each call.',
    capabilitiesLabel: 'Unified relay capabilities',
    capabilities: ['Compatible API', 'Policy routing', 'Usage records'],
    modelsAction: 'Browse available models',
    usageAction: 'Inspect usage',
  },
  contact: {
    lineOne: 'Start from one entry',
    lineTwo: 'Connect the rest of AI',
    description: 'Create a key, choose a model, and bring creation and requests into one clear operating path.',
    primaryCta: 'Get started',
    dashboardCta: 'Open dashboard',
    quickStartCta: 'View quick start',
  },
}

const { t, locale } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const pageRef = ref<HTMLElement | null>(null)
const stageRef = ref<HTMLElement | null>(null)
const activeSection = ref<SectionId>('home')
const isDark = ref(document.documentElement.classList.contains('dark'))
const loaderVisible = ref(true)
const loaderProgress = ref(4)
const sceneReady = ref(false)
const ambientEnabled = ref(false)
const isTransitioning = ref(false)

let motionContext: gsap.Context | null = null
let motionMatchMedia: ReturnType<typeof gsap.matchMedia> | null = null
let sectionTransitionTimeline: gsap.core.Timeline | null = null
let introTimeline: gsap.core.Timeline | null = null
let loaderTimer = 0
let loaderSafetyTimer = 0
let loaderHideTimer = 0
let wheelIdleTimer = 0
let wheelAccumulator = 0
let wheelDirection = 0
let wheelGestureConsumed = false
let touchStartX: number | null = null
let touchStartY: number | null = null
let homeMounted = false
let prefersReducedMotion = false
let viewportLocked = false
let previousHtmlOverflow = ''

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
const canvasEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.canvas))
const modelPlazaRequiresAuth = computed(() => appStore.cachedPublicSettings?.model_plaza_require_auth === true)
const showModelPlazaEntry = computed(() => modelPlazaEnabled.value && (isAuthenticated.value || !modelPlazaRequiresAuth.value))
const relayDestination = computed(() => showModelPlazaEntry.value ? '/model-plaza' : '/key-usage')
const apiBaseUrl = computed(() => {
  const configured = appStore.cachedPublicSettings?.api_base_url
  return typeof configured === 'string' && configured.trim() ? configured.trim().replace(/\/+$/, '') : window.location.origin
})
const navigationItems = computed(() => SECTION_ORDER.map(id => ({ id, label: copy.value.nav[id] })))
const activeSectionIndex = computed(() => Math.max(0, SECTION_ORDER.indexOf(activeSection.value)))
const activeSectionLabel = computed(() => navigationItems.value[activeSectionIndex.value]?.label || '')
const nextSectionLabel = computed(() => navigationItems.value[activeSectionIndex.value + 1]?.label || '')
const sceneProgress = computed(() => activeSectionIndex.value / Math.max(1, SECTION_ORDER.length - 1))

function toggleTheme(event?: MouseEvent) {
  isDark.value = toggleThemeWithTransition(isDark.value, event)
}

function sectionElements() {
  return pageRef.value
    ? Array.from(pageRef.value.querySelectorAll<HTMLElement>('[data-home-section]'))
    : []
}

function setSectionVisibility(index = activeSectionIndex.value) {
  sectionElements().forEach((section, sectionIndex) => {
    gsap.set(section, {
      autoAlpha: sectionIndex === index ? 1 : 0,
      scale: sectionIndex === index ? 1 : 0.992,
    })
  })
}

function sectionFromLocationHash(): SectionId | null {
  const hashSection = window.location.hash.slice(1)
  return SECTION_ORDER.includes(hashSection as SectionId) ? hashSection as SectionId : null
}

function syncLocationHash(id: SectionId) {
  const nextHash = id === 'home' ? '' : `#${id}`
  if (window.location.hash === nextHash) return

  window.history.replaceState(
    window.history.state,
    '',
    `${window.location.pathname}${window.location.search}${nextHash}`,
  )
}

function revealCurrentSection() {
  const section = sectionElements()[activeSectionIndex.value]
  if (!section) return

  introTimeline?.kill()
  if (prefersReducedMotion) {
    gsap.set(section, { autoAlpha: 1, scale: 1 })
    return
  }

  introTimeline = gsap.timeline({ defaults: { overwrite: 'auto' } })
    .fromTo(section, { autoAlpha: 0, scale: 1.012 }, {
      autoAlpha: 1,
      scale: 1,
      duration: 0.48,
      ease: 'power2.out',
    })
}

function transitionToSection(targetIndex: number, updateHash = true) {
  const boundedIndex = Math.max(0, Math.min(SECTION_ORDER.length - 1, targetIndex))
  if (boundedIndex === activeSectionIndex.value || isTransitioning.value || loaderVisible.value) return false

  const sections = sectionElements()
  const currentSection = sections[activeSectionIndex.value]
  const targetSection = sections[boundedIndex]
  if (!currentSection || !targetSection) return false

  sectionTransitionTimeline?.kill()
  introTimeline?.kill()

  if (prefersReducedMotion) {
    activeSection.value = SECTION_ORDER[boundedIndex]
    if (updateHash) syncLocationHash(activeSection.value)
    setSectionVisibility(boundedIndex)
    return true
  }

  isTransitioning.value = true
  sectionTransitionTimeline = gsap.timeline({
    defaults: { overwrite: 'auto' },
    onComplete: () => {
      setSectionVisibility(boundedIndex)
      isTransitioning.value = false
      sectionTransitionTimeline = null
    },
  })
    .to(currentSection, {
      autoAlpha: 0,
      scale: 0.988,
      duration: 0.16,
      ease: 'power1.in',
    })
    .add(() => {
      activeSection.value = SECTION_ORDER[boundedIndex]
      if (updateHash) syncLocationHash(activeSection.value)
      gsap.set(targetSection, { autoAlpha: 0, scale: 1.012 })
    })
    .to(targetSection, {
      autoAlpha: 1,
      scale: 1,
      duration: 0.3,
      ease: 'power2.out',
    })

  return true
}

function goToSection(id: SectionId) {
  transitionToSection(SECTION_ORDER.indexOf(id))
}

function handleHashChange() {
  if (hasHomeContent.value || compactHomeEnabled.value) return
  const targetSection = window.location.hash ? sectionFromLocationHash() : 'home'
  if (!targetSection) return

  const targetIndex = SECTION_ORDER.indexOf(targetSection)
  if (loaderVisible.value) {
    activeSection.value = targetSection
    return
  }
  transitionToSection(targetIndex, false)
}

function stepSection(direction: -1 | 1) {
  return transitionToSection(activeSectionIndex.value + direction)
}

function resetWheelSession() {
  wheelAccumulator = 0
  wheelDirection = 0
  wheelGestureConsumed = false
  wheelIdleTimer = 0
}

function scheduleWheelReset() {
  window.clearTimeout(wheelIdleTimer)
  wheelIdleTimer = window.setTimeout(resetWheelSession, WHEEL_IDLE_MS)
}

function handleWheel(event: WheelEvent) {
  scheduleWheelReset()
  if (loaderVisible.value || isTransitioning.value || wheelGestureConsumed) return
  if (Math.abs(event.deltaY) <= Math.abs(event.deltaX) || event.deltaY === 0) return

  const direction = event.deltaY > 0 ? 1 : -1
  if (wheelDirection !== 0 && direction !== wheelDirection) wheelAccumulator = 0
  wheelDirection = direction
  wheelAccumulator += Math.abs(event.deltaY)

  if (wheelAccumulator < WHEEL_THRESHOLD) return
  wheelGestureConsumed = stepSection(direction)
  wheelAccumulator = 0
}

function handleTouchStart(event: TouchEvent) {
  const touch = event.changedTouches[0]
  touchStartX = touch?.clientX ?? null
  touchStartY = touch?.clientY ?? null
}

function resetTouchGesture() {
  touchStartX = null
  touchStartY = null
}

function handleTouchEnd(event: TouchEvent) {
  const touch = event.changedTouches[0]
  if (!touch || touchStartX === null || touchStartY === null) {
    resetTouchGesture()
    return
  }

  const deltaX = touch.clientX - touchStartX
  const deltaY = touch.clientY - touchStartY
  resetTouchGesture()
  if (Math.abs(deltaY) < TOUCH_THRESHOLD || Math.abs(deltaY) <= Math.abs(deltaX)) return
  stepSection(deltaY < 0 ? 1 : -1)
}

function isEditableTarget(target: EventTarget | null) {
  return target instanceof HTMLElement && Boolean(target.closest('input, textarea, select, [contenteditable="true"]'))
}

function handleKeydown(event: KeyboardEvent) {
  if (hasHomeContent.value || compactHomeEnabled.value || event.repeat || isEditableTarget(event.target)) return

  let handled = true
  switch (event.key) {
    case 'ArrowDown':
    case 'PageDown':
    case ' ':
      stepSection(1)
      break
    case 'ArrowUp':
    case 'PageUp':
      stepSection(-1)
      break
    case 'Home':
      transitionToSection(0)
      break
    case 'End':
      transitionToSection(SECTION_ORDER.length - 1)
      break
    default:
      handled = false
  }

  if (handled) event.preventDefault()
}

function lockOfficialViewport() {
  if (viewportLocked) return
  viewportLocked = true
  previousHtmlOverflow = document.documentElement.style.overflow
  document.documentElement.style.overflow = 'hidden'
  window.scrollTo({ top: 0, left: 0, behavior: 'auto' })
  window.addEventListener('keydown', handleKeydown)
}

function unlockOfficialViewport() {
  if (!viewportLocked) return
  viewportLocked = false
  document.documentElement.style.overflow = previousHtmlOverflow
  window.removeEventListener('keydown', handleKeydown)
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
    revealCurrentSection()
    loaderHideTimer = 0
  }, window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 0 : 220)
}

function initializeMotion() {
  const root = pageRef.value
  if (!root) return

  motionContext = gsap.context(() => {
    const motionQuery = window.matchMedia('(prefers-reduced-motion: reduce)')
    if (typeof motionQuery.addEventListener !== 'function') {
      prefersReducedMotion = motionQuery.matches
      setSectionVisibility()
      if (loaderVisible.value) {
        gsap.set(sectionElements()[activeSectionIndex.value], { autoAlpha: 0, scale: 1 })
      } else {
        revealCurrentSection()
      }
      return
    }

    motionMatchMedia = gsap.matchMedia()
    motionMatchMedia.add({
      reduceMotion: '(prefers-reduced-motion: reduce)',
      allowMotion: '(prefers-reduced-motion: no-preference)',
    }, context => {
      prefersReducedMotion = Boolean(context.conditions?.reduceMotion)
      setSectionVisibility()
      if (loaderVisible.value) {
        gsap.set(sectionElements()[activeSectionIndex.value], { autoAlpha: 0, scale: 1 })
      } else {
        revealCurrentSection()
      }
    })
  }, root)
}

function cleanupMotion() {
  window.clearTimeout(wheelIdleTimer)
  wheelIdleTimer = 0
  resetWheelSession()
  resetTouchGesture()
  sectionTransitionTimeline?.kill()
  sectionTransitionTimeline = null
  introTimeline?.kill()
  introTimeline = null
  isTransitioning.value = false
  motionMatchMedia?.revert()
  motionMatchMedia = null
  motionContext?.revert()
  motionContext = null
}

async function syncHomeMode() {
  cleanupMotion()
  unlockOfficialViewport()
  activeSection.value = sectionFromLocationHash() || 'home'

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
  lockOfficialViewport()
  initializeMotion()
}

watch([hasHomeContent, compactHomeEnabled], () => {
  if (homeMounted) void syncHomeMode()
})

onMounted(() => {
  homeMounted = true
  isDark.value = document.documentElement.classList.contains('dark')
  authStore.checkAuth()
  if (!appStore.publicSettingsLoaded) appStore.fetchPublicSettings()
  window.addEventListener('hashchange', handleHashChange)
  void syncHomeMode()
})

onBeforeUnmount(() => {
  homeMounted = false
  cleanupMotion()
  unlockOfficialViewport()
  clearLoaderTimers()
  ambientEnabled.value = false
  window.removeEventListener('hashchange', handleHashChange)
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
.compact-home-content h1 { margin: 18px 0 10px; font-size: 48px; font-weight: 680; letter-spacing: 0; }
.compact-home-content p { margin: 0 auto 28px; max-width: 520px; color: var(--mr-text-muted); line-height: 1.7; white-space: pre-wrap; }
.compact-home-logo { margin: 16px auto 0; }
.compact-home-footer { padding: 20px 16px; border-top: 1px solid var(--mr-border); color: var(--mr-text-subtle); font-size: 12px; text-align: center; }
.brand-mark { display: grid; width: 30px; height: 30px; flex: 0 0 auto; place-items: center; overflow: hidden; border: 1px solid var(--mr-primary-border); border-radius: 8px; background: var(--mr-primary); }
.brand-mark img, .kinetic-wordmark-mark img, .kinetic-loader-mark img { width: 100%; height: 100%; object-fit: cover; }
.brand-mark-large { width: 54px; height: 54px; }
.home-icon-button, .home-quiet-link { display: inline-flex; min-width: 36px; min-height: 36px; align-items: center; justify-content: center; gap: 6px; padding: 0 9px; border: 1px solid transparent; border-radius: 8px; color: var(--mr-text-muted); font-size: 12px; transition: color var(--motion-fast) var(--ease-standard), background-color var(--motion-fast) var(--ease-standard), border-color var(--motion-fast) var(--ease-standard), transform var(--motion-fast) var(--ease-standard); }
.home-icon-button:hover, .home-quiet-link:hover { border-color: var(--mr-border); color: var(--mr-text); background: var(--mr-surface-subtle); transform: translateY(-1px); }
.home-solid-button, .home-primary-button { display: inline-flex; min-height: 38px; align-items: center; justify-content: center; gap: 7px; padding: 0 14px; border-radius: 8px; color: #fff; background: var(--mr-primary); font-size: 12px; font-weight: 650; transition: background-color var(--motion-fast) var(--ease-standard), transform var(--motion-fast) var(--ease-standard); }
.home-solid-button:hover, .home-primary-button:hover { background: var(--mr-primary-strong); transform: translateY(-1px); }
.home-overline { color: var(--mr-primary); font-size: 11px; font-weight: 700; letter-spacing: 0; }

.kinetic-home {
  --kinetic-text: rgba(255, 255, 255, 0.96);
  --kinetic-body: rgba(244, 247, 250, 0.78);
  --kinetic-muted: rgba(233, 238, 244, 0.58);
  --kinetic-line: rgba(255, 255, 255, 0.18);
  --kinetic-surface: rgba(11, 15, 19, 0.64);
  position: relative;
  width: 100%;
  height: 100dvh;
  min-height: 100svh;
  overflow: hidden;
  overscroll-behavior: none;
  color: var(--kinetic-text);
  background: var(--color-bg-deep);
  color-scheme: dark;
  font-family: "Noto Sans SC Variable", "Noto Sans SC", system-ui, sans-serif;
  touch-action: pan-x pinch-zoom;
}

.kinetic-world,
.kinetic-world-shade,
.kinetic-ambient,
.kinetic-loader {
  position: absolute;
  inset: 0;
}

.kinetic-world { z-index: 0; }
.kinetic-ambient { z-index: 1; opacity: 0.58; pointer-events: none; }
.kinetic-world-shade {
  z-index: 2;
  background: rgba(5, 9, 11, 0.22);
  box-shadow:
    inset 58vw 0 34vw -20vw rgba(3, 7, 9, 0.76),
    inset 0 18vh 18vh -18vh rgba(3, 7, 9, 0.46),
    inset 0 -20vh 18vh -16vh rgba(3, 7, 9, 0.48);
  pointer-events: none;
}

.kinetic-header {
  position: absolute;
  top: 0;
  right: 0;
  left: 0;
  z-index: 12;
  display: flex;
  min-height: 76px;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  padding: max(16px, env(safe-area-inset-top)) clamp(24px, 4vw, 64px) 12px;
}

.kinetic-wordmark {
  display: inline-flex;
  min-width: 0;
  min-height: 44px;
  align-items: center;
  gap: 11px;
  color: var(--kinetic-text);
  text-align: left;
}

.kinetic-wordmark-mark {
  display: grid;
  width: 34px;
  height: 34px;
  flex: 0 0 auto;
  place-items: center;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.26);
  border-radius: 8px;
  background: color-mix(in srgb, var(--mr-primary) 76%, rgba(14, 20, 25, 0.9));
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.2), var(--shadow-sm);
  font: 700 15px/1 ui-monospace, SFMono-Regular, Menlo, monospace;
}

.kinetic-wordmark-copy { display: grid; min-width: 0; gap: 1px; }
.kinetic-wordmark-copy strong { overflow: hidden; max-width: 220px; font-size: 14px; font-weight: 700; text-overflow: ellipsis; white-space: nowrap; }
.kinetic-wordmark-copy small { color: var(--kinetic-muted); font: 600 9px/1.2 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0; }

.kinetic-top-nav { display: flex; align-items: center; gap: 16px; }
.kinetic-top-nav a,
.kinetic-top-nav button {
  display: inline-flex;
  min-height: 44px;
  align-items: center;
  justify-content: center;
  border-bottom: 1px solid transparent;
  color: var(--kinetic-body);
  font-size: 12px;
  font-weight: 650;
  transition: color var(--motion-fast) var(--ease-standard), border-color var(--motion-fast) var(--ease-standard);
}
.kinetic-top-nav a:hover,
.kinetic-top-nav button:hover,
.kinetic-top-nav button.is-current { border-color: color-mix(in srgb, var(--mr-secondary) 72%, transparent); color: var(--kinetic-text); }
.kinetic-nav-signal { display: inline-flex; width: 18px; align-items: center; justify-content: center; }
.kinetic-nav-signal i { width: 5px; height: 5px; border-radius: 50%; background: var(--mr-success); box-shadow: 0 0 0 4px color-mix(in srgb, var(--mr-success) 16%, transparent); }

#kinetic-main { position: absolute; inset: 0; z-index: 4; overflow: hidden; outline: none; }
.kinetic-section {
  position: absolute;
  inset: 0;
  display: grid;
  opacity: 0;
  visibility: hidden;
  transform: scale(0.992);
  transform-origin: 28% 52%;
  pointer-events: none;
}
.kinetic-section.is-active { opacity: 1; visibility: visible; transform: scale(1); pointer-events: auto; }

.kinetic-section-inner {
  display: flex;
  width: min(100% - 128px, 1280px);
  height: 100%;
  align-items: center;
  margin: 0 auto;
  padding: 100px 0 88px;
}

.kinetic-section-copy { width: min(610px, 52vw); min-width: 0; }
.kinetic-section-label {
  display: block;
  margin-bottom: 20px;
  color: color-mix(in srgb, var(--mr-secondary) 82%, white);
  font: 700 11px/1.4 ui-monospace, SFMono-Regular, Menlo, monospace;
  letter-spacing: 0;
  text-transform: uppercase;
}

.kinetic-hero-copy h1,
.kinetic-section-copy h2 {
  max-width: 760px;
  margin: 0;
  color: var(--kinetic-text);
  font-weight: 680;
  letter-spacing: 0;
  text-wrap: balance;
}
.kinetic-hero-copy h1 { font-size: 72px; line-height: 1.02; }
.kinetic-section-copy h2 { font-size: 50px; line-height: 1.14; }
.kinetic-contact-content h2 span { display: block; }
.kinetic-hero-tagline { max-width: 610px; margin: 22px 0 0; color: var(--kinetic-text); font-size: 24px; font-weight: 560; line-height: 1.45; text-wrap: balance; }
.kinetic-section-description { max-width: 560px; margin: 18px 0 0; color: var(--kinetic-body); font-size: 16px; line-height: 1.75; text-wrap: pretty; }

.kinetic-section-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; margin-top: 30px; }
.kinetic-primary-action,
.kinetic-secondary-action {
  display: inline-flex;
  min-height: 44px;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 0 16px;
  border: 1px solid transparent;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 700;
  transition: color var(--motion-fast) var(--ease-standard), background-color var(--motion-fast) var(--ease-standard), border-color var(--motion-fast) var(--ease-standard), transform var(--motion-fast) var(--ease-standard), box-shadow var(--motion-fast) var(--ease-standard);
}
.kinetic-primary-action {
  color: #fff;
  background: color-mix(in srgb, var(--mr-primary) 88%, white);
  box-shadow: 0 12px 28px color-mix(in srgb, var(--mr-primary) 24%, transparent), inset 0 1px 0 rgba(255, 255, 255, 0.24);
}
.kinetic-primary-action:hover { background: color-mix(in srgb, var(--mr-primary-strong) 88%, white); transform: translateY(-1px); }
.kinetic-secondary-action { border-color: var(--kinetic-line); color: var(--kinetic-text); background: var(--kinetic-surface); backdrop-filter: blur(16px) saturate(112%); }
.kinetic-secondary-action:hover { border-color: color-mix(in srgb, var(--mr-secondary) 58%, white); background: rgba(16, 22, 28, 0.8); transform: translateY(-1px); }

.kinetic-hero-endpoint {
  display: flex;
  width: fit-content;
  max-width: 100%;
  min-height: 38px;
  align-items: center;
  gap: 12px;
  overflow-wrap: anywhere;
  margin-top: 22px;
  padding: 8px 11px;
  border: 1px solid rgba(255, 255, 255, 0.14);
  border-radius: 8px;
  color: rgba(255, 255, 255, 0.82);
  background: rgba(7, 11, 14, 0.54);
  font: 500 11px/1.5 ui-monospace, SFMono-Regular, Menlo, monospace;
  backdrop-filter: blur(14px);
}
.kinetic-hero-endpoint span { flex: 0 0 auto; color: var(--kinetic-muted); }

.kinetic-capability-list {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 22px;
  margin: 28px 0 0;
  padding: 0;
  color: var(--kinetic-text);
  font-size: 12px;
  font-weight: 650;
  list-style: none;
}
.kinetic-capability-list li { display: inline-flex; align-items: center; gap: 9px; }
.kinetic-capability-list li::before { width: 5px; height: 5px; flex: 0 0 auto; border-radius: 50%; background: var(--mr-secondary); box-shadow: 0 0 0 4px color-mix(in srgb, var(--mr-secondary) 12%, transparent); content: ''; }

.kinetic-about-layout {
  display: grid;
  grid-template-columns: minmax(0, 0.92fr) minmax(440px, 0.88fr);
  gap: clamp(52px, 7vw, 108px);
}
.kinetic-about-intro { width: 100%; }
.kinetic-about-directory {
  display: grid;
  min-width: 0;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  align-content: center;
  gap: 30px 34px;
}
.kinetic-about-group {
  min-width: 0;
  padding-top: 14px;
  border-top: 1px solid var(--kinetic-line);
}
.kinetic-about-group > span {
  display: block;
  margin-bottom: 7px;
  color: color-mix(in srgb, var(--mr-secondary) 80%, white);
  font: 650 9px/1.2 ui-monospace, SFMono-Regular, Menlo, monospace;
}
.kinetic-about-group h3 {
  margin: 0;
  color: var(--kinetic-text);
  font-size: 15px;
  font-weight: 700;
  line-height: 1.4;
}
.kinetic-about-group ul {
  display: flex;
  flex-wrap: wrap;
  gap: 7px 13px;
  margin: 12px 0 0;
  padding: 0;
  color: var(--kinetic-body);
  font-size: 12px;
  line-height: 1.5;
  list-style: none;
}
.kinetic-about-group li { position: relative; padding-left: 10px; }
.kinetic-about-group li::before {
  position: absolute;
  top: 0.66em;
  left: 0;
  width: 3px;
  height: 3px;
  border-radius: 50%;
  background: var(--mr-secondary);
  content: '';
}
.kinetic-about-industries { grid-column: 1 / -1; }

.kinetic-section-index {
  position: absolute;
  top: 50%;
  right: clamp(18px, 3vw, 48px);
  z-index: 13;
  display: grid;
  gap: 2px;
  transform: translateY(-50%);
}
.kinetic-section-index button {
  position: relative;
  display: grid;
  width: 44px;
  height: 44px;
  place-items: center;
  color: var(--kinetic-text);
}
.kinetic-section-index button::before {
  position: absolute;
  top: 50%;
  right: 38px;
  max-width: 180px;
  padding: 5px 8px;
  border: 1px solid var(--kinetic-line);
  border-radius: 6px;
  opacity: 0;
  color: var(--kinetic-text);
  background: rgba(8, 12, 15, 0.82);
  box-shadow: var(--shadow-sm);
  content: attr(data-label);
  font-size: 11px;
  pointer-events: none;
  transform: translate(5px, -50%);
  transition: opacity var(--motion-fast) var(--ease-standard), transform var(--motion-fast) var(--ease-standard);
  white-space: nowrap;
}
.kinetic-section-index button:hover::before,
.kinetic-section-index button:focus-visible::before { opacity: 1; transform: translate(0, -50%); }
.kinetic-section-index i {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.42);
  box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.08);
  transition: height var(--motion-base) var(--ease-enter), background-color var(--motion-base) var(--ease-standard), box-shadow var(--motion-base) var(--ease-standard);
}
.kinetic-section-index button.is-active i { height: 18px; background: color-mix(in srgb, var(--mr-secondary) 76%, white); box-shadow: 0 0 0 4px color-mix(in srgb, var(--mr-secondary) 12%, transparent); }

.kinetic-next-section {
  position: absolute;
  bottom: max(20px, env(safe-area-inset-bottom));
  left: 50%;
  z-index: 13;
  display: grid;
  width: 44px;
  height: 44px;
  place-items: center;
  border: 1px solid var(--kinetic-line);
  border-radius: 50%;
  color: var(--kinetic-text);
  background: rgba(7, 11, 14, 0.42);
  backdrop-filter: blur(14px);
  transform: translateX(-50%);
  transition: color var(--motion-fast) var(--ease-standard), border-color var(--motion-fast) var(--ease-standard), background-color var(--motion-fast) var(--ease-standard);
}
.kinetic-next-section:hover { border-color: color-mix(in srgb, var(--mr-secondary) 58%, white); color: color-mix(in srgb, var(--mr-secondary) 68%, white); background: rgba(10, 15, 19, 0.72); }

.kinetic-footer {
  position: absolute;
  right: clamp(24px, 4vw, 64px);
  bottom: max(18px, env(safe-area-inset-bottom));
  left: clamp(24px, 4vw, 64px);
  display: flex;
  min-height: 40px;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  color: var(--kinetic-muted);
  font-size: 10px;
}
.kinetic-footer-links { display: flex; align-items: center; gap: 18px; }
.kinetic-footer-links a { color: var(--kinetic-body); transition: color var(--motion-fast) var(--ease-standard); }
.kinetic-footer-links a:hover { color: var(--kinetic-text); }

.kinetic-loader { z-index: 30; display: grid; place-items: center; background: rgba(7, 10, 12, 0.9); }
.kinetic-loader-shell { display: flex; min-height: 42px; align-items: center; gap: 14px; }
.kinetic-loader-mark { display: grid; width: 42px; height: 42px; place-items: center; overflow: hidden; border: 1px solid rgba(255, 255, 255, 0.24); border-radius: 8px; color: #fff; background: color-mix(in srgb, var(--mr-primary) 78%, rgba(10, 14, 18, 0.9)); font: 700 17px/1 ui-monospace, SFMono-Regular, Menlo, monospace; }
.kinetic-loader-status { display: grid; min-width: 140px; align-items: center; grid-template-columns: 1fr auto; gap: 16px; color: var(--kinetic-body); font-size: 11px; line-height: 1; }
.kinetic-loader-status > span, .kinetic-loader-status strong { display: inline-flex; min-height: 16px; align-items: center; }
.kinetic-loader-status strong { min-width: 2ch; justify-content: flex-end; color: var(--kinetic-text); font: 600 12px/1 ui-monospace, SFMono-Regular, Menlo, monospace; text-align: right; }
.kinetic-loader-enter-active, .kinetic-loader-leave-active { transition: opacity 220ms var(--ease-standard); }
.kinetic-loader-enter-from, .kinetic-loader-leave-to { opacity: 0; }

.kinetic-skip-link {
  position: absolute;
  top: 8px;
  left: 16px;
  z-index: 40;
  padding: 9px 12px;
  border-radius: 8px;
  color: #fff;
  background: var(--mr-primary);
  transform: translateY(-160%);
  transition: transform var(--motion-fast) var(--ease-enter);
}
.kinetic-skip-link:focus { transform: translateY(0); }
.kinetic-sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; margin: -1px; padding: 0; border: 0; clip: rect(0, 0, 0, 0); white-space: nowrap; }

.kinetic-home button:focus-visible,
.kinetic-home a:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--mr-secondary) 68%, white);
  outline-offset: 3px;
}

@media (max-width: 900px) {
  .kinetic-world-shade { background: rgba(5, 9, 11, 0.34); box-shadow: inset 72vw 0 38vw -18vw rgba(3, 7, 9, 0.74), inset 0 -22vh 20vh -15vh rgba(3, 7, 9, 0.58); }
  .kinetic-section-inner { width: min(100% - 64px, 760px); }
  .kinetic-section-copy { width: min(580px, 72vw); }
  .kinetic-about-layout { grid-template-columns: minmax(0, 1fr) minmax(310px, 0.7fr); gap: 44px; }
  .kinetic-about-intro { width: 100%; }
  .kinetic-about-directory { grid-template-columns: 1fr; gap: 18px; }
  .kinetic-about-industries { grid-column: auto; }
  .kinetic-hero-copy h1 { font-size: 60px; }
  .kinetic-section-copy h2 { font-size: 44px; }
}

@media (max-width: 700px) {
  .kinetic-home { touch-action: pan-x pinch-zoom; }
  .kinetic-world-shade { background: rgba(4, 8, 10, 0.3); box-shadow: inset 0 48vh 30vh -13vh rgba(3, 7, 9, 0.82), inset 0 -18vh 18vh -12vh rgba(3, 7, 9, 0.5); }
  .kinetic-ambient { opacity: 0.34; }
  .kinetic-header { min-height: 64px; padding: max(10px, env(safe-area-inset-top)) 16px 8px; }
  .kinetic-wordmark-copy small { display: none; }
  .kinetic-wordmark-copy strong { max-width: 112px; }
  .kinetic-top-nav { gap: 10px; }
  .kinetic-canvas-nav { display: none !important; }
  .kinetic-nav-signal { display: none; }
  .kinetic-section-inner { width: calc(100% - 40px); align-items: flex-start; padding: 104px 0 122px; }
  .kinetic-section-copy { width: 100%; }
  .kinetic-about-layout { display: flex; overflow: hidden; flex-direction: column; align-items: stretch; gap: 22px; }
  .kinetic-about-intro { flex: 0 0 auto; }
  .kinetic-about-directory { width: 100%; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px 20px; }
  .kinetic-about-industries { grid-column: 1 / -1; }
  .kinetic-about-group { padding-top: 10px; }
  .kinetic-about-group > span { margin-bottom: 4px; }
  .kinetic-about-group ul { gap: 5px 11px; margin-top: 7px; font-size: 11px; }
  .kinetic-section-label { margin-bottom: 14px; font-size: 10px; }
  .kinetic-hero-copy h1 { font-size: 44px; line-height: 1.08; }
  .kinetic-section-copy h2 { max-width: 520px; font-size: 38px; line-height: 1.18; }
  .kinetic-hero-tagline { margin-top: 16px; font-size: 19px; line-height: 1.45; }
  .kinetic-section-description { max-width: 500px; margin-top: 12px; font-size: 14px; line-height: 1.65; }
  .kinetic-section-actions { margin-top: 22px; }
  .kinetic-hero-endpoint { margin-top: 16px; }
  .kinetic-capability-list { gap: 10px 16px; margin-top: 20px; }
  .kinetic-section-index { top: auto; right: 12px; bottom: max(18px, env(safe-area-inset-bottom)); display: flex; transform: none; }
  .kinetic-section-index button { width: 34px; height: 34px; }
  .kinetic-section-index button::before { display: none; }
  .kinetic-section-index button.is-active i { width: 18px; height: 5px; }
  .kinetic-next-section { bottom: max(16px, env(safe-area-inset-bottom)); left: 20px; width: 36px; height: 36px; transform: none; }
  .kinetic-footer { right: 20px; bottom: max(58px, calc(env(safe-area-inset-bottom) + 48px)); left: 20px; display: grid; gap: 8px; }
  .kinetic-footer-links { gap: 14px; }
}

@media (max-width: 430px) {
  .compact-home-nav { width: min(100% - 20px, 1180px); }
  .compact-home-actions :deep(.locale-switcher) { display: none; }
  .compact-home-content h1 { font-size: 38px; }
  .kinetic-wordmark-copy { display: none; }
  .kinetic-top-nav { gap: 8px; }
  .kinetic-top-nav a, .kinetic-top-nav button { font-size: 11px; }
  .kinetic-section-inner { width: calc(100% - 32px); padding-top: 94px; }
  .kinetic-hero-copy h1 { font-size: 40px; }
  .kinetic-section-copy h2 { font-size: 34px; }
  .kinetic-section-actions { gap: 8px; }
  .kinetic-about-layout { gap: 16px; }
  .kinetic-about-intro .kinetic-section-description { font-size: 13px; line-height: 1.55; }
  .kinetic-about-directory { gap: 10px 14px; }
  .kinetic-about-group h3 { font-size: 13px; }
  .kinetic-about-group ul { gap: 4px 9px; font-size: 10.5px; }
  .kinetic-primary-action, .kinetic-secondary-action { padding: 0 13px; font-size: 12px; }
  .kinetic-hero-endpoint { display: grid; gap: 3px; font-size: 10px; }
  .kinetic-footer-links { flex-wrap: wrap; gap: 8px 12px; }
}

@media (max-height: 720px) {
  .kinetic-section-inner { padding-top: 84px; padding-bottom: 78px; }
  .kinetic-section-label { margin-bottom: 10px; }
  .kinetic-hero-copy h1 { font-size: 42px; }
  .kinetic-section-copy h2 { font-size: 36px; }
  .kinetic-hero-tagline { margin-top: 12px; font-size: 18px; }
  .kinetic-section-description { margin-top: 10px; line-height: 1.55; }
  .kinetic-section-actions { margin-top: 16px; }
  .kinetic-hero-endpoint { margin-top: 12px; }
  .kinetic-capability-list { margin-top: 16px; }
  .kinetic-about-layout { gap: 12px; }
  .kinetic-about-intro h2 { font-size: 30px; line-height: 1.14; }
  .kinetic-about-intro .kinetic-section-description { max-width: 680px; font-size: 12.5px; line-height: 1.5; }
  .kinetic-about-intro .kinetic-section-actions { margin-top: 12px; }
  .kinetic-about-group { padding-top: 7px; }
  .kinetic-about-group ul { margin-top: 5px; font-size: 10px; line-height: 1.35; }
}

@media (min-width: 701px) and (max-height: 720px) {
  .kinetic-about-directory { grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 18px; }
  .kinetic-about-industries { grid-column: auto; }
}

@media (prefers-reduced-motion: reduce) {
  .kinetic-loader-enter-active,
  .kinetic-loader-leave-active,
  .kinetic-section-index i,
  .kinetic-section-index button::before,
  .kinetic-primary-action,
  .kinetic-secondary-action,
  .kinetic-next-section { transition-duration: 1ms; }
}
</style>
