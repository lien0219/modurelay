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

  <div
    v-else-if="compactHomeEnabled"
    data-testid="compact-home"
    class="compact-home flex min-h-screen flex-col"
  >
    <header class="compact-home-header">
      <nav class="compact-home-nav" aria-label="Home navigation">
        <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="compact-home-brand">
          <span class="brand-mark"><img :src="siteLogo || brand.logo" :alt="siteName" /></span>
          <span>{{ siteName }}</span>
        </router-link>
        <div class="compact-home-actions">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="home-icon-button"
            :title="copy.chrome.docs"
          >
            <Icon name="book" size="sm" />
          </a>
          <router-link v-if="showModelPlazaEntry" to="/model-plaza" class="home-quiet-link">
            <Icon name="grid" size="sm" />
            <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
          </router-link>
          <button
            type="button"
            class="home-icon-button"
            :title="isDark ? copy.chrome.light : copy.chrome.dark"
            @click="toggleTheme"
          >
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
          {{ isAuthenticated ? copy.hero.dashboardCta : copy.hero.primaryCta }}
          <Icon name="arrowRight" size="sm" />
        </router-link>
      </div>
    </main>
    <footer class="compact-home-footer">&copy; {{ currentYear }} {{ siteName }}</footer>
  </div>

  <div
    v-else
    ref="pageRef"
    class="home-page at-home"
    :class="{
      'at-home-scrolled': isScrolled,
      'at-menu-is-open': menuOpen,
      'at-scene-is-ready': !loaderVisible
    }"
  >
    <a class="at-skip-link" href="#at-main">{{ copy.chrome.skip }}</a>

    <div class="at-fixed-world" aria-hidden="true">
      <HomeHeroScene @ready="handleSceneReady" />
    </div>

    <Transition name="at-loader">
      <div v-if="loaderVisible" class="at-loader" role="status" :aria-label="copy.chrome.loading">
        <div class="at-loader-brand"><span>M</span><strong>MODURELAY</strong></div>
        <div class="at-loader-meta">
          <span>{{ copy.chrome.loading }}</span>
          <span>{{ String(Math.round(loaderProgress)).padStart(3, '0') }}</span>
        </div>
        <div class="at-loader-track"><span :style="{ transform: `scaleX(${loaderProgress / 100})` }"></span></div>
      </div>
    </Transition>

    <header class="at-header">
      <button type="button" class="at-brand" :aria-label="copy.nav.home" @click="scrollToSection('home')">
        <span class="at-brand-mark"><img :src="siteLogo || brand.logo" :alt="siteName" /></span>
        <span class="at-brand-copy"><strong>{{ siteName }}</strong><small>AI RELAY SYSTEMS</small></span>
      </button>

      <div class="at-header-coordinate" aria-hidden="true">
        <span>{{ copy.chrome.coordinate }}</span>
        <i></i>
        <span>{{ String(activeIndex + 1).padStart(2, '0') }} / {{ String(navigationItems.length).padStart(2, '0') }}</span>
      </div>

      <div class="at-header-actions">
        <LocaleSwitcher class="at-locale" />
        <router-link
          v-if="showModelPlazaEntry"
          to="/model-plaza"
          class="at-icon-control"
          :title="t('nav.modelPlaza')"
          :aria-label="t('nav.modelPlaza')"
        >
          <Icon name="grid" size="sm" />
        </router-link>
        <button
          type="button"
          class="at-icon-control"
          :title="isDark ? copy.chrome.light : copy.chrome.dark"
          :aria-label="isDark ? copy.chrome.light : copy.chrome.dark"
          @click="toggleTheme"
        >
          <Icon v-if="isDark" name="sun" size="sm" />
          <Icon v-else name="moon" size="sm" />
        </button>
        <button
          ref="menuButtonRef"
          type="button"
          class="at-menu-control"
          :aria-expanded="menuOpen"
          aria-controls="at-home-menu"
          @click="toggleMenu"
        >
          <span>{{ menuOpen ? copy.chrome.close : copy.chrome.menu }}</span>
          <i aria-hidden="true"></i>
        </button>
      </div>
    </header>

    <aside class="at-side-status" aria-hidden="true">
      <span class="at-side-status-dot"></span>
      <span>{{ copy.chrome.system }}</span>
      <i></i>
      <span>{{ Math.round(scrollProgress).toString().padStart(3, '0') }}%</span>
    </aside>

    <main id="at-main" class="at-main">
      <section id="home" data-home-section class="at-chapter at-hero" aria-labelledby="at-hero-title">
        <div class="at-hero-kicker at-reveal">
          <span>OPENAI-COMPATIBLE</span>
          <span>ROUTING / METERING / FAILOVER</span>
        </div>

        <h1 id="at-hero-title" class="at-hero-title" :aria-label="copy.hero.ariaTitle">
          <span class="at-hero-line at-hero-line-one at-reveal">{{ copy.hero.lineOne }}</span>
          <span class="at-hero-line at-hero-line-two at-reveal">{{ copy.hero.lineTwo }}</span>
        </h1>

        <div class="at-hero-bottom">
          <div class="at-hero-intro at-reveal">
            <p>{{ copy.hero.description }}</p>
            <div class="at-inline-facts" :aria-label="copy.hero.factsLabel">
              <span v-for="fact in copy.hero.facts" :key="fact"><i></i>{{ fact }}</span>
            </div>
          </div>
          <div class="at-hero-cta at-reveal">
            <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="at-orbit-link">
              <span>{{ isAuthenticated ? copy.hero.dashboardCta : copy.hero.primaryCta }}</span>
              <Icon name="arrowRight" size="sm" />
            </router-link>
            <button type="button" class="at-text-link" @click="scrollToSection('integrate')">
              {{ copy.hero.explore }}<Icon name="arrowDown" size="sm" />
            </button>
          </div>
        </div>
      </section>

      <section id="integrate" data-home-section class="at-chapter at-chapter-left" aria-labelledby="at-integrate-title">
        <article class="at-story-panel at-story-panel-wide at-reveal">
          <div class="at-chapter-label"><span>01</span><i></i><span>INTEGRATE</span></div>
          <h2 id="at-integrate-title">
            <span>{{ copy.integrate.lineOne }}</span>
            <span>{{ copy.integrate.lineTwo }}</span>
          </h2>
          <p>{{ copy.integrate.description }}</p>

          <div class="at-endpoints" :aria-label="copy.integrate.endpointsLabel">
            <div v-for="endpoint in copy.integrate.endpoints" :key="endpoint.path" class="at-endpoint-row">
              <span>{{ endpoint.method }}</span>
              <code>{{ endpoint.path }}</code>
              <small>{{ endpoint.label }}</small>
            </div>
          </div>

          <code class="at-command">curl {{ apiBaseUrl }}/v1/chat/completions</code>
        </article>
      </section>

      <section id="routing" data-home-section class="at-chapter at-chapter-right" aria-labelledby="at-routing-title">
        <article class="at-story-panel at-reveal">
          <div class="at-chapter-label"><span>02</span><i></i><span>ROUTING CORE</span></div>
          <h2 id="at-routing-title">
            <span>{{ copy.routing.lineOne }}</span>
            <span>{{ copy.routing.lineTwo }}</span>
          </h2>
          <p>{{ copy.routing.description }}</p>
          <ol class="at-system-list">
            <li v-for="(item, index) in copy.routing.items" :key="item.title">
              <span>{{ String(index + 1).padStart(2, '0') }}</span>
              <div><strong>{{ item.title }}</strong><small>{{ item.description }}</small></div>
            </li>
          </ol>
        </article>
      </section>

      <section id="observability" data-home-section class="at-chapter at-chapter-left" aria-labelledby="at-observability-title">
        <article class="at-story-panel at-story-panel-wide at-reveal">
          <div class="at-chapter-label"><span>03</span><i></i><span>OBSERVABILITY</span></div>
          <h2 id="at-observability-title">
            <span>{{ copy.observability.lineOne }}</span>
            <span>{{ copy.observability.lineTwo }}</span>
          </h2>
          <p>{{ copy.observability.description }}</p>
          <div class="at-signal-board" :aria-label="copy.observability.boardLabel">
            <div v-for="metric in copy.observability.metrics" :key="metric.label">
              <span>{{ metric.label }}</span>
              <strong>{{ metric.value }}</strong>
              <small><i></i>{{ metric.detail }}</small>
            </div>
          </div>
          <router-link to="/key-usage" class="at-underlined-link">
            {{ copy.observability.cta }}<Icon name="arrowRight" size="sm" />
          </router-link>
        </article>
      </section>

      <section id="learning" data-home-section class="at-chapter at-chapter-right" aria-labelledby="at-learning-title">
        <article class="at-story-panel at-reveal">
          <div class="at-chapter-label"><span>04</span><i></i><span>AI LEARNING</span></div>
          <h2 id="at-learning-title">
            <span>{{ copy.learning.lineOne }}</span>
            <span>{{ copy.learning.lineTwo }}</span>
          </h2>
          <p>{{ copy.learning.description }}</p>
          <div class="at-topic-cloud" :aria-label="copy.learning.topicsLabel">
            <span v-for="topic in copy.learning.topics" :key="topic">{{ topic }}</span>
          </div>
          <router-link :to="learningEntry" class="at-orbit-link at-orbit-link-small">
            <span>{{ copy.learning.cta }}</span><Icon name="arrowRight" size="sm" />
          </router-link>
        </article>
      </section>

      <section id="contact" data-home-section class="at-chapter at-contact" aria-labelledby="at-contact-title">
        <div class="at-contact-copy at-reveal">
          <div class="at-chapter-label"><span>05</span><i></i><span>BEGIN</span></div>
          <h2 id="at-contact-title">
            <span>{{ copy.contact.lineOne }}</span>
            <span>{{ copy.contact.lineTwo }}</span>
          </h2>
          <p>{{ copy.contact.description }}</p>
          <div class="at-contact-actions">
            <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="at-orbit-link">
              <span>{{ isAuthenticated ? copy.hero.dashboardCta : copy.contact.primaryCta }}</span>
              <Icon name="arrowRight" size="sm" />
            </router-link>
            <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="at-text-link">
              {{ copy.contact.docsCta }}<Icon name="externalLink" size="sm" />
            </a>
          </div>
        </div>

        <footer class="home-footer at-footer">
          <span>&copy; {{ currentYear }} {{ siteName }}</span>
          <span>{{ copy.footer.tagline }}</span>
          <span>MODURELAY / SYSTEM 01</span>
        </footer>
      </section>
    </main>

    <nav class="at-pill-nav" :style="pillStyle" :aria-label="copy.chrome.chapterNavigation">
      <span class="at-pill-current">{{ String(activeIndex + 1).padStart(2, '0') }}</span>
      <div class="at-pill-items">
        <button
          v-for="(item, index) in navigationItems"
          :key="item.id"
          type="button"
          :class="{ 'is-active': activeSection === item.id }"
          :aria-current="activeSection === item.id ? 'step' : undefined"
          :aria-label="item.label"
          @click="scrollToSection(item.id)"
        >
          <i></i><span>{{ item.shortLabel }}</span><small>{{ String(index + 1).padStart(2, '0') }}</small>
        </button>
      </div>
      <span class="at-pill-word">{{ activeNavigationItem?.shortLabel }}</span>
    </nav>

    <Transition name="at-menu">
      <div
        v-if="menuOpen"
        id="at-home-menu"
        ref="menuPanelRef"
        class="at-menu-overlay"
        role="dialog"
        aria-modal="true"
        :aria-label="copy.chrome.menu"
      >
        <div class="at-menu-meta">
          <span>MODURELAY / INDEX</span>
          <span>{{ copy.chrome.menuHint }}</span>
        </div>
        <div class="at-menu-grid">
          <div class="at-menu-links">
            <button
              v-for="(item, index) in navigationItems"
              :key="item.id"
              type="button"
              :class="{ 'is-current': activeSection === item.id }"
              @click="selectMenuSection(item.id)"
            >
              <span>{{ String(index + 1).padStart(2, '0') }}</span>
              <strong>{{ item.label }}</strong>
              <Icon name="arrowRight" size="md" />
            </button>
          </div>
          <div class="at-menu-aside">
            <div>
              <span>{{ copy.chrome.quickLinks }}</span>
              <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">{{ copy.chrome.docs }}</a>
              <router-link v-if="showModelPlazaEntry" to="/model-plaza" @click="closeMenu">{{ t('nav.modelPlaza') }}</router-link>
              <router-link :to="isAuthenticated ? dashboardPath : '/login'" @click="closeMenu">
                {{ isAuthenticated ? copy.chrome.dashboard : copy.chrome.login }}
              </router-link>
            </div>
            <p>{{ siteSubtitle }}</p>
          </div>
        </div>
      </div>
    </Transition>
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
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import { toggleThemeWithTransition } from '@/utils/themeTransition'

type SectionId = 'home' | 'integrate' | 'routing' | 'observability' | 'learning' | 'contact'

const zhCopy = {
  chrome: {
    coordinate: '杭州 / 30.2°N 120.2°E', system: 'RELAY ONLINE', menu: '菜单', close: '关闭',
    menuHint: '滚动穿行，点击直达', quickLinks: '快速入口', docs: '文档', dashboard: '控制台', login: '登录',
    light: '切换浅色模式', dark: '切换深色模式', loading: '正在构建中继空间', skip: '跳到主要内容', chapterNavigation: '首页章节导航'
  },
  nav: { home: '首页', integrate: '统一接入', routing: '路由核心', observability: '可观测性', learning: 'AI 学习', contact: '开始使用' },
  navShort: { home: 'HOME', integrate: 'API', routing: 'ROUTE', observability: 'SIGNAL', learning: 'LEARN', contact: 'BEGIN' },
  hero: {
    ariaTitle: '一个入口，连接每个模型', lineOne: '一个入口', lineTwo: '连接每个模型',
    description: 'ModuRelay 把 OpenAI 兼容接口、多提供商路由、账户池、用量计量与故障切换放进同一条可靠链路。',
    primaryCta: '启动中继', dashboardCta: '进入控制台', explore: '向下探索', factsLabel: '核心能力', facts: ['统一 API', '策略路由', '用量可见']
  },
  integrate: {
    lineOne: '调用方式不变', lineTwo: '能力随路由扩展',
    description: '替换 base URL，继续使用熟悉的 SDK 与请求结构。鉴权、模型发现和响应格式保持清晰。', endpointsLabel: '支持的接口示例',
    endpoints: [
      { method: 'POST', path: '/v1/chat/completions', label: '对话补全' },
      { method: 'POST', path: '/v1/responses', label: 'Responses API' },
      { method: 'GET', path: '/v1/models', label: '模型发现' }
    ]
  },
  routing: {
    lineOne: '请求进入核心', lineTwo: '策略决定路径', description: '依据模型能力、分组策略与线路健康选择可用路径；异常发生时按既有配置继续转发。',
    items: [
      { title: 'ACCOUNT POOLS', description: '将提供商账户组织成可维护的资源池。' },
      { title: 'POLICY ROUTING', description: '按分组、模型和优先级匹配转发策略。' },
      { title: 'FAILOVER', description: '线路异常时切换到配置允许的可用路径。' }
    ]
  },
  observability: {
    lineOne: '每个请求', lineTwo: '都留下信号', description: '请求、模型、密钥、用量与线路状态汇聚在同一套运行视图中，让排查有上下文。',
    boardLabel: '可观测性指标', cta: '查询密钥用量',
    metrics: [
      { label: 'REQUEST', value: 'TRACE', detail: '请求上下文' },
      { label: 'USAGE', value: 'METER', detail: 'Token 与额度' },
      { label: 'ROUTE', value: 'HEALTH', detail: '线路状态' }
    ]
  },
  learning: {
    lineOne: '从系统能力', lineTwo: '走向 AI 实践', description: '围绕 Agent、工作流、记忆与具身智能，把技术判断整理成案例、练习和可以复盘的学习路径。',
    topicsLabel: '学习主题', topics: ['AGENT', 'WORKFLOW', 'MEMORY', 'EMBODIED AI'], cta: '进入 AI 学习'
  },
  contact: { lineOne: '让应用只面对', lineTwo: '一个可靠入口', description: '从兼容端点开始，把模型、路由、账户与用量带回同一条链路。', primaryCta: '开始使用', docsCta: '阅读文档' },
  footer: { tagline: '接入简单，运行清楚' }
}

const enCopy = {
  chrome: {
    coordinate: 'HANGZHOU / 30.2°N 120.2°E', system: 'RELAY ONLINE', menu: 'MENU', close: 'CLOSE',
    menuHint: 'Scroll through space or jump directly', quickLinks: 'QUICK LINKS', docs: 'Documentation', dashboard: 'Dashboard', login: 'Sign in',
    light: 'Switch to light mode', dark: 'Switch to dark mode', loading: 'Building relay space', skip: 'Skip to main content', chapterNavigation: 'Homepage chapter navigation'
  },
  nav: { home: 'Home', integrate: 'Unified API', routing: 'Routing core', observability: 'Observability', learning: 'AI Learning', contact: 'Get started' },
  navShort: { home: 'HOME', integrate: 'API', routing: 'ROUTE', observability: 'SIGNAL', learning: 'LEARN', contact: 'BEGIN' },
  hero: {
    ariaTitle: 'One entry point for every model', lineOne: 'One entry point', lineTwo: 'Every model in reach',
    description: 'ModuRelay brings OpenAI-compatible APIs, multi-provider routing, account pools, usage metering and failover into one reliable path.',
    primaryCta: 'Start the relay', dashboardCta: 'Open dashboard', explore: 'Scroll to explore', factsLabel: 'Core capabilities', facts: ['Unified API', 'Policy routing', 'Visible usage']
  },
  integrate: {
    lineOne: 'Keep the call familiar', lineTwo: 'Expand through routing',
    description: 'Change the base URL and keep using the SDKs and request shapes you know. Authentication, model discovery and responses stay clear.', endpointsLabel: 'Supported endpoint examples',
    endpoints: [
      { method: 'POST', path: '/v1/chat/completions', label: 'Chat completions' },
      { method: 'POST', path: '/v1/responses', label: 'Responses API' },
      { method: 'GET', path: '/v1/models', label: 'Model discovery' }
    ]
  },
  routing: {
    lineOne: 'Requests enter the core', lineTwo: 'Policy finds the path', description: 'Choose an available route from model capability, group policy and route health, then follow configured failover when needed.',
    items: [
      { title: 'ACCOUNT POOLS', description: 'Organize provider accounts into maintainable resource pools.' },
      { title: 'POLICY ROUTING', description: 'Match forwarding policy by group, model and priority.' },
      { title: 'FAILOVER', description: 'Move to an allowed available route when a line fails.' }
    ]
  },
  observability: {
    lineOne: 'Every request', lineTwo: 'Leaves a signal', description: 'Bring request, model, key, usage and route status into one operational view with the context needed to investigate.',
    boardLabel: 'Observability signals', cta: 'Look up key usage',
    metrics: [
      { label: 'REQUEST', value: 'TRACE', detail: 'Request context' },
      { label: 'USAGE', value: 'METER', detail: 'Tokens and quota' },
      { label: 'ROUTE', value: 'HEALTH', detail: 'Line status' }
    ]
  },
  learning: {
    lineOne: 'From system capability', lineTwo: 'To AI practice', description: 'Turn technical judgement around agents, workflows, memory and embodied AI into cases, exercises and reviewable learning paths.',
    topicsLabel: 'Learning topics', topics: ['AGENT', 'WORKFLOW', 'MEMORY', 'EMBODIED AI'], cta: 'Enter AI Learning'
  },
  contact: { lineOne: 'Give your application', lineTwo: 'One reliable entry', description: 'Start with a compatible endpoint and bring models, routing, accounts and usage back into one path.', primaryCta: 'Get started', docsCta: 'Read the docs' },
  footer: { tagline: 'Simple to connect. Clear to operate.' }
}

const { t, locale } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const pageRef = ref<HTMLElement | null>(null)
const menuPanelRef = ref<HTMLElement | null>(null)
const menuButtonRef = ref<HTMLButtonElement | null>(null)
const activeSection = ref<SectionId>('home')
const isScrolled = ref(false)
const scrollProgress = ref(0)
const scrollVelocity = ref(0)
const isDark = ref(document.documentElement.classList.contains('dark'))
const menuOpen = ref(false)
const loaderVisible = ref(true)
const loaderProgress = ref(6)

let sectionObserver: IntersectionObserver | null = null
let revealObserver: IntersectionObserver | null = null
let scrollFrame = 0
let velocityTimer = 0
let loaderTimer = 0
let loaderSafetyTimer = 0
let loaderHideTimer = 0
let previousScrollY = 0
let previousScrollTime = 0
let previousHtmlOverflow = ''
let homeMatchMedia: ReturnType<typeof gsap.matchMedia> | null = null
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
const navigationItems = computed(() => (Object.keys(copy.value.nav) as SectionId[]).map((id) => ({
  id,
  label: copy.value.nav[id],
  shortLabel: copy.value.navShort[id]
})))
const activeIndex = computed(() => Math.max(0, navigationItems.value.findIndex(item => item.id === activeSection.value)))
const activeNavigationItem = computed(() => navigationItems.value[activeIndex.value])
const pillStyle = computed(() => ({
  '--pill-skew': `${scrollVelocity.value * -4.5}deg`,
  '--pill-stretch': `${1 + Math.abs(scrollVelocity.value) * 0.075}`
}))

function toggleTheme(event?: MouseEvent) {
  isDark.value = toggleThemeWithTransition(isDark.value, event)
}

function scrollToSection(id: SectionId) {
  const target = document.getElementById(id)
  if (!target) return
  target.scrollIntoView({
    behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth',
    block: 'start'
  })
}

function selectMenuSection(id: SectionId) {
  closeMenu(false)
  window.requestAnimationFrame(() => scrollToSection(id))
}

async function toggleMenu() {
  if (menuOpen.value) {
    closeMenu()
    return
  }
  menuOpen.value = true
  await nextTick()
  menuPanelRef.value?.querySelector<HTMLButtonElement>('button')?.focus()
}

function closeMenu(restoreFocus = true) {
  if (!menuOpen.value) return
  menuOpen.value = false
  if (restoreFocus) window.requestAnimationFrame(() => menuButtonRef.value?.focus())
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && menuOpen.value) closeMenu()
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
  loaderProgress.value = 6
  loaderTimer = window.setInterval(() => {
    loaderProgress.value = Math.min(92, loaderProgress.value + Math.max(1, (94 - loaderProgress.value) * 0.075))
  }, 90)
  loaderSafetyTimer = window.setTimeout(handleSceneReady, 5200)
}

function handleSceneReady() {
  window.clearInterval(loaderTimer)
  window.clearTimeout(loaderSafetyTimer)
  loaderTimer = 0
  loaderSafetyTimer = 0
  loaderProgress.value = 100
  const delay = window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 0 : 360
  loaderHideTimer = window.setTimeout(() => {
    loaderVisible.value = false
    loaderHideTimer = 0
  }, delay)
}

function updateScrollState() {
  cancelAnimationFrame(scrollFrame)
  scrollFrame = requestAnimationFrame(() => {
    const now = performance.now()
    const scrollTop = window.scrollY || document.documentElement.scrollTop
    const scrollable = Math.max(document.documentElement.scrollHeight - window.innerHeight, 1)
    const deltaTime = Math.max(16, now - previousScrollTime)
    const instantaneousVelocity = (scrollTop - previousScrollY) / deltaTime
    scrollVelocity.value = Math.max(-1, Math.min(1, instantaneousVelocity * 0.72))
    scrollProgress.value = Math.min(100, Math.max(0, (scrollTop / scrollable) * 100))
    isScrolled.value = scrollTop > 32
    previousScrollY = scrollTop
    previousScrollTime = now
    window.clearTimeout(velocityTimer)
    velocityTimer = window.setTimeout(() => { scrollVelocity.value = 0 }, 120)
  })
}

function initializeHomeMotion() {
  const root = pageRef.value
  if (!root) return
  const sections = Array.from(root.querySelectorAll<HTMLElement>('[data-home-section]'))
  if ('IntersectionObserver' in window) {
    sectionObserver = new IntersectionObserver((entries) => {
      const visibleEntries = entries
        .filter(entry => entry.isIntersecting)
        .sort((a, b) => b.intersectionRatio - a.intersectionRatio)
      if (visibleEntries[0]?.target.id) activeSection.value = visibleEntries[0].target.id as SectionId
    }, { rootMargin: '-25% 0px -55% 0px', threshold: [0.08, 0.2, 0.45, 0.7] })
    sections.forEach(section => sectionObserver?.observe(section))
  }

  const reveals = Array.from(root.querySelectorAll<HTMLElement>('.at-reveal'))
  const motionQuery = window.matchMedia('(prefers-reduced-motion: reduce)')
  const supportsMediaListeners = typeof motionQuery.addEventListener === 'function' || typeof motionQuery.addListener === 'function'
  if (!supportsMediaListeners || motionQuery.matches) {
    gsap.set(reveals, { autoAlpha: 1, y: 0, rotateX: 0 })
    return
  }

  homeMatchMedia = gsap.matchMedia()
  homeMatchMedia.add({ reduceMotion: '(prefers-reduced-motion: reduce)' }, (context) => {
    if (context.conditions?.reduceMotion) {
      gsap.set(reveals, { autoAlpha: 1, y: 0, rotateX: 0 })
      return
    }

    const heroReveals = Array.from(root.querySelectorAll<HTMLElement>('.at-hero .at-reveal'))
    gsap.timeline({ defaults: { duration: 0.82, ease: 'power3.out' } })
      .fromTo(heroReveals, { autoAlpha: 0, y: 36, rotateX: -3 }, { autoAlpha: 1, y: 0, rotateX: 0, stagger: 0.075 })

    if ('IntersectionObserver' in window) {
      revealObserver = new IntersectionObserver((entries, observer) => {
        entries.forEach((entry) => {
          if (!entry.isIntersecting || entry.target.closest('.at-hero')) return
          gsap.fromTo(entry.target, { autoAlpha: 0, y: 42, rotateX: -2 }, {
            autoAlpha: 1, y: 0, rotateX: 0, duration: 0.72, ease: 'power3.out'
          })
          observer.unobserve(entry.target)
        })
      }, { rootMargin: '0px 0px -14% 0px', threshold: 0.12 })
      reveals.forEach(element => revealObserver?.observe(element))
    } else {
      gsap.set(reveals, { autoAlpha: 1, y: 0, rotateX: 0 })
    }

    return () => revealObserver?.disconnect()
  })
}

function cleanupHomeMotion() {
  cancelAnimationFrame(scrollFrame)
  window.clearTimeout(velocityTimer)
  window.removeEventListener('scroll', updateScrollState)
  window.removeEventListener('resize', updateScrollState)
  window.removeEventListener('keydown', onKeydown)
  sectionObserver?.disconnect()
  sectionObserver = null
  revealObserver?.disconnect()
  revealObserver = null
  homeMatchMedia?.revert()
  homeMatchMedia = null
}

async function syncHomeMode() {
  cleanupHomeMotion()
  closeMenu(false)
  isScrolled.value = false
  scrollProgress.value = 0
  activeSection.value = 'home'
  if (hasHomeContent.value || compactHomeEnabled.value) {
    clearLoaderTimers()
    loaderVisible.value = false
    return
  }

  beginLoader()
  await nextTick()
  if (hasHomeContent.value || compactHomeEnabled.value) return
  previousScrollY = window.scrollY
  previousScrollTime = performance.now()
  initializeHomeMotion()
  updateScrollState()
  window.addEventListener('scroll', updateScrollState, { passive: true })
  window.addEventListener('resize', updateScrollState, { passive: true })
  window.addEventListener('keydown', onKeydown)
}

watch(menuOpen, (open) => {
  if (open) {
    previousHtmlOverflow = document.documentElement.style.overflow
    document.documentElement.style.overflow = 'hidden'
  } else {
    document.documentElement.style.overflow = previousHtmlOverflow
  }
})

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
  cleanupHomeMotion()
  clearLoaderTimers()
  document.documentElement.style.overflow = previousHtmlOverflow
})
</script>

<style scoped>
.compact-home { background: var(--mr-canvas); color: var(--mr-text); }
.compact-home-header { border-bottom: 1px solid var(--mr-border); background: color-mix(in srgb, var(--mr-surface) 78%, transparent); backdrop-filter: blur(18px) saturate(135%); -webkit-backdrop-filter: blur(18px) saturate(135%); }
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

.at-home {
  --at-bg: #080a10;
  --at-ink: #f3f5fb;
  --at-muted: #9aa2b5;
  --at-subtle: #6f788d;
  --at-line: rgba(207, 216, 241, 0.19);
  --at-line-strong: rgba(224, 230, 249, 0.34);
  --at-panel: rgba(11, 14, 23, 0.68);
  --at-panel-strong: rgba(11, 14, 23, 0.88);
  --at-menu: rgba(7, 9, 15, 0.97);
  --at-primary: #818cf8;
  --at-accent: #67e8f9;
  position: relative;
  width: 100%;
  max-width: 100vw;
  min-height: 100vh;
  overflow: clip;
  color: var(--at-ink);
  background: var(--at-bg);
  font-family: "Noto Sans SC Variable", "Noto Sans SC", system-ui, sans-serif;
  font-synthesis: none;
}

:global(html:not(.dark)) .at-home {
  --at-bg: #dfe5f2;
  --at-ink: #111526;
  --at-muted: #4d5568;
  --at-subtle: #687185;
  --at-line: rgba(48, 57, 79, 0.2);
  --at-line-strong: rgba(36, 44, 66, 0.36);
  --at-panel: rgba(238, 242, 250, 0.7);
  --at-panel-strong: rgba(239, 243, 250, 0.9);
  --at-menu: rgba(224, 230, 241, 0.98);
  --at-primary: #4338ca;
  --at-accent: #087e9b;
}

.at-skip-link { position: fixed; top: 10px; left: 50%; z-index: 120; padding: 10px 16px; border-radius: 999px; color: #fff; background: var(--color-primary); transform: translate(-50%, -160%); transition: transform var(--motion-fast) var(--ease-enter); }
.at-skip-link:focus { transform: translate(-50%, 0); }
.at-fixed-world { position: fixed; inset: 0; z-index: 0; pointer-events: none; }
.at-main { position: relative; z-index: 2; }

.at-header {
  position: fixed;
  top: 0;
  right: 0;
  left: 0;
  z-index: 72;
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  gap: 20px;
  min-height: 86px;
  padding: 0 clamp(18px, 3vw, 48px);
  border-bottom: 1px solid transparent;
  transition: background-color var(--motion-base) var(--ease-standard), border-color var(--motion-base) var(--ease-standard);
}
.at-home-scrolled .at-header { border-color: var(--at-line); background: color-mix(in srgb, var(--at-panel-strong) 82%, transparent); backdrop-filter: blur(18px) saturate(118%); -webkit-backdrop-filter: blur(18px) saturate(118%); }
.at-brand { display: inline-flex; width: max-content; min-width: 0; align-items: center; gap: 12px; color: var(--at-ink); text-align: left; }
.at-brand-mark { display: grid; width: 38px; height: 38px; flex: 0 0 auto; place-items: center; overflow: hidden; border: 1px solid var(--at-line-strong); border-radius: 50%; background: var(--at-panel-strong); }
.at-brand-mark img { width: 76%; height: 76%; object-fit: contain; }
.at-brand-copy { display: flex; min-width: 0; flex-direction: column; gap: 2px; }
.at-brand-copy strong { overflow: hidden; max-width: 180px; font-size: 12px; font-weight: 720; letter-spacing: 0.02em; text-overflow: ellipsis; white-space: nowrap; }
.at-brand-copy small { color: var(--at-subtle); font: 650 8px/1.2 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.15em; }
.at-header-coordinate { display: flex; align-items: center; gap: 12px; color: var(--at-muted); font: 600 9px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.12em; }
.at-header-coordinate i { display: block; width: 28px; height: 1px; background: var(--at-line-strong); }
.at-header-actions { display: flex; align-items: center; justify-content: flex-end; gap: 7px; }
.at-icon-control, .at-menu-control { display: inline-flex; min-width: 38px; min-height: 38px; align-items: center; justify-content: center; border: 1px solid var(--at-line); color: var(--at-ink); background: var(--at-panel); backdrop-filter: blur(14px); -webkit-backdrop-filter: blur(14px); transition: color var(--motion-fast) var(--ease-standard), border-color var(--motion-fast) var(--ease-standard), background-color var(--motion-fast) var(--ease-standard), transform var(--motion-fast) var(--ease-standard); }
.at-icon-control { border-radius: 50%; }
.at-menu-control { gap: 11px; min-width: 94px; padding: 0 15px; border-radius: 999px; font: 700 9px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.12em; }
.at-menu-control i { position: relative; display: block; width: 13px; height: 9px; border-top: 1px solid currentColor; border-bottom: 1px solid currentColor; }
.at-menu-is-open .at-menu-control i { height: 13px; border: 0; }
.at-menu-is-open .at-menu-control i::before, .at-menu-is-open .at-menu-control i::after { position: absolute; top: 6px; left: 0; width: 13px; height: 1px; background: currentColor; content: ''; }
.at-menu-is-open .at-menu-control i::before { transform: rotate(45deg); }
.at-menu-is-open .at-menu-control i::after { transform: rotate(-45deg); }
.at-icon-control:hover, .at-menu-control:hover { border-color: var(--at-line-strong); background: var(--at-panel-strong); transform: translateY(-1px); }
.at-locale { color: var(--at-ink); }
.at-header :deep(button:focus-visible), .at-header :deep(a:focus-visible), .at-pill-nav button:focus-visible, .at-story-panel a:focus-visible, .at-story-panel button:focus-visible, .at-contact a:focus-visible, .at-menu-overlay button:focus-visible, .at-menu-overlay a:focus-visible { outline: 2px solid var(--at-primary); outline-offset: 3px; }

.at-side-status { position: fixed; top: 50%; left: clamp(12px, 2vw, 34px); z-index: 24; display: flex; align-items: center; gap: 10px; color: var(--at-subtle); font: 650 8px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.16em; transform: translate(-42%, -50%) rotate(-90deg); transform-origin: center; pointer-events: none; }
.at-side-status-dot { width: 5px; height: 5px; border-radius: 50%; background: var(--color-success); box-shadow: 0 0 0 4px color-mix(in srgb, var(--color-success) 13%, transparent); }
.at-side-status i { display: block; width: 44px; height: 1px; background: var(--at-line-strong); }

.at-chapter { position: relative; display: grid; min-height: 100svh; align-items: center; padding: clamp(124px, 14vh, 164px) clamp(28px, 7vw, 112px) clamp(120px, 15vh, 172px); scroll-margin-top: 0; pointer-events: none; }
.at-chapter > * { pointer-events: auto; }
.at-hero { display: flex; min-height: 100svh; flex-direction: column; justify-content: space-between; padding-top: clamp(126px, 15vh, 170px); }
.at-hero-kicker { display: flex; width: 100%; justify-content: space-between; gap: 20px; color: var(--at-muted); font: 650 9px/1.2 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.16em; }
.at-hero-title { width: 100%; margin: auto 0; color: var(--at-ink); font-size: clamp(64px, 10.4vw, 174px); font-weight: 640; letter-spacing: -0.075em; line-height: 0.78; text-transform: uppercase; perspective: 1000px; }
.at-hero-line { display: block; white-space: nowrap; }
.at-hero-line-two { color: transparent; text-align: right; -webkit-text-stroke: 1px color-mix(in srgb, var(--at-ink) 78%, transparent); }
.at-hero-bottom { display: flex; width: 100%; align-items: flex-end; justify-content: space-between; gap: 32px; }
.at-hero-intro { max-width: 480px; }
.at-hero-intro > p, .at-story-panel > p, .at-contact-copy > p { margin: 0; color: var(--at-muted); font-size: clamp(13px, 1.2vw, 16px); line-height: 1.75; }
.at-inline-facts { display: flex; flex-wrap: wrap; gap: 16px; margin-top: 20px; color: var(--at-subtle); font: 620 9px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.11em; }
.at-inline-facts span { display: inline-flex; align-items: center; gap: 7px; }
.at-inline-facts i { width: 4px; height: 4px; border-radius: 50%; background: var(--color-success); }
.at-hero-cta, .at-contact-actions { display: flex; align-items: center; gap: 18px; }

.at-orbit-link { position: relative; display: inline-flex; width: 126px; height: 126px; flex: 0 0 auto; align-items: center; justify-content: center; gap: 9px; border: 1px solid var(--at-line-strong); border-radius: 50%; color: var(--at-ink); background: color-mix(in srgb, var(--at-panel) 84%, transparent); font-size: 11px; font-weight: 650; text-align: center; backdrop-filter: blur(16px); -webkit-backdrop-filter: blur(16px); transition: color var(--motion-base) var(--ease-standard), background-color var(--motion-base) var(--ease-standard), border-color var(--motion-base) var(--ease-standard), transform var(--motion-base) var(--ease-enter); }
.at-orbit-link::before { position: absolute; inset: 8px; border: 1px solid color-mix(in srgb, var(--at-primary) 34%, transparent); border-radius: inherit; content: ''; transition: transform var(--motion-slow) var(--ease-enter); }
.at-orbit-link:hover { border-color: var(--at-primary); color: #fff; background: var(--color-primary); transform: rotate(-4deg); }
.at-orbit-link:hover::before { transform: rotate(22deg); }
.at-orbit-link-small { width: 108px; height: 108px; margin-top: 28px; }
.at-text-link, .at-underlined-link { display: inline-flex; align-items: center; gap: 9px; color: var(--at-ink); font-size: 11px; font-weight: 650; }
.at-text-link { padding: 10px 0; }
.at-text-link:hover, .at-underlined-link:hover { color: var(--at-primary); }
.at-underlined-link { margin-top: 24px; padding-bottom: 6px; border-bottom: 1px solid var(--at-line-strong); }

.at-chapter-left { justify-items: start; }
.at-chapter-right { justify-items: end; }
.at-story-panel { width: min(100%, 590px); padding: clamp(25px, 3vw, 42px); border: 1px solid var(--at-line); border-radius: 2px; background: var(--at-panel); box-shadow: 0 22px 70px rgba(0, 0, 0, 0.12); backdrop-filter: blur(18px) saturate(112%); -webkit-backdrop-filter: blur(18px) saturate(112%); }
.at-story-panel-wide { width: min(100%, 660px); }
.at-chapter-label { display: flex; align-items: center; gap: 12px; margin-bottom: clamp(26px, 4vh, 44px); color: var(--at-subtle); font: 700 9px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.16em; }
.at-chapter-label i { display: block; width: 46px; height: 1px; background: var(--at-line-strong); }
.at-story-panel h2, .at-contact-copy h2 { margin: 0; color: var(--at-ink); font-size: clamp(39px, 5.5vw, 78px); font-weight: 590; letter-spacing: -0.06em; line-height: 0.96; }
.at-story-panel h2 span, .at-contact-copy h2 span { display: block; }
.at-story-panel > p { max-width: 520px; margin-top: 24px; }

.at-endpoints { margin-top: 30px; border-top: 1px solid var(--at-line); }
.at-endpoint-row { display: grid; grid-template-columns: 52px minmax(0, 1fr) auto; align-items: center; gap: 14px; min-height: 48px; border-bottom: 1px solid var(--at-line); }
.at-endpoint-row > span { color: var(--at-accent); font: 700 8px/1 ui-monospace, SFMono-Regular, Menlo, monospace; }
.at-endpoint-row code { overflow: hidden; color: var(--at-ink); font: 560 11px/1.4 ui-monospace, SFMono-Regular, Menlo, monospace; text-overflow: ellipsis; white-space: nowrap; }
.at-endpoint-row small { color: var(--at-subtle); font-size: 10px; }
.at-command { display: block; overflow: hidden; margin-top: 18px; padding: 13px 15px; border: 1px solid var(--at-line); color: var(--at-muted); background: color-mix(in srgb, var(--at-panel-strong) 84%, transparent); font: 540 10px/1.4 ui-monospace, SFMono-Regular, Menlo, monospace; text-overflow: ellipsis; white-space: nowrap; }

.at-system-list { margin: 30px 0 0; padding: 0; border-top: 1px solid var(--at-line); list-style: none; }
.at-system-list li { display: grid; grid-template-columns: 34px 1fr; gap: 16px; padding: 17px 0; border-bottom: 1px solid var(--at-line); }
.at-system-list li > span { color: var(--at-primary); font: 700 9px/1.4 ui-monospace, SFMono-Regular, Menlo, monospace; }
.at-system-list div { display: flex; flex-direction: column; gap: 6px; }
.at-system-list strong { color: var(--at-ink); font: 680 11px/1.3 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.08em; }
.at-system-list small { color: var(--at-muted); font-size: 11px; line-height: 1.6; }

.at-signal-board { display: grid; grid-template-columns: repeat(3, 1fr); margin-top: 30px; border: 1px solid var(--at-line); }
.at-signal-board > div { display: flex; min-width: 0; flex-direction: column; gap: 10px; padding: 18px; border-right: 1px solid var(--at-line); }
.at-signal-board > div:last-child { border-right: 0; }
.at-signal-board span { color: var(--at-subtle); font: 700 8px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.12em; }
.at-signal-board strong { color: var(--at-ink); font: 560 clamp(17px, 2vw, 24px)/1 ui-monospace, SFMono-Regular, Menlo, monospace; }
.at-signal-board small { display: inline-flex; align-items: center; gap: 7px; color: var(--at-muted); font-size: 9px; }
.at-signal-board small i { width: 5px; height: 5px; border-radius: 50%; background: var(--color-success); }
.at-topic-cloud { display: flex; flex-wrap: wrap; gap: 7px; margin-top: 28px; }
.at-topic-cloud span { padding: 9px 12px; border: 1px solid var(--at-line); color: var(--at-muted); font: 700 8px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.1em; }

.at-contact { display: flex; min-height: 100svh; flex-direction: column; align-items: center; justify-content: center; text-align: center; }
.at-contact-copy { display: flex; max-width: 1000px; flex-direction: column; align-items: center; }
.at-contact-copy h2 { font-size: clamp(50px, 8.4vw, 134px); line-height: 0.86; text-transform: uppercase; }
.at-contact-copy > p { max-width: 540px; margin-top: 30px; }
.at-contact-actions { margin-top: 34px; }
.at-contact .at-chapter-label { margin-bottom: 34px; }
.at-footer { position: absolute; right: clamp(20px, 4vw, 60px); bottom: 28px; left: clamp(20px, 4vw, 60px); display: flex; justify-content: space-between; gap: 20px; padding-top: 18px; border-top: 1px solid var(--at-line); color: var(--at-subtle); font: 600 8px/1.4 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.11em; text-align: left; }

.at-pill-nav { position: fixed; bottom: max(20px, env(safe-area-inset-bottom)); left: 50%; z-index: 60; display: flex; min-height: 52px; align-items: center; gap: 10px; padding: 6px 8px; border: 1px solid var(--at-line); border-radius: 999px; color: var(--at-ink); background: var(--at-panel-strong); box-shadow: 0 18px 44px rgba(0, 0, 0, 0.16); backdrop-filter: blur(20px) saturate(124%); -webkit-backdrop-filter: blur(20px) saturate(124%); transform: translateX(-50%) skewX(var(--pill-skew)) scaleX(var(--pill-stretch)); transition: transform 140ms var(--ease-exit), border-color var(--motion-fast) var(--ease-standard); transform-origin: center; }
.at-pill-current, .at-pill-word { min-width: 32px; color: var(--at-subtle); font: 700 8px/1 ui-monospace, SFMono-Regular, Menlo, monospace; text-align: center; letter-spacing: 0.08em; }
.at-pill-word { min-width: 48px; }
.at-pill-items { display: flex; align-items: center; gap: 1px; }
.at-pill-items button { position: relative; display: grid; min-width: 34px; min-height: 38px; place-items: center; border-radius: 999px; color: var(--at-subtle); transition: color var(--motion-fast) var(--ease-standard), background-color var(--motion-fast) var(--ease-standard), transform var(--motion-fast) var(--ease-standard); }
.at-pill-items button > i { width: 5px; height: 5px; border-radius: 50%; background: currentColor; }
.at-pill-items button > span, .at-pill-items button > small { display: none; }
.at-pill-items button:hover, .at-pill-items button.is-active { color: var(--at-ink); background: color-mix(in srgb, var(--at-ink) 10%, transparent); }
.at-pill-items button.is-active { min-width: 42px; transform: scaleX(calc(1 / var(--pill-stretch))); }
.at-pill-items button.is-active > i { width: 17px; border-radius: 4px; background: var(--at-primary); }

.at-loader { position: fixed; inset: 0; z-index: 100; display: flex; flex-direction: column; justify-content: space-between; padding: clamp(26px, 4vw, 54px); color: #f4f6fb; background: #080a10; pointer-events: none; }
.at-loader-brand { display: inline-flex; align-items: center; gap: 14px; font-size: 13px; letter-spacing: 0.12em; }
.at-loader-brand span { display: grid; width: 42px; height: 42px; place-items: center; border: 1px solid rgba(224, 230, 249, 0.34); border-radius: 50%; font-weight: 800; }
.at-loader-meta { display: flex; justify-content: space-between; color: #8d96aa; font: 650 9px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.14em; }
.at-loader-track { position: absolute; right: clamp(26px, 4vw, 54px); bottom: clamp(26px, 4vw, 54px); left: clamp(26px, 4vw, 54px); height: 1px; overflow: hidden; background: rgba(224, 230, 249, 0.2); }
.at-loader-track span { display: block; width: 100%; height: 100%; background: #818cf8; transform-origin: left; transition: transform 90ms linear; }
.at-loader-enter-active, .at-loader-leave-active { transition: opacity 360ms var(--ease-standard), visibility 360ms var(--ease-standard); }
.at-loader-enter-from, .at-loader-leave-to { opacity: 0; visibility: hidden; }

.at-menu-overlay { position: fixed; inset: 0; z-index: 64; overflow: auto; padding: 118px clamp(24px, 7vw, 112px) 70px; color: var(--at-ink); background: var(--at-menu); }
.at-menu-meta { display: flex; justify-content: space-between; gap: 20px; padding-bottom: 20px; border-bottom: 1px solid var(--at-line); color: var(--at-subtle); font: 650 9px/1.4 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.13em; }
.at-menu-grid { display: grid; grid-template-columns: minmax(0, 1.4fr) minmax(240px, 0.6fr); gap: clamp(38px, 8vw, 130px); padding-top: 24px; }
.at-menu-links { display: flex; flex-direction: column; }
.at-menu-links button { display: grid; grid-template-columns: 48px 1fr auto; align-items: center; gap: 18px; padding: 15px 0; border-bottom: 1px solid var(--at-line); color: var(--at-muted); text-align: left; transition: color var(--motion-fast) var(--ease-standard), padding-left var(--motion-base) var(--ease-enter); }
.at-menu-links button > span { color: var(--at-subtle); font: 700 9px/1 ui-monospace, SFMono-Regular, Menlo, monospace; }
.at-menu-links button strong { font-size: clamp(34px, 5.5vw, 76px); font-weight: 560; letter-spacing: -0.055em; line-height: 1; }
.at-menu-links button:hover, .at-menu-links button.is-current { padding-left: 10px; color: var(--at-ink); }
.at-menu-links button.is-current > span { color: var(--at-primary); }
.at-menu-aside { display: flex; flex-direction: column; justify-content: space-between; gap: 40px; padding: 20px 0; }
.at-menu-aside > div { display: flex; flex-direction: column; align-items: flex-start; gap: 14px; }
.at-menu-aside > div > span { margin-bottom: 4px; color: var(--at-subtle); font: 700 9px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.12em; }
.at-menu-aside a { padding-bottom: 4px; border-bottom: 1px solid var(--at-line); color: var(--at-ink); font-size: 14px; }
.at-menu-aside p { max-width: 300px; margin: 0; color: var(--at-muted); font-size: 12px; line-height: 1.7; }
.at-menu-enter-active, .at-menu-leave-active { transition: opacity 320ms var(--ease-standard), transform 420ms var(--ease-enter); }
.at-menu-enter-from, .at-menu-leave-to { opacity: 0; transform: translateY(-18px); }

@supports not ((backdrop-filter: blur(1px))) {
  .at-home-scrolled .at-header, .at-icon-control, .at-menu-control, .at-story-panel, .at-pill-nav, .at-orbit-link { background: var(--at-panel-strong); }
}

@media (max-width: 900px) {
  .at-header { grid-template-columns: 1fr auto; min-height: 74px; }
  .at-header-coordinate, .at-side-status { display: none; }
  .at-locale { display: none; }
  .at-chapter { padding-right: 28px; padding-left: 28px; }
  .at-hero-title { font-size: clamp(60px, 12.2vw, 108px); line-height: 0.84; }
  .at-story-panel { width: min(100%, 620px); }
  .at-menu-grid { grid-template-columns: 1fr; }
  .at-menu-aside { display: grid; grid-template-columns: 1fr 1fr; }
}

@media (max-width: 640px) {
  .at-header { min-height: 68px; padding: 0 16px; }
  .at-brand-copy small, .at-header-actions > .at-icon-control:first-of-type { display: none; }
  .at-brand-copy strong { max-width: 92px; font-size: 11px; }
  .at-brand-mark { width: 34px; height: 34px; }
  .at-icon-control { min-width: 34px; min-height: 34px; }
  .at-menu-control { min-width: 70px; min-height: 34px; padding: 0 11px; }
  .at-chapter { min-height: 100svh; padding: 104px 18px 116px; }
  .at-hero { padding-top: 104px; }
  .at-hero-kicker { flex-direction: column; gap: 6px; }
  .at-hero-title { margin: 42px 0 auto; font-size: clamp(50px, 15.5vw, 74px); line-height: 0.86; }
  .at-hero-line { white-space: normal; }
  .at-hero-line-two { margin-top: 8px; text-align: left; }
  .at-hero-bottom { flex-direction: column; align-items: flex-start; gap: 22px; }
  .at-hero-intro > p { max-width: 340px; font-size: 13px; }
  .at-hero-cta { width: 100%; justify-content: space-between; }
  .at-orbit-link { width: 98px; height: 98px; font-size: 10px; }
  .at-story-panel { padding: 24px 20px; background: var(--at-panel-strong); }
  .at-story-panel h2 { font-size: clamp(38px, 12vw, 54px); }
  .at-endpoint-row { grid-template-columns: 44px minmax(0, 1fr); gap: 8px; }
  .at-endpoint-row small { display: none; }
  .at-signal-board { grid-template-columns: 1fr; }
  .at-signal-board > div { border-right: 0; border-bottom: 1px solid var(--at-line); }
  .at-signal-board > div:last-child { border-bottom: 0; }
  .at-contact { padding-bottom: 150px; }
  .at-contact-copy h2 { font-size: clamp(48px, 14vw, 72px); line-height: 0.9; }
  .at-contact-actions { flex-direction: column; }
  .at-footer { flex-direction: column; gap: 6px; }
  .at-footer span:nth-child(2) { display: none; }
  .at-pill-nav { bottom: max(12px, env(safe-area-inset-bottom)); min-height: 48px; }
  .at-pill-current, .at-pill-word { display: none; }
  .at-pill-items button { min-width: 37px; }
  .at-menu-overlay { padding: 94px 20px 38px; }
  .at-menu-meta span:last-child { display: none; }
  .at-menu-links button { grid-template-columns: 32px 1fr auto; }
  .at-menu-links button strong { font-size: clamp(32px, 10vw, 48px); }
  .at-menu-aside { grid-template-columns: 1fr; }
  .compact-home-nav { width: min(100% - 20px, 1180px); }
  .compact-home-actions :deep(.locale-switcher) { display: none; }
}

@media (prefers-reduced-motion: reduce) {
  .at-header, .at-icon-control, .at-menu-control, .at-orbit-link, .at-orbit-link::before, .at-text-link, .at-underlined-link, .at-pill-nav, .at-pill-items button, .at-menu-links button, .at-loader, .at-loader-track span, .at-menu-overlay { transition-duration: 1ms !important; }
  .at-pill-nav { transform: translateX(-50%); }
  .at-reveal { opacity: 1 !important; transform: none !important; visibility: visible !important; }
  .at-loader { display: none; }
}
</style>
