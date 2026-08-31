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
            :title="copy.nav.docs"
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
            :title="isDark ? copy.nav.light : copy.nav.dark"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="sm" />
            <Icon v-else name="moon" size="sm" />
          </button>
          <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="home-solid-button">
            {{ isAuthenticated ? copy.nav.dashboard : copy.nav.login }}
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

  <div v-else ref="pageRef" class="home-page" :class="{ 'home-page-scrolled': isScrolled }">
    <div class="home-background" aria-hidden="true">
      <span class="home-background-light home-background-light-one"></span>
      <span class="home-background-light home-background-light-two"></span>
      <span class="home-background-rule home-background-rule-one"></span>
      <span class="home-background-rule home-background-rule-two"></span>
    </div>

    <header class="home-header">
      <div class="home-scroll-progress" :style="{ width: `${scrollProgress}%` }"></div>
      <nav class="home-navbar" aria-label="Home navigation">
        <button class="home-brand" type="button" @click="scrollToSection('home')">
          <span class="brand-mark"><img :src="siteLogo || brand.logo" :alt="siteName" /></span>
          <span class="home-brand-name">{{ siteName }}</span>
        </button>

        <div class="home-nav-links">
          <template v-for="item in navigationItems" :key="item.id">
            <router-link
              v-if="item.id === 'learning'"
              :to="learningEntry"
              class="home-nav-link"
              :class="{ 'is-active': activeSection === item.id }"
              :aria-current="activeSection === item.id ? 'page' : undefined"
            >
              {{ item.label }}
            </router-link>
            <button
              v-else
              type="button"
              class="home-nav-link"
              :class="{ 'is-active': activeSection === item.id }"
              :aria-current="activeSection === item.id ? 'page' : undefined"
              @click="scrollToSection(item.id)"
            >
              {{ item.label }}
            </button>
          </template>
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
          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="home-quiet-link"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="sm" />
            <span class="hidden lg:inline">{{ t('nav.modelPlaza') }}</span>
          </router-link>
          <button
            type="button"
            class="home-icon-button"
            :title="isDark ? copy.nav.light : copy.nav.dark"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="sm" />
            <Icon v-else name="moon" size="sm" />
          </button>
          <router-link v-if="isAuthenticated" :to="dashboardPath" class="home-solid-button">
            <span class="home-user-avatar">{{ userInitial }}</span>
            <span class="hidden sm:inline">{{ copy.nav.dashboard }}</span>
          </router-link>
          <router-link v-else to="/login" class="home-solid-button" :title="copy.nav.login" :aria-label="copy.nav.login">
            <span class="home-auth-label">{{ copy.nav.login }}</span>
            <Icon name="arrowRight" size="xs" />
          </router-link>
        </div>
      </nav>
    </header>

    <main>
      <section id="home" data-home-section class="home-section home-hero-section">
        <div class="home-container home-hero-layout">
          <div class="home-hero-copy">
            <div class="home-overline home-hero-reveal">{{ copy.hero.eyebrow }}</div>
            <h1 class="home-hero-title home-hero-reveal">
              <span>{{ copy.hero.title }}</span>
              <span class="home-hero-title-accent">{{ copy.hero.titleAccent }}</span>
            </h1>
            <p class="home-hero-subtitle home-hero-reveal">{{ copy.hero.subtitle }}</p>
            <p class="home-hero-description home-hero-reveal">{{ copy.hero.description }}</p>
            <div class="home-hero-actions home-hero-reveal">
              <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="home-primary-button">
                {{ isAuthenticated ? copy.hero.dashboardCta : copy.hero.primaryCta }}
                <Icon name="arrowRight" size="sm" />
              </router-link>
              <router-link to="/key-usage" class="home-secondary-button">
                {{ copy.hero.secondaryCta }}
                <Icon name="chart" size="sm" />
              </router-link>
            </div>
            <div class="home-hero-facts home-hero-reveal">
              <span v-for="fact in copy.hero.facts" :key="fact"><i></i>{{ fact }}</span>
            </div>
          </div>

          <div class="home-hero-scene home-hero-reveal">
            <div
              ref="heroStageRef"
              class="home-hero-stage-frame"
              @pointermove="handleHeroPointerMove"
              @pointerleave="resetHeroPointer"
            >
              <div class="home-stage-topline" aria-hidden="true">
                <span>MODURELAY / ROUTING VIEW</span>
                <span class="home-stage-status"><i></i> READY</span>
              </div>
              <div class="home-stage-corner home-stage-corner-tl" aria-hidden="true"></div>
              <div class="home-stage-corner home-stage-corner-tr" aria-hidden="true"></div>
              <div class="home-stage-corner home-stage-corner-bl" aria-hidden="true"></div>
              <div class="home-stage-corner home-stage-corner-br" aria-hidden="true"></div>
              <HomeHeroScene />
              <div class="home-stage-scanline" aria-hidden="true"></div>
              <div class="home-stage-footline" aria-hidden="true">
                <span>OPENAI-COMPATIBLE</span>
                <span>MODELS / ROUTES / USAGE</span>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section class="home-capability-strip" aria-label="ModuRelay capabilities">
        <div class="home-container home-capability-grid">
          <span v-for="item in copy.capabilities.strip" :key="item"><i></i>{{ item }}</span>
        </div>
      </section>

      <section id="integrate" data-home-section class="home-section home-section-light">
        <div class="home-container home-two-column">
          <div class="home-section-copy home-reveal">
            <span class="home-section-index">01 / INTEGRATE</span>
            <h2>{{ copy.integrate.title }}</h2>
            <p>{{ copy.integrate.description }}</p>
            <div class="home-endpoint-list">
              <div v-for="endpoint in copy.integrate.endpoints" :key="endpoint.path" class="home-endpoint-row">
                <span class="endpoint-method">{{ endpoint.method }}</span>
                <code>{{ endpoint.path }}</code>
                <span>{{ endpoint.label }}</span>
              </div>
            </div>
          </div>

          <div class="home-code-panel home-reveal">
            <div class="home-code-header">
              <div class="home-window-dots"><i></i><i></i><i></i></div>
              <div class="home-code-tabs" role="tablist" :aria-label="copy.integrate.codeLabel">
                <button
                  v-for="sample in codeSampleOptions"
                  :key="sample.key"
                  type="button"
                  role="tab"
                  :aria-selected="activeCodeSample === sample.key"
                  :class="{ 'is-active': activeCodeSample === sample.key }"
                  @click="activeCodeSample = sample.key"
                >{{ sample.label }}</button>
              </div>
            </div>
            <Transition name="home-code-swap" mode="out-in">
              <pre :key="activeCodeSample"><code>{{ activeCode }}</code></pre>
            </Transition>
            <div class="home-code-footer"><span class="home-status-dot"></span>{{ copy.integrate.codeFooter }}</div>
          </div>
        </div>
      </section>

      <section id="routing" data-home-section class="home-section home-section-contrast">
        <div class="home-container">
          <div class="home-section-heading home-reveal">
            <span class="home-section-index">02 / ROUTING CORE</span>
            <h2>{{ copy.routing.title }}</h2>
            <p>{{ copy.routing.description }}</p>
          </div>
          <div class="routing-visual home-reveal">
            <div class="routing-node routing-node-request">
              <span class="routing-node-icon"><Icon name="link" size="md" /></span>
              <strong>{{ copy.routing.request.title }}</strong>
              <code>/v1/chat/completions</code>
            </div>
            <div class="routing-connector"><span></span></div>
            <div class="routing-core-node">
              <div class="routing-core-mark"><span>M</span></div>
              <strong>Relay Core</strong>
              <small>{{ copy.routing.coreCaption }}</small>
            </div>
            <div class="routing-connector"><span></span></div>
            <div class="routing-node routing-node-routes">
              <span class="routing-node-label">{{ copy.routing.routesLabel }}</span>
              <div v-for="route in copy.routing.routes" :key="route.title" class="routing-route-row">
                <span class="home-status-dot"></span>
                <strong>{{ route.title }}</strong>
                <small>{{ route.detail }}</small>
              </div>
            </div>
          </div>
          <div class="routing-feature-grid">
            <article v-for="feature in copy.routing.features" :key="feature.title" class="routing-feature home-reveal">
              <span class="routing-feature-number">{{ feature.number }}</span>
              <div><h3>{{ feature.title }}</h3><p>{{ feature.description }}</p></div>
            </article>
          </div>
        </div>
      </section>

      <section id="observability" data-home-section class="home-section home-section-light">
        <div class="home-container home-two-column home-observability-layout">
          <div class="home-product-preview home-reveal" aria-label="ModuRelay control and observability preview">
            <div class="preview-window-bar"><span>MODURELAY / PREVIEW</span><span class="preview-live"><i></i>SAMPLE</span></div>
            <div class="preview-body">
              <aside class="preview-sidebar">
                <span class="preview-logo">M</span>
                <i></i><i></i><i></i><i></i>
              </aside>
              <div class="preview-main">
                <div class="preview-title-row"><strong>{{ copy.observability.previewTitle }}</strong><span>Workspace view</span></div>
                <div class="preview-kpis">
                  <div><span>Requests</span><strong>Usage</strong><em>tracked</em></div>
                  <div><span>Configured routes</span><strong>Policy</strong><em class="is-cyan">ready</em></div>
                </div>
                <div class="preview-chart" aria-hidden="true">
                  <i></i><i></i><i></i><i></i><i></i><i></i><i></i><i></i><i></i><i></i><i></i><i></i>
                  <svg viewBox="0 0 520 120" preserveAspectRatio="none"><path d="M0 96 C30 87 42 92 62 70 S96 88 120 73 S147 35 171 61 S202 73 228 44 S262 58 286 42 S321 70 348 40 S380 61 406 25 S435 53 460 34 S489 42 520 9" /></svg>
                </div>
                <div class="preview-table">
                  <div v-for="route in copy.observability.previewRoutes" :key="route" class="preview-table-row"><span>{{ route }}</span><span class="home-status-dot"></span><strong>Ready</strong><small>route</small></div>
                </div>
              </div>
            </div>
          </div>
          <div class="home-section-copy home-reveal">
            <span class="home-section-index">03 / OBSERVABILITY</span>
            <h2>{{ copy.observability.title }}</h2>
            <p>{{ copy.observability.description }}</p>
            <div class="home-observability-list">
              <div v-for="item in copy.observability.items" :key="item.title">
                <span class="home-list-icon"><Icon :name="item.icon" size="sm" /></span>
                <div><strong>{{ item.title }}</strong><p>{{ item.description }}</p></div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section id="workflow" data-home-section class="home-section home-section-contrast home-workflow-section">
        <div class="home-container">
          <div class="home-section-heading home-reveal">
            <span class="home-section-index">04 / WORKFLOW</span>
            <h2>{{ copy.workflow.title }}</h2>
            <p>{{ copy.workflow.description }}</p>
          </div>
          <div class="home-workflow-grid">
            <article v-for="(step, index) in copy.workflow.steps" :key="step.title" class="home-workflow-step home-reveal">
              <div class="workflow-step-top"><span>0{{ index + 1 }}</span><Icon :name="step.icon" size="md" /></div>
              <h3>{{ step.title }}</h3>
              <p>{{ step.description }}</p>
              <div v-if="index < copy.workflow.steps.length - 1" class="workflow-arrow" aria-hidden="true"><Icon name="arrowRight" size="sm" /></div>
            </article>
          </div>
        </div>
      </section>

      <section id="learning" data-home-section class="home-section home-learning-section">
        <div class="home-container home-learning-layout">
          <div class="home-section-copy home-reveal">
            <span class="home-section-index">{{ copy.learning.eyebrow }}</span>
            <h2>{{ copy.learning.title }}</h2>
            <p>{{ copy.learning.description }}</p>
            <div class="home-learning-points">
              <span v-for="point in copy.learning.points" :key="point"><i></i>{{ point }}</span>
            </div>
            <router-link :to="learningEntry" class="home-primary-button home-learning-cta">
              {{ copy.learning.cta }}
              <Icon name="arrowRight" size="sm" />
            </router-link>
          </div>

          <div class="home-learning-preview home-reveal" aria-label="AI learning capability preview">
            <div class="home-learning-preview-topline">
              <span>AI LEARNING / CAPABILITY MAP</span>
              <span><i></i>{{ copy.learning.status }}</span>
            </div>
            <div class="home-learning-orbit" aria-hidden="true">
              <span class="home-learning-orbit-ring home-learning-orbit-ring-one"></span>
              <span class="home-learning-orbit-ring home-learning-orbit-ring-two"></span>
              <div class="home-learning-core"><strong>AI</strong><small>KNOWLEDGE</small></div>
              <span v-for="(node, index) in copy.learning.nodes" :key="node.label" class="home-learning-node" :class="`home-learning-node-${index + 1}`">
                <span class="home-learning-node-icon"><Icon :name="node.icon" size="sm" /></span>
                <span><strong>{{ node.label }}</strong><small>{{ node.detail }}</small></span>
              </span>
            </div>
            <div class="home-learning-preview-footer">
              <span>{{ copy.learning.footer }}</span>
              <router-link :to="learningEntry"><Icon name="arrowUp" size="xs" /></router-link>
            </div>
          </div>
        </div>
      </section>

      <section id="contact" data-home-section class="home-section home-final-section">
        <div class="home-container home-final-panel home-reveal">
          <div>
            <span class="home-section-index">MODURELAY</span>
            <h2>{{ copy.contact.title }}</h2>
            <p>{{ copy.contact.description }}</p>
          </div>
          <div class="home-final-actions">
            <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="home-primary-button">
              {{ isAuthenticated ? copy.hero.dashboardCta : copy.contact.primaryCta }}
              <Icon name="arrowRight" size="sm" />
            </router-link>
            <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="home-secondary-button">{{ copy.contact.docsCta }}<Icon name="book" size="sm" /></a>
          </div>
        </div>
      </section>
    </main>

    <footer class="home-footer">
      <div class="home-container home-footer-grid">
        <div class="home-footer-brand">
          <div class="home-brand-line"><span class="brand-mark"><img :src="siteLogo || brand.logo" :alt="siteName" /></span><strong>{{ siteName }}</strong></div>
          <p>{{ copy.footer.description }}</p>
        </div>
        <div class="home-footer-links"><strong>{{ copy.footer.product }}</strong><button type="button" @click="scrollToSection('integrate')">{{ copy.nav.integrate }}</button><button type="button" @click="scrollToSection('routing')">{{ copy.nav.routing }}</button><button type="button" @click="scrollToSection('observability')">{{ copy.nav.observability }}</button></div>
        <div class="home-footer-links"><strong>{{ copy.footer.resources }}</strong><a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">{{ copy.nav.docs }}</a><router-link to="/key-usage">{{ copy.footer.usage }}</router-link><router-link to="/model-plaza">{{ t('nav.modelPlaza') }}</router-link><router-link :to="learningEntry">{{ copy.nav.learning }}</router-link></div>
        <div class="home-footer-links"><strong>{{ copy.footer.account }}</strong><router-link :to="isAuthenticated ? dashboardPath : '/login'">{{ isAuthenticated ? copy.nav.dashboard : copy.nav.login }}</router-link><button type="button" @click="scrollToSection('contact')">{{ copy.nav.contact }}</button></div>
      </div>
      <div class="home-container home-footer-bottom"><span>&copy; {{ currentYear }} {{ siteName }}. {{ copy.footer.rights }}</span><span>{{ copy.footer.tagline }}</span></div>
    </footer>

    <button v-show="isScrolled" type="button" class="back-to-top" :title="copy.nav.backToTop" @click="scrollToSection('home')"><Icon name="arrowUp" size="sm" /></button>
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

type SectionId = 'home' | 'integrate' | 'routing' | 'observability' | 'workflow' | 'learning' | 'contact'
type CodeSampleKey = 'curl' | 'python' | 'node'

const zhCopy = {
  nav: { home: '首页', integrate: '接入', routing: '路由核心', observability: '可观测性', workflow: '工作流', learning: 'AI 学习', contact: '联系我们', docs: '文档', light: '切换浅色模式', dark: '切换深色模式', dashboard: '控制台', login: '登录', backToTop: '返回顶部' },
  hero: { eyebrow: 'OPENAI-COMPATIBLE API GATEWAY', title: '一个 API', titleAccent: '连接模型与能力', subtitle: '统一接入 自动路由 稳定返回', description: '用熟悉的 OpenAI compatible 接口接入模型，管理密钥，追踪用量，把复杂的路由与故障切换交给 ModuRelay。', primaryCta: '开始使用', dashboardCta: '进入控制台', secondaryCta: '用量查询', facts: ['统一接入', '智能路由', '用量可见'] },
  capabilities: { strip: ['统一 API 网关', '多模型路由', '健康检查', '用量与额度'] },
  integrate: { title: '从一个端点开始', description: '沿用熟悉的接口和 SDK 轻松接入。模型选择、路由策略和响应处理统一由网关完成。', codeLabel: 'Code examples', codeFooter: 'Request shape validated at the gateway', endpoints: [{ method: 'POST', path: '/v1/chat/completions', label: 'Chat Completions' }, { method: 'POST', path: '/v1/responses', label: 'Responses API' }, { method: 'GET', path: '/v1/models', label: 'Model discovery' }] },
  routing: { title: '每一次请求都有清晰去处', description: 'ModuRelay 根据模型能力、线路健康和分组策略选择路径。出现异常时自动切换，让应用保持稳定。', request: { title: 'Your application' }, coreCaption: 'policy + health + usage', routesLabel: 'Available routes', routes: [{ title: 'OpenAI compatible', detail: 'primary route' }, { title: 'Responses API', detail: 'capability match' }, { title: 'Fallback route', detail: 'on provider error' }], features: [{ number: '01', title: 'Provider pools', description: '将可用账户按分组组织，让路由策略清楚可维护。' }, { number: '02', title: 'Failover', description: 'Provider 出错时根据策略切换可用线路，减少中断。' }, { number: '03', title: 'Usage-aware', description: '围绕请求、模型和密钥保留完整的用量上下文。' }] },
  observability: { title: '运行状态始终清楚', description: '请求、用量、额度和线路状态集中呈现，需要排查时直接找到原因。', previewTitle: 'Operations snapshot', previewRoutes: ['OpenAI / Chat', 'Responses / Primary', 'Fallback / Health'], items: [{ icon: 'chart' as const, title: 'Usage', description: '按日期、模型和密钥查看请求与 token。' }, { icon: 'shield' as const, title: 'Channel status', description: '查看线路健康状态和响应表现。' }, { icon: 'key' as const, title: 'API keys', description: '创建、管理并安全使用接入密钥。' }] },
  workflow: { title: '让请求稳定抵达', description: '应用只面对一个兼容接口。网关负责选择线路、处理异常并返回结果。', steps: [{ icon: 'link' as const, title: 'Connect', description: '使用 API key 指向 ModuRelay gateway。' }, { icon: 'server' as const, title: 'Relay', description: 'Relay Core 读取分组和路由配置。' }, { icon: 'sync' as const, title: 'Route', description: '选择合适的 Provider，必要时执行切换。' }, { icon: 'check' as const, title: 'Respond', description: '返回兼容响应，同时保留用量信息。' }] },
  learning: { eyebrow: '05 / AI LEARNING', title: '让 AI 经验变成可学习的能力', description: '面向老师与技术专家，把 Agent 设计、工作流、记忆反馈和具身智能整理成真实案例与练习，从看懂到做出结果。', points: ['老师能力：把复杂 AI 讲清楚，让学习有路径', '技术专家：把架构与工程判断落到真实任务', '学习方式：案例理解、动手练习、结果复盘'], cta: '进入 AI 学习', status: '公开内容', footer: '点击主题，查看学习内容', nodes: [{ label: 'Agent', detail: '可教的架构', icon: 'brain' as const }, { label: 'Workflow', detail: '技术专家实践', icon: 'cpu' as const }, { label: 'Memory', detail: '反馈闭环', icon: 'database' as const }, { label: 'Embodied', detail: '真实落地', icon: 'beaker' as const }] },
  contact: { title: '从一个端点开始让系统更简单', description: '用一个清晰的入口连接模型、路由与用量信息。', primaryCta: '开始使用', docsCta: '查看文档' },
  footer: { description: '一个面向模型接入与运行管理的 OpenAI compatible API 网关。', product: '产品', resources: '资源', account: '账户', usage: '用量查询', rights: '保留所有权利', tagline: '接入简单 运行清楚' }
}

const enCopy = {
  nav: { home: 'Home', integrate: 'Integrate', routing: 'Routing core', observability: 'Observability', workflow: 'Workflow', learning: 'AI Learning', contact: 'Contact', docs: 'Docs', light: 'Switch to light mode', dark: 'Switch to dark mode', dashboard: 'Dashboard', login: 'Sign in', backToTop: 'Back to top' },
  hero: { eyebrow: 'OPENAI-COMPATIBLE API GATEWAY', title: 'One API', titleAccent: 'Connect every capability', subtitle: 'One entry point Smart routing Clear responses', description: 'Use a familiar OpenAI compatible interface to connect models, manage keys and track usage while ModuRelay handles routing and failover.', primaryCta: 'Get started', dashboardCta: 'Open dashboard', secondaryCta: 'Usage lookup', facts: ['One entry point', 'Smart routing', 'Visible usage'] },
  capabilities: { strip: ['Unified API gateway', 'Multi-model routing', 'Health checks', 'Usage & quota'] },
  integrate: { title: 'Start with one endpoint', description: 'Keep the interface and SDKs you already know. The gateway handles model choice, routing and response handling in one place.', codeLabel: 'Code examples', codeFooter: 'Request shape validated at the gateway', endpoints: [{ method: 'POST', path: '/v1/chat/completions', label: 'Chat Completions' }, { method: 'POST', path: '/v1/responses', label: 'Responses API' }, { method: 'GET', path: '/v1/models', label: 'Model discovery' }] },
  routing: { title: 'Every request has a clear destination', description: 'ModuRelay chooses a path from model capability, route health and group policy. When something fails, it switches cleanly so the app can keep moving.', request: { title: 'Your application' }, coreCaption: 'policy + health + usage', routesLabel: 'Available routes', routes: [{ title: 'OpenAI compatible', detail: 'primary route' }, { title: 'Responses API', detail: 'capability match' }, { title: 'Fallback route', detail: 'on provider error' }], features: [{ number: '01', title: 'Provider pools', description: 'Organize available accounts into clear, maintainable groups.' }, { number: '02', title: 'Failover', description: 'Switch to an available route when a Provider returns an error.' }, { number: '03', title: 'Usage-aware', description: 'Keep request, model and key context visible in usage data.' }] },
  observability: { title: 'Keep runtime easy to read', description: 'See requests, usage, quota and route status together, then get to the reason when something needs attention.', previewTitle: 'Operations snapshot', previewRoutes: ['OpenAI / Chat', 'Responses / Primary', 'Fallback / Health'], items: [{ icon: 'chart' as const, title: 'Usage', description: 'Review requests and tokens by date, model and key.' }, { icon: 'shield' as const, title: 'Channel status', description: 'See route health and response behavior.' }, { icon: 'key' as const, title: 'API keys', description: 'Create, manage and use access keys securely.' }] },
  workflow: { title: 'Help every request arrive', description: 'The client sees one compatible API. The gateway chooses the route, handles exceptions and returns the result.', steps: [{ icon: 'link' as const, title: 'Connect', description: 'Point an existing client at the ModuRelay gateway.' }, { icon: 'server' as const, title: 'Relay', description: 'Relay Core reads group and route configuration.' }, { icon: 'sync' as const, title: 'Route', description: 'Choose the right Provider and fail over when needed.' }, { icon: 'check' as const, title: 'Respond', description: 'Return a compatible response with usage context.' }] },
  learning: { eyebrow: '05 / AI LEARNING', title: 'Turn AI experience into learnable capability', description: 'For teachers and technical experts, real cases turn Agent design, workflows, memory and embodied intelligence into something you can understand and practise.', points: ['Teacher capability: make complex AI clear and teachable', 'Technical experts: bring architecture and engineering judgement into real tasks', 'Learning method: understand, practise and review'], cta: 'Enter AI Learning', status: 'PUBLIC SUMMARY', footer: 'Select a topic to explore', nodes: [{ label: 'Agent', detail: 'Teach the architecture', icon: 'brain' as const }, { label: 'Workflow', detail: 'Expert practice', icon: 'cpu' as const }, { label: 'Memory', detail: 'Feedback loop', icon: 'database' as const }, { label: 'Embodied', detail: 'Real-world proof', icon: 'beaker' as const }] },
  contact: { title: 'Start with one endpoint and simplify the system', description: 'Bring models, routing and usage into one clear entry point.', primaryCta: 'Get started', docsCta: 'Read the docs' },
  footer: { description: 'An OpenAI compatible API gateway for model access and runtime management.', product: 'Product', resources: 'Resources', account: 'Account', usage: 'Usage lookup', rights: 'All rights reserved', tagline: 'Simple to connect Clear to operate' }
}

const { t, locale } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const pageRef = ref<HTMLElement | null>(null)
const activeSection = ref<SectionId>('home')
const isScrolled = ref(false)
const scrollProgress = ref(0)
const isDark = ref(document.documentElement.classList.contains('dark'))
const activeCodeSample = ref<CodeSampleKey>('curl')
const heroStageRef = ref<HTMLElement | null>(null)
let sectionObserver: IntersectionObserver | null = null
let revealObserver: IntersectionObserver | null = null
let scrollFrame = 0
let homeMatchMedia: ReturnType<typeof gsap.matchMedia> | null = null

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
const userInitial = computed(() => authStore.user?.email?.charAt(0).toUpperCase() || 'M')
const currentYear = computed(() => new Date().getFullYear())
const modelPlazaEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))
const modelPlazaRequiresAuth = computed(() => appStore.cachedPublicSettings?.model_plaza_require_auth === true)
const showModelPlazaEntry = computed(() => modelPlazaEnabled.value && (isAuthenticated.value || !modelPlazaRequiresAuth.value))
const learningEntry = computed(() => isAuthenticated.value
  ? '/ai-learning'
  : { path: '/login', query: { redirect: '/ai-learning' } })
const apiBaseUrl = computed(() => {
  const configured = appStore.cachedPublicSettings?.api_base_url
  return typeof configured === 'string' && configured.trim() ? configured.trim().replace(/\/+$/, '') : window.location.origin
})

const navigationItems = computed(() => [
  { id: 'home' as const, label: copy.value.nav.home },
  { id: 'integrate' as const, label: copy.value.nav.integrate },
  { id: 'routing' as const, label: copy.value.nav.routing },
  { id: 'observability' as const, label: copy.value.nav.observability },
  { id: 'workflow' as const, label: copy.value.nav.workflow },
  { id: 'learning' as const, label: copy.value.nav.learning },
  { id: 'contact' as const, label: copy.value.nav.contact }
])

const codeSampleOptions: Array<{ key: CodeSampleKey; label: string }> = [
  { key: 'curl', label: 'cURL' },
  { key: 'python', label: 'Python' },
  { key: 'node', label: 'Node.js' }
]
const codeSamples = computed<Record<CodeSampleKey, string>>(() => ({
  curl: `curl ${apiBaseUrl.value}/v1/chat/completions --header "Authorization: Bearer $MODURELAY_API_KEY" --header "Content-Type: application/json" --data-raw '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hello, ModuRelay!"}]}'`,
  python: `import os

from openai import OpenAI

client = OpenAI(
    base_url="${apiBaseUrl.value}/v1",
    api_key=os.environ["MODURELAY_API_KEY"],
)
response = client.chat.completions.create(
    model="gpt-4o-mini",
    messages=[{"role": "user", "content": "Hello"}],
)`,
  node: `const client = new OpenAI({
  baseURL: "${apiBaseUrl.value}/v1",
  apiKey: process.env.MODURELAY_API_KEY,
});

const response = await client.chat.completions.create({
  model: "gpt-4o-mini",
  messages: [{ role: "user", content: "Hello" }],
});`
}))
const activeCode = computed(() => codeSamples.value[activeCodeSample.value])

function toggleTheme(event?: MouseEvent) {
  isDark.value = toggleThemeWithTransition(isDark.value, event)
}

function handleHeroPointerMove(event: PointerEvent) {
  const stage = heroStageRef.value
  if (!stage || event.pointerType === 'touch') return
  const rect = stage.getBoundingClientRect()
  const x = (event.clientX - rect.left) / Math.max(rect.width, 1) - 0.5
  const y = (event.clientY - rect.top) / Math.max(rect.height, 1) - 0.5
  stage.style.setProperty('--hero-rotate-x', `${y * -3.2}deg`)
  stage.style.setProperty('--hero-rotate-y', `${x * 4.2}deg`)
  stage.style.setProperty('--hero-light-x', `${50 + x * 34}%`)
  stage.style.setProperty('--hero-light-y', `${44 + y * 28}%`)
}

function resetHeroPointer() {
  const stage = heroStageRef.value
  if (!stage) return
  stage.style.setProperty('--hero-rotate-x', '0deg')
  stage.style.setProperty('--hero-rotate-y', '0deg')
  stage.style.setProperty('--hero-light-x', '50%')
  stage.style.setProperty('--hero-light-y', '44%')
}

function scrollToSection(id: SectionId) {
  const target = document.getElementById(id)
  if (!target) return
  target.scrollIntoView({ behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth', block: 'start' })
}

function updateScrollState() {
  cancelAnimationFrame(scrollFrame)
  scrollFrame = requestAnimationFrame(() => {
    const scrollTop = window.scrollY || document.documentElement.scrollTop
    const scrollable = Math.max(document.documentElement.scrollHeight - window.innerHeight, 1)
    isScrolled.value = scrollTop > 24
    scrollProgress.value = Math.min(100, Math.max(0, (scrollTop / scrollable) * 100))
  })
}

function initializeHomeMotion() {
  const root = pageRef.value
  if (!root) return
  const sections = Array.from(root.querySelectorAll<HTMLElement>('[data-home-section]'))
  if ('IntersectionObserver' in window) {
    sectionObserver = new IntersectionObserver((entries) => {
      const visible = entries.filter(entry => entry.isIntersecting).sort((a, b) => b.intersectionRatio - a.intersectionRatio)[0]
      if (visible?.target.id) activeSection.value = visible.target.id as SectionId
    }, { rootMargin: '-22% 0px -58% 0px', threshold: [0.05, 0.2, 0.45, 0.7] })
    sections.forEach(section => sectionObserver?.observe(section))
  }

  const hero = root.querySelectorAll<HTMLElement>('.home-hero-reveal')
  const reveals = root.querySelectorAll<HTMLElement>('.home-reveal')
  const motionQuery = window.matchMedia('(prefers-reduced-motion: reduce)')
  const supportsMediaListeners = typeof motionQuery.addEventListener === 'function' || typeof motionQuery.addListener === 'function'
  if (!supportsMediaListeners) {
    gsap.set([...hero, ...reveals], { autoAlpha: 1, y: 0 })
    return
  }

  homeMatchMedia = gsap.matchMedia()
  homeMatchMedia.add({ reduceMotion: '(prefers-reduced-motion: reduce)' }, (context) => {
    const reduced = Boolean(context.conditions?.reduceMotion)
    if (reduced) {
      gsap.set([...hero, ...reveals], { autoAlpha: 1, y: 0 })
      return
    }
    const intro = gsap.timeline({ defaults: { duration: 0.58, ease: 'power3.out' } })
    intro.fromTo(hero, { autoAlpha: 0, y: 18 }, { autoAlpha: 1, y: 0, stagger: 0.055 })
    if ('IntersectionObserver' in window) {
      revealObserver = new IntersectionObserver((entries, observer) => {
        entries.forEach((entry) => {
          if (!entry.isIntersecting) return
          gsap.to(entry.target, { autoAlpha: 1, y: 0, duration: 0.48, ease: 'power2.out' })
          observer.unobserve(entry.target)
        })
      }, { rootMargin: '0px 0px -12% 0px', threshold: 0.08 })
      reveals.forEach(element => revealObserver?.observe(element))
    } else {
      gsap.set(reveals, { autoAlpha: 1, y: 0 })
    }
    return () => revealObserver?.disconnect()
  })
}

function cleanupHomeMotion() {
  cancelAnimationFrame(scrollFrame)
  window.removeEventListener('scroll', updateScrollState)
  window.removeEventListener('resize', updateScrollState)
  sectionObserver?.disconnect()
  sectionObserver = null
  revealObserver?.disconnect()
  revealObserver = null
  homeMatchMedia?.revert()
  homeMatchMedia = null
}

async function syncHomeMode() {
  cleanupHomeMotion()
  isScrolled.value = false
  scrollProgress.value = 0
  if (hasHomeContent.value || compactHomeEnabled.value) return

  await nextTick()
  if (hasHomeContent.value || compactHomeEnabled.value) return
  initializeHomeMotion()
  updateScrollState()
  window.addEventListener('scroll', updateScrollState, { passive: true })
  window.addEventListener('resize', updateScrollState, { passive: true })
}

let homeMounted = false
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

.home-page { --home-section-border: color-mix(in srgb, var(--mr-border) 74%, transparent); position: relative; width: 100%; max-width: 100vw; min-height: 100vh; overflow: clip; color: var(--mr-text); background: var(--mr-canvas); font-family: "Noto Sans SC Variable", system-ui, sans-serif; font-synthesis: none; }
:global(.dark) .home-page { --home-section-border: color-mix(in srgb, var(--mr-border-strong) 68%, transparent); }
.home-background { position: absolute; inset: 0; overflow: hidden; pointer-events: none; }
.home-background-light, .home-background-rule { position: absolute; display: block; }
.home-background-light { width: 46%; height: 1px; opacity: 0.18; background: linear-gradient(90deg, transparent, var(--mr-primary), transparent); transform-origin: center; }
.home-background-light-one { top: 19%; right: -12%; transform: rotate(-19deg); }
.home-background-light-two { top: 31%; left: -14%; opacity: 0.12; transform: rotate(23deg); }
.home-background-rule { width: min(40vw, 560px); height: 1px; background: var(--home-section-border); opacity: 0.5; transform: rotate(-19deg); }
.home-background-rule-one { top: 25%; right: -10%; }
.home-background-rule-two { top: 67%; left: -13%; transform: rotate(23deg); }
.home-header { position: sticky; top: 0; z-index: 30; border-bottom: 1px solid transparent; background: color-mix(in srgb, var(--mr-surface) 66%, transparent); box-shadow: inset 0 -1px 0 color-mix(in srgb, var(--mr-border) 35%, transparent); backdrop-filter: blur(20px) saturate(145%); -webkit-backdrop-filter: blur(20px) saturate(145%); transition: background-color 220ms var(--ease-standard), border-color 220ms var(--ease-standard), box-shadow 220ms var(--ease-standard); }
.home-page-scrolled .home-header { border-color: var(--home-section-border); background: color-mix(in srgb, var(--mr-canvas) 94%, transparent); }
.home-scroll-progress { position: absolute; top: 0; left: 0; z-index: 1; height: 2px; background: var(--mr-primary); transition: width 120ms linear; }
.home-navbar { display: flex; min-height: 72px; align-items: center; gap: 28px; width: min(100% - 48px, 1240px); margin: 0 auto; }
.home-brand { display: inline-flex; min-width: max-content; align-items: center; gap: 10px; color: var(--mr-text); font: inherit; font-size: 15px; font-weight: 650; }
.home-brand-name { overflow: hidden; max-width: 180px; text-overflow: ellipsis; white-space: nowrap; }
.brand-mark { display: grid; width: 30px; height: 30px; flex: 0 0 auto; place-items: center; overflow: hidden; border: 1px solid rgba(99, 102, 241, 0.28); border-radius: 8px; background: var(--mr-primary); box-shadow: 0 6px 16px rgba(79, 70, 229, 0.18); }
.brand-mark-large { width: 64px; height: 64px; border-radius: 16px; }
.brand-mark img { width: 100%; height: 100%; object-fit: contain; }
.home-nav-links { display: flex; min-width: 0; align-items: center; gap: 4px; margin-left: auto; }
.home-nav-link { position: relative; min-height: 40px; padding: 0 10px; color: var(--mr-text-muted); font-size: 12px; font-weight: 550; white-space: nowrap; transition: color 160ms ease; }
.home-nav-link::after { content: ''; position: absolute; right: 10px; bottom: 4px; left: 10px; height: 2px; border-radius: 2px; background: var(--mr-primary); transform: scaleX(0); transition: transform 160ms ease; }
.home-nav-link:hover, .home-nav-link.is-active { color: var(--mr-text); }
.home-nav-link.is-active::after { transform: scaleX(1); }
.home-nav-actions { display: flex; align-items: center; gap: 4px; }
.home-icon-button, .home-quiet-link { display: inline-flex; min-width: 36px; min-height: 36px; align-items: center; justify-content: center; gap: 6px; padding: 0 9px; border: 1px solid transparent; border-radius: 8px; color: var(--mr-text-muted); font-size: 12px; transition: color 180ms var(--ease-standard), background-color 180ms var(--ease-standard), border-color 180ms var(--ease-standard), transform 180ms var(--ease-standard); }
.home-icon-button:hover, .home-quiet-link:hover { border-color: var(--mr-border); color: var(--mr-text); background: color-mix(in srgb, var(--mr-surface) 76%, transparent); transform: translateY(-1px); }
.home-solid-button, .home-primary-button { position: relative; display: inline-flex; min-height: 38px; align-items: center; justify-content: center; gap: 7px; overflow: hidden; padding: 0 14px; border-radius: 8px; color: #fff; background: var(--mr-primary); box-shadow: 0 8px 18px color-mix(in srgb, var(--mr-primary) 20%, transparent); font-size: 12px; font-weight: 650; transition: background-color 180ms var(--ease-standard), box-shadow 180ms var(--ease-standard), transform 180ms var(--ease-standard); }
.home-solid-button::after, .home-primary-button::after { position: absolute; top: -30%; left: -42%; width: 26%; height: 160%; background: rgba(255, 255, 255, 0.2); content: ''; transform: skewX(-18deg) translateX(-180%); transition: transform 520ms var(--ease-enter); }
.home-solid-button:hover::after, .home-primary-button:hover::after { transform: skewX(-18deg) translateX(620%); }
.home-solid-button:hover, .home-primary-button:hover { background: var(--mr-primary-strong); box-shadow: 0 10px 22px rgba(79, 70, 229, 0.23); transform: translateY(-1px); }
.home-user-avatar { display: grid; width: 21px; height: 21px; place-items: center; border-radius: 6px; color: var(--mr-primary-strong); background: #fff; font-size: 10px; font-weight: 700; }
.home-container { width: min(100% - 48px, 1180px); margin: 0 auto; }
.home-section { position: relative; z-index: 1; scroll-margin-top: 90px; }
.home-hero-section { min-height: min(860px, calc(100vh - 72px)); padding: 86px 0 92px; }
.home-hero-layout { display: grid; min-height: 620px; grid-template-columns: minmax(0, 0.92fr) minmax(0, 1.08fr); align-items: center; gap: 52px; }
.home-hero-copy { max-width: 560px; }
.home-overline, .home-section-index { color: var(--mr-primary); font-size: 11px; font-weight: 700; letter-spacing: 0.14em; }
.home-hero-title { max-width: 560px; margin: 18px 0 0; font-size: clamp(48px, 5.4vw, 72px); font-weight: 670; letter-spacing: -0.055em; line-height: 1.03; }
.home-hero-title span { display: block; }
.home-hero-title-accent { color: var(--mr-primary); text-shadow: 0 10px 30px color-mix(in srgb, var(--mr-primary) 18%, transparent); }
.home-hero-subtitle { max-width: 520px; margin: 26px 0 0; color: var(--mr-text); font-size: clamp(18px, 2vw, 23px); font-weight: 520; line-height: 1.42; }
.home-hero-description { max-width: 510px; margin: 14px 0 0; color: var(--mr-text-muted); font-size: 14px; line-height: 1.75; }
.home-hero-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; margin-top: 28px; }
.home-secondary-button { display: inline-flex; min-height: 38px; align-items: center; justify-content: center; gap: 7px; padding: 0 14px; border: 1px solid var(--mr-border-strong); border-radius: 8px; color: var(--mr-text); background: var(--mr-surface); font-size: 12px; font-weight: 600; transition: background-color 160ms ease, border-color 160ms ease, transform 160ms ease; }
.home-secondary-button:hover { border-color: var(--mr-primary); background: var(--mr-surface-subtle); transform: translateY(-1px); }
.home-hero-facts { display: flex; flex-wrap: wrap; gap: 16px; margin-top: 34px; color: var(--mr-text-subtle); font-size: 11px; }
.home-hero-facts span { display: inline-flex; align-items: center; gap: 7px; }
.home-hero-facts i, .home-capability-grid i { width: 5px; height: 5px; border-radius: 50%; background: var(--mr-success); }
.home-hero-scene { min-width: 0; perspective: 1500px; }
.home-hero-stage-frame { --hero-rotate-x: 0deg; --hero-rotate-y: 0deg; --hero-light-x: 50%; --hero-light-y: 44%; position: relative; width: 100%; max-width: 100%; min-width: 0; min-height: 584px; overflow: hidden; border: 1px solid color-mix(in srgb, var(--mr-border-strong) 72%, var(--mr-primary)); border-radius: 26px; background: color-mix(in srgb, var(--mr-surface) 42%, transparent); box-shadow: inset 0 1px 0 var(--glass-highlight), 0 28px 70px color-mix(in srgb, var(--mr-primary) 12%, transparent), var(--shadow-overlay); backdrop-filter: blur(18px) saturate(138%); -webkit-backdrop-filter: blur(18px) saturate(138%); transform: perspective(1500px) rotateX(var(--hero-rotate-x)) rotateY(var(--hero-rotate-y)); transform-style: preserve-3d; transition: transform 560ms var(--ease-enter), border-color 240ms var(--ease-standard), box-shadow 240ms var(--ease-standard); }
.home-hero-stage-frame::before { position: absolute; inset: 0; z-index: 1; border: 1px solid color-mix(in srgb, var(--mr-primary) 16%, transparent); border-radius: inherit; content: ''; pointer-events: none; transform: translateZ(12px); }
.home-hero-stage-frame::after { position: absolute; top: 14%; right: 13%; bottom: 13%; left: 13%; border-right: 1px solid color-mix(in srgb, var(--mr-secondary) 18%, transparent); border-left: 1px solid color-mix(in srgb, var(--mr-primary) 18%, transparent); content: ''; pointer-events: none; transform: translateZ(4px); }
.home-hero-stage-frame:hover { border-color: color-mix(in srgb, var(--mr-primary) 46%, var(--mr-border-strong)); box-shadow: inset 0 1px 0 var(--glass-highlight), 0 34px 82px color-mix(in srgb, var(--mr-primary) 18%, transparent), var(--shadow-overlay); }
.home-stage-topline, .home-stage-footline { position: absolute; right: 24px; left: 24px; z-index: 5; display: flex; align-items: center; justify-content: space-between; gap: 14px; color: var(--mr-text-subtle); font: 700 9px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.12em; pointer-events: none; }
.home-stage-topline { top: 22px; }
.home-stage-footline { bottom: 20px; color: color-mix(in srgb, var(--mr-text-subtle) 78%, transparent); font-size: 8px; }
.home-stage-status { display: inline-flex; align-items: center; gap: 7px; color: var(--mr-success); }
.home-stage-status i { width: 5px; height: 5px; border-radius: 50%; background: currentColor; box-shadow: 0 0 0 4px color-mix(in srgb, currentColor 12%, transparent); }
.home-stage-corner { position: absolute; z-index: 5; width: 22px; height: 22px; border-color: color-mix(in srgb, var(--mr-primary) 52%, transparent); border-style: solid; pointer-events: none; }
.home-stage-corner-tl { top: 18px; left: 18px; border-width: 1px 0 0 1px; }
.home-stage-corner-tr { top: 18px; right: 18px; border-width: 1px 1px 0 0; }
.home-stage-corner-bl { bottom: 18px; left: 18px; border-width: 0 0 1px 1px; }
.home-stage-corner-br { right: 18px; bottom: 18px; border-width: 0 1px 1px 0; }
.home-stage-scanline { position: absolute; top: 18%; left: 10%; z-index: 4; width: 80%; height: 1px; opacity: 0.42; background: linear-gradient(90deg, transparent, var(--mr-secondary), transparent); box-shadow: 0 0 18px color-mix(in srgb, var(--mr-secondary) 30%, transparent); pointer-events: none; animation: home-stage-scan 5.6s var(--ease-standard) infinite; }
.home-stage-footline span:last-child { color: var(--mr-text-subtle); }
.home-hero-stage-frame :deep(.hero-orbit-stage) { min-height: 584px; }
.home-capability-strip { position: relative; z-index: 2; border-top: 1px solid var(--home-section-border); border-bottom: 1px solid var(--home-section-border); background: var(--mr-surface); }
.home-capability-grid { display: grid; grid-template-columns: repeat(4, 1fr); }
.home-capability-grid span { display: inline-flex; min-height: 58px; align-items: center; justify-content: center; gap: 9px; border-left: 1px solid var(--home-section-border); color: var(--mr-text-muted); font-size: 11px; font-weight: 600; }
.home-capability-grid span:last-child { border-right: 1px solid var(--home-section-border); }
.home-section-light { background: var(--mr-canvas); }
.home-section-contrast { border-top: 1px solid var(--home-section-border); border-bottom: 1px solid var(--home-section-border); background: var(--mr-surface); }
.home-two-column { display: grid; grid-template-columns: minmax(0, 0.85fr) minmax(0, 1.15fr); align-items: center; gap: 76px; }
.home-section-light .home-two-column { padding-top: 130px; padding-bottom: 130px; }
.home-section-copy { max-width: 500px; }
.home-section-copy h2, .home-section-heading h2, .home-final-panel h2 { margin: 14px 0 0; color: var(--mr-text); font-size: clamp(30px, 4vw, 52px); font-weight: 650; letter-spacing: -0.045em; line-height: 1.08; }
.home-section-copy > p, .home-section-heading > p, .home-final-panel p { margin: 18px 0 0; color: var(--mr-text-muted); font-size: 14px; line-height: 1.8; }
.home-endpoint-list { margin-top: 30px; border-top: 1px solid var(--home-section-border); }
.home-endpoint-row { display: grid; grid-template-columns: 42px minmax(0, 1fr); gap: 10px; padding: 13px 0; border-bottom: 1px solid var(--home-section-border); }
.home-endpoint-row code { overflow: hidden; color: var(--mr-text); font: 12px ui-monospace, SFMono-Regular, Menlo, monospace; text-overflow: ellipsis; white-space: nowrap; }
.home-endpoint-row > span:last-child { grid-column: 2; color: var(--mr-text-subtle); font-size: 11px; }
.endpoint-method { color: var(--mr-primary); font: 700 10px ui-monospace, SFMono-Regular, Menlo, monospace; }
.home-code-panel { overflow: hidden; border: 1px solid color-mix(in srgb, var(--mr-border-strong) 80%, transparent); border-radius: 16px; background: color-mix(in srgb, var(--mr-surface) 68%, transparent); box-shadow: inset 0 1px 0 var(--glass-highlight), 0 24px 58px color-mix(in srgb, var(--mr-primary) 9%, transparent); backdrop-filter: blur(16px) saturate(125%); -webkit-backdrop-filter: blur(16px) saturate(125%); }
:global(.dark) .home-code-panel { box-shadow: inset 0 1px 0 var(--glass-highlight), 0 26px 66px rgba(0, 0, 0, 0.3); }
.home-code-header, .preview-window-bar { display: flex; min-height: 52px; align-items: center; justify-content: space-between; gap: 14px; padding: 0 18px; border-bottom: 1px solid var(--home-section-border); }
.home-window-dots { display: flex; gap: 5px; }
.home-window-dots i { width: 7px; height: 7px; border-radius: 50%; background: var(--mr-border-strong); }
.home-code-tabs { display: flex; gap: 4px; }
.home-code-tabs button { min-height: 30px; padding: 0 9px; border-radius: 6px; color: var(--mr-text-subtle); font-size: 11px; transition: color 160ms ease, background-color 160ms ease; }
.home-code-tabs button.is-active, .home-code-tabs button:hover { color: var(--mr-text); background: var(--mr-surface-subtle); }
.home-code-panel pre { min-height: 290px; margin: 0; padding: 24px; overflow: auto; color: var(--mr-text); background: var(--mr-surface-subtle); font: 12px/1.85 ui-monospace, SFMono-Regular, Menlo, monospace; }
.home-code-swap-enter-active, .home-code-swap-leave-active { transition: opacity 220ms var(--ease-standard), transform 220ms var(--ease-standard); }
.home-code-swap-enter-from { opacity: 0; transform: translateY(8px); }
.home-code-swap-leave-to { opacity: 0; transform: translateY(-8px); }
.home-code-footer { display: flex; min-height: 42px; align-items: center; gap: 8px; padding: 0 18px; color: var(--mr-text-subtle); font-size: 10px; }
.home-status-dot { display: inline-block; width: 6px; height: 6px; flex: 0 0 auto; border-radius: 50%; background: var(--mr-success); }
.home-section-heading { max-width: 610px; padding: 100px 0 48px; }
.routing-visual { display: grid; grid-template-columns: minmax(160px, 1fr) 80px minmax(160px, 0.78fr) 80px minmax(220px, 1fr); align-items: center; padding: 34px; border: 1px solid var(--mr-border); border-radius: 16px; background: color-mix(in srgb, var(--mr-surface-subtle) 76%, transparent); box-shadow: inset 0 1px 0 var(--glass-highlight); backdrop-filter: blur(14px) saturate(120%); -webkit-backdrop-filter: blur(14px) saturate(120%); }
.routing-node, .routing-core-node { min-width: 0; padding: 18px; border: 1px solid var(--mr-border); border-radius: 10px; background: color-mix(in srgb, var(--mr-surface) 78%, transparent); box-shadow: inset 0 1px 0 var(--glass-highlight); transition: border-color 220ms var(--ease-standard), transform 220ms var(--ease-standard), background-color 220ms var(--ease-standard); }
.routing-node:hover, .routing-core-node:hover { border-color: var(--mr-primary-border); background: color-mix(in srgb, var(--mr-surface-raised) 84%, transparent); transform: translateY(-2px); }
.routing-node { display: flex; flex-direction: column; gap: 8px; }
.routing-node-icon { display: grid; width: 34px; height: 34px; place-items: center; border-radius: 8px; color: var(--mr-primary); background: var(--mr-primary); background: color-mix(in srgb, var(--mr-primary) 12%, transparent); }
.routing-node strong, .routing-core-node strong { color: var(--mr-text); font-size: 13px; }
.routing-node code { overflow: hidden; color: var(--mr-text-subtle); font: 10px ui-monospace, SFMono-Regular, Menlo, monospace; text-overflow: ellipsis; white-space: nowrap; }
.routing-connector { position: relative; height: 1px; background: var(--mr-border-strong); }
.routing-connector span { position: absolute; top: -3px; right: 0; width: 7px; height: 7px; border-top: 1px solid var(--mr-primary); border-right: 1px solid var(--mr-primary); transform: rotate(45deg); }
.routing-core-node { display: grid; justify-items: center; gap: 9px; text-align: center; }
.routing-core-mark { display: grid; width: 56px; height: 56px; place-items: center; border: 1px solid var(--mr-primary); border-radius: 14px; color: #fff; background: var(--mr-primary); box-shadow: 0 10px 24px rgba(79, 70, 229, 0.25); font-size: 22px; font-weight: 700; }
.routing-core-node small { color: var(--mr-text-subtle); font: 10px ui-monospace, SFMono-Regular, Menlo, monospace; }
.routing-node-label { color: var(--mr-text-subtle); font-size: 10px; font-weight: 700; letter-spacing: 0.12em; text-transform: uppercase; }
.routing-route-row { display: grid; grid-template-columns: 8px minmax(0, 1fr); gap: 8px; align-items: center; padding-top: 11px; }
.routing-route-row strong { overflow: hidden; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.routing-route-row small { grid-column: 2; color: var(--mr-text-subtle); font-size: 10px; }
.routing-feature-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 1px; margin: 1px 0 100px; border: 1px solid var(--mr-border); background: var(--mr-border); }
.routing-feature { display: grid; grid-template-columns: 34px 1fr; gap: 12px; min-height: 160px; padding: 22px; background: var(--mr-surface); }
.routing-feature-number { color: var(--mr-primary); font: 700 11px ui-monospace, SFMono-Regular, Menlo, monospace; }
.routing-feature h3, .home-workflow-step h3 { margin: 0; color: var(--mr-text); font-size: 14px; font-weight: 650; }
.routing-feature p, .home-workflow-step p { margin: 9px 0 0; color: var(--mr-text-muted); font-size: 12px; line-height: 1.7; }
.home-observability-layout { grid-template-columns: minmax(0, 1.15fr) minmax(0, 0.85fr); }
.home-product-preview { overflow: hidden; border: 1px solid color-mix(in srgb, var(--mr-border-strong) 82%, transparent); border-radius: 16px; background: color-mix(in srgb, var(--mr-surface) 72%, transparent); box-shadow: inset 0 1px 0 var(--glass-highlight), 0 24px 58px color-mix(in srgb, var(--mr-primary) 9%, transparent); backdrop-filter: blur(16px) saturate(125%); -webkit-backdrop-filter: blur(16px) saturate(125%); }
.preview-window-bar { color: var(--mr-text-subtle); font: 700 10px ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.08em; }
.preview-live { display: inline-flex; align-items: center; gap: 6px; color: var(--mr-success); letter-spacing: 0; }
.preview-body { display: grid; grid-template-columns: 48px 1fr; min-height: 380px; }
.preview-sidebar { display: flex; flex-direction: column; align-items: center; gap: 22px; padding: 18px 0; border-right: 1px solid var(--home-section-border); background: var(--mr-surface-subtle); }
.preview-sidebar i { width: 14px; height: 2px; border-radius: 2px; background: var(--mr-border-strong); }
.preview-logo { display: grid; width: 24px; height: 24px; place-items: center; border-radius: 6px; color: #fff; background: var(--mr-primary); font-size: 11px; font-weight: 700; }
.preview-main { min-width: 0; padding: 22px; }
.preview-title-row { display: flex; justify-content: space-between; gap: 10px; color: var(--mr-text); font-size: 12px; }
.preview-title-row span { color: var(--mr-text-subtle); font-size: 10px; }
.preview-kpis { display: grid; grid-template-columns: repeat(2, 1fr); gap: 10px; margin-top: 18px; }
.preview-kpis > div { padding: 12px; border: 1px solid var(--mr-border); border-radius: 8px; }
.preview-kpis span, .preview-kpis strong, .preview-kpis em { display: block; }
.preview-kpis span { color: var(--mr-text-subtle); font-size: 10px; }
.preview-kpis strong { margin-top: 7px; color: var(--mr-text); font-size: 22px; font-weight: 620; }
.preview-kpis em { margin-top: 5px; color: var(--mr-success); font-size: 10px; font-style: normal; }
.preview-kpis em.is-cyan { color: var(--mr-secondary); }
.preview-chart { position: relative; height: 122px; margin-top: 12px; overflow: hidden; border-bottom: 1px solid var(--mr-border); background: linear-gradient(to bottom, color-mix(in srgb, var(--mr-primary) 7%, transparent), transparent); }
.preview-chart svg { position: absolute; inset: 0; width: 100%; height: 100%; }
.preview-chart path { fill: none; stroke: var(--mr-primary); stroke-width: 2; vector-effect: non-scaling-stroke; }
.preview-chart i { position: absolute; bottom: 0; width: 1px; height: 100%; background: var(--mr-border); opacity: 0.6; }
.preview-chart i:nth-child(1) { left: 8%; }.preview-chart i:nth-child(2) { left: 16%; }.preview-chart i:nth-child(3) { left: 24%; }.preview-chart i:nth-child(4) { left: 32%; }.preview-chart i:nth-child(5) { left: 40%; }.preview-chart i:nth-child(6) { left: 48%; }.preview-chart i:nth-child(7) { left: 56%; }.preview-chart i:nth-child(8) { left: 64%; }.preview-chart i:nth-child(9) { left: 72%; }.preview-chart i:nth-child(10) { left: 80%; }.preview-chart i:nth-child(11) { left: 88%; }.preview-chart i:nth-child(12) { left: 96%; }
.preview-table { margin-top: 14px; }
.preview-table-row { display: grid; grid-template-columns: minmax(0, 1fr) 8px 52px 45px; gap: 8px; align-items: center; padding: 7px 0; border-bottom: 1px solid var(--mr-border); color: var(--mr-text-muted); font-size: 10px; }
.preview-table-row span:first-child { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.preview-table-row strong { color: var(--mr-success); font-size: 10px; font-weight: 600; }.preview-table-row small { color: var(--mr-text-subtle); text-align: right; }
.home-observability-list { margin-top: 30px; border-top: 1px solid var(--home-section-border); }
.home-observability-list > div { display: grid; grid-template-columns: 34px 1fr; gap: 12px; padding: 15px 0; border-bottom: 1px solid var(--home-section-border); }
.home-list-icon { display: grid; width: 30px; height: 30px; place-items: center; border-radius: 8px; color: var(--mr-primary); background: color-mix(in srgb, var(--mr-primary) 10%, transparent); }
.home-observability-list strong { color: var(--mr-text); font-size: 13px; }.home-observability-list p { margin: 4px 0 0; color: var(--mr-text-muted); font-size: 11px; line-height: 1.6; }
.home-workflow-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 1px; padding-bottom: 100px; background: var(--mr-border); }
.home-workflow-step { position: relative; min-height: 190px; padding: 22px; background: var(--mr-surface); }
.workflow-step-top { display: flex; align-items: center; justify-content: space-between; color: var(--mr-primary); }.workflow-step-top > span { font: 700 11px ui-monospace, SFMono-Regular, Menlo, monospace; }.workflow-step-top > svg { opacity: 0.85; }
.home-workflow-step h3 { margin-top: 36px; }.workflow-arrow { position: absolute; top: 31px; right: -15px; z-index: 2; display: grid; width: 30px; height: 30px; place-items: center; border: 1px solid var(--mr-border-strong); border-radius: 50%; color: var(--mr-primary); background: var(--mr-surface); }
.home-page::before { position: absolute; inset: 0; z-index: 0; background-image: linear-gradient(color-mix(in srgb, var(--mr-border) 22%, transparent) 1px, transparent 1px), linear-gradient(90deg, color-mix(in srgb, var(--mr-border) 22%, transparent) 1px, transparent 1px); background-size: 46px 46px; content: ''; opacity: 0.2; pointer-events: none; mask-image: linear-gradient(to bottom, black 0%, transparent 42%); }
.home-page::after { position: absolute; top: 86px; right: 7%; z-index: 0; width: 280px; height: 280px; border-radius: 50%; background: radial-gradient(circle at 34% 28%, color-mix(in srgb, var(--mr-secondary) 22%, transparent), transparent 66%); content: ''; filter: blur(3px); opacity: 0.55; pointer-events: none; animation: home-background-float 13s ease-in-out infinite alternate; }
.home-hero-title-accent { color: transparent; background: linear-gradient(105deg, var(--mr-primary) 8%, var(--mr-secondary) 76%, #ec4899); -webkit-background-clip: text; background-clip: text; }
.home-solid-button, .home-primary-button { border: 1px solid color-mix(in srgb, var(--mr-primary) 58%, transparent); background: linear-gradient(135deg, var(--mr-primary), var(--mr-secondary)); background-size: 150% 150%; box-shadow: 0 11px 24px color-mix(in srgb, var(--mr-primary) 24%, transparent), inset 0 1px 0 rgba(255, 255, 255, 0.28); transition: background-position 260ms ease, box-shadow 180ms var(--ease-standard), transform 180ms var(--ease-standard), filter 180ms ease; }
.home-solid-button:hover, .home-primary-button:hover { background: linear-gradient(135deg, var(--mr-primary), var(--mr-secondary)); background-position: 100% 0; filter: saturate(1.08); }
.home-learning-section { position: relative; overflow: hidden; border-top: 1px solid var(--home-section-border); background: radial-gradient(circle at 86% 44%, color-mix(in srgb, var(--mr-primary) 9%, transparent), transparent 33rem), color-mix(in srgb, var(--mr-canvas) 92%, var(--mr-surface)); }
.home-learning-section::before { position: absolute; inset: 0; background: radial-gradient(circle at 12% 72%, color-mix(in srgb, var(--mr-secondary) 7%, transparent), transparent 25rem); content: ''; pointer-events: none; }
.home-learning-layout { position: relative; z-index: 1; display: grid; grid-template-columns: minmax(0, 0.82fr) minmax(0, 1.18fr); align-items: center; gap: 68px; padding: 122px 0; }
.home-learning-layout .home-section-copy { max-width: 500px; }
.home-learning-layout .home-section-copy h2 { max-width: 500px; }
.home-learning-layout .home-section-copy > p { max-width: 470px; }
.home-learning-points { display: grid; gap: 11px; margin-top: 26px; color: var(--mr-text-muted); font-size: 12px; }
.home-learning-points span { display: flex; align-items: center; gap: 10px; }
.home-learning-points i { display: inline-block; width: 7px; height: 7px; flex: 0 0 auto; border-radius: 50%; background: linear-gradient(135deg, var(--mr-primary), var(--mr-secondary)); box-shadow: 0 0 0 4px color-mix(in srgb, var(--mr-primary) 10%, transparent); }
.home-learning-cta { margin-top: 28px; }
.home-learning-preview { position: relative; min-height: 452px; overflow: hidden; border: 1px solid color-mix(in srgb, var(--mr-primary) 28%, var(--mr-border-strong)); border-radius: 28px; background: radial-gradient(circle at 50% 45%, color-mix(in srgb, var(--mr-primary) 11%, transparent), transparent 42%), color-mix(in srgb, var(--mr-surface) 64%, transparent); box-shadow: inset 0 1px 0 var(--glass-highlight), 0 28px 70px color-mix(in srgb, var(--mr-primary) 12%, transparent); backdrop-filter: blur(16px) saturate(130%); -webkit-backdrop-filter: blur(16px) saturate(130%); }
.home-learning-preview::before { position: absolute; inset: 14px; border: 1px solid color-mix(in srgb, var(--mr-primary) 18%, transparent); border-radius: 20px; content: ''; pointer-events: none; }
.home-learning-preview-topline, .home-learning-preview-footer { position: absolute; right: 27px; left: 27px; z-index: 3; display: flex; align-items: center; justify-content: space-between; gap: 16px; color: var(--mr-text-subtle); font: 700 9px ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.12em; }
.home-learning-preview-topline { top: 25px; }
.home-learning-preview-topline span:last-child { display: inline-flex; align-items: center; gap: 7px; color: var(--mr-success); }
.home-learning-preview-topline i { display: inline-block; width: 6px; height: 6px; border-radius: 50%; background: var(--mr-success); box-shadow: 0 0 0 4px color-mix(in srgb, var(--mr-success) 12%, transparent); }
.home-learning-preview-footer { bottom: 24px; color: var(--mr-text-subtle); letter-spacing: 0.03em; }
.home-learning-preview-footer a { display: grid; width: 28px; height: 28px; place-items: center; border: 1px solid var(--home-section-border); border-radius: 50%; color: var(--mr-primary); background: color-mix(in srgb, var(--mr-surface-raised) 70%, transparent); transition: transform 180ms ease, border-color 180ms ease, background-color 180ms ease; }
.home-learning-preview-footer a:hover { border-color: var(--mr-primary); background: var(--mr-surface-raised); transform: translateY(-2px); }
.home-learning-orbit { position: relative; width: min(100%, 520px); height: 360px; margin: 58px auto 38px; perspective: 850px; transform-style: preserve-3d; }
.home-learning-orbit::before { position: absolute; top: 50%; left: 50%; width: 240px; height: 240px; border-radius: 50%; background: radial-gradient(circle, color-mix(in srgb, var(--mr-primary) 20%, transparent), transparent 69%); content: ''; filter: blur(12px); transform: translate(-50%, -50%); animation: home-learning-pulse 4.8s ease-in-out infinite; }
.home-learning-orbit-ring { position: absolute; top: 50%; left: 50%; width: 320px; height: 170px; border: 1px solid color-mix(in srgb, var(--mr-primary) 36%, transparent); border-radius: 50%; transform: translate(-50%, -50%) rotateX(68deg) rotateZ(-18deg); animation: home-learning-orbit-spin 13s linear infinite; }
.home-learning-orbit-ring-two { width: 240px; height: 300px; border-color: color-mix(in srgb, var(--mr-secondary) 32%, transparent); transform: translate(-50%, -50%) rotateY(68deg) rotateZ(24deg); animation-direction: reverse; animation-duration: 17s; }
.home-learning-core { position: absolute; top: 50%; left: 50%; z-index: 2; display: grid; width: 132px; height: 132px; place-content: center; justify-items: center; border: 1px solid color-mix(in srgb, var(--mr-primary) 62%, transparent); border-radius: 50%; color: #fff; background: linear-gradient(145deg, color-mix(in srgb, var(--mr-primary) 84%, #111827), color-mix(in srgb, var(--mr-secondary) 62%, #111827)); box-shadow: 0 0 0 13px color-mix(in srgb, var(--mr-primary) 7%, transparent), 0 18px 35px rgba(17, 24, 39, 0.24), inset 0 2px 0 rgba(255, 255, 255, 0.42); transform: translate(-50%, -50%) translateZ(54px); }
.home-learning-core::before { position: absolute; inset: -24px; border: 1px solid color-mix(in srgb, var(--mr-secondary) 25%, transparent); border-radius: inherit; content: ''; animation: home-learning-core-ring 9s linear infinite; }
.home-learning-core strong, .home-learning-core small { position: relative; z-index: 1; }
.home-learning-core strong { font-size: 38px; font-weight: 800; letter-spacing: -0.1em; }
.home-learning-core small { margin-top: 3px; color: rgba(255, 255, 255, 0.72); font: 700 8px ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.12em; }
.home-learning-node { position: absolute; z-index: 4; display: flex; align-items: center; gap: 9px; min-width: 135px; padding: 10px 12px 10px 10px; border: 1px solid color-mix(in srgb, var(--mr-primary) 30%, var(--mr-border-strong)); border-radius: 17px; color: var(--mr-text); background: linear-gradient(145deg, color-mix(in srgb, var(--mr-surface-raised) 86%, transparent), color-mix(in srgb, var(--mr-primary) 9%, transparent)); box-shadow: inset 0 1px 0 var(--glass-highlight), 0 14px 28px rgba(17, 24, 39, 0.12); backdrop-filter: blur(11px) saturate(135%); -webkit-backdrop-filter: blur(11px) saturate(135%); animation: home-learning-node-float 5.8s ease-in-out infinite; }
.home-learning-node:hover { border-color: color-mix(in srgb, var(--mr-secondary) 60%, var(--mr-border-strong)); box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.45), 0 18px 34px color-mix(in srgb, var(--mr-primary) 18%, transparent); }
.home-learning-node-icon { display: grid; width: 28px; height: 28px; flex: 0 0 auto; place-items: center; border-radius: 10px; color: var(--mr-primary); background: color-mix(in srgb, var(--mr-primary) 12%, transparent); }
.home-learning-node > span:last-child { display: grid; gap: 3px; }
.home-learning-node strong { font-size: 11px; font-weight: 750; }
.home-learning-node small { color: var(--mr-text-subtle); font: 700 8px ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.05em; text-transform: uppercase; }
.home-learning-node-1 { top: 20%; left: 4%; animation-delay: -1s; }
.home-learning-node-2 { top: 16%; right: 3%; animation-delay: -3.2s; }
.home-learning-node-3 { bottom: 12%; left: 5%; animation-delay: -4.3s; }
.home-learning-node-4 { right: 2%; bottom: 15%; animation-delay: -2.1s; }
.home-final-section { padding: 100px 0; }.home-final-panel { display: flex; align-items: center; justify-content: space-between; gap: 48px; padding: 44px; border: 1px solid color-mix(in srgb, var(--mr-border-strong) 82%, transparent); border-radius: 16px; background: color-mix(in srgb, var(--mr-surface) 76%, transparent); box-shadow: inset 0 1px 0 var(--glass-highlight), 0 24px 56px color-mix(in srgb, var(--mr-primary) 8%, transparent); backdrop-filter: blur(16px) saturate(125%); -webkit-backdrop-filter: blur(16px) saturate(125%); }.home-final-panel > div:first-child { max-width: 670px; }.home-final-actions { display: flex; flex-wrap: wrap; gap: 9px; flex: 0 0 auto; }
.home-footer { position: relative; z-index: 1; border-top: 1px solid var(--home-section-border); background: var(--mr-surface); }.home-footer-grid { display: grid; grid-template-columns: 1.45fr repeat(3, 0.7fr); gap: 44px; padding: 52px 0 44px; }.home-brand-line { display: flex; align-items: center; gap: 10px; color: var(--mr-text); font-size: 15px; }.home-footer-brand p { max-width: 320px; margin: 14px 0 0; color: var(--mr-text-muted); font-size: 11px; line-height: 1.75; }.home-footer-links { display: flex; flex-direction: column; align-items: flex-start; gap: 10px; }.home-footer-links strong { margin-bottom: 4px; color: var(--mr-text); font-size: 12px; }.home-footer-links a, .home-footer-links button { color: var(--mr-text-muted); font-size: 11px; transition: color 160ms ease; }.home-footer-links a:hover, .home-footer-links button:hover { color: var(--mr-primary); }.home-footer-bottom { display: flex; justify-content: space-between; gap: 20px; padding: 18px 0 22px; border-top: 1px solid var(--home-section-border); color: var(--mr-text-subtle); font-size: 10px; }
.back-to-top { position: fixed; right: 22px; bottom: 22px; z-index: 20; display: grid; width: 40px; height: 40px; place-items: center; border: 1px solid var(--mr-border-strong); border-radius: 8px; color: var(--mr-primary); background: var(--mr-surface); box-shadow: 0 10px 24px rgba(31, 41, 55, 0.12); transition: transform 160ms ease, background-color 160ms ease; }.back-to-top:hover { background: var(--mr-surface-subtle); transform: translateY(-2px); }
.home-reveal, .home-hero-reveal { will-change: transform, opacity; }

@keyframes home-stage-scan { 0%, 18% { transform: translateY(0); opacity: 0; } 28% { opacity: 0.42; } 82% { opacity: 0.42; } 94%, 100% { transform: translateY(350px); opacity: 0; } }
@keyframes home-background-float { from { transform: translate3d(0, 0, 0) scale(0.96); } to { transform: translate3d(-22px, 24px, 0) scale(1.06); } }
@keyframes home-learning-pulse { 0%, 100% { opacity: 0.42; transform: translate(-50%, -50%) scale(0.92); } 50% { opacity: 0.78; transform: translate(-50%, -50%) scale(1.08); } }
@keyframes home-learning-orbit-spin { to { transform: translate(-50%, -50%) rotateX(68deg) rotateZ(342deg); } }
@keyframes home-learning-core-ring { to { transform: rotateZ(360deg) scale(1.04); } }
@keyframes home-learning-node-float { 0%, 100% { transform: translate3d(0, 0, 0); } 50% { transform: translate3d(0, -7px, 0); } }

@media (max-width: 1080px) { .home-navbar { gap: 14px; }.home-nav-links { order: 3; width: 100%; overflow-x: auto; margin-left: 0; scrollbar-width: none; }.home-nav-links::-webkit-scrollbar { display: none; }.home-navbar { flex-wrap: wrap; padding: 9px 0; }.home-hero-section { padding-top: 54px; }.home-hero-layout { grid-template-columns: minmax(0, 0.9fr) minmax(0, 1.1fr); gap: 20px; }.routing-visual { grid-template-columns: 1fr 42px 0.9fr 42px 1.2fr; padding: 20px; } }
@media (max-width: 820px) { .home-hero-layout, .home-two-column { grid-template-columns: 1fr; }.home-hero-layout { min-height: auto; }.home-hero-copy { max-width: 680px; }.home-hero-scene { width: min(100%, 720px); margin: 0 auto; }.home-hero-stage-frame, .home-hero-stage-frame :deep(.hero-orbit-stage) { min-height: 520px; }.home-section-light .home-two-column { padding-top: 76px; padding-bottom: 76px; }.routing-visual { grid-template-columns: 1fr; gap: 20px; }.routing-connector { width: 1px; height: 26px; margin: 0 auto; }.routing-connector span { top: auto; right: -3px; bottom: 0; transform: rotate(135deg); }.routing-feature-grid { grid-template-columns: 1fr; margin-bottom: 76px; }.home-observability-layout { display: flex; flex-direction: column-reverse; }.home-workflow-grid { grid-template-columns: repeat(2, 1fr); padding-bottom: 76px; }.home-workflow-step:nth-child(2) .workflow-arrow { display: none; }.home-footer-grid { grid-template-columns: 1.2fr repeat(2, 1fr); }.home-footer-brand { grid-column: 1 / -1; } }
@media (max-width: 620px) { .home-container, .home-navbar, .compact-home-nav { width: calc(100% - 28px); max-width: 1180px; }.home-brand-name, .home-doc-link { display: none; }.home-nav-actions { margin-left: auto; }.home-quiet-link span { display: none; }.home-hero-section { padding: 44px 0 56px; }.home-hero-title { font-size: clamp(42px, 14vw, 64px); }.home-hero-subtitle { font-size: 18px; }.home-hero-actions { align-items: stretch; flex-direction: column; }.home-primary-button, .home-secondary-button { width: 100%; }.home-hero-facts { gap: 10px 14px; }.home-capability-grid { grid-template-columns: repeat(2, 1fr); }.home-capability-grid span { min-height: 48px; }.home-capability-grid span:nth-child(3) { border-left: 1px solid var(--home-section-border); }.home-capability-grid span:nth-child(2), .home-capability-grid span:nth-child(4) { border-right: 1px solid var(--home-section-border); }.home-section-copy h2, .home-section-heading h2, .home-final-panel h2 { font-size: 34px; }.home-section-heading { padding: 70px 0 34px; }.home-code-panel pre { min-height: 250px; padding: 17px; font-size: 10px; }.home-code-header { padding: 0 12px; }.home-window-dots { display: none; }.routing-visual { padding: 14px; }.routing-feature { min-height: auto; }.preview-body { grid-template-columns: 38px 1fr; }.preview-main { padding: 14px; }.preview-kpis strong { font-size: 18px; }.home-workflow-grid { grid-template-columns: 1fr; }.home-workflow-step { min-height: auto; }.workflow-arrow { display: none; }.home-final-section { padding: 70px 0; }.home-final-panel { align-items: flex-start; flex-direction: column; padding: 26px; }.home-final-actions { width: 100%; flex-direction: column; }.home-footer-grid { grid-template-columns: 1fr 1fr; gap: 32px 20px; }.home-footer-brand { grid-column: 1 / -1; }.home-footer-bottom { align-items: flex-start; flex-direction: column; }.compact-home-nav { flex-wrap: wrap; padding: 10px 0; }.compact-home-brand { max-width: 42%; }.compact-home-actions { max-width: 58%; flex-wrap: wrap; justify-content: flex-end; }.compact-home-main { min-height: 62vh; padding: 48px 14px; } }
@media (max-width: 620px) {
  .home-page { width: 100vw; }
  .home-container,
  .home-navbar,
  .compact-home-nav { width: calc(100vw - 28px); max-width: none; }
  .home-hero-layout {
    display: block;
    width: calc(100vw - 28px);
  }
  .home-hero-scene {
    width: 100%;
    margin: 20px auto 0;
  }
  .home-hero-stage-frame,
  .home-hero-stage-frame :deep(.hero-orbit-stage) {
    min-height: 468px;
  }
  .home-stage-topline,
  .home-stage-footline {
    right: 18px;
    left: 18px;
  }
  .home-stage-topline { gap: 8px; font-size: 8px; letter-spacing: 0.08em; }
  .home-stage-footline span:last-child {
    display: none;
  }
  .home-navbar {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 120px;
    align-items: center;
    gap: 8px 10px;
    flex-wrap: nowrap;
    position: relative;
  }
  .home-brand { min-width: 0; }
  .home-nav-actions { display: flex; width: 120px; max-width: 120px; min-width: 0; margin-left: 0; justify-self: end; justify-content: flex-end; gap: 3px; }
  .home-nav-actions > .relative,
  .home-nav-actions > .relative > button {
    width: 38px;
    min-width: 38px;
  }
  .home-nav-actions > .relative > button { justify-content: center; padding: 0; }
  .home-nav-actions > .relative > button > .text-base { display: inline; }
  .home-nav-actions > .relative > button > svg { display: none; }
  .home-nav-actions .home-solid-button { width: 38px; padding: 0; }
  .home-nav-actions .home-auth-label { display: none; }
  .home-nav-links {
    grid-column: 1 / -1;
    width: 100%;
    min-width: 0;
    margin-left: 0;
    padding-bottom: 1px;
  }
  .home-hero-copy,
  .home-hero-title,
  .home-hero-subtitle,
  .home-hero-description {
    width: 100%;
    min-width: 0;
    max-width: 100%;
    overflow-wrap: anywhere;
  }
  .home-hero-title {
    font-size: clamp(39px, 13vw, 54px);
    letter-spacing: -0.045em;
  }
  .home-hero-title span { max-width: 100%; }
  .home-hero-subtitle { line-height: 1.5; }
}
@media (max-width: 820px) {
  .home-learning-layout { grid-template-columns: 1fr; gap: 38px; padding: 88px 0; }
  .home-learning-layout .home-section-copy { max-width: 680px; }
  .home-learning-preview { min-height: 430px; }
}
@media (max-width: 620px) {
  .home-learning-layout { gap: 30px; padding: 72px 0; }
  .home-learning-preview { min-height: 420px; border-radius: 22px; }
  .home-learning-preview-topline, .home-learning-preview-footer { right: 20px; left: 20px; font-size: 8px; }
  .home-learning-orbit { height: 340px; margin-top: 58px; }
  .home-learning-orbit-ring { width: 250px; height: 140px; }
  .home-learning-orbit-ring-two { width: 190px; height: 245px; }
  .home-learning-core { width: 112px; height: 112px; }
  .home-learning-core strong { font-size: 32px; }
  .home-learning-node { min-width: 116px; padding: 8px; gap: 7px; border-radius: 14px; }
  .home-learning-node-icon { width: 24px; height: 24px; border-radius: 8px; }
  .home-learning-node strong { font-size: 10px; }
  .home-learning-node small { font-size: 7px; }
  .home-learning-node-1 { left: 0; }
  .home-learning-node-2 { right: 0; }
  .home-learning-node-3 { left: 0; }
  .home-learning-node-4 { right: 0; }
}
@media (prefers-reduced-motion: reduce) { .home-page *, .home-page *::before, .home-page *::after { scroll-behavior: auto !important; transition-duration: 1ms !important; animation-duration: 1ms !important; animation-iteration-count: 1 !important; }.home-reveal, .home-hero-reveal { opacity: 1 !important; transform: none !important; }.home-header, .home-hero-stage-frame, .home-code-panel, .home-product-preview, .home-final-panel { backdrop-filter: none; -webkit-backdrop-filter: none; }.home-hero-stage-frame { transform: none; }.home-stage-scanline { display: none; } }

/* Final visual pass: quiet chrome, stronger depth, and one clear hero focal point. */
.home-page {
  background:
    radial-gradient(760px 520px at 82% 0%, color-mix(in srgb, var(--mr-secondary) 8%, transparent), transparent 70%),
    radial-gradient(620px 460px at 4% 28%, color-mix(in srgb, var(--mr-primary) 6%, transparent), transparent 72%),
    var(--mr-canvas);
}
:global(.dark) .home-page {
  background:
    radial-gradient(760px 520px at 82% 0%, color-mix(in srgb, var(--mr-secondary) 10%, transparent), transparent 70%),
    radial-gradient(620px 460px at 4% 28%, color-mix(in srgb, var(--mr-primary) 10%, transparent), transparent 72%),
    var(--mr-canvas);
}
.home-header { background: transparent; box-shadow: none; backdrop-filter: none; -webkit-backdrop-filter: none; }
.home-page-scrolled .home-header { border-color: color-mix(in srgb, var(--mr-border-strong) 74%, transparent); background: color-mix(in srgb, var(--mr-canvas) 82%, transparent); box-shadow: 0 12px 34px color-mix(in srgb, var(--mr-primary) 7%, transparent); backdrop-filter: blur(18px) saturate(135%); -webkit-backdrop-filter: blur(18px) saturate(135%); }
.home-navbar { min-height: 84px; gap: 24px; width: min(100% - 56px, 1200px); }
.home-brand { transition: transform 180ms var(--ease-standard), opacity 180ms ease; }
.home-brand:hover { opacity: 0.82; transform: translateY(-1px); }
.brand-mark { border-radius: 10px; box-shadow: 0 9px 22px color-mix(in srgb, var(--mr-primary) 20%, transparent), inset 0 1px 0 rgba(255, 255, 255, 0.28); }
.home-nav-links { gap: 2px; padding: 5px; border: 1px solid color-mix(in srgb, var(--mr-border-strong) 58%, transparent); border-radius: 999px; background: color-mix(in srgb, var(--mr-surface) 52%, transparent); box-shadow: inset 0 1px 0 var(--glass-highlight), 0 8px 22px color-mix(in srgb, var(--mr-primary) 5%, transparent); backdrop-filter: blur(14px) saturate(125%); -webkit-backdrop-filter: blur(14px) saturate(125%); }
.home-nav-link { min-height: 34px; padding: 0 13px; border-radius: 999px; color: var(--mr-text-muted); transition: color 180ms var(--ease-standard), background-color 180ms var(--ease-standard), transform 180ms var(--ease-standard); }
.home-nav-link::after { display: none; }
.home-nav-link:hover { color: var(--mr-text); background: color-mix(in srgb, var(--mr-surface-raised) 62%, transparent); transform: translateY(-1px); }
.home-nav-link.is-active { color: var(--mr-text); background: var(--mr-surface-raised); box-shadow: 0 4px 12px color-mix(in srgb, var(--mr-primary) 10%, transparent), inset 0 1px 0 var(--glass-highlight); }
.home-icon-button, .home-quiet-link { border-radius: 999px; }
.home-solid-button, .home-primary-button { min-height: 42px; padding-inline: 17px; border-radius: 999px; }
.home-hero-section { min-height: min(900px, calc(100vh - 84px)); padding: 72px 0 96px; }
.home-hero-layout { min-height: 650px; grid-template-columns: minmax(0, 0.86fr) minmax(0, 1.14fr); gap: 72px; }
.home-hero-copy { max-width: 520px; }
.home-hero-title { margin-top: 20px; max-width: 620px; font-size: clamp(58px, 6.7vw, 94px); letter-spacing: -0.075em; line-height: 0.98; }
.home-hero-subtitle { max-width: 470px; margin-top: 28px; font-size: clamp(19px, 2.1vw, 25px); line-height: 1.4; }
.home-hero-description { max-width: 455px; margin-top: 16px; font-size: 13px; line-height: 1.9; }
.home-hero-actions { margin-top: 30px; }
.home-secondary-button { min-height: 42px; padding-inline: 17px; border-radius: 999px; background: color-mix(in srgb, var(--mr-surface) 66%, transparent); }
.home-hero-facts { margin-top: 36px; }
.home-hero-stage-frame { min-height: 632px; border-radius: 34px; border-color: color-mix(in srgb, var(--mr-primary) 28%, var(--mr-border-strong)); background: linear-gradient(145deg, color-mix(in srgb, var(--mr-surface-raised) 70%, transparent), color-mix(in srgb, var(--mr-primary) 8%, var(--mr-canvas))); box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.22), inset 0 -24px 60px color-mix(in srgb, var(--mr-primary) 5%, transparent), 0 34px 90px color-mix(in srgb, var(--mr-primary) 14%, transparent); backdrop-filter: blur(12px) saturate(132%); -webkit-backdrop-filter: blur(12px) saturate(132%); }
.home-hero-stage-frame::before { border-radius: inherit; border-color: color-mix(in srgb, var(--mr-secondary) 18%, transparent); }
.home-hero-stage-frame:hover { transform: perspective(1500px) rotateX(var(--hero-rotate-x)) rotateY(var(--hero-rotate-y)) translateY(-4px); border-color: color-mix(in srgb, var(--mr-secondary) 48%, var(--mr-border-strong)); box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.28), inset 0 -24px 60px color-mix(in srgb, var(--mr-primary) 7%, transparent), 0 42px 100px color-mix(in srgb, var(--mr-primary) 21%, transparent); }
.home-hero-stage-frame :deep(.hero-orbit-stage) { min-height: 632px; }
.home-capability-strip { background: color-mix(in srgb, var(--mr-surface) 54%, transparent); backdrop-filter: blur(12px); -webkit-backdrop-filter: blur(12px); }
.home-capability-grid { min-height: 76px; }
.home-section-light { background: color-mix(in srgb, var(--mr-surface) 22%, var(--mr-canvas)); }
.home-section-contrast { background: color-mix(in srgb, var(--mr-surface) 66%, transparent); }
.home-section-copy h2, .home-section-heading h2, .home-final-panel h2 { font-size: clamp(42px, 5vw, 66px); letter-spacing: -0.06em; line-height: 1.03; }
.home-section-copy > p, .home-section-heading > p, .home-final-panel p { max-width: 620px; font-size: 13px; line-height: 1.9; }
.home-code-panel, .routing-visual, .home-product-preview, .home-final-panel { border-radius: 24px; }
.home-code-panel { background: color-mix(in srgb, var(--mr-surface-raised) 70%, transparent); box-shadow: inset 0 1px 0 var(--glass-highlight), 0 30px 72px color-mix(in srgb, var(--mr-primary) 10%, transparent); }
.routing-visual { background: linear-gradient(145deg, color-mix(in srgb, var(--mr-surface-raised) 76%, transparent), color-mix(in srgb, var(--mr-primary) 6%, var(--mr-surface))); box-shadow: inset 0 1px 0 var(--glass-highlight), 0 24px 58px color-mix(in srgb, var(--mr-primary) 8%, transparent); }
.routing-node, .routing-core-node { border-radius: 16px; }
.home-workflow-step { transition: background-color 180ms var(--ease-standard), transform 180ms var(--ease-standard), box-shadow 180ms var(--ease-standard); }
.home-workflow-step:hover { position: relative; z-index: 1; transform: translateY(-3px); box-shadow: 0 18px 34px color-mix(in srgb, var(--mr-primary) 10%, transparent); }
.home-learning-layout { align-items: stretch; gap: 74px; padding: 140px 0; }
.home-learning-layout > .home-section-copy { align-self: center; }
.home-learning-layout .home-section-copy h2 { max-width: 620px; font-size: clamp(42px, 4.8vw, 66px); }
.home-learning-preview { min-height: 500px; border-radius: 32px; background: radial-gradient(circle at 50% 45%, color-mix(in srgb, var(--mr-primary) 14%, transparent), transparent 42%), linear-gradient(145deg, color-mix(in srgb, var(--mr-surface-raised) 82%, transparent), color-mix(in srgb, var(--mr-primary) 9%, var(--mr-canvas))); box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.24), 0 32px 80px color-mix(in srgb, var(--mr-primary) 13%, transparent); }
.home-learning-node { border-radius: 18px; box-shadow: inset 0 1px 0 var(--glass-highlight), 0 18px 34px color-mix(in srgb, var(--mr-primary) 10%, transparent); }
.home-final-section { padding: 118px 0; }
.home-final-panel { padding: 50px; border-radius: 28px; background: linear-gradient(135deg, color-mix(in srgb, var(--mr-surface-raised) 84%, transparent), color-mix(in srgb, var(--mr-primary) 8%, transparent)); }
.back-to-top { border-radius: 999px; }

@media (max-width: 1080px) {
  .home-navbar { gap: 16px; flex-wrap: wrap; padding: 10px 0; }
  .home-nav-links { order: 3; width: 100%; margin-left: 0; justify-content: center; overflow-x: auto; }
  .home-hero-layout { grid-template-columns: minmax(0, 0.9fr) minmax(0, 1.1fr); gap: 34px; }
  .home-hero-stage-frame, .home-hero-stage-frame :deep(.hero-orbit-stage) { min-height: 570px; }
}
@media (max-width: 820px) {
  .home-hero-section { padding: 58px 0 76px; }
  .home-hero-layout { display: grid; grid-template-columns: 1fr; gap: 42px; }
  .home-hero-copy { max-width: 680px; }
  .home-hero-stage-frame, .home-hero-stage-frame :deep(.hero-orbit-stage) { min-height: 520px; }
  .home-section-light .home-two-column { padding-top: 88px; padding-bottom: 88px; }
  .home-learning-layout { gap: 44px; padding: 106px 0; }
  .home-learning-preview { min-height: 460px; }
  .home-final-panel { padding: 38px; }
}
@media (max-width: 620px) {
  .home-navbar { min-height: 78px; }
  .home-nav-links { justify-content: flex-start; padding: 4px; }
  .home-nav-link { min-height: 32px; padding-inline: 11px; font-size: 11px; }
  .home-hero-section { padding: 44px 0 62px; }
  .home-hero-title { font-size: clamp(42px, 13vw, 58px); }
  .home-hero-subtitle { margin-top: 22px; font-size: 18px; }
  .home-hero-stage-frame, .home-hero-stage-frame :deep(.hero-orbit-stage) { min-height: 468px; border-radius: 26px; }
  .home-section-copy h2, .home-section-heading h2, .home-final-panel h2 { font-size: 36px; }
  .home-learning-layout { gap: 34px; padding: 82px 0; }
  .home-learning-preview { min-height: 430px; border-radius: 24px; }
  .home-final-section { padding: 78px 0; }
  .home-final-panel { padding: 28px; border-radius: 22px; }
}
@media (prefers-reduced-motion: reduce) {
  .home-page::after, .home-learning-orbit::before, .home-learning-orbit-ring, .home-learning-core::before, .home-learning-node { animation: none !important; }
  .home-hero-stage-frame:hover { transform: none !important; }
}
</style>
