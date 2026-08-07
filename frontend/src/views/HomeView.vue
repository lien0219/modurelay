<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="hasHomeContent" class="min-h-screen">
    <!-- iframe mode -->
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Compact Home Page -->
  <div
    v-else-if="compactHomeEnabled"
    data-testid="compact-home"
    class="flex min-h-screen flex-col bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white"
  >
    <header class="border-b border-gray-200 px-4 py-4 sm:px-6 dark:border-dark-800">
      <nav class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 sm:gap-4">
        <div class="flex min-w-0 flex-1 items-center gap-3">
          <img
            :src="siteLogo || brand.logo"
            :alt="siteName"
            class="h-9 w-9 shrink-0 rounded-lg object-contain"
          />
          <span class="min-w-0 truncate text-base font-semibold">{{ siteName }}</span>
        </div>
        <div class="flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>
          <button
            type="button"
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="inline-flex min-h-10 shrink-0 items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-200"
          >
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="flex min-w-0 flex-1 items-center justify-center px-4 py-16 sm:px-6">
      <div class="min-w-0 max-w-2xl text-center">
        <img
          :src="siteLogo || brand.logo"
          :alt="siteName"
          class="mx-auto mb-6 h-20 w-20 rounded-2xl object-contain"
        />
        <h1 class="[overflow-wrap:anywhere] text-3xl font-bold md:text-4xl">{{ siteName }}</h1>
        <p class="mt-4 whitespace-pre-wrap [overflow-wrap:anywhere] text-base text-gray-600 dark:text-dark-300">{{ siteSubtitle }}</p>
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="mt-8 inline-flex min-h-10 items-center justify-center rounded-lg bg-primary-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-primary-700"
        >
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.login') }}
        </router-link>
      </div>
    </main>

    <footer class="min-w-0 border-t border-gray-200 px-4 py-5 text-center text-sm text-gray-500 [overflow-wrap:anywhere] sm:px-6 dark:border-dark-800 dark:text-dark-400">
      &copy; {{ currentYear }} {{ siteName }}
    </footer>
  </div>

  <!-- ModuRelay Official Home Page -->
  <div
    v-else
    ref="pageRef"
    class="home-page"
    :class="{ 'home-page-scrolled': isScrolled }"
  >
    <div class="home-background" aria-hidden="true">
      <span class="home-glow home-glow-one"></span>
      <span class="home-glow home-glow-two"></span>
      <span class="home-glow home-glow-three"></span>
      <span class="home-grid"></span>
      <span class="home-noise"></span>
    </div>

    <!-- Sticky navigation linked with page sections -->
    <header class="home-header">
      <div class="home-scroll-progress" :style="{ width: `${scrollProgress}%` }"></div>
      <nav class="home-navbar" aria-label="Home navigation">
        <button class="home-brand" type="button" @click="scrollToSection('home')">
          <span class="home-brand-logo">
            <img :src="siteLogo || brand.logo" :alt="siteName" />
          </span>
          <span class="home-brand-name">{{ siteName }}</span>
        </button>

        <div class="home-nav-links" role="list">
          <button
            v-for="item in navigationItems"
            :key="item.id"
            type="button"
            class="home-nav-link"
            :class="{ 'is-active': activeSection === item.id }"
            :aria-current="activeSection === item.id ? 'page' : undefined"
            @click="scrollToSection(item.id)"
          >
            <span>{{ item.label }}</span>
          </button>
        </div>

        <div class="home-nav-actions">
          <LocaleSwitcher />

          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="home-icon-button home-doc-link"
            :title="copy.nav.docs"
          >
            <Icon name="book" size="sm" />
          </a>

          <button
            type="button"
            class="home-icon-button"
            :title="isDark ? copy.nav.light : copy.nav.dark"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="sm" />
            <Icon v-else name="moon" size="sm" />
          </button>

          <router-link
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="home-login-button"
          >
            <span class="home-user-avatar">{{ userInitial }}</span>
            <span>{{ copy.nav.dashboard }}</span>
          </router-link>
          <router-link v-else to="/login" class="home-login-button">
            <span>{{ copy.nav.login }}</span>
            <Icon name="arrowRight" size="xs" :stroke-width="2" />
          </router-link>
        </div>
      </nav>
    </header>

    <main>
      <!-- Hero -->
      <section id="home" data-home-section class="home-section hero-section">
        <div class="home-container hero-layout">
          <div class="hero-copy">
            <div class="hero-eyebrow hero-enter hero-enter-one">
              <span class="hero-eyebrow-dot"></span>
              {{ copy.hero.eyebrow }}
            </div>

            <h1 class="hero-title hero-enter hero-enter-two">
              <span class="hero-title-gradient">ModuRelay</span>
              <span>{{ copy.hero.titleSuffix }}</span>
            </h1>

            <p class="hero-subtitle hero-enter hero-enter-three">
              {{ copy.hero.subtitle }}
            </p>
            <p class="hero-description hero-enter hero-enter-four">
              {{ copy.hero.description }}
            </p>

            <div class="hero-actions hero-enter hero-enter-five">
              <router-link
                :to="isAuthenticated ? dashboardPath : '/login'"
                class="home-primary-button"
              >
                {{ isAuthenticated ? copy.hero.dashboardCta : copy.hero.primaryCta }}
                <span class="button-arrow">
                  <Icon name="arrowRight" size="sm" :stroke-width="2" />
                </span>
              </router-link>

              <button class="home-secondary-button" type="button" @click="scrollToSection('contact')">
                {{ copy.hero.secondaryCta }}
                <Icon name="chat" size="sm" />
              </button>

              <router-link to="/key-usage" class="home-text-button">
                {{ copy.hero.quotaCta }}
                <Icon name="arrowRight" size="xs" />
              </router-link>
            </div>

            <div class="hero-trust hero-enter hero-enter-six">
              <span v-for="item in copy.hero.trust" :key="item">
                <i></i>{{ item }}
              </span>
            </div>
          </div>

          <div class="hero-scene-wrap hero-enter hero-enter-scene">
            <HomeHeroScene :services="heroSceneServices" />
          </div>
        </div>

        <div class="home-container quick-service-wrap hero-enter hero-enter-seven">
          <div class="quick-service-bar">
            <button
              v-for="service in services"
              :key="service.key"
              type="button"
              class="quick-service-item"
              :data-tone="service.tone"
              @click="scrollToSection('services')"
            >
              <span><Icon :name="service.icon" size="sm" /></span>
              {{ service.title }}
            </button>
          </div>
        </div>
      </section>

      <!-- Core services -->
      <section id="services" data-home-section class="home-section services-section">
        <div class="home-container">
          <div class="section-heading reveal-on-scroll">
            <span class="section-kicker">01 / SERVICES</span>
            <h2>{{ copy.services.title }}</h2>
            <p>{{ copy.services.subtitle }}</p>
          </div>

          <div class="service-grid">
            <article
              v-for="(service, index) in services"
              :key="service.key"
              class="service-card reveal-on-scroll"
              :data-tone="service.tone"
              :style="{ '--reveal-delay': `${index * 70}ms` }"
              tabindex="0"
            >
              <div class="service-card-topline"></div>
              <div class="service-icon-wrap">
                <span class="service-icon-orbit"></span>
                <Icon :name="service.icon" size="lg" :stroke-width="1.7" />
              </div>
              <span class="service-number">0{{ index + 1 }}</span>
              <h3>{{ service.title }}</h3>
              <p>{{ service.description }}</p>
              <button type="button" class="service-link" @click="scrollToSection('contact')">
                {{ copy.services.learnMore }}
                <Icon name="arrowRight" size="xs" />
              </button>
            </article>
          </div>
        </div>
      </section>

      <!-- Advantages / solutions -->
      <section id="solutions" data-home-section class="home-section solutions-section">
        <div class="home-container solutions-layout">
          <div class="solutions-copy reveal-on-scroll">
            <span class="section-kicker">02 / ADVANTAGES</span>
            <h2>{{ copy.solutions.title }}</h2>
            <p>{{ copy.solutions.description }}</p>

            <div class="solution-stats">
              <div v-for="metric in copy.solutions.metrics" :key="metric.label" class="solution-stat">
                <strong>{{ metric.value }}</strong>
                <span>{{ metric.label }}</span>
              </div>
            </div>
          </div>

          <div class="advantage-grid">
            <article
              v-for="(advantage, index) in advantages"
              :key="advantage.title"
              class="advantage-card reveal-on-scroll"
              :style="{ '--reveal-delay': `${index * 80}ms` }"
            >
              <span class="advantage-icon"><Icon :name="advantage.icon" size="md" /></span>
              <div>
                <h3>{{ advantage.title }}</h3>
                <p>{{ advantage.description }}</p>
              </div>
            </article>
          </div>
        </div>
      </section>

      <!-- Interactive capability rail -->
      <section class="capability-rail-section" aria-label="ModuRelay capabilities">
        <div class="capability-rail">
          <div class="capability-track">
            <template v-for="round in 2" :key="round">
              <span v-for="item in copy.capabilities" :key="`${round}-${item}`">
                <i></i>{{ item }}
              </span>
            </template>
          </div>
        </div>
      </section>

      <!-- Process -->
      <section id="process" data-home-section class="home-section process-section">
        <div class="home-container">
          <div class="section-heading reveal-on-scroll">
            <span class="section-kicker">03 / WORKFLOW</span>
            <h2>{{ copy.process.title }}</h2>
            <p>{{ copy.process.subtitle }}</p>
          </div>

          <div class="process-grid">
            <article
              v-for="(step, index) in processSteps"
              :key="step.title"
              class="process-card reveal-on-scroll"
              :style="{ '--reveal-delay': `${index * 100}ms` }"
            >
              <div class="process-line" aria-hidden="true"></div>
              <div class="process-step-head">
                <span class="process-step-number">{{ String(index + 1).padStart(2, '0') }}</span>
                <span class="process-step-icon"><Icon :name="step.icon" size="md" /></span>
              </div>
              <h3>{{ step.title }}</h3>
              <p>{{ step.description }}</p>
            </article>
          </div>
        </div>
      </section>

      <!-- Advertising -->
      <section id="advertising" data-home-section class="home-section advertising-section">
        <div class="home-container">
          <article class="advertising-panel reveal-on-scroll">
            <div class="ad-grid" aria-hidden="true"></div>
            <div class="ad-copy">
              <span class="ad-badge">{{ copy.ad.badge }}</span>
              <h2>{{ copy.ad.title }}</h2>
              <p>{{ copy.ad.description }}</p>
              <button type="button" class="home-primary-button" @click="scrollToSection('contact')">
                {{ copy.ad.cta }}
                <span class="button-arrow"><Icon name="arrowRight" size="sm" /></span>
              </button>
            </div>

            <div class="ad-visual" aria-hidden="true">
              <div class="ad-hologram">
                <span>AD</span>
                <i class="ad-ring ad-ring-one"></i>
                <i class="ad-ring ad-ring-two"></i>
                <i class="ad-beam"></i>
              </div>
            </div>

            <div class="ad-metrics">
              <div v-for="metric in copy.ad.metrics" :key="metric.label">
                <strong>{{ metric.value }}</strong>
                <span>{{ metric.label }}</span>
              </div>
            </div>
          </article>
        </div>
      </section>

      <!-- Contact CTA -->
      <section id="contact" data-home-section class="home-section contact-section">
        <div class="home-container">
          <div class="contact-panel reveal-on-scroll">
            <div>
              <span class="section-kicker">04 / CONTACT</span>
              <h2>{{ copy.contact.title }}</h2>
              <p>{{ copy.contact.description }}</p>
            </div>
            <div class="contact-actions">
              <router-link
                :to="isAuthenticated ? dashboardPath : '/login'"
                class="home-primary-button"
              >
                {{ isAuthenticated ? copy.hero.dashboardCta : copy.contact.primaryCta }}
                <span class="button-arrow"><Icon name="arrowRight" size="sm" /></span>
              </router-link>
              <a
                v-if="docUrl"
                :href="docUrl"
                target="_blank"
                rel="noopener noreferrer"
                class="home-secondary-button"
              >
                {{ copy.contact.docsCta }}
                <Icon name="book" size="sm" />
              </a>
            </div>
          </div>
        </div>
      </section>
    </main>

    <footer class="home-footer">
      <div class="home-container footer-grid">
        <div class="footer-brand-block">
          <div class="footer-brand-line">
            <span class="home-brand-logo"><img :src="siteLogo || brand.logo" :alt="siteName" /></span>
            <strong>{{ siteName }}</strong>
          </div>
          <p>{{ copy.footer.description }}</p>
        </div>

        <div class="footer-link-group">
          <strong>{{ copy.footer.services }}</strong>
          <button v-for="service in services" :key="service.key" type="button" @click="scrollToSection('services')">
            {{ service.title }}
          </button>
        </div>

        <div class="footer-link-group">
          <strong>{{ copy.footer.resources }}</strong>
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">{{ copy.nav.docs }}</a>
          <router-link to="/key-usage">{{ copy.hero.quotaCta }}</router-link>
          <button type="button" @click="scrollToSection('process')">{{ copy.nav.process }}</button>
        </div>

        <div class="footer-link-group">
          <strong>{{ copy.footer.cooperation }}</strong>
          <button type="button" @click="scrollToSection('advertising')">{{ copy.nav.advertising }}</button>
          <button type="button" @click="scrollToSection('contact')">{{ copy.nav.contact }}</button>
          <router-link to="/login">{{ copy.nav.login }}</router-link>
        </div>
      </div>

      <div class="home-container footer-bottom">
        <span>&copy; {{ currentYear }} {{ siteName }}. {{ copy.footer.rights }}</span>
        <span>{{ copy.footer.tagline }}</span>
      </div>
    </footer>

    <button
      v-show="isScrolled"
      type="button"
      class="back-to-top"
      :title="copy.nav.backToTop"
      @click="scrollToSection('home')"
    >
      <Icon name="arrowUp" size="sm" />
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { brand } from '@/config/brand'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import HomeHeroScene from '@/components/home/HomeHeroScene.vue'
import { sanitizeUrl } from '@/utils/url'

type SectionId = 'home' | 'services' | 'solutions' | 'process' | 'advertising' | 'contact'
type HomeIcon = 'link' | 'server' | 'shield' | 'chat' | 'chart' | 'sync' | 'play' | 'check'
type ServiceTone = 'teal' | 'blue' | 'violet' | 'cyan' | 'amber'

interface ServiceItem {
  key: string
  title: string
  description: string
  caption: string
  icon: HomeIcon
  tone: ServiceTone
}

const zhCopy = {
  nav: {
    home: '首页', services: '产品服务', solutions: '解决方案', process: '服务流程',
    advertising: '广告合作', contact: '联系我们', docs: '查看文档', light: '切换浅色模式',
    dark: '切换深色模式', dashboard: '控制台', login: '登录', backToTop: '返回顶部'
  },
  hero: {
    eyebrow: '一站式数字服务与 AI 接入平台',
    titleSuffix: '官方平台',
    subtitle: '一站式 AI API 转换与数字服务解决方案',
    description: '为开发者、企业与个人提供稳定、安全、高效的下游 API 对接、账号资源、网络代理、接码与商务推广服务。',
    primaryCta: '立即接入', dashboardCta: '进入控制台', secondaryCta: '咨询合作', quotaCta: '额度查询',
    trust: ['稳定可靠', '极速响应', '安全保障']
  },
  services: {
    title: '核心服务', subtitle: '从 API 接入到数字资源配套，为不同业务阶段提供可组合的服务能力。', learnMore: '咨询详情',
    items: [
      { title: '下游对接', description: '提供标准化 API 接口与完整对接支持，帮助平台和开发者快速完成模型能力集成。', caption: 'API 对接与集成' },
      { title: '账号售卖', description: '多平台优质账号资源，按实际业务场景提供灵活选择与交付支持。', caption: '多平台账号资源' },
      { title: 'VPN 代理服务', description: '覆盖多地区的稳定代理节点，满足跨区域访问、网络加速与业务连接需求。', caption: '全球节点稳定高速' },
      { title: '接码服务', description: '覆盖多个国家和地区的验证码接收能力，流程清晰，响应高效。', caption: '多地区接码能力' },
      { title: '广告位出租', description: '开放官网优质流量资源位，支持品牌展示、产品曝光与商务合作。', caption: '精准曝光高效转化' }
    ]
  },
  solutions: {
    title: '为什么选择 ModuRelay',
    description: '围绕稳定性、资源覆盖、交付效率与成本控制持续优化，让服务真正支撑业务长期增长。',
    metrics: [
      { value: '7×24', label: '持续服务' }, { value: '99.99%', label: '可用性目标' }, { value: 'Multi', label: '多资源组合' }
    ],
    items: [
      { title: '稳定可靠', description: '多重容错、健康检查与自动切换机制，保障核心链路持续可用。' },
      { title: '多线路资源', description: '整合多渠道与多线路资源，按业务需求灵活匹配。' },
      { title: '高性价比', description: '透明定价与灵活配置，在稳定体验和成本之间取得平衡。' },
      { title: '技术支持', description: '从咨询、测试到正式使用，提供完整的技术协助与问题跟进。' }
    ]
  },
  capabilities: ['统一 API 网关', '多模型路由', '账号资源服务', '全球网络代理', '验证码接收', '商务广告合作', '实时用量计费', '智能健康检查'],
  process: {
    title: '清晰高效的服务流程', subtitle: '从需求确认到稳定运行，每一步都有明确交付与技术支持。',
    items: [
      { title: '咨询需求', description: '沟通使用场景、目标规模与资源要求，确认适合的解决方案。' },
      { title: '开通服务', description: '根据方案完成账号、接口或资源配置，并提供必要的接入信息。' },
      { title: '对接测试', description: '协助完成联调与验证，确保接口、网络和业务流程稳定兼容。' },
      { title: '稳定使用', description: '正式投入使用，持续关注运行状态并提供后续服务支持。' }
    ]
  },
  ad: {
    badge: '合作共赢 · 流量变现', title: '广告位出租',
    description: '开放 ModuRelay 官网优质展示资源，为品牌、工具与数字服务提供精准曝光机会。支持首页焦点位、服务区推荐位与专题合作。',
    cta: '洽谈广告合作',
    metrics: [
      { value: '多场景', label: '展示资源位' }, { value: '精准', label: '目标用户触达' }, { value: '灵活', label: '合作周期' }, { value: '可追踪', label: '投放效果' }
    ]
  },
  contact: {
    title: '准备好连接更多业务能力了吗？',
    description: '告诉我们你的目标和使用场景，我们将协助匹配合适的产品服务与接入方案。',
    primaryCta: '立即开始', docsCta: '查看接入文档'
  },
  footer: {
    description: '一站式 AI API 转换与数字服务平台，让连接更简单，让服务更智能。',
    services: '产品服务', resources: '资源中心', cooperation: '商务合作', rights: '保留所有权利。', tagline: '稳定连接 · 灵活服务 · 持续成长'
  }
}

const enCopy = {
  nav: {
    home: 'Home', services: 'Services', solutions: 'Solutions', process: 'Workflow',
    advertising: 'Advertising', contact: 'Contact', docs: 'Documentation', light: 'Switch to light mode',
    dark: 'Switch to dark mode', dashboard: 'Dashboard', login: 'Sign in', backToTop: 'Back to top'
  },
  hero: {
    eyebrow: 'One-stop digital services and AI access platform',
    titleSuffix: 'Official Platform',
    subtitle: 'AI API conversion and digital service solutions in one place',
    description: 'Stable, secure and efficient downstream API integration, account resources, network proxy, verification and promotion services for developers, businesses and individuals.',
    primaryCta: 'Get started', dashboardCta: 'Open dashboard', secondaryCta: 'Talk to us', quotaCta: 'Check quota',
    trust: ['Reliable', 'Fast response', 'Secure']
  },
  services: {
    title: 'Core services', subtitle: 'Composable capabilities from API integration to digital resources for every stage of your business.', learnMore: 'Learn more',
    items: [
      { title: 'Downstream integration', description: 'Standardized APIs and complete integration support for platforms and developers.', caption: 'API integration' },
      { title: 'Account marketplace', description: 'Quality multi-platform account resources with flexible delivery options.', caption: 'Account resources' },
      { title: 'VPN proxy service', description: 'Stable multi-region proxy nodes for cross-region access and business connectivity.', caption: 'Global stable nodes' },
      { title: 'Verification service', description: 'Efficient verification-code receiving capabilities across multiple regions.', caption: 'Multi-region verification' },
      { title: 'Advertising slots', description: 'Premium website placements for brand exposure, product promotion and partnerships.', caption: 'Targeted exposure' }
    ]
  },
  solutions: {
    title: 'Why choose ModuRelay',
    description: 'We continuously improve reliability, coverage, delivery and cost efficiency to support long-term business growth.',
    metrics: [
      { value: '24/7', label: 'Service' }, { value: '99.99%', label: 'Availability goal' }, { value: 'Multi', label: 'Resource mix' }
    ],
    items: [
      { title: 'Reliable', description: 'Health checks, failover and resilient architecture keep critical routes available.' },
      { title: 'Multi-route resources', description: 'Flexible resources across providers and network routes.' },
      { title: 'Cost efficient', description: 'Transparent pricing and flexible options balance experience and cost.' },
      { title: 'Technical support', description: 'End-to-end assistance from consultation and testing to production use.' }
    ]
  },
  capabilities: ['Unified API gateway', 'Multi-model routing', 'Account resources', 'Global network proxy', 'Verification service', 'Advertising partnership', 'Real-time billing', 'Health monitoring'],
  process: {
    title: 'A clear and efficient workflow', subtitle: 'Clear delivery and technical support from requirements to stable operation.',
    items: [
      { title: 'Discuss needs', description: 'Align on your scenarios, scale and resource requirements.' },
      { title: 'Activate service', description: 'Configure accounts, APIs or resources and provide access details.' },
      { title: 'Integration testing', description: 'Validate interfaces, networks and business flows for compatibility.' },
      { title: 'Run with confidence', description: 'Go live with ongoing monitoring and service support.' }
    ]
  },
  ad: {
    badge: 'Grow together · Monetize traffic', title: 'Advertising placements',
    description: 'Premium ModuRelay placements for brands, tools and digital services, including hero promotions, service recommendations and custom campaigns.',
    cta: 'Discuss advertising',
    metrics: [
      { value: 'Multi', label: 'Placement types' }, { value: 'Targeted', label: 'Audience reach' }, { value: 'Flexible', label: 'Campaign period' }, { value: 'Trackable', label: 'Performance' }
    ]
  },
  contact: {
    title: 'Ready to connect more capabilities?',
    description: 'Tell us your goals and use cases. We will help match the right services and integration plan.',
    primaryCta: 'Get started', docsCta: 'View integration docs'
  },
  footer: {
    description: 'A one-stop AI API conversion and digital service platform. Connect simply and operate intelligently.',
    services: 'Services', resources: 'Resources', cooperation: 'Partnerships', rights: 'All rights reserved.', tagline: 'Reliable connections · Flexible services · Sustainable growth'
  }
}

const { t, locale } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

const pageRef = ref<HTMLElement | null>(null)
const activeSection = ref<SectionId>('home')
const isScrolled = ref(false)
const scrollProgress = ref(0)
const isDark = ref(document.documentElement.classList.contains('dark'))

let sectionObserver: IntersectionObserver | null = null
let revealObserver: IntersectionObserver | null = null
let scrollFrame = 0

const copy = computed(() => locale.value === 'zh' ? zhCopy : enCopy)
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || brand.name)
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || brand.slogan)
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)

// Check if homeContent is a URL (for iframe display)
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const userInitial = computed(() => authStore.user?.email?.charAt(0).toUpperCase() || 'M')
const currentYear = computed(() => new Date().getFullYear())

const navigationItems = computed(() => [
  { id: 'home' as const, label: copy.value.nav.home },
  { id: 'services' as const, label: copy.value.nav.services },
  { id: 'solutions' as const, label: copy.value.nav.solutions },
  { id: 'process' as const, label: copy.value.nav.process },
  { id: 'advertising' as const, label: copy.value.nav.advertising },
  { id: 'contact' as const, label: copy.value.nav.contact }
])

const serviceMeta: Array<{ key: string; icon: HomeIcon; tone: ServiceTone }> = [
  { key: 'integration', icon: 'link', tone: 'teal' },
  { key: 'accounts', icon: 'server', tone: 'blue' },
  { key: 'vpn', icon: 'shield', tone: 'violet' },
  { key: 'verification', icon: 'chat', tone: 'cyan' },
  { key: 'advertising', icon: 'chart', tone: 'amber' }
]

const services = computed<ServiceItem[]>(() => copy.value.services.items.map((item, index) => ({
  ...item,
  ...serviceMeta[index]
})))

const heroSceneServices = computed(() => services.value.map(({ title, caption, icon, tone }) => ({
  label: title,
  caption,
  icon: icon as 'link' | 'server' | 'shield' | 'chat' | 'chart',
  tone
})))

const advantageIcons: HomeIcon[] = ['shield', 'sync', 'chart', 'chat']
const advantages = computed(() => copy.value.solutions.items.map((item, index) => ({
  ...item,
  icon: advantageIcons[index]
})))

const processIcons: HomeIcon[] = ['chat', 'play', 'sync', 'check']
const processSteps = computed(() => copy.value.process.items.map((item, index) => ({
  ...item,
  icon: processIcons[index]
})))

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function scrollToSection(id: SectionId) {
  const target = document.getElementById(id)
  if (!target) return
  const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  target.scrollIntoView({ behavior: reduceMotion ? 'auto' : 'smooth', block: 'start' })
}

function updateScrollState() {
  cancelAnimationFrame(scrollFrame)
  scrollFrame = requestAnimationFrame(() => {
    const scrollTop = window.scrollY || document.documentElement.scrollTop
    const scrollable = Math.max(document.documentElement.scrollHeight - window.innerHeight, 1)
    isScrolled.value = scrollTop > 24
    scrollProgress.value = Math.min(100, Math.max(0, (scrollTop / scrollable) * 100))
    pageRef.value?.style.setProperty('--hero-parallax', `${Math.min(scrollTop * 0.045, 38)}px`)
  })
}

function initializeObservers() {
  const sections = Array.from(document.querySelectorAll<HTMLElement>('[data-home-section]'))
  sectionObserver = new IntersectionObserver((entries) => {
    const visible = entries
      .filter(entry => entry.isIntersecting)
      .sort((a, b) => b.intersectionRatio - a.intersectionRatio)[0]
    if (visible?.target.id) activeSection.value = visible.target.id as SectionId
  }, {
    rootMargin: '-22% 0px -58% 0px',
    threshold: [0.05, 0.2, 0.45, 0.7]
  })
  sections.forEach(section => sectionObserver?.observe(section))

  const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  const revealElements = Array.from(document.querySelectorAll<HTMLElement>('.reveal-on-scroll'))
  if (reduceMotion) {
    revealElements.forEach(element => element.classList.add('is-visible'))
    return
  }

  revealObserver = new IntersectionObserver((entries, observer) => {
    entries.forEach(entry => {
      if (!entry.isIntersecting) return
      entry.target.classList.add('is-visible')
      observer.unobserve(entry.target)
    })
  }, { rootMargin: '0px 0px -12% 0px', threshold: 0.08 })
  revealElements.forEach(element => revealObserver?.observe(element))
}

onMounted(() => {
  isDark.value = document.documentElement.classList.contains('dark')
  authStore.checkAuth()
  if (!appStore.publicSettingsLoaded) appStore.fetchPublicSettings()

  if (!hasHomeContent.value && !compactHomeEnabled.value) {
    initializeObservers()
    updateScrollState()
    window.addEventListener('scroll', updateScrollState, { passive: true })
    window.addEventListener('resize', updateScrollState, { passive: true })
  }
})

onBeforeUnmount(() => {
  cancelAnimationFrame(scrollFrame)
  window.removeEventListener('scroll', updateScrollState)
  window.removeEventListener('resize', updateScrollState)
  sectionObserver?.disconnect()
  revealObserver?.disconnect()
})
</script>

<style scoped>
.home-page {
  --hero-parallax: 0px;
  position: relative;
  min-height: 100vh;
  overflow: clip;
  color: #0f172a;
  background: #f7fafc;
  font-family: "Noto Sans SC Variable", system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  font-synthesis: none;
  transition: color 300ms ease, background-color 300ms ease;
}

:global(.dark) .home-page {
  color: #e2e8f0;
  background: #030914;
}

.home-background {
  position: absolute;
  inset: 0;
  z-index: 0;
  overflow: hidden;
  pointer-events: none;
}

.home-grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(15, 118, 110, 0.055) 1px, transparent 1px),
    linear-gradient(90deg, rgba(15, 118, 110, 0.055) 1px, transparent 1px);
  background-size: 64px 64px;
  mask-image: linear-gradient(to bottom, black 0%, rgba(0, 0, 0, 0.9) 48%, transparent 100%);
}

:global(.dark) .home-grid {
  background-image:
    linear-gradient(rgba(45, 212, 191, 0.045) 1px, transparent 1px),
    linear-gradient(90deg, rgba(45, 212, 191, 0.045) 1px, transparent 1px);
}

.home-glow {
  position: absolute;
  border-radius: 999px;
  filter: blur(90px);
  opacity: 0.38;
  transform: translateY(var(--hero-parallax));
}
.home-glow-one { right: -12%; top: -3%; width: 640px; height: 640px; background: rgba(14, 165, 233, 0.2); }
.home-glow-two { left: -18%; top: 28%; width: 620px; height: 620px; background: rgba(20, 184, 166, 0.16); }
.home-glow-three { right: 12%; top: 53%; width: 520px; height: 520px; background: rgba(139, 92, 246, 0.1); }
:global(.dark) .home-glow-one { background: rgba(14, 165, 233, 0.15); opacity: 0.48; }
:global(.dark) .home-glow-two { background: rgba(20, 184, 166, 0.13); opacity: 0.48; }
:global(.dark) .home-glow-three { background: rgba(139, 92, 246, 0.1); opacity: 0.42; }

.home-noise {
  position: absolute;
  inset: 0;
  opacity: 0.018;
  background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 180 180' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='.9' numOctaves='3' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)' opacity='.7'/%3E%3C/svg%3E");
}

.home-container { width: min(100% - 40px, 1280px); margin-inline: auto; }
.home-section { position: relative; z-index: 1; scroll-margin-top: 104px; }

.home-header {
  position: sticky;
  top: 0;
  z-index: 50;
  border-bottom: 1px solid transparent;
  background: rgba(247, 250, 252, 0.7);
  backdrop-filter: blur(18px) saturate(130%);
  transition: border-color 220ms ease, background 220ms ease, box-shadow 220ms ease;
}

:global(.dark) .home-header { background: rgba(3, 9, 20, 0.68); }
.home-page-scrolled .home-header {
  border-color: rgba(148, 163, 184, 0.2);
  box-shadow: 0 12px 32px rgba(15, 23, 42, 0.07);
}
:global(.dark) .home-page-scrolled .home-header {
  border-color: rgba(51, 65, 85, 0.62);
  box-shadow: 0 18px 46px rgba(0, 0, 0, 0.32);
}

.home-scroll-progress {
  position: absolute;
  left: 0;
  bottom: -1px;
  height: 2px;
  background: linear-gradient(90deg, #14b8a6, #38bdf8, #8b5cf6);
  box-shadow: 0 0 12px rgba(20, 184, 166, 0.65);
  transition: width 70ms linear;
}

.home-navbar {
  display: grid;
  grid-template-columns: minmax(170px, 1fr) auto minmax(220px, 1fr);
  align-items: center;
  gap: 24px;
  width: min(100% - 40px, 1280px);
  min-height: 72px;
  margin-inline: auto;
}

.home-brand { display: inline-flex; width: fit-content; align-items: center; gap: 10px; text-align: left; }
.home-brand-logo { display: grid; width: 38px; height: 38px; flex: 0 0 auto; place-items: center; overflow: hidden; border-radius: 12px; background: linear-gradient(145deg, #0f766e, #0369a1); box-shadow: 0 8px 22px rgba(13, 148, 136, 0.24); }
.home-brand-logo img { width: 100%; height: 100%; object-fit: contain; }
.home-brand-name { max-width: 180px; overflow: hidden; color: #0f172a; font-size: 17px; font-weight: 700; text-overflow: ellipsis; white-space: nowrap; }
:global(.dark) .home-brand-name { color: #f8fafc; }

.home-nav-links { display: flex; align-items: center; justify-content: center; gap: 2px; }
.home-nav-link { position: relative; padding: 11px 13px; color: #64748b; font-size: 13px; font-weight: 600; transition: color 180ms ease; }
.home-nav-link::after { content: ''; position: absolute; right: 14px; bottom: 2px; left: 14px; height: 2px; border-radius: 99px; background: linear-gradient(90deg, #14b8a6, #38bdf8); transform: scaleX(0); transition: transform 220ms ease; }
.home-nav-link:hover, .home-nav-link.is-active { color: #0f766e; }
.home-nav-link.is-active::after { transform: scaleX(1); }
:global(.dark) .home-nav-link { color: #94a3b8; }
:global(.dark) .home-nav-link:hover, :global(.dark) .home-nav-link.is-active { color: #5eead4; }

.home-nav-actions { display: flex; align-items: center; justify-content: flex-end; gap: 8px; }
.home-icon-button { display: grid; width: 34px; height: 34px; place-items: center; border: 1px solid transparent; border-radius: 10px; color: #64748b; transition: all 180ms ease; }
.home-icon-button:hover { border-color: rgba(13, 148, 136, 0.18); color: #0f766e; background: rgba(255, 255, 255, 0.7); }
:global(.dark) .home-icon-button { color: #94a3b8; }
:global(.dark) .home-icon-button:hover { border-color: rgba(45, 212, 191, 0.18); color: #5eead4; background: rgba(15, 23, 42, 0.72); }

.home-login-button { display: inline-flex; min-height: 34px; align-items: center; gap: 7px; padding: 6px 12px; border: 1px solid rgba(13, 148, 136, 0.2); border-radius: 10px; color: white; background: linear-gradient(135deg, #0f766e, #0d9488); box-shadow: 0 7px 20px rgba(13, 148, 136, 0.2); font-size: 12px; font-weight: 600; transition: transform 180ms ease, box-shadow 180ms ease; }
.home-login-button:hover { transform: translateY(-1px); box-shadow: 0 10px 26px rgba(13, 148, 136, 0.28); }
.home-user-avatar { display: grid; width: 21px; height: 21px; place-items: center; border-radius: 7px; color: #0f766e; background: rgba(255, 255, 255, 0.9); font-size: 10px; }

.hero-section { min-height: 920px; padding: 76px 0 56px; }
.hero-layout { display: grid; grid-template-columns: minmax(0, 0.9fr) minmax(520px, 1.1fr); align-items: center; gap: 58px; }
.hero-copy { position: relative; z-index: 3; }
.hero-eyebrow { display: inline-flex; align-items: center; gap: 9px; margin-bottom: 22px; padding: 7px 12px; border: 1px solid rgba(13, 148, 136, 0.16); border-radius: 999px; color: #0f766e; background: rgba(255, 255, 255, 0.58); box-shadow: 0 8px 24px rgba(15, 23, 42, 0.05); font-size: 12px; font-weight: 600; backdrop-filter: blur(10px); }
:global(.dark) .hero-eyebrow { border-color: rgba(45, 212, 191, 0.18); color: #5eead4; background: rgba(15, 23, 42, 0.54); }
.hero-eyebrow-dot { width: 7px; height: 7px; border-radius: 50%; background: #10b981; box-shadow: 0 0 10px #10b981; animation: home-pulse 1.8s ease-in-out infinite; }
.hero-title { display: flex; flex-wrap: wrap; gap: 0 15px; margin: 0; color: #0f172a; font-size: clamp(48px, 5.4vw, 78px); font-weight: 800; letter-spacing: -0.035em; line-height: 1.08; }
:global(.dark) .hero-title { color: #f8fafc; }
.hero-title-gradient { color: transparent; background: linear-gradient(105deg, #0f766e 10%, #14b8a6 52%, #0284c7 96%); background-clip: text; -webkit-background-clip: text; }
:global(.dark) .hero-title-gradient { background-image: linear-gradient(105deg, #2dd4bf 8%, #67e8f9 56%, #60a5fa 96%); }
.hero-subtitle { max-width: 650px; margin: 23px 0 0; color: #334155; font-size: clamp(21px, 2vw, 29px); font-weight: 600; letter-spacing: -0.01em; line-height: 1.42; }
:global(.dark) .hero-subtitle { color: #cbd5e1; }
.hero-description { max-width: 620px; margin: 17px 0 0; color: #64748b; font-size: 15px; line-height: 1.9; }
:global(.dark) .hero-description { color: #94a3b8; }

.hero-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 12px; margin-top: 32px; }
.home-primary-button, .home-secondary-button, .home-text-button { display: inline-flex; align-items: center; justify-content: center; gap: 10px; min-height: 48px; border-radius: 13px; font-size: 14px; font-weight: 700; transition: transform 180ms ease, border-color 180ms ease, box-shadow 180ms ease, background 180ms ease; }
.home-primary-button { padding: 10px 17px 10px 21px; color: white; background: linear-gradient(135deg, #0f766e, #14b8a6); box-shadow: 0 12px 30px rgba(13, 148, 136, 0.28); }
.home-primary-button:hover { transform: translateY(-2px); box-shadow: 0 16px 38px rgba(13, 148, 136, 0.36); }
.button-arrow { display: grid; width: 29px; height: 29px; place-items: center; border-radius: 9px; background: rgba(255, 255, 255, 0.15); }
.home-secondary-button { padding: 10px 20px; border: 1px solid rgba(13, 148, 136, 0.2); color: #0f766e; background: rgba(255, 255, 255, 0.62); backdrop-filter: blur(10px); }
.home-secondary-button:hover { transform: translateY(-2px); border-color: rgba(13, 148, 136, 0.38); background: rgba(255, 255, 255, 0.9); }
:global(.dark) .home-secondary-button { border-color: rgba(45, 212, 191, 0.2); color: #99f6e4; background: rgba(15, 23, 42, 0.62); }
:global(.dark) .home-secondary-button:hover { border-color: rgba(45, 212, 191, 0.4); background: rgba(15, 23, 42, 0.86); }
.home-text-button { min-height: 42px; padding: 8px 10px; color: #64748b; }
.home-text-button:hover { color: #0f766e; }
:global(.dark) .home-text-button { color: #94a3b8; }
:global(.dark) .home-text-button:hover { color: #5eead4; }

.hero-trust { display: flex; flex-wrap: wrap; gap: 20px; margin-top: 25px; color: #64748b; font-size: 12px; }
.hero-trust span { display: inline-flex; align-items: center; gap: 7px; }
.hero-trust i { width: 18px; height: 18px; border: 1px solid rgba(16, 185, 129, 0.24); border-radius: 50%; background: radial-gradient(circle, rgba(16, 185, 129, 0.24), transparent 68%); }
.hero-trust i::after { content: '✓'; display: grid; place-items: center; color: #059669; font-size: 10px; }
:global(.dark) .hero-trust { color: #94a3b8; }
.hero-scene-wrap { min-width: 0; }

.quick-service-wrap { margin-top: 38px; }
.quick-service-bar { display: flex; width: fit-content; max-width: 100%; margin-inline: auto; overflow-x: auto; border: 1px solid rgba(148, 163, 184, 0.18); border-radius: 999px; background: rgba(255, 255, 255, 0.66); box-shadow: 0 14px 38px rgba(15, 23, 42, 0.08); backdrop-filter: blur(16px); scrollbar-width: none; }
.quick-service-bar::-webkit-scrollbar { display: none; }
:global(.dark) .quick-service-bar { border-color: rgba(51, 65, 85, 0.62); background: rgba(9, 18, 33, 0.72); box-shadow: 0 18px 44px rgba(0, 0, 0, 0.28); }
.quick-service-item { display: inline-flex; flex: 0 0 auto; align-items: center; gap: 9px; padding: 11px 20px; color: #475569; font-size: 12px; font-weight: 600; transition: color 180ms ease, background 180ms ease; }
.quick-service-item + .quick-service-item { border-left: 1px solid rgba(148, 163, 184, 0.16); }
.quick-service-item span { display: grid; width: 28px; height: 28px; place-items: center; border-radius: 9px; color: #0f766e; background: rgba(20, 184, 166, 0.12); }
.quick-service-item[data-tone='blue'] span { color: #2563eb; background: rgba(59, 130, 246, 0.12); }
.quick-service-item[data-tone='violet'] span { color: #7c3aed; background: rgba(139, 92, 246, 0.12); }
.quick-service-item[data-tone='cyan'] span { color: #0891b2; background: rgba(6, 182, 212, 0.12); }
.quick-service-item[data-tone='amber'] span { color: #d97706; background: rgba(245, 158, 11, 0.12); }
.quick-service-item:hover { color: #0f766e; background: rgba(20, 184, 166, 0.06); }
:global(.dark) .quick-service-item { color: #cbd5e1; }
:global(.dark) .quick-service-item:hover { color: #5eead4; background: rgba(20, 184, 166, 0.07); }

.services-section, .solutions-section, .process-section, .advertising-section, .contact-section { padding: 110px 0; }
.section-heading { max-width: 720px; margin: 0 auto 54px; text-align: center; }
.section-kicker { display: inline-block; margin-bottom: 12px; color: #0d9488; font-size: 11px; font-weight: 800; letter-spacing: 0.16em; }
:global(.dark) .section-kicker { color: #5eead4; }
.section-heading h2, .solutions-copy h2, .ad-copy h2, .contact-panel h2 { margin: 0; color: #0f172a; font-size: clamp(34px, 4vw, 52px); font-weight: 800; letter-spacing: -0.04em; line-height: 1.14; }
:global(.dark) .section-heading h2, :global(.dark) .solutions-copy h2, :global(.dark) .ad-copy h2, :global(.dark) .contact-panel h2 { color: #f8fafc; }
.section-heading p, .solutions-copy > p, .contact-panel p { margin: 17px 0 0; color: #64748b; font-size: 15px; line-height: 1.85; }
:global(.dark) .section-heading p, :global(.dark) .solutions-copy > p, :global(.dark) .contact-panel p { color: #94a3b8; }

.service-grid { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 16px; }
.service-card { position: relative; min-height: 328px; overflow: hidden; padding: 28px 23px 24px; border: 1px solid rgba(148, 163, 184, 0.18); border-radius: 22px; background: linear-gradient(155deg, rgba(255, 255, 255, 0.78), rgba(248, 250, 252, 0.56)); box-shadow: 0 18px 50px rgba(15, 23, 42, 0.07); backdrop-filter: blur(14px); transition: transform 260ms ease, border-color 260ms ease, box-shadow 260ms ease; }
.service-card:hover, .service-card:focus-visible { transform: translateY(-8px); border-color: rgba(20, 184, 166, 0.34); box-shadow: 0 24px 64px rgba(13, 148, 136, 0.14); outline: none; }
:global(.dark) .service-card { border-color: rgba(51, 65, 85, 0.68); background: linear-gradient(155deg, rgba(13, 25, 43, 0.84), rgba(5, 14, 28, 0.72)); box-shadow: 0 20px 58px rgba(0, 0, 0, 0.25); }
:global(.dark) .service-card:hover, :global(.dark) .service-card:focus-visible { border-color: rgba(45, 212, 191, 0.3); box-shadow: 0 28px 72px rgba(0, 0, 0, 0.34), 0 0 34px rgba(20, 184, 166, 0.08); }
.service-card-topline { position: absolute; top: 0; right: 20%; left: 20%; height: 2px; background: linear-gradient(90deg, transparent, #14b8a6, transparent); opacity: 0.65; }
.service-card[data-tone='blue'] .service-card-topline { background: linear-gradient(90deg, transparent, #3b82f6, transparent); }
.service-card[data-tone='violet'] .service-card-topline { background: linear-gradient(90deg, transparent, #8b5cf6, transparent); }
.service-card[data-tone='cyan'] .service-card-topline { background: linear-gradient(90deg, transparent, #06b6d4, transparent); }
.service-card[data-tone='amber'] .service-card-topline { background: linear-gradient(90deg, transparent, #f59e0b, transparent); }
.service-icon-wrap { position: relative; display: grid; width: 58px; height: 58px; place-items: center; margin-bottom: 28px; border-radius: 18px; color: #0f766e; background: linear-gradient(145deg, rgba(20, 184, 166, 0.18), rgba(13, 148, 136, 0.08)); box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.45), 0 12px 28px rgba(13, 148, 136, 0.14); }
.service-card[data-tone='blue'] .service-icon-wrap { color: #2563eb; background: linear-gradient(145deg, rgba(59, 130, 246, 0.18), rgba(37, 99, 235, 0.08)); }
.service-card[data-tone='violet'] .service-icon-wrap { color: #7c3aed; background: linear-gradient(145deg, rgba(139, 92, 246, 0.18), rgba(124, 58, 237, 0.08)); }
.service-card[data-tone='cyan'] .service-icon-wrap { color: #0891b2; background: linear-gradient(145deg, rgba(6, 182, 212, 0.18), rgba(8, 145, 178, 0.08)); }
.service-card[data-tone='amber'] .service-icon-wrap { color: #d97706; background: linear-gradient(145deg, rgba(245, 158, 11, 0.18), rgba(217, 119, 6, 0.08)); }
.service-icon-orbit { position: absolute; inset: -8px; border: 1px dashed currentColor; border-radius: 20px; opacity: 0.18; transition: transform 700ms ease; }
.service-card:hover .service-icon-orbit { transform: rotate(135deg); }
.service-number { position: absolute; top: 28px; right: 22px; color: rgba(100, 116, 139, 0.45); font-size: 11px; font-weight: 800; letter-spacing: 0.12em; }
.service-card h3 { margin: 0; color: #0f172a; font-size: 18px; font-weight: 700; }
:global(.dark) .service-card h3 { color: #f8fafc; }
.service-card p { margin: 13px 0 24px; color: #64748b; font-size: 13px; line-height: 1.78; }
:global(.dark) .service-card p { color: #94a3b8; }
.service-link { position: absolute; left: 23px; bottom: 22px; display: inline-flex; align-items: center; gap: 7px; color: #0f766e; font-size: 12px; font-weight: 700; }
:global(.dark) .service-link { color: #5eead4; }
.service-link:hover { gap: 10px; }

.solutions-section { background: linear-gradient(180deg, transparent, rgba(20, 184, 166, 0.045), transparent); }
:global(.dark) .solutions-section { background: linear-gradient(180deg, transparent, rgba(20, 184, 166, 0.035), transparent); }
.solutions-layout { display: grid; grid-template-columns: minmax(0, 0.8fr) minmax(0, 1.2fr); align-items: center; gap: 78px; }
.solutions-copy { max-width: 520px; }
.solution-stats { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; margin-top: 35px; }
.solution-stat { padding: 16px; border: 1px solid rgba(148, 163, 184, 0.16); border-radius: 15px; background: rgba(255, 255, 255, 0.5); }
:global(.dark) .solution-stat { border-color: rgba(51, 65, 85, 0.6); background: rgba(15, 23, 42, 0.48); }
.solution-stat strong, .solution-stat span { display: block; }
.solution-stat strong { color: #0f766e; font-size: 22px; font-weight: 800; }
:global(.dark) .solution-stat strong { color: #5eead4; }
.solution-stat span { margin-top: 4px; color: #64748b; font-size: 10px; }
:global(.dark) .solution-stat span { color: #94a3b8; }
.advantage-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
.advantage-card { display: flex; min-height: 170px; gap: 18px; padding: 24px; border: 1px solid rgba(148, 163, 184, 0.17); border-radius: 20px; background: rgba(255, 255, 255, 0.64); box-shadow: 0 14px 42px rgba(15, 23, 42, 0.055); transition: transform 220ms ease, border-color 220ms ease; }
.advantage-card:hover { transform: translateY(-4px); border-color: rgba(20, 184, 166, 0.28); }
:global(.dark) .advantage-card { border-color: rgba(51, 65, 85, 0.66); background: rgba(10, 21, 37, 0.7); box-shadow: 0 18px 46px rgba(0, 0, 0, 0.2); }
.advantage-icon { display: grid; width: 46px; height: 46px; flex: 0 0 auto; place-items: center; border: 1px solid rgba(20, 184, 166, 0.18); border-radius: 15px; color: #0f766e; background: rgba(20, 184, 166, 0.1); }
:global(.dark) .advantage-icon { color: #5eead4; background: rgba(20, 184, 166, 0.08); }
.advantage-card h3 { margin: 3px 0 9px; color: #0f172a; font-size: 16px; font-weight: 700; }
:global(.dark) .advantage-card h3 { color: #f8fafc; }
.advantage-card p { margin: 0; color: #64748b; font-size: 12px; line-height: 1.75; }
:global(.dark) .advantage-card p { color: #94a3b8; }

.capability-rail-section { position: relative; z-index: 1; padding: 22px 0; overflow: hidden; }
.capability-rail { transform: rotate(-1.2deg) scale(1.02); border-block: 1px solid rgba(20, 184, 166, 0.16); background: rgba(20, 184, 166, 0.055); }
:global(.dark) .capability-rail { border-color: rgba(45, 212, 191, 0.12); background: rgba(20, 184, 166, 0.045); }
.capability-track { display: flex; width: max-content; animation: capability-marquee 34s linear infinite; }
.capability-track span { display: inline-flex; align-items: center; gap: 11px; padding: 15px 26px; color: #475569; font-size: 12px; font-weight: 600; white-space: nowrap; }
.capability-track i { width: 5px; height: 5px; border-radius: 50%; background: #14b8a6; box-shadow: 0 0 8px #14b8a6; }
:global(.dark) .capability-track span { color: #94a3b8; }

.process-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 18px; }
.process-card { position: relative; min-height: 250px; padding: 25px; border: 1px solid rgba(148, 163, 184, 0.17); border-radius: 22px; background: rgba(255, 255, 255, 0.66); box-shadow: 0 16px 48px rgba(15, 23, 42, 0.06); }
:global(.dark) .process-card { border-color: rgba(51, 65, 85, 0.68); background: rgba(9, 20, 36, 0.72); box-shadow: 0 20px 50px rgba(0, 0, 0, 0.22); }
.process-card:not(:last-child) .process-line { position: absolute; top: 48px; left: calc(100% + 1px); width: 18px; height: 1px; background: linear-gradient(90deg, #14b8a6, rgba(20, 184, 166, 0.1)); }
.process-card:not(:last-child) .process-line::after { content: ''; position: absolute; right: 0; top: -3px; width: 7px; height: 7px; border-top: 1px solid #14b8a6; border-right: 1px solid #14b8a6; transform: rotate(45deg); }
.process-step-head { display: flex; align-items: center; justify-content: space-between; }
.process-step-number { color: #0d9488; font-size: 12px; font-weight: 800; letter-spacing: 0.14em; }
.process-step-icon { display: grid; width: 45px; height: 45px; place-items: center; border-radius: 14px; color: #0f766e; background: rgba(20, 184, 166, 0.11); }
:global(.dark) .process-step-number, :global(.dark) .process-step-icon { color: #5eead4; }
.process-card h3 { margin: 45px 0 12px; color: #0f172a; font-size: 19px; font-weight: 700; }
:global(.dark) .process-card h3 { color: #f8fafc; }
.process-card p { margin: 0; color: #64748b; font-size: 12px; line-height: 1.78; }
:global(.dark) .process-card p { color: #94a3b8; }

.advertising-panel { position: relative; display: grid; grid-template-columns: 1fr 0.58fr; min-height: 480px; overflow: hidden; padding: 56px 60px 140px; border: 1px solid rgba(13, 148, 136, 0.24); border-radius: 30px; background: linear-gradient(125deg, rgba(236, 254, 255, 0.9), rgba(240, 253, 250, 0.72) 50%, rgba(239, 246, 255, 0.82)); box-shadow: 0 30px 90px rgba(13, 148, 136, 0.13); }
:global(.dark) .advertising-panel { border-color: rgba(45, 212, 191, 0.2); background: linear-gradient(125deg, rgba(5, 35, 42, 0.9), rgba(4, 20, 33, 0.92) 52%, rgba(9, 18, 43, 0.94)); box-shadow: 0 38px 100px rgba(0, 0, 0, 0.34), 0 0 70px rgba(20, 184, 166, 0.08); }
.ad-grid { position: absolute; inset: 0; opacity: 0.32; background-image: linear-gradient(rgba(20, 184, 166, 0.1) 1px, transparent 1px), linear-gradient(90deg, rgba(20, 184, 166, 0.1) 1px, transparent 1px); background-size: 38px 38px; mask-image: radial-gradient(circle at 70% 42%, black, transparent 68%); }
.ad-copy { position: relative; z-index: 2; max-width: 620px; }
.ad-badge { display: inline-flex; margin-bottom: 18px; padding: 7px 11px; border: 1px solid rgba(13, 148, 136, 0.18); border-radius: 999px; color: #0f766e; background: rgba(255, 255, 255, 0.5); font-size: 11px; font-weight: 700; }
:global(.dark) .ad-badge { color: #5eead4; background: rgba(15, 23, 42, 0.48); }
.ad-copy p { max-width: 610px; margin: 19px 0 28px; color: #475569; font-size: 14px; line-height: 1.85; }
:global(.dark) .ad-copy p { color: #94a3b8; }
.ad-visual { position: relative; z-index: 1; display: grid; place-items: center; }
.ad-hologram { position: relative; display: grid; width: 190px; height: 190px; place-items: center; border: 1px solid rgba(20, 184, 166, 0.34); border-radius: 34px; color: white; background: linear-gradient(145deg, rgba(13, 148, 136, 0.9), rgba(3, 105, 161, 0.84)); box-shadow: 0 0 70px rgba(20, 184, 166, 0.32), inset 0 1px 0 rgba(255, 255, 255, 0.28); transform: rotateX(58deg) rotateZ(-36deg); animation: ad-float 5s ease-in-out infinite; }
.ad-hologram > span { font-size: 55px; font-weight: 800; transform: rotateZ(36deg) rotateX(-58deg); text-shadow: 0 0 22px rgba(255, 255, 255, 0.46); }
.ad-ring { position: absolute; inset: -30px; border: 1px solid rgba(20, 184, 166, 0.35); border-radius: 50%; animation: ring-rotate 12s linear infinite; }
.ad-ring-two { inset: -58px; border-style: dashed; border-color: rgba(56, 189, 248, 0.26); animation-direction: reverse; animation-duration: 18s; }
.ad-beam { position: absolute; left: 50%; top: 90%; width: 180px; height: 140px; background: linear-gradient(to bottom, rgba(20, 184, 166, 0.22), transparent); clip-path: polygon(35% 0, 65% 0, 100% 100%, 0 100%); transform: translateX(-50%); filter: blur(5px); }
.ad-metrics { position: absolute; right: 36px; bottom: 28px; left: 36px; z-index: 3; display: grid; grid-template-columns: repeat(4, 1fr); overflow: hidden; border: 1px solid rgba(13, 148, 136, 0.17); border-radius: 18px; background: rgba(255, 255, 255, 0.58); backdrop-filter: blur(14px); }
:global(.dark) .ad-metrics { border-color: rgba(45, 212, 191, 0.14); background: rgba(4, 15, 29, 0.66); }
.ad-metrics > div { padding: 17px 20px; text-align: center; }
.ad-metrics > div + div { border-left: 1px solid rgba(148, 163, 184, 0.18); }
.ad-metrics strong, .ad-metrics span { display: block; }
.ad-metrics strong { color: #0f766e; font-size: 18px; font-weight: 800; }
:global(.dark) .ad-metrics strong { color: #5eead4; }
.ad-metrics span { margin-top: 4px; color: #64748b; font-size: 10px; }
:global(.dark) .ad-metrics span { color: #94a3b8; }

.contact-section { padding-bottom: 120px; }
.contact-panel { display: flex; align-items: center; justify-content: space-between; gap: 50px; padding: 46px 52px; border: 1px solid rgba(148, 163, 184, 0.18); border-radius: 26px; background: rgba(255, 255, 255, 0.68); box-shadow: 0 22px 70px rgba(15, 23, 42, 0.08); backdrop-filter: blur(16px); }
:global(.dark) .contact-panel { border-color: rgba(51, 65, 85, 0.68); background: rgba(8, 19, 35, 0.72); box-shadow: 0 26px 80px rgba(0, 0, 0, 0.28); }
.contact-panel > div:first-child { max-width: 760px; }
.contact-panel h2 { font-size: clamp(30px, 3.6vw, 46px); }
.contact-actions { display: flex; flex: 0 0 auto; flex-direction: column; gap: 11px; }

.home-footer { position: relative; z-index: 1; border-top: 1px solid rgba(148, 163, 184, 0.18); background: rgba(248, 250, 252, 0.62); }
:global(.dark) .home-footer { border-color: rgba(51, 65, 85, 0.58); background: rgba(2, 7, 16, 0.7); }
.footer-grid { display: grid; grid-template-columns: 1.45fr repeat(3, 0.7fr); gap: 60px; padding-block: 54px 44px; }
.footer-brand-line { display: flex; align-items: center; gap: 11px; color: #0f172a; font-size: 17px; }
:global(.dark) .footer-brand-line { color: #f8fafc; }
.footer-brand-block p { max-width: 380px; margin: 17px 0 0; color: #64748b; font-size: 12px; line-height: 1.8; }
:global(.dark) .footer-brand-block p { color: #94a3b8; }
.footer-link-group { display: flex; flex-direction: column; align-items: flex-start; gap: 10px; }
.footer-link-group strong { margin-bottom: 6px; color: #0f172a; font-size: 13px; }
:global(.dark) .footer-link-group strong { color: #e2e8f0; }
.footer-link-group button, .footer-link-group a { color: #64748b; font-size: 11px; transition: color 180ms ease; }
.footer-link-group button:hover, .footer-link-group a:hover { color: #0f766e; }
:global(.dark) .footer-link-group button, :global(.dark) .footer-link-group a { color: #94a3b8; }
:global(.dark) .footer-link-group button:hover, :global(.dark) .footer-link-group a:hover { color: #5eead4; }
.footer-bottom { display: flex; align-items: center; justify-content: space-between; gap: 20px; padding-block: 20px; border-top: 1px solid rgba(148, 163, 184, 0.15); color: #94a3b8; font-size: 10px; }
:global(.dark) .footer-bottom { border-color: rgba(51, 65, 85, 0.48); color: #64748b; }

.back-to-top { position: fixed; right: 24px; bottom: 24px; z-index: 40; display: grid; width: 42px; height: 42px; place-items: center; border: 1px solid rgba(20, 184, 166, 0.24); border-radius: 13px; color: #0f766e; background: rgba(255, 255, 255, 0.78); box-shadow: 0 12px 34px rgba(15, 23, 42, 0.12); backdrop-filter: blur(12px); transition: transform 180ms ease, background 180ms ease; }
.back-to-top:hover { transform: translateY(-3px); background: white; }
:global(.dark) .back-to-top { color: #5eead4; background: rgba(8, 20, 37, 0.8); box-shadow: 0 16px 40px rgba(0, 0, 0, 0.34); }

.reveal-on-scroll { opacity: 0; transform: translateY(32px); transition: opacity 700ms ease var(--reveal-delay, 0ms), transform 700ms cubic-bezier(0.22, 1, 0.36, 1) var(--reveal-delay, 0ms); }
.reveal-on-scroll.is-visible { opacity: 1; transform: translateY(0); }
.hero-enter { opacity: 0; animation: hero-enter 720ms cubic-bezier(0.22, 1, 0.36, 1) forwards; }
.hero-enter-one { animation-delay: 80ms; }
.hero-enter-two { animation-delay: 150ms; }
.hero-enter-three { animation-delay: 230ms; }
.hero-enter-four { animation-delay: 300ms; }
.hero-enter-five { animation-delay: 380ms; }
.hero-enter-six { animation-delay: 450ms; }
.hero-enter-scene { animation-delay: 210ms; }
.hero-enter-seven { animation-delay: 520ms; }

@keyframes hero-enter { from { opacity: 0; transform: translateY(26px); } to { opacity: 1; transform: translateY(0); } }
@keyframes home-pulse { 0%, 100% { opacity: 0.55; } 50% { opacity: 1; } }
@keyframes capability-marquee { to { transform: translateX(-50%); } }
@keyframes ad-float { 0%, 100% { margin-top: 0; } 50% { margin-top: -15px; } }
@keyframes ring-rotate { to { transform: rotate(360deg); } }

@media (max-width: 1180px) {
  .home-navbar { grid-template-columns: minmax(150px, 1fr) auto; }
  .home-nav-links { grid-column: 1 / -1; grid-row: 2; order: 3; width: 100%; justify-content: flex-start; overflow-x: auto; padding-bottom: 7px; scrollbar-width: none; }
  .home-nav-links::-webkit-scrollbar { display: none; }
  .home-nav-actions { grid-column: 2; grid-row: 1; }
  .home-header { padding-top: 4px; }
  .hero-section { padding-top: 64px; }
  .hero-layout { grid-template-columns: minmax(0, 0.9fr) minmax(470px, 1.1fr); gap: 34px; }
  .service-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .service-card:nth-child(4), .service-card:nth-child(5) { min-height: 300px; }
}

@media (max-width: 900px) {
  .hero-section { min-height: auto; }
  .hero-layout { grid-template-columns: 1fr; }
  .hero-copy { max-width: 760px; text-align: center; margin-inline: auto; }
  .hero-title, .hero-actions, .hero-trust { justify-content: center; }
  .hero-subtitle, .hero-description { margin-inline: auto; }
  .hero-scene-wrap { width: min(100%, 720px); margin-inline: auto; }
  .quick-service-wrap { margin-top: 24px; }
  .solutions-layout { grid-template-columns: 1fr; gap: 45px; }
  .solutions-copy { max-width: 720px; text-align: center; margin-inline: auto; }
  .process-grid { grid-template-columns: repeat(2, 1fr); }
  .process-card:nth-child(2) .process-line { display: none; }
  .advertising-panel { grid-template-columns: 1fr; padding: 48px 38px 150px; }
  .ad-visual { position: absolute; right: 8%; top: 70px; opacity: 0.45; }
  .ad-copy { max-width: 70%; }
  .contact-panel { align-items: flex-start; flex-direction: column; }
  .contact-actions { flex-direction: row; }
  .footer-grid { grid-template-columns: 1.2fr repeat(3, 1fr); gap: 30px; }
}

@media (max-width: 680px) {
  .home-container, .home-navbar { width: min(100% - 24px, 1280px); }
  .home-navbar { min-height: 62px; gap: 10px; }
  .home-brand-name, .home-doc-link { display: none; }
  .home-nav-actions { gap: 4px; }
  .home-login-button { padding-inline: 10px; }
  .home-nav-link { padding: 9px 10px; font-size: 11px; }
  .hero-section { padding: 42px 0 38px; }
  .hero-title { justify-content: center; font-size: clamp(42px, 14vw, 64px); }
  .hero-subtitle { font-size: 20px; }
  .hero-description { font-size: 13px; }
  .hero-actions { flex-direction: column; }
  .home-primary-button, .home-secondary-button { width: min(100%, 310px); }
  .hero-trust { gap: 12px; }
  .quick-service-item { padding: 9px 14px; }
  .services-section, .solutions-section, .process-section, .advertising-section, .contact-section { padding: 76px 0; }
  .service-grid, .advantage-grid, .process-grid { grid-template-columns: 1fr; }
  .service-card { min-height: 290px; }
  .solution-stats { grid-template-columns: 1fr 1fr 1fr; }
  .process-card { min-height: 215px; }
  .process-card .process-line { display: none !important; }
  .advertising-panel { min-height: 620px; padding: 38px 24px 230px; border-radius: 24px; }
  .ad-copy { max-width: 100%; }
  .ad-visual { right: 50%; top: auto; bottom: 120px; transform: translateX(50%) scale(0.72); }
  .ad-metrics { right: 16px; bottom: 16px; left: 16px; grid-template-columns: repeat(2, 1fr); }
  .ad-metrics > div:nth-child(3) { border-left: 0; border-top: 1px solid rgba(148, 163, 184, 0.18); }
  .ad-metrics > div:nth-child(4) { border-top: 1px solid rgba(148, 163, 184, 0.18); }
  .contact-panel { padding: 34px 24px; }
  .contact-actions { width: 100%; flex-direction: column; }
  .footer-grid { grid-template-columns: 1fr 1fr; gap: 34px 24px; }
  .footer-brand-block { grid-column: 1 / -1; }
  .footer-bottom { align-items: flex-start; flex-direction: column; }
  .back-to-top { right: 14px; bottom: 14px; }
}

@media (prefers-reduced-motion: reduce) {
  .home-page *, .home-page *::before, .home-page *::after { scroll-behavior: auto !important; animation-duration: 0.01ms !important; animation-iteration-count: 1 !important; transition-duration: 0.01ms !important; }
  .reveal-on-scroll { opacity: 1; transform: none; }
  .capability-track { animation: none; }
}
</style>
