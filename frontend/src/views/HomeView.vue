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

  <div v-else ref="pageRef" class="home-page theory-home" :class="{ 'theory-home-scrolled': isScrolled }">
    <div class="theory-scene-layer" aria-hidden="true">
      <HomeHeroScene />
    </div>
    <div class="theory-scene-wash" aria-hidden="true"></div>
    <div class="theory-scene-haze theory-scene-haze-one" aria-hidden="true"></div>
    <div class="theory-scene-haze theory-scene-haze-two" aria-hidden="true"></div>

    <header class="theory-header">
      <div class="theory-progress" :style="{ width: `${scrollProgress}%` }"></div>
      <nav class="theory-header-inner" aria-label="Home navigation">
        <button class="theory-brand" type="button" @click="scrollToSection('home')">
          <span class="theory-brand-mark"><img :src="siteLogo || brand.logo" :alt="siteName" /></span>
          <span>{{ siteName }}</span>
        </button>

        <div class="theory-nav" aria-label="Page sections">
          <template v-for="item in navigationItems" :key="item.id">
            <router-link
              v-if="item.id === 'learning'"
              :to="learningEntry"
              class="theory-nav-link"
              :class="{ 'is-active': activeSection === item.id }"
              :aria-current="activeSection === item.id ? 'page' : undefined"
            >{{ item.label }}</router-link>
            <button
              v-else
              type="button"
              class="theory-nav-link"
              :class="{ 'is-active': activeSection === item.id }"
              :aria-current="activeSection === item.id ? 'page' : undefined"
              @click="scrollToSection(item.id)"
            >{{ item.label }}</button>
          </template>
        </div>

        <div class="theory-actions">
          <LocaleSwitcher />
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="theory-action-button" :title="copy.nav.docs" aria-label="Documentation">
            <Icon name="book" size="sm" />
          </a>
          <router-link v-if="showModelPlazaEntry" to="/model-plaza" class="theory-action-button" :title="t('nav.modelPlaza')" :aria-label="t('nav.modelPlaza')">
            <Icon name="grid" size="sm" />
          </router-link>
          <button type="button" class="theory-action-button" :title="isDark ? copy.nav.light : copy.nav.dark" @click="toggleTheme">
            <Icon v-if="isDark" name="sun" size="sm" />
            <Icon v-else name="moon" size="sm" />
          </button>
          <router-link v-if="isAuthenticated" :to="dashboardPath" class="theory-auth-button">
            <span class="theory-user-avatar">{{ userInitial }}</span>
            <span class="theory-auth-label">{{ copy.nav.dashboard }}</span>
          </router-link>
          <router-link v-else to="/login" class="theory-auth-button" :title="copy.nav.login" :aria-label="copy.nav.login">
            <span>{{ copy.nav.login }}</span><Icon name="arrowRight" size="xs" />
          </router-link>
        </div>
      </nav>
    </header>

    <aside class="theory-side-rail" aria-hidden="true">
      <span>MODURELAY / 2026</span>
      <span class="theory-side-rail-line"></span>
      <span>RELAY CORE</span>
    </aside>

    <main class="theory-main">
      <section id="home" data-home-section class="theory-hero">
        <div class="theory-hero-inner">
          <div class="theory-hero-copy">
            <div class="theory-eyebrow home-hero-reveal"><span class="theory-live-dot"></span>{{ copy.hero.eyebrow }}</div>
            <h1 class="theory-hero-title home-hero-reveal">
              <span>{{ copy.hero.title }}</span>
              <span class="theory-hero-title-accent">{{ copy.hero.titleAccent }}</span>
            </h1>
            <p class="theory-hero-subtitle home-hero-reveal">{{ copy.hero.subtitle }}</p>
            <p class="theory-hero-description home-hero-reveal">{{ copy.hero.description }}</p>
            <div class="theory-hero-actions home-hero-reveal">
              <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="theory-primary-button">
                {{ isAuthenticated ? copy.hero.dashboardCta : copy.hero.primaryCta }}<Icon name="arrowRight" size="sm" />
              </router-link>
              <router-link to="/key-usage" class="theory-secondary-button">
                {{ copy.hero.secondaryCta }}<Icon name="chart" size="sm" />
              </router-link>
            </div>
            <div class="theory-hero-facts home-hero-reveal">
              <span v-for="fact in copy.hero.facts" :key="fact"><i></i>{{ fact }}</span>
            </div>
          </div>

          <div class="theory-telemetry home-hero-reveal" aria-hidden="true">
            <div class="theory-telemetry-head"><span>LIVE SYSTEM</span><span class="theory-telemetry-status"><i></i>READY</span></div>
            <div class="theory-telemetry-rule"></div>
            <div class="theory-telemetry-row"><span>REQUEST</span><strong>/v1/chat/completions</strong></div>
            <div class="theory-telemetry-row"><span>ROUTE</span><strong>POLICY MATCH</strong></div>
            <div class="theory-telemetry-row"><span>USAGE</span><strong>TRACKED</strong></div>
          </div>
        </div>

        <div class="theory-hero-footer">
          <span>OPENAI COMPATIBLE <i></i> MODEL ROUTE USAGE</span>
          <button type="button" class="theory-scroll-cue" @click="scrollToSection('integrate')"><span>SCROLL TO EXPLORE</span><Icon name="arrowDown" size="sm" /></button>
        </div>
      </section>

      <section id="integrate" data-home-section class="theory-section theory-section-surface">
        <div class="theory-section-inner">
          <div class="theory-chapter-label home-reveal">01 / INTEGRATE</div>
          <div class="theory-section-heading home-reveal">
            <h2>{{ copy.integrate.title }}</h2>
            <p>{{ copy.integrate.description }}</p>
          </div>
          <div class="theory-integrate-grid">
            <div class="theory-endpoint-list home-reveal">
              <div v-for="endpoint in copy.integrate.endpoints" :key="endpoint.path" class="theory-endpoint-row">
                <span class="theory-endpoint-method">{{ endpoint.method }}</span>
                <div><code>{{ endpoint.path }}</code><span>{{ endpoint.label }}</span></div>
                <Icon name="arrowRight" size="sm" />
              </div>
            </div>
            <div class="theory-terminal home-reveal">
              <div class="theory-terminal-header">
                <div class="theory-window-dots"><i></i><i></i><i></i></div>
                <div class="theory-code-tabs" role="tablist" :aria-label="copy.integrate.codeLabel">
                  <button v-for="sample in codeSampleOptions" :key="sample.key" type="button" role="tab" :aria-selected="activeCodeSample === sample.key" :class="{ 'is-active': activeCodeSample === sample.key }" @click="activeCodeSample = sample.key">{{ sample.label }}</button>
                </div>
              </div>
              <Transition name="theory-code-swap" mode="out-in">
                <pre :key="activeCodeSample"><code>{{ activeCode }}</code></pre>
              </Transition>
              <div class="theory-terminal-footer"><span class="theory-live-dot"></span>{{ copy.integrate.codeFooter }}</div>
            </div>
          </div>
        </div>
      </section>

      <section id="routing" data-home-section class="theory-section theory-section-dark">
        <div class="theory-section-inner">
          <div class="theory-chapter-label home-reveal">02 / ROUTING CORE</div>
          <div class="theory-section-heading theory-section-heading-wide home-reveal">
            <h2>{{ copy.routing.title }}</h2>
            <p>{{ copy.routing.description }}</p>
          </div>
          <div class="theory-route-map home-reveal">
            <div class="theory-route-track theory-route-track-one"></div>
            <div class="theory-route-track theory-route-track-two"></div>
            <div class="theory-route-node theory-route-node-request">
              <span class="theory-route-node-index">01</span><Icon name="link" size="md" /><strong>{{ copy.routing.request.title }}</strong><code>/v1/chat/completions</code>
            </div>
            <div class="theory-route-core"><span class="theory-route-core-ring"></span><span class="theory-route-core-mark">M</span><strong>RELAY CORE</strong><small>{{ copy.routing.coreCaption }}</small></div>
            <div class="theory-route-node theory-route-node-output">
              <span class="theory-route-node-index">02</span><span class="theory-route-node-label">{{ copy.routing.routesLabel }}</span>
              <div v-for="route in copy.routing.routes" :key="route.title" class="theory-route-output-row"><i></i><strong>{{ route.title }}</strong><small>{{ route.detail }}</small></div>
            </div>
          </div>
          <div class="theory-feature-row">
            <article v-for="feature in copy.routing.features" :key="feature.title" class="theory-feature home-reveal">
              <span>{{ feature.number }}</span><div><h3>{{ feature.title }}</h3><p>{{ feature.description }}</p></div>
            </article>
          </div>
        </div>
      </section>

      <section id="observability" data-home-section class="theory-section theory-section-surface theory-observability-section">
        <div class="theory-section-inner theory-split-section">
          <div class="theory-observability-copy home-reveal">
            <div class="theory-chapter-label">03 / OBSERVABILITY</div>
            <h2>{{ copy.observability.title }}</h2>
            <p>{{ copy.observability.description }}</p>
            <div class="theory-observability-list">
              <div v-for="item in copy.observability.items" :key="item.title" class="theory-observability-item"><span><Icon :name="item.icon" size="sm" /></span><div><strong>{{ item.title }}</strong><p>{{ item.description }}</p></div></div>
            </div>
          </div>
          <div class="theory-console home-reveal" aria-label="ModuRelay control and observability preview">
            <div class="theory-console-top"><span>MODURELAY / OPERATIONS</span><span><i></i>LIVE SAMPLE</span></div>
            <div class="theory-console-title"><strong>{{ copy.observability.previewTitle }}</strong><span>Workspace view</span></div>
            <div class="theory-console-kpis"><div><span>REQUESTS</span><strong>12.8K</strong><em>tracked</em></div><div><span>ROUTES</span><strong>04</strong><em>healthy</em></div><div><span>LATENCY</span><strong>182ms</strong><em>stable</em></div></div>
            <div class="theory-console-chart" aria-hidden="true"><span v-for="index in 12" :key="index"></span><svg viewBox="0 0 620 150" preserveAspectRatio="none"><path d="M0 122 C34 111 48 118 75 94 S116 107 141 86 S181 52 208 82 S252 99 281 65 S318 79 349 59 S395 91 425 49 S465 72 496 41 S543 61 573 26 S603 36 620 15" /></svg></div>
            <div class="theory-console-routes"><div v-for="route in copy.observability.previewRoutes" :key="route"><span>{{ route }}</span><i></i><strong>READY</strong><small>route</small></div></div>
          </div>
        </div>
      </section>

      <section id="workflow" data-home-section class="theory-section theory-section-dark theory-workflow-section">
        <div class="theory-section-inner">
          <div class="theory-chapter-label home-reveal">04 / WORKFLOW</div>
          <div class="theory-section-heading home-reveal"><h2>{{ copy.workflow.title }}</h2><p>{{ copy.workflow.description }}</p></div>
          <div class="theory-workflow-track">
            <article v-for="(step, index) in copy.workflow.steps" :key="step.title" class="theory-workflow-step home-reveal">
              <div class="theory-workflow-step-top"><span>0{{ index + 1 }}</span><Icon :name="step.icon" size="md" /></div><h3>{{ step.title }}</h3><p>{{ step.description }}</p><span v-if="index < copy.workflow.steps.length - 1" class="theory-workflow-connector"><i></i></span>
            </article>
          </div>
        </div>
      </section>

      <section id="learning" data-home-section class="theory-section theory-learning-section">
        <div class="theory-section-inner theory-learning-layout">
          <div class="theory-learning-copy home-reveal">
            <div class="theory-chapter-label">{{ copy.learning.eyebrow }}</div>
            <h2>{{ copy.learning.title }}</h2>
            <p>{{ copy.learning.description }}</p>
            <div class="theory-learning-points"><span v-for="point in copy.learning.points" :key="point"><i></i>{{ point }}</span></div>
            <router-link :to="learningEntry" class="theory-primary-button">{{ copy.learning.cta }}<Icon name="arrowRight" size="sm" /></router-link>
          </div>
          <div class="theory-learning-field home-reveal" aria-label="AI learning capability map">
            <div class="theory-learning-field-head"><span>AI LEARNING / CAPABILITY MAP</span><span><i></i>{{ copy.learning.status }}</span></div>
            <div class="theory-learning-field-body" aria-hidden="true"><span class="theory-learning-orbit theory-learning-orbit-one"></span><span class="theory-learning-orbit theory-learning-orbit-two"></span><span class="theory-learning-orbit theory-learning-orbit-three"></span><div class="theory-learning-core"><strong>AI</strong><small>LEARNING CORE</small></div><span v-for="(node, index) in copy.learning.nodes" :key="node.label" class="theory-learning-node" :class="`theory-learning-node-${index + 1}`"><span class="theory-learning-node-icon"><Icon :name="node.icon" size="sm" /></span><span><strong>{{ node.label }}</strong><small>{{ node.detail }}</small></span></span></div>
            <div class="theory-learning-field-foot"><span>{{ copy.learning.footer }}</span><Icon name="arrowRight" size="sm" /></div>
          </div>
        </div>
      </section>

      <section id="contact" data-home-section class="theory-section theory-contact-section">
        <div class="theory-contact-panel home-reveal">
          <div><div class="theory-chapter-label">MODURELAY</div><h2>{{ copy.contact.title }}</h2><p>{{ copy.contact.description }}</p></div>
          <div class="theory-contact-actions"><router-link :to="isAuthenticated ? dashboardPath : '/login'" class="theory-primary-button">{{ isAuthenticated ? copy.hero.dashboardCta : copy.contact.primaryCta }}<Icon name="arrowRight" size="sm" /></router-link><a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="theory-secondary-button">{{ copy.contact.docsCta }}<Icon name="book" size="sm" /></a></div>
        </div>
      </section>
    </main>

    <footer class="theory-footer">
      <div class="theory-footer-main"><div class="theory-footer-brand"><div class="theory-brand-line"><span class="theory-brand-mark"><img :src="siteLogo || brand.logo" :alt="siteName" /></span><strong>{{ siteName }}</strong></div><p>{{ copy.footer.description }}</p></div><div class="theory-footer-links"><strong>{{ copy.footer.product }}</strong><button type="button" @click="scrollToSection('integrate')">{{ copy.nav.integrate }}</button><button type="button" @click="scrollToSection('routing')">{{ copy.nav.routing }}</button><button type="button" @click="scrollToSection('observability')">{{ copy.nav.observability }}</button></div><div class="theory-footer-links"><strong>{{ copy.footer.resources }}</strong><a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">{{ copy.nav.docs }}</a><router-link to="/key-usage">{{ copy.footer.usage }}</router-link><router-link :to="learningEntry">{{ copy.nav.learning }}</router-link></div><div class="theory-footer-links"><strong>{{ copy.footer.account }}</strong><router-link :to="isAuthenticated ? dashboardPath : '/login'">{{ isAuthenticated ? copy.nav.dashboard : copy.nav.login }}</router-link><button type="button" @click="scrollToSection('contact')">{{ copy.nav.contact }}</button></div></div>
      <div class="theory-footer-bottom"><span>&copy; {{ currentYear }} {{ siteName }}. {{ copy.footer.rights }}</span><span>{{ copy.footer.tagline }}</span></div>
    </footer>

    <button v-show="isScrolled" type="button" class="theory-back-to-top" :title="copy.nav.backToTop" @click="scrollToSection('home')"><Icon name="arrowUp" size="sm" /></button>
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
  hero: { eyebrow: 'OPENAI-COMPATIBLE API GATEWAY', title: '模型很多', titleAccent: '入口只要一个', subtitle: '让接入保持熟悉 让运行保持清楚', description: '沿用 OpenAI 兼容接口，把多模型路由、健康切换与用量追踪交给 ModuRelay。应用只需要关心结果。', primaryCta: '开始使用', dashboardCta: '进入控制台', secondaryCta: '用量查询', facts: ['统一接入', '按策略路由', '用量可见'] },
  capabilities: { strip: ['统一 API 网关', '多模型路由', '健康检查', '用量与额度'] },
  integrate: { title: '从一个请求开始', description: '不改掉熟悉的调用方式。替换 base URL，剩下的接入、选择与返回由网关完成。', codeLabel: 'Code examples', codeFooter: '请求已通过网关校验', endpoints: [{ method: 'POST', path: '/v1/chat/completions', label: 'Chat Completions' }, { method: 'POST', path: '/v1/responses', label: 'Responses API' }, { method: 'GET', path: '/v1/models', label: 'Model discovery' }] },
  routing: { title: '路由有策略 结果有去处', description: '按模型能力、线路健康和分组策略选择路径，异常时自动切换。每一次请求都有清晰上下文。', request: { title: 'Your application' }, coreCaption: 'policy + health + usage', routesLabel: 'Available routes', routes: [{ title: 'OpenAI compatible', detail: 'primary route' }, { title: 'Responses API', detail: 'capability match' }, { title: 'Fallback route', detail: 'on provider error' }], features: [{ number: '01', title: 'Provider pools', description: '将可用账户组织成清晰的资源池，让策略保持可维护。' }, { number: '02', title: 'Failover', description: '线路出现异常时按策略切换，减少应用中断。' }, { number: '03', title: 'Usage-aware', description: '请求、模型和密钥上下文统一留在用量记录里。' }] },
  observability: { title: '让运行状态可见', description: '请求、用量、额度和线路状态聚合到同一个视图。需要排查时，信息就在眼前。', previewTitle: 'Operations snapshot', previewRoutes: ['OpenAI / Chat', 'Responses / Primary', 'Fallback / Health'], items: [{ icon: 'chart' as const, title: 'Usage', description: '按日期、模型和密钥查看请求与 token。' }, { icon: 'shield' as const, title: 'Channel status', description: '查看线路健康状态和响应表现。' }, { icon: 'key' as const, title: 'API keys', description: '创建、管理并安全使用接入密钥。' }] },
  workflow: { title: '从接入到返回 只经过必要步骤', description: '应用只面对一个兼容接口。网关负责选择线路、处理异常并返回结果。', steps: [{ icon: 'link' as const, title: 'Connect', description: '使用 API key 指向 ModuRelay gateway。' }, { icon: 'server' as const, title: 'Relay', description: 'Relay Core 读取分组和路由配置。' }, { icon: 'sync' as const, title: 'Route', description: '选择合适的 Provider，必要时执行切换。' }, { icon: 'check' as const, title: 'Respond', description: '返回兼容响应，同时保留用量信息。' }] },
  learning: { eyebrow: '05 / AI LEARNING', title: '让 AI 经验变成可学习的能力', description: '面向老师与技术专家，把 Agent 设计、工作流、记忆反馈和具身智能整理成真实案例与练习，从看懂到做出结果。', points: ['老师能力：把复杂 AI 讲清楚，让学习有路径', '技术专家：把架构与工程判断落到真实任务', '学习方式：案例理解、动手练习、结果复盘'], cta: '进入 AI 学习', status: '公开内容', footer: '点击主题，查看学习内容', nodes: [{ label: 'Agent', detail: '可教的架构', icon: 'brain' as const }, { label: 'Workflow', detail: '技术专家实践', icon: 'cpu' as const }, { label: 'Memory', detail: '反馈闭环', icon: 'database' as const }, { label: 'Embodied', detail: '真实落地', icon: 'beaker' as const }] },
  contact: { title: '把复杂度留给网关', description: '从一个兼容端点开始，让模型、路由和用量回到同一条链路。', primaryCta: '开始使用', docsCta: '查看文档' },
  footer: { description: '一个面向模型接入与运行管理的 OpenAI compatible API 网关。', product: '产品', resources: '资源', account: '账户', usage: '用量查询', rights: '保留所有权利', tagline: '接入简单 运行清楚' }
}

const enCopy = {
  nav: { home: 'Home', integrate: 'Integrate', routing: 'Routing core', observability: 'Observability', workflow: 'Workflow', learning: 'AI Learning', contact: 'Contact', docs: 'Docs', light: 'Switch to light mode', dark: 'Switch to dark mode', dashboard: 'Dashboard', login: 'Sign in', backToTop: 'Back to top' },
  hero: { eyebrow: 'OPENAI-COMPATIBLE API GATEWAY', title: 'Many models', titleAccent: 'One entry point', subtitle: 'Keep integration familiar Keep runtime clear', description: 'Use an OpenAI-compatible interface while ModuRelay handles multi-model routing, health-based failover and usage tracking. Your application stays focused on the result.', primaryCta: 'Get started', dashboardCta: 'Open dashboard', secondaryCta: 'Usage lookup', facts: ['One entry point', 'Policy-based routing', 'Visible usage'] },
  capabilities: { strip: ['Unified API gateway', 'Multi-model routing', 'Health checks', 'Usage & quota'] },
  integrate: { title: 'Start with one request', description: 'Keep the calling pattern you already know. Replace the base URL and let the gateway handle access, selection and response handling.', codeLabel: 'Code examples', codeFooter: 'Request shape validated at the gateway', endpoints: [{ method: 'POST', path: '/v1/chat/completions', label: 'Chat Completions' }, { method: 'POST', path: '/v1/responses', label: 'Responses API' }, { method: 'GET', path: '/v1/models', label: 'Model discovery' }] },
  routing: { title: 'Policy finds the path Result keeps moving', description: 'ModuRelay chooses from model capability, route health and group policy, then fails over when a route needs attention.', request: { title: 'Your application' }, coreCaption: 'policy + health + usage', routesLabel: 'Available routes', routes: [{ title: 'OpenAI compatible', detail: 'primary route' }, { title: 'Responses API', detail: 'capability match' }, { title: 'Fallback route', detail: 'on provider error' }], features: [{ number: '01', title: 'Provider pools', description: 'Organize available accounts into clear, maintainable groups.' }, { number: '02', title: 'Failover', description: 'Switch to an available route when a Provider returns an error.' }, { number: '03', title: 'Usage-aware', description: 'Keep request, model and key context visible in usage data.' }] },
  observability: { title: 'Keep runtime easy to read', description: 'See requests, usage, quota and route status together, then get to the reason when something needs attention.', previewTitle: 'Operations snapshot', previewRoutes: ['OpenAI / Chat', 'Responses / Primary', 'Fallback / Health'], items: [{ icon: 'chart' as const, title: 'Usage', description: 'Review requests and tokens by date, model and key.' }, { icon: 'shield' as const, title: 'Channel status', description: 'See route health and response behavior.' }, { icon: 'key' as const, title: 'API keys', description: 'Create, manage and use access keys securely.' }] },
  workflow: { title: 'Only the necessary steps', description: 'The client sees one compatible API. The gateway chooses the route, handles exceptions and returns the result.', steps: [{ icon: 'link' as const, title: 'Connect', description: 'Point an existing client at the ModuRelay gateway.' }, { icon: 'server' as const, title: 'Relay', description: 'Relay Core reads group and route configuration.' }, { icon: 'sync' as const, title: 'Route', description: 'Choose the right Provider and fail over when needed.' }, { icon: 'check' as const, title: 'Respond', description: 'Return a compatible response with usage context.' }] },
  learning: { eyebrow: '05 / AI LEARNING', title: 'Turn AI experience into learnable capability', description: 'For teachers and technical experts, real cases turn Agent design, workflows, memory and embodied intelligence into something you can understand and practise.', points: ['Teacher capability: make complex AI clear and teachable', 'Technical experts: bring architecture and engineering judgement into real tasks', 'Learning method: understand, practise and review'], cta: 'Enter AI Learning', status: 'PUBLIC SUMMARY', footer: 'Select a topic to explore', nodes: [{ label: 'Agent', detail: 'Teach the architecture', icon: 'brain' as const }, { label: 'Workflow', detail: 'Expert practice', icon: 'cpu' as const }, { label: 'Memory', detail: 'Feedback loop', icon: 'database' as const }, { label: 'Embodied', detail: 'Real-world proof', icon: 'beaker' as const }] },
  contact: { title: 'Leave the complexity to the gateway', description: 'Start with one compatible endpoint and bring models, routing and usage back into one clear path.', primaryCta: 'Get started', docsCta: 'Read the docs' },
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
.home-nav-link { display: inline-flex; min-height: 34px; align-items: center; justify-content: center; padding: 0 13px; border-radius: 999px; color: var(--mr-text-muted); line-height: 1; white-space: nowrap; transition: color 180ms var(--ease-standard), background-color 180ms var(--ease-standard), transform 180ms var(--ease-standard); }
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
/* Active Theory-inspired hero: one original Relay Core in a quiet dark field. */
.home-page::before { display: none; }
.home-page::after { opacity: 0.22; }
.home-background { opacity: 0; }
.home-hero-stage-frame {
  overflow: hidden;
  border-color: color-mix(in srgb, var(--mr-border-strong) 78%, var(--mr-primary));
  background: var(--color-bg-deep);
  box-shadow: 0 30px 80px rgba(5, 8, 18, 0.2), inset 0 1px 0 rgba(255, 255, 255, 0.13);
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
}
.home-hero-stage-frame::before {
  inset: 1px;
  border-color: color-mix(in srgb, var(--mr-secondary) 22%, transparent);
  border-radius: inherit;
  transform: translateZ(2px);
}
.home-hero-stage-frame::after { display: none; }
.home-hero-stage-frame:hover {
  transform: translateY(-2px);
  border-color: color-mix(in srgb, var(--mr-secondary) 46%, var(--mr-border-strong));
  box-shadow: 0 36px 88px rgba(5, 8, 18, 0.25), inset 0 1px 0 rgba(255, 255, 255, 0.18);
}
.home-stage-topline,
.home-stage-footline {
  color: rgba(226, 232, 240, 0.6);
}
.home-stage-status { color: #86efac; }
.home-stage-footline { color: rgba(148, 163, 184, 0.6); }
.home-stage-footline span:last-child { color: rgba(165, 180, 252, 0.66); }
.home-hero-stage-frame :deep(.hero-orbit-stage) { min-height: 632px; }

@media (max-width: 1080px) {
  .home-hero-layout { gap: 42px; }
}

@media (max-width: 820px) {
  .home-hero-stage-frame :deep(.hero-orbit-stage) { min-height: 520px; }
}

@media (max-width: 620px) {
  .home-hero-stage-frame :deep(.hero-orbit-stage) { min-height: 468px; }
}

@media (prefers-reduced-motion: reduce) {
  .home-hero-stage-frame:hover { transform: none; }
}

/* Immersive official home: a scene-first composition with product chapters below it. */
.theory-home {
  --theory-bg: #070a12;
  --theory-bg-deep: #04060c;
  --theory-surface: rgba(16, 21, 35, 0.76);
  --theory-surface-strong: #0e1423;
  --theory-ink: #f5f7ff;
  --theory-muted: rgba(204, 211, 229, 0.7);
  --theory-subtle: rgba(165, 176, 201, 0.56);
  --theory-line: rgba(159, 171, 204, 0.2);
  --theory-indigo: #8589ff;
  --theory-cyan: #64e7ee;
  position: relative;
  width: 100%;
  min-height: 100vh;
  overflow: clip;
  isolation: isolate;
  color: var(--theory-ink);
  background: var(--theory-bg);
  font-family: "Noto Sans SC Variable", system-ui, sans-serif;
  font-synthesis: none;
}

.theory-home.home-page::before,
.theory-home.home-page::after { display: none; }

:global(.dark) .theory-home {
  --theory-bg: #03050a;
  --theory-bg-deep: #010208;
  --theory-surface: rgba(11, 15, 27, 0.82);
  --theory-surface-strong: #090d18;
  --theory-line: rgba(151, 166, 204, 0.24);
}

.theory-home button,
.theory-home a { font: inherit; }
.theory-home button { cursor: pointer; }
.theory-home button:focus-visible,
.theory-home a:focus-visible { outline: 2px solid var(--theory-cyan); outline-offset: 4px; }
.theory-home :where(h1, h2, h3, p) { margin: 0; }

.theory-scene-layer {
  position: absolute;
  z-index: 0;
  top: 0;
  right: 0;
  left: 0;
  height: min(100vh, 980px);
  min-height: 720px;
  overflow: hidden;
  background: #070a12;
  pointer-events: auto;
}

.theory-scene-layer :deep(.hero-orbit-stage) {
  width: 100%;
  max-width: none;
  height: 100%;
  min-height: 100%;
  transform: translate3d(10vw, 0, 0) scale(1.04);
  transform-origin: center;
}

.theory-scene-layer :deep(.scene-shell) { background: #070a12; }
.theory-scene-wash {
  position: absolute;
  z-index: 1;
  top: 0;
  left: 0;
  width: min(78%, 1080px);
  height: min(100vh, 980px);
  background: linear-gradient(90deg, #070a12 0%, rgba(7, 10, 18, 0.96) 28%, rgba(7, 10, 18, 0.68) 55%, rgba(7, 10, 18, 0) 100%);
  pointer-events: none;
}

.theory-scene-haze {
  position: absolute;
  z-index: 1;
  width: 36vw;
  height: 36vw;
  min-width: 300px;
  min-height: 300px;
  border-radius: 50%;
  filter: blur(36px);
  opacity: 0.32;
  pointer-events: none;
}
.theory-scene-haze-one { top: 8%; right: -12%; background: radial-gradient(circle, rgba(76, 225, 235, 0.26), transparent 68%); }
.theory-scene-haze-two { bottom: -16%; left: 20%; background: radial-gradient(circle, rgba(113, 102, 255, 0.2), transparent 68%); }

.theory-header {
  position: sticky;
  z-index: 20;
  top: 0;
  border-bottom: 1px solid rgba(159, 171, 204, 0.1);
  background: rgba(7, 10, 18, 0.28);
  backdrop-filter: blur(18px) saturate(125%);
  -webkit-backdrop-filter: blur(18px) saturate(125%);
  transition: background-color 220ms var(--ease-standard), border-color 220ms var(--ease-standard), box-shadow 220ms var(--ease-standard);
}
.theory-home-scrolled .theory-header { border-color: var(--theory-line); background: rgba(5, 7, 13, 0.8); box-shadow: 0 14px 42px rgba(0, 0, 0, 0.18); }
.theory-progress { position: absolute; top: 0; left: 0; z-index: 2; height: 2px; background: linear-gradient(90deg, var(--theory-indigo), var(--theory-cyan)); transition: width 120ms linear; }
.theory-header-inner { display: flex; align-items: center; gap: 20px; width: min(calc(100% - 64px), 1400px); min-height: 76px; margin: 0 auto; }
.theory-brand { display: inline-flex; min-width: max-content; align-items: center; gap: 10px; padding: 0; border: 0; color: var(--theory-ink); background: transparent; font-size: 14px; font-weight: 720; letter-spacing: -0.02em; transition: opacity 180ms ease, transform 180ms var(--ease-standard); }
.theory-brand:hover { opacity: 0.8; transform: translateY(-1px); }
.theory-brand-mark { display: grid; width: 30px; height: 30px; flex: 0 0 auto; place-items: center; overflow: hidden; border: 1px solid rgba(125, 231, 236, 0.38); border-radius: 10px; background: linear-gradient(145deg, #4f56d8, #137d84); box-shadow: 0 8px 24px rgba(38, 94, 192, 0.25), inset 0 1px 0 rgba(255, 255, 255, 0.32); }
.theory-brand-mark img { width: 100%; height: 100%; object-fit: contain; }
.theory-nav { display: flex; min-width: 0; align-items: center; justify-content: center; gap: 2px; margin: 0 auto; padding: 4px; overflow-x: auto; border: 1px solid rgba(159, 171, 204, 0.17); border-radius: 999px; background: rgba(14, 19, 33, 0.58); box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.06), 0 10px 30px rgba(0, 0, 0, 0.12); scrollbar-width: none; }
.theory-nav::-webkit-scrollbar { display: none; }
.theory-nav-link { display: inline-flex; min-height: 32px; flex: 0 0 auto; align-items: center; justify-content: center; padding: 0 12px; border: 0; border-radius: 999px; color: var(--theory-subtle); background: transparent; font-size: 11px; font-weight: 590; line-height: 1; white-space: nowrap; transition: color 180ms ease, background-color 180ms ease, transform 180ms var(--ease-standard); }
.theory-nav-link:hover { color: var(--theory-ink); background: rgba(154, 163, 255, 0.1); transform: translateY(-1px); }
.theory-nav-link.is-active { color: #fff; background: rgba(116, 123, 255, 0.2); box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.12); }
.theory-actions { display: flex; min-width: max-content; align-items: center; gap: 4px; }
.theory-actions :deep(button) { color: var(--theory-muted); }
.theory-action-button { display: inline-flex; width: 34px; height: 34px; align-items: center; justify-content: center; border: 1px solid transparent; border-radius: 50%; color: var(--theory-muted); background: transparent; transition: color 180ms ease, border-color 180ms ease, background-color 180ms ease, transform 180ms var(--ease-standard); }
.theory-action-button:hover { border-color: var(--theory-line); color: var(--theory-ink); background: rgba(159, 171, 204, 0.1); transform: translateY(-1px); }
.theory-auth-button { display: inline-flex; min-height: 36px; align-items: center; justify-content: center; gap: 7px; padding: 0 13px; border: 1px solid rgba(129, 228, 235, 0.4); border-radius: 999px; color: #071018; background: linear-gradient(120deg, #9296ff, #67e5e8); box-shadow: 0 10px 26px rgba(64, 133, 223, 0.25), inset 0 1px 0 rgba(255, 255, 255, 0.5); font-size: 11px; font-weight: 760; transition: filter 180ms ease, transform 180ms var(--ease-standard), box-shadow 180ms ease; }
.theory-auth-button:hover { filter: saturate(1.12) brightness(1.06); box-shadow: 0 13px 30px rgba(64, 133, 223, 0.34), inset 0 1px 0 rgba(255, 255, 255, 0.6); transform: translateY(-2px); }
.theory-user-avatar { display: grid; width: 20px; height: 20px; place-items: center; border-radius: 50%; color: #fff; background: rgba(8, 15, 34, 0.58); font-size: 9px; font-weight: 800; }

.theory-side-rail { position: absolute; z-index: 4; top: 50%; left: max(24px, calc((100vw - 1400px) / 2)); display: grid; gap: 10px; color: rgba(181, 192, 218, 0.48); font: 700 9px/1.1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.14em; writing-mode: vertical-rl; transform: translateY(-50%); pointer-events: none; }
.theory-side-rail-line { width: 1px; height: 56px; margin: 5px 0; background: linear-gradient(var(--theory-cyan), transparent); }

.theory-main { position: relative; z-index: 2; }
.theory-hero { position: relative; display: flex; min-height: max(680px, calc(100svh - 76px)); height: min(920px, calc(100svh - 76px)); flex-direction: column; justify-content: center; padding: 68px 0 76px; scroll-margin-top: 76px; }
.theory-hero-inner { display: grid; width: min(calc(100% - 64px), 1400px); min-height: 0; grid-template-columns: minmax(0, 0.94fr) minmax(240px, 0.42fr); align-items: center; gap: 48px; margin: auto; }
.theory-hero-copy { position: relative; z-index: 4; max-width: 690px; }
.theory-eyebrow, .theory-chapter-label { display: inline-flex; align-items: center; gap: 9px; color: var(--theory-indigo); font: 720 10px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.18em; text-transform: uppercase; }
.theory-live-dot { display: inline-block; width: 6px; height: 6px; flex: 0 0 auto; border-radius: 50%; background: var(--theory-cyan); box-shadow: 0 0 0 5px rgba(100, 231, 238, 0.1), 0 0 16px rgba(100, 231, 238, 0.56); }
.theory-hero-title { max-width: 760px; margin-top: 27px; color: var(--theory-ink); font-size: clamp(66px, 8.2vw, 128px); font-weight: 700; letter-spacing: -0.085em; line-height: 0.88; }
.theory-hero-title span { display: block; }
.theory-hero-title-accent { color: transparent; background: linear-gradient(105deg, #8b8fff 4%, #63e4ea 82%); -webkit-background-clip: text; background-clip: text; }
.theory-hero-subtitle { max-width: 560px; margin-top: 31px !important; color: rgba(241, 244, 255, 0.9); font-size: clamp(17px, 2vw, 23px); font-weight: 520; letter-spacing: -0.03em; line-height: 1.35; }
.theory-hero-description { max-width: 520px; margin-top: 14px !important; color: var(--theory-muted); font-size: 13px; line-height: 1.9; }
.theory-hero-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; margin-top: 29px; }
.theory-primary-button, .theory-secondary-button { display: inline-flex; min-height: 42px; align-items: center; justify-content: center; gap: 9px; border-radius: 999px; font-size: 12px; font-weight: 700; transition: transform 180ms var(--ease-standard), box-shadow 180ms ease, border-color 180ms ease, background-color 180ms ease, color 180ms ease; }
.theory-primary-button { padding: 0 17px; border: 1px solid rgba(130, 225, 235, 0.42); color: #071018; background: linear-gradient(120deg, #8d92ff, #64e5e9); box-shadow: 0 14px 30px rgba(64, 133, 223, 0.28), inset 0 1px 0 rgba(255, 255, 255, 0.52); }
.theory-secondary-button { padding: 0 16px; border: 1px solid var(--theory-line); color: var(--theory-ink); background: rgba(17, 22, 37, 0.48); }
.theory-primary-button:hover, .theory-secondary-button:hover { transform: translateY(-2px); }
.theory-primary-button:hover { box-shadow: 0 18px 38px rgba(64, 133, 223, 0.36), inset 0 1px 0 rgba(255, 255, 255, 0.62); filter: saturate(1.08); }
.theory-secondary-button:hover { border-color: rgba(141, 146, 255, 0.58); background: rgba(116, 123, 255, 0.12); }
.theory-hero-facts { display: flex; flex-wrap: wrap; gap: 18px; margin-top: 31px; color: var(--theory-subtle); font-size: 11px; }
.theory-hero-facts span { display: inline-flex; align-items: center; gap: 8px; }
.theory-hero-facts i, .theory-telemetry-status i, .theory-terminal-footer .theory-live-dot { display: inline-block; width: 5px; height: 5px; border-radius: 50%; background: #6ef0af; box-shadow: 0 0 0 4px rgba(110, 240, 175, 0.1); }
.theory-telemetry { position: relative; z-index: 4; align-self: end; justify-self: end; width: min(100%, 260px); margin-bottom: 13%; padding: 18px 0 2px; border-top: 1px solid rgba(161, 176, 212, 0.32); border-bottom: 1px solid rgba(161, 176, 212, 0.18); color: var(--theory-subtle); font: 700 9px/1.4 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.1em; }
.theory-telemetry-head, .theory-telemetry-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.theory-telemetry-head { color: rgba(226, 231, 248, 0.7); }
.theory-telemetry-status { display: inline-flex; align-items: center; gap: 7px; color: #6ef0af; }
.theory-telemetry-rule { height: 1px; margin: 15px 0 5px; background: rgba(161, 176, 212, 0.16); }
.theory-telemetry-row { padding: 9px 0; border-bottom: 1px solid rgba(161, 176, 212, 0.1); }
.theory-telemetry-row:last-child { border-bottom: 0; }
.theory-telemetry-row strong { color: rgba(233, 238, 255, 0.82); font-size: 9px; font-weight: 680; letter-spacing: 0.04em; }
.theory-hero-footer { display: flex; position: absolute; right: max(32px, calc((100% - 1400px) / 2)); bottom: 27px; left: max(32px, calc((100% - 1400px) / 2)); z-index: 4; align-items: center; justify-content: space-between; gap: 20px; color: rgba(173, 183, 207, 0.5); font: 700 9px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.12em; }
.theory-hero-footer > span { display: inline-flex; align-items: center; gap: 10px; }
.theory-hero-footer > span i { display: inline-block; width: 3px; height: 3px; border-radius: 50%; background: var(--theory-cyan); }
.theory-scroll-cue { display: inline-flex; align-items: center; gap: 12px; padding: 0; border: 0; color: rgba(215, 221, 240, 0.68); background: transparent; font: inherit; letter-spacing: 0.1em; transition: color 180ms ease, transform 180ms var(--ease-standard); }
.theory-scroll-cue:hover { color: #fff; transform: translateY(2px); }
.theory-scroll-cue svg { color: var(--theory-cyan); animation: theory-scroll-cue 2.2s ease-in-out infinite; }

.theory-section { position: relative; min-height: 760px; padding: 136px 0; border-top: 1px solid var(--theory-line); scroll-margin-top: 76px; }
.theory-section-surface { background: linear-gradient(180deg, rgba(15, 20, 34, 0.94), rgba(8, 11, 19, 0.98)); }
.theory-section-dark { background: var(--theory-bg-deep); }
.theory-section-inner { width: min(calc(100% - 64px), 1240px); margin: 0 auto; }
.theory-section-heading { max-width: 760px; margin-top: 24px; }
.theory-section-heading-wide { max-width: 910px; }
.theory-section-heading h2, .theory-observability-copy h2, .theory-learning-copy h2, .theory-contact-panel h2 { color: var(--theory-ink); font-size: clamp(48px, 6.5vw, 90px); font-weight: 680; letter-spacing: -0.08em; line-height: 0.94; }
.theory-section-heading p, .theory-observability-copy > p, .theory-learning-copy > p, .theory-contact-panel p { max-width: 650px; margin-top: 24px; color: var(--theory-muted); font-size: 14px; line-height: 1.9; }

.theory-integrate-grid { display: grid; grid-template-columns: minmax(0, 0.77fr) minmax(0, 1.23fr); align-items: start; gap: 72px; margin-top: 78px; }
.theory-endpoint-list { border-top: 1px solid var(--theory-line); }
.theory-endpoint-row { display: grid; grid-template-columns: 48px minmax(0, 1fr) 24px; align-items: center; gap: 14px; min-height: 84px; border-bottom: 1px solid var(--theory-line); color: var(--theory-subtle); transition: padding 180ms var(--ease-standard), background-color 180ms ease, color 180ms ease; }
.theory-endpoint-row:hover { padding-right: 8px; padding-left: 8px; color: var(--theory-ink); background: rgba(126, 135, 255, 0.06); }
.theory-endpoint-method { color: var(--theory-cyan); font: 700 10px ui-monospace, SFMono-Regular, Menlo, monospace; }
.theory-endpoint-row div { display: grid; min-width: 0; gap: 7px; }
.theory-endpoint-row code { overflow: hidden; color: var(--theory-ink); font: 12px ui-monospace, SFMono-Regular, Menlo, monospace; text-overflow: ellipsis; white-space: nowrap; }
.theory-endpoint-row div span { color: var(--theory-subtle); font-size: 11px; }
.theory-endpoint-row svg { color: var(--theory-indigo); transition: transform 180ms var(--ease-standard); }
.theory-endpoint-row:hover svg { transform: translateX(3px); }
.theory-terminal { position: relative; min-height: 342px; overflow: hidden; border: 1px solid rgba(152, 168, 209, 0.25); border-radius: 18px; background: rgba(3, 6, 13, 0.84); box-shadow: 0 30px 80px rgba(0, 0, 0, 0.24), inset 0 1px 0 rgba(255, 255, 255, 0.08); }
.theory-terminal::after { position: absolute; top: 0; right: 8%; bottom: 0; width: 1px; background: linear-gradient(transparent, rgba(100, 231, 238, 0.28), transparent); content: ''; opacity: 0.55; pointer-events: none; }
.theory-terminal-header { display: flex; align-items: center; justify-content: space-between; min-height: 48px; padding: 0 18px; border-bottom: 1px solid rgba(152, 168, 209, 0.18); }
.theory-window-dots { display: flex; gap: 5px; }
.theory-window-dots i { width: 6px; height: 6px; border-radius: 50%; background: rgba(168, 181, 215, 0.45); }
.theory-window-dots i:first-child { background: rgba(255, 123, 149, 0.72); }.theory-window-dots i:nth-child(2) { background: rgba(255, 209, 113, 0.72); }.theory-window-dots i:nth-child(3) { background: rgba(110, 240, 175, 0.72); }
.theory-code-tabs { display: flex; align-items: center; gap: 2px; }
.theory-code-tabs button { padding: 6px 8px; border: 0; border-radius: 6px; color: var(--theory-subtle); background: transparent; font: 700 9px ui-monospace, SFMono-Regular, Menlo, monospace; transition: color 160ms ease, background-color 160ms ease; }
.theory-code-tabs button:hover, .theory-code-tabs button.is-active { color: var(--theory-ink); background: rgba(126, 135, 255, 0.14); }
.theory-terminal pre { min-height: 244px; margin: 0; padding: 28px 25px; overflow: auto; color: #dce3ff; font: 12px/1.9 ui-monospace, SFMono-Regular, Menlo, monospace; white-space: pre-wrap; }
.theory-terminal code { font: inherit; }
.theory-terminal-footer { display: flex; align-items: center; gap: 9px; min-height: 48px; padding: 0 18px; border-top: 1px solid rgba(152, 168, 209, 0.16); color: rgba(181, 194, 222, 0.58); font-size: 10px; }
.theory-code-swap-enter-active, .theory-code-swap-leave-active { transition: opacity 160ms ease, transform 160ms var(--ease-standard); }
.theory-code-swap-enter-from { opacity: 0; transform: translateY(6px); }.theory-code-swap-leave-to { opacity: 0; transform: translateY(-6px); }

.theory-route-map { position: relative; display: grid; min-height: 370px; grid-template-columns: minmax(0, 1fr) 180px minmax(0, 1fr); align-items: center; gap: 48px; margin-top: 72px; padding: 46px; overflow: hidden; border: 1px solid rgba(154, 169, 212, 0.2); border-radius: 24px; background: radial-gradient(circle at 50% 50%, rgba(113, 109, 255, 0.13), transparent 34%), rgba(12, 16, 28, 0.78); box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.08), 0 32px 80px rgba(0, 0, 0, 0.2); }
.theory-route-map::before, .theory-route-map::after { position: absolute; left: 50%; width: 64%; height: 1px; content: ''; pointer-events: none; transform: translateX(-50%); }
.theory-route-map::before { top: 29%; background: linear-gradient(90deg, transparent, rgba(100, 231, 238, 0.42), transparent); }.theory-route-map::after { bottom: 26%; background: linear-gradient(90deg, transparent, rgba(133, 137, 255, 0.42), transparent); }
.theory-route-track { position: absolute; z-index: 0; top: 50%; height: 1px; background: linear-gradient(90deg, transparent, rgba(100, 231, 238, 0.52), transparent); }
.theory-route-track-one { right: 22%; left: 11%; }.theory-route-track-two { right: 11%; left: 22%; background: linear-gradient(90deg, transparent, rgba(133, 137, 255, 0.48), transparent); transform: translateY(1px); }
.theory-route-node, .theory-route-core { position: relative; z-index: 2; }
.theory-route-node { min-height: 164px; padding: 22px; border: 1px solid rgba(154, 169, 212, 0.26); border-radius: 16px; background: rgba(14, 19, 33, 0.86); box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.07), 0 16px 36px rgba(0, 0, 0, 0.16); }
.theory-route-node-request { display: grid; align-content: center; gap: 10px; }.theory-route-node-request svg { color: var(--theory-cyan); }.theory-route-node strong { color: var(--theory-ink); font-size: 13px; }.theory-route-node code { color: var(--theory-subtle); font: 10px ui-monospace, SFMono-Regular, Menlo, monospace; }.theory-route-node-index { position: absolute; top: 16px; right: 18px; color: var(--theory-indigo); font: 700 9px ui-monospace, SFMono-Regular, Menlo, monospace; }
.theory-route-core { display: grid; min-height: 180px; place-items: center; align-content: center; gap: 8px; text-align: center; }.theory-route-core-ring { position: absolute; width: 166px; height: 166px; border: 1px solid rgba(100, 231, 238, 0.4); border-radius: 50%; box-shadow: 0 0 0 18px rgba(133, 137, 255, 0.05), 0 0 60px rgba(100, 231, 238, 0.16); animation: theory-core-ring 11s linear infinite; }.theory-route-core-mark { display: grid; width: 86px; height: 86px; place-items: center; border: 1px solid rgba(151, 164, 255, 0.7); border-radius: 50%; color: #fff; background: linear-gradient(145deg, #33328f, #0f6471); box-shadow: inset 0 2px 0 rgba(255, 255, 255, 0.35), 0 16px 36px rgba(0, 0, 0, 0.32); font-size: 32px; font-weight: 800; }.theory-route-core strong { color: var(--theory-ink); font: 700 10px ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.13em; }.theory-route-core small { color: var(--theory-subtle); font: 9px ui-monospace, SFMono-Regular, Menlo, monospace; }
.theory-route-node-output { display: grid; align-content: center; gap: 10px; }.theory-route-node-label { color: var(--theory-subtle); font: 700 9px ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.12em; }.theory-route-output-row { display: grid; grid-template-columns: 7px minmax(0, 1fr) auto; align-items: center; gap: 7px; }.theory-route-output-row i { width: 5px; height: 5px; border-radius: 50%; background: #6ef0af; box-shadow: 0 0 0 4px rgba(110, 240, 175, 0.08); }.theory-route-output-row strong { font-size: 11px; }.theory-route-output-row small { color: var(--theory-subtle); font-size: 9px; }
.theory-feature-row { display: grid; grid-template-columns: repeat(3, 1fr); margin-top: 42px; border-top: 1px solid var(--theory-line); border-bottom: 1px solid var(--theory-line); }.theory-feature { display: grid; grid-template-columns: 38px minmax(0, 1fr); gap: 12px; min-height: 168px; padding: 24px 25px 22px 0; border-right: 1px solid var(--theory-line); }.theory-feature + .theory-feature { padding-left: 25px; }.theory-feature:last-child { border-right: 0; }.theory-feature > span { color: var(--theory-indigo); font: 700 10px ui-monospace, SFMono-Regular, Menlo, monospace; }.theory-feature h3 { color: var(--theory-ink); font-size: 13px; font-weight: 700; }.theory-feature p { margin-top: 10px; color: var(--theory-subtle); font-size: 11px; line-height: 1.7; }

.theory-observability-section { background: linear-gradient(130deg, #0b0f1b 0%, #11182b 60%, #0b0f1b 100%); }.theory-split-section { display: grid; grid-template-columns: minmax(0, 0.78fr) minmax(0, 1.22fr); align-items: center; gap: 76px; }.theory-observability-copy { max-width: 500px; }.theory-observability-copy h2, .theory-learning-copy h2 { margin-top: 24px; }.theory-observability-list { margin-top: 34px; border-top: 1px solid var(--theory-line); }.theory-observability-item { display: grid; grid-template-columns: 34px minmax(0, 1fr); gap: 12px; padding: 15px 0; border-bottom: 1px solid var(--theory-line); }.theory-observability-item > span { display: grid; width: 30px; height: 30px; place-items: center; border: 1px solid rgba(133, 137, 255, 0.24); border-radius: 9px; color: var(--theory-indigo); background: rgba(133, 137, 255, 0.1); }.theory-observability-item strong { color: var(--theory-ink); font-size: 12px; }.theory-observability-item p { margin-top: 4px; color: var(--theory-subtle); font-size: 11px; line-height: 1.65; }
.theory-console { position: relative; padding: 22px; overflow: hidden; border: 1px solid rgba(154, 169, 212, 0.26); border-radius: 22px; background: rgba(5, 8, 15, 0.72); box-shadow: 0 36px 80px rgba(0, 0, 0, 0.3), inset 0 1px 0 rgba(255, 255, 255, 0.08); }.theory-console::before { position: absolute; top: -28%; right: -12%; width: 48%; height: 70%; border-radius: 50%; background: radial-gradient(circle, rgba(100, 231, 238, 0.16), transparent 68%); content: ''; pointer-events: none; }.theory-console-top, .theory-console-title, .theory-console-routes > div { display: flex; align-items: center; justify-content: space-between; gap: 14px; }.theory-console-top { color: var(--theory-subtle); font: 700 9px ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.12em; }.theory-console-top span:last-child { display: inline-flex; align-items: center; gap: 7px; color: #6ef0af; }.theory-console-top i { width: 5px; height: 5px; border-radius: 50%; background: currentColor; }.theory-console-title { margin-top: 36px; }.theory-console-title strong { color: var(--theory-ink); font-size: 17px; }.theory-console-title span { color: var(--theory-subtle); font-size: 10px; }.theory-console-kpis { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; margin-top: 18px; }.theory-console-kpis > div { display: grid; gap: 7px; padding: 14px; border: 1px solid rgba(154, 169, 212, 0.16); border-radius: 10px; background: rgba(18, 25, 42, 0.55); }.theory-console-kpis span { color: var(--theory-subtle); font: 700 8px ui-monospace, SFMono-Regular, Menlo, monospace; }.theory-console-kpis strong { color: var(--theory-ink); font-size: 21px; letter-spacing: -0.04em; }.theory-console-kpis em { color: #6ef0af; font-size: 9px; font-style: normal; }.theory-console-chart { position: relative; height: 142px; margin-top: 16px; overflow: hidden; border-top: 1px solid rgba(154, 169, 212, 0.14); border-bottom: 1px solid rgba(154, 169, 212, 0.18); }.theory-console-chart span { position: absolute; bottom: 0; width: 1px; height: 100%; background: rgba(154, 169, 212, 0.12); }.theory-console-chart span:nth-child(1) { left: 8%; }.theory-console-chart span:nth-child(2) { left: 16%; }.theory-console-chart span:nth-child(3) { left: 24%; }.theory-console-chart span:nth-child(4) { left: 32%; }.theory-console-chart span:nth-child(5) { left: 40%; }.theory-console-chart span:nth-child(6) { left: 48%; }.theory-console-chart span:nth-child(7) { left: 56%; }.theory-console-chart span:nth-child(8) { left: 64%; }.theory-console-chart span:nth-child(9) { left: 72%; }.theory-console-chart span:nth-child(10) { left: 80%; }.theory-console-chart span:nth-child(11) { left: 88%; }.theory-console-chart span:nth-child(12) { left: 96%; }.theory-console-chart svg { position: absolute; inset: 0; width: 100%; height: 100%; }.theory-console-chart path { fill: none; stroke: var(--theory-cyan); stroke-width: 2; vector-effect: non-scaling-stroke; filter: drop-shadow(0 0 7px rgba(100, 231, 238, 0.48)); }.theory-console-routes { margin-top: 13px; }.theory-console-routes > div { padding: 8px 0; border-bottom: 1px solid rgba(154, 169, 212, 0.1); color: var(--theory-subtle); font-size: 10px; }.theory-console-routes > div:last-child { border-bottom: 0; }.theory-console-routes i { width: 5px; height: 5px; margin-left: auto; border-radius: 50%; background: #6ef0af; }.theory-console-routes strong { color: #6ef0af; font-size: 9px; }.theory-console-routes small { width: 33px; color: var(--theory-subtle); font-size: 9px; text-align: right; }

.theory-workflow-section { min-height: 620px; }.theory-workflow-track { display: grid; grid-template-columns: repeat(4, 1fr); margin-top: 72px; border-top: 1px solid var(--theory-line); border-bottom: 1px solid var(--theory-line); }.theory-workflow-step { position: relative; min-height: 232px; padding: 24px 26px 24px 0; border-right: 1px solid var(--theory-line); }.theory-workflow-step + .theory-workflow-step { padding-left: 26px; }.theory-workflow-step:last-child { border-right: 0; }.theory-workflow-step-top { display: flex; align-items: center; justify-content: space-between; color: var(--theory-indigo); }.theory-workflow-step-top > span { font: 700 10px ui-monospace, SFMono-Regular, Menlo, monospace; }.theory-workflow-step h3 { margin-top: 52px; color: var(--theory-ink); font-size: 16px; }.theory-workflow-step p { max-width: 210px; margin-top: 11px; color: var(--theory-subtle); font-size: 11px; line-height: 1.7; }.theory-workflow-connector { position: absolute; top: 35px; right: -5px; z-index: 2; display: block; width: 9px; height: 9px; border-top: 1px solid var(--theory-cyan); border-right: 1px solid var(--theory-cyan); transform: rotate(45deg); }.theory-workflow-connector i { position: absolute; top: 3px; right: 3px; width: 48px; height: 1px; background: linear-gradient(90deg, var(--theory-cyan), transparent); transform: rotate(-45deg); transform-origin: right center; }

.theory-learning-section { min-height: 820px; background: radial-gradient(circle at 82% 47%, rgba(133, 137, 255, 0.14), transparent 30%), linear-gradient(145deg, #080c18, #12182a 62%, #080b14); }.theory-learning-layout { display: grid; grid-template-columns: minmax(0, 0.76fr) minmax(0, 1.24fr); align-items: center; gap: 72px; }.theory-learning-copy { max-width: 520px; }.theory-learning-points { display: grid; gap: 11px; margin-top: 29px; color: var(--theory-muted); font-size: 12px; }.theory-learning-points span { display: flex; align-items: flex-start; gap: 10px; line-height: 1.6; }.theory-learning-points i { display: inline-block; width: 6px; height: 6px; flex: 0 0 auto; margin-top: 6px; border-radius: 50%; background: linear-gradient(135deg, var(--theory-indigo), var(--theory-cyan)); box-shadow: 0 0 0 4px rgba(133, 137, 255, 0.1); }.theory-learning-copy .theory-primary-button { margin-top: 30px; }
.theory-learning-field { position: relative; min-height: 482px; overflow: hidden; border: 1px solid rgba(154, 169, 212, 0.28); border-radius: 24px; background: radial-gradient(circle at 50% 49%, rgba(133, 137, 255, 0.18), transparent 28%), rgba(5, 8, 16, 0.7); box-shadow: 0 34px 90px rgba(0, 0, 0, 0.28), inset 0 1px 0 rgba(255, 255, 255, 0.08); }.theory-learning-field::before, .theory-learning-field::after { position: absolute; top: 19%; bottom: 19%; width: 1px; background: linear-gradient(transparent, rgba(100, 231, 238, 0.28), transparent); content: ''; }.theory-learning-field::before { left: 11%; }.theory-learning-field::after { right: 11%; }.theory-learning-field-head, .theory-learning-field-foot { position: absolute; right: 24px; left: 24px; z-index: 5; display: flex; align-items: center; justify-content: space-between; gap: 16px; color: var(--theory-subtle); font: 700 9px ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.12em; }.theory-learning-field-head { top: 24px; }.theory-learning-field-head span:last-child { display: inline-flex; align-items: center; gap: 7px; color: #6ef0af; }.theory-learning-field-head i { width: 5px; height: 5px; border-radius: 50%; background: currentColor; }.theory-learning-field-foot { bottom: 23px; letter-spacing: 0.04em; }.theory-learning-field-foot svg { color: var(--theory-cyan); }
.theory-learning-field-body { position: absolute; top: 54px; right: 0; bottom: 50px; left: 0; }.theory-learning-orbit { position: absolute; top: 50%; left: 50%; border: 1px solid rgba(133, 137, 255, 0.36); border-radius: 50%; transform: translate(-50%, -50%) rotateX(66deg) rotateZ(-18deg); animation: theory-learning-orbit 14s linear infinite; }.theory-learning-orbit-one { width: 340px; height: 180px; }.theory-learning-orbit-two { width: 248px; height: 340px; border-color: rgba(100, 231, 238, 0.34); transform: translate(-50%, -50%) rotateY(68deg) rotateZ(20deg); animation-direction: reverse; animation-duration: 18s; }.theory-learning-orbit-three { width: 470px; height: 250px; border-color: rgba(133, 137, 255, 0.16); transform: translate(-50%, -50%) rotateX(76deg) rotateZ(42deg); animation-duration: 22s; }.theory-learning-core { position: absolute; top: 50%; left: 50%; z-index: 2; display: grid; width: 124px; height: 124px; place-content: center; justify-items: center; border: 1px solid rgba(151, 164, 255, 0.72); border-radius: 50%; color: #fff; background: linear-gradient(145deg, #3b3c9e, #0c6972); box-shadow: 0 0 0 14px rgba(133, 137, 255, 0.06), inset 0 2px 0 rgba(255, 255, 255, 0.36), 0 22px 40px rgba(0, 0, 0, 0.34); transform: translate(-50%, -50%); }.theory-learning-core::before { position: absolute; inset: -25px; border: 1px solid rgba(100, 231, 238, 0.23); border-radius: inherit; content: ''; animation: theory-learning-core-ring 9s linear infinite; }.theory-learning-core strong, .theory-learning-core small { position: relative; z-index: 1; }.theory-learning-core strong { font-size: 37px; font-weight: 820; letter-spacing: -0.1em; }.theory-learning-core small { margin-top: 5px; color: rgba(233, 242, 255, 0.72); font: 700 8px ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.12em; }.theory-learning-node { position: absolute; z-index: 4; display: flex; align-items: center; gap: 9px; min-width: 144px; padding: 10px 12px 10px 10px; border: 1px solid rgba(154, 169, 212, 0.3); border-radius: 14px; color: var(--theory-ink); background: rgba(17, 24, 42, 0.78); box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.1), 0 16px 34px rgba(0, 0, 0, 0.2); backdrop-filter: blur(10px); -webkit-backdrop-filter: blur(10px); animation: theory-learning-node 5.8s ease-in-out infinite; }.theory-learning-node:hover { border-color: rgba(100, 231, 238, 0.6); background: rgba(24, 34, 58, 0.92); }.theory-learning-node-icon { display: grid; width: 28px; height: 28px; flex: 0 0 auto; place-items: center; border-radius: 9px; color: var(--theory-cyan); background: rgba(100, 231, 238, 0.1); }.theory-learning-node > span:last-child { display: grid; gap: 3px; }.theory-learning-node strong { font-size: 11px; }.theory-learning-node small { color: var(--theory-subtle); font: 700 8px ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.03em; }.theory-learning-node-1 { top: 15%; left: 7%; animation-delay: -1.2s; }.theory-learning-node-2 { top: 12%; right: 7%; animation-delay: -3.2s; }.theory-learning-node-3 { bottom: 13%; left: 8%; animation-delay: -4.2s; }.theory-learning-node-4 { right: 7%; bottom: 12%; animation-delay: -2.1s; }

.theory-contact-section { min-height: 440px; padding: 112px 0 130px; background: var(--theory-bg); }.theory-contact-panel { display: flex; align-items: end; justify-content: space-between; gap: 54px; width: min(calc(100% - 64px), 1240px); margin: 0 auto; padding-top: 45px; border-top: 1px solid rgba(159, 171, 204, 0.32); }.theory-contact-panel h2 { max-width: 800px; margin-top: 24px; }.theory-contact-panel p { max-width: 580px; }.theory-contact-actions { display: flex; flex: 0 0 auto; flex-wrap: wrap; gap: 9px; }
.theory-footer { position: relative; z-index: 2; border-top: 1px solid var(--theory-line); background: #03050a; }.theory-footer-main { display: grid; grid-template-columns: 1.5fr repeat(3, 0.68fr); gap: 42px; width: min(calc(100% - 64px), 1240px); margin: 0 auto; padding: 50px 0 44px; }.theory-footer-brand p { max-width: 300px; margin-top: 14px; color: var(--theory-subtle); font-size: 11px; line-height: 1.75; }.theory-brand-line { display: flex; align-items: center; gap: 10px; color: var(--theory-ink); font-size: 14px; }.theory-footer-links { display: flex; flex-direction: column; align-items: flex-start; gap: 11px; }.theory-footer-links strong { margin-bottom: 4px; color: var(--theory-ink); font-size: 11px; }.theory-footer-links a, .theory-footer-links button { padding: 0; border: 0; color: var(--theory-subtle); background: transparent; font-size: 11px; transition: color 160ms ease; }.theory-footer-links a:hover, .theory-footer-links button:hover { color: var(--theory-cyan); }.theory-footer-bottom { display: flex; justify-content: space-between; gap: 20px; width: min(calc(100% - 64px), 1240px); margin: 0 auto; padding: 18px 0 22px; border-top: 1px solid var(--theory-line); color: rgba(165, 176, 201, 0.45); font-size: 10px; }.theory-back-to-top { position: fixed; right: 24px; bottom: 24px; z-index: 30; display: grid; width: 42px; height: 42px; place-items: center; border: 1px solid rgba(159, 171, 204, 0.3); border-radius: 50%; color: var(--theory-cyan); background: rgba(12, 17, 29, 0.82); box-shadow: 0 12px 30px rgba(0, 0, 0, 0.28); backdrop-filter: blur(12px); -webkit-backdrop-filter: blur(12px); transition: transform 180ms var(--ease-standard), border-color 180ms ease, background-color 180ms ease; }.theory-back-to-top:hover { border-color: rgba(100, 231, 238, 0.65); background: rgba(21, 31, 52, 0.96); transform: translateY(-3px); }

@keyframes theory-scroll-cue { 0%, 100% { transform: translateY(-2px); opacity: 0.6; } 50% { transform: translateY(3px); opacity: 1; } }
@keyframes theory-core-ring { to { transform: rotate(360deg); } }
@keyframes theory-learning-orbit { to { transform: translate(-50%, -50%) rotateX(66deg) rotateZ(342deg); } }
@keyframes theory-learning-core-ring { to { transform: rotateZ(360deg) scale(1.05); } }
@keyframes theory-learning-node { 0%, 100% { transform: translate3d(0, 0, 0); } 50% { transform: translate3d(0, -7px, 0); } }

@media (max-width: 1180px) {
  .theory-header-inner, .theory-hero-inner, .theory-section-inner, .theory-contact-panel, .theory-footer-main, .theory-footer-bottom { width: calc(100% - 48px); }
  .theory-nav-link { padding-inline: 9px; }
  .theory-hero-footer { right: 24px; left: 24px; }
  .theory-side-rail { left: 13px; }
  .theory-integrate-grid, .theory-split-section, .theory-learning-layout { gap: 48px; }
}

@media (max-width: 900px) {
  .theory-header-inner { flex-wrap: wrap; gap: 9px 16px; padding: 10px 0; }
  .theory-nav { order: 3; width: 100%; justify-content: flex-start; margin: 0; }
  .theory-actions { margin-left: auto; }
  .theory-hero { height: auto; min-height: 800px; }
  .theory-hero-inner { grid-template-columns: minmax(0, 1fr) 220px; gap: 25px; }
  .theory-hero-title { font-size: clamp(58px, 10vw, 92px); }
  .theory-telemetry { width: 210px; margin-bottom: 6%; }
  .theory-integrate-grid, .theory-split-section, .theory-learning-layout { grid-template-columns: 1fr; }
  .theory-integrate-grid { margin-top: 58px; }
  .theory-section-heading h2, .theory-observability-copy h2, .theory-learning-copy h2, .theory-contact-panel h2 { font-size: clamp(46px, 8vw, 70px); }
  .theory-observability-copy { max-width: 680px; }
  .theory-console { max-width: 780px; }
  .theory-learning-field { min-height: 450px; }
  .theory-contact-panel { align-items: flex-start; flex-direction: column; }
}

@media (max-width: 680px) {
  .theory-header-inner, .theory-hero-inner, .theory-section-inner, .theory-contact-panel, .theory-footer-main, .theory-footer-bottom { width: calc(100% - 32px); }
  .theory-header-inner { min-height: 70px; }
  .theory-brand { font-size: 13px; }
  .theory-brand-mark { width: 28px; height: 28px; }
  .theory-actions { gap: 1px; }
  .theory-action-button { width: 31px; height: 31px; }
  .theory-auth-button { min-height: 34px; padding-inline: 11px; }
  .theory-auth-label { display: none; }
  .theory-nav-link { min-height: 30px; padding-inline: 10px; font-size: 10px; }
  .theory-side-rail { display: none; }
  .theory-scene-layer { min-height: 690px; height: 760px; }
  .theory-scene-layer :deep(.hero-orbit-stage) { transform: translate3d(13vw, 0, 0) scale(1.04); }
  .theory-scene-wash { width: 100%; height: 760px; background: linear-gradient(180deg, rgba(7, 10, 18, 0.94) 0%, rgba(7, 10, 18, 0.78) 42%, rgba(7, 10, 18, 0.2) 100%); }
  .theory-scene-haze { width: 80vw; height: 80vw; }
  .theory-hero { min-height: 760px; padding: 54px 0 78px; }
  .theory-hero-inner { display: block; }
  .theory-hero-copy { max-width: 100%; }
  .theory-hero-title { margin-top: 22px; font-size: clamp(52px, 15vw, 74px); line-height: 0.92; }
  .theory-hero-subtitle { margin-top: 24px !important; font-size: 18px; }
  .theory-hero-description { font-size: 12px; line-height: 1.8; }
  .theory-hero-actions { flex-direction: column; align-items: stretch; }
  .theory-primary-button, .theory-secondary-button { width: 100%; }
  .theory-hero-facts { gap: 10px 14px; margin-top: 24px; }
  .theory-telemetry { display: none; }
  .theory-hero-footer { right: 16px; bottom: 22px; left: 16px; align-items: flex-end; font-size: 8px; }
  .theory-hero-footer > span { max-width: 132px; line-height: 1.45; }
  .theory-scroll-cue span { display: none; }
  .theory-section { min-height: auto; padding: 92px 0; }
  .theory-section-heading { margin-top: 20px; }
  .theory-section-heading h2, .theory-observability-copy h2, .theory-learning-copy h2, .theory-contact-panel h2 { font-size: clamp(42px, 12vw, 62px); }
  .theory-section-heading p, .theory-observability-copy > p, .theory-learning-copy > p, .theory-contact-panel p { margin-top: 18px; font-size: 12px; line-height: 1.8; }
  .theory-integrate-grid { gap: 38px; margin-top: 44px; }
  .theory-endpoint-row { min-height: 74px; grid-template-columns: 42px minmax(0, 1fr) 20px; gap: 9px; }
  .theory-endpoint-row div span { font-size: 10px; }
  .theory-terminal { min-height: 340px; border-radius: 16px; }
  .theory-terminal-header { padding-inline: 14px; }
  .theory-terminal pre { min-height: 245px; padding: 20px 15px; font-size: 10px; }
  .theory-terminal-footer { padding-inline: 14px; font-size: 9px; }
  .theory-route-map { display: grid; min-height: auto; grid-template-columns: 1fr; gap: 26px; margin-top: 44px; padding: 22px; }
  .theory-route-map::before, .theory-route-map::after { left: 50%; width: 1px; height: 68%; background: linear-gradient(transparent, rgba(100, 231, 238, 0.38), transparent); }
  .theory-route-map::before { top: 17%; }.theory-route-map::after { top: 17%; bottom: auto; transform: translateX(-50%) translateX(4px); }
  .theory-route-track { right: auto; left: 50%; width: 1px; height: 78px; top: 24%; background: linear-gradient(transparent, rgba(100, 231, 238, 0.48), transparent); transform: translateX(-50%); }
  .theory-route-track-two { top: auto; right: auto; bottom: 24%; left: 50%; background: linear-gradient(transparent, rgba(133, 137, 255, 0.46), transparent); transform: translateX(-50%); }
  .theory-route-node, .theory-route-core { width: 100%; }.theory-route-core { min-height: 166px; }.theory-route-core-ring { width: 150px; height: 150px; }
  .theory-feature-row { grid-template-columns: 1fr; margin-top: 30px; }.theory-feature, .theory-feature + .theory-feature { min-height: auto; padding: 20px 0; border-right: 0; border-bottom: 1px solid var(--theory-line); }.theory-feature:last-child { border-bottom: 0; }
  .theory-split-section { gap: 42px; }.theory-console { padding: 15px; border-radius: 17px; }.theory-console-title { margin-top: 28px; }.theory-console-kpis strong { font-size: 17px; }.theory-console-kpis > div { padding: 10px; }.theory-console-chart { height: 120px; }
  .theory-workflow-section { padding-bottom: 100px; }.theory-workflow-track { grid-template-columns: 1fr; margin-top: 44px; }.theory-workflow-step, .theory-workflow-step + .theory-workflow-step { min-height: 170px; padding: 21px 0; border-right: 0; border-bottom: 1px solid var(--theory-line); }.theory-workflow-step:last-child { border-bottom: 0; }.theory-workflow-step h3 { margin-top: 32px; }.theory-workflow-connector { display: none; }
  .theory-learning-section { min-height: auto; }.theory-learning-layout { gap: 42px; }.theory-learning-field { min-height: 410px; border-radius: 18px; }.theory-learning-field-head, .theory-learning-field-foot { right: 16px; left: 16px; font-size: 8px; }.theory-learning-field-body { top: 50px; }.theory-learning-orbit-one { width: 260px; height: 140px; }.theory-learning-orbit-two { width: 195px; height: 260px; }.theory-learning-orbit-three { width: 340px; height: 190px; }.theory-learning-core { width: 102px; height: 102px; }.theory-learning-core strong { font-size: 31px; }.theory-learning-node { min-width: 116px; padding: 8px; border-radius: 12px; }.theory-learning-node-icon { width: 24px; height: 24px; }.theory-learning-node strong { font-size: 10px; }.theory-learning-node small { font-size: 7px; }.theory-learning-node-1 { left: 0; }.theory-learning-node-2 { right: 0; }.theory-learning-node-3 { left: 0; }.theory-learning-node-4 { right: 0; }
  .theory-contact-section { padding: 86px 0 100px; }.theory-contact-panel { gap: 34px; padding-top: 34px; }.theory-contact-actions { width: 100%; flex-direction: column; }.theory-footer-main { grid-template-columns: 1fr 1fr; gap: 34px 20px; padding: 40px 0 34px; }.theory-footer-brand { grid-column: 1 / -1; }.theory-footer-bottom { align-items: flex-start; flex-direction: column; gap: 8px; }.theory-back-to-top { right: 16px; bottom: 16px; width: 38px; height: 38px; }
}

@media (prefers-reduced-motion: reduce) {
  .theory-home *, .theory-home *::before, .theory-home *::after { scroll-behavior: auto !important; animation-duration: 1ms !important; animation-iteration-count: 1 !important; transition-duration: 1ms !important; }
  .theory-home .home-reveal, .theory-home .home-hero-reveal { opacity: 1 !important; transform: none !important; }
  .theory-scene-layer :deep(.hero-orbit-stage) { transform: none; }
  .theory-header { backdrop-filter: none; -webkit-backdrop-filter: none; }
}
</style>
