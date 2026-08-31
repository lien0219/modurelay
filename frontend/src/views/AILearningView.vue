<template>
  <component :is="isAuthenticated ? AppLayout : 'div'" class="learning-layout">
    <div ref="pageRef" class="learning-page">
      <div class="learning-background" aria-hidden="true">
        <span class="learning-orb learning-orb-one"></span>
        <span class="learning-orb learning-orb-two"></span>
        <span class="learning-orb learning-orb-three"></span>
      </div>

      <header v-if="!isAuthenticated" class="learning-header">
        <nav class="learning-navbar" aria-label="AI learning navigation">
          <router-link to="/home" class="learning-brand">
            <span class="learning-brand-mark"><img :src="siteLogo || brand.logo" :alt="siteName" /></span>
            <span>{{ siteName }}</span>
          </router-link>

          <div class="learning-nav-links">
            <router-link to="/home">{{ copy.nav.home }}</router-link>
            <router-link to="/ai-learning" class="is-active">{{ copy.nav.learning }}</router-link>
            <router-link v-if="showModelPlazaEntry" to="/model-plaza">{{ copy.nav.modelPlaza }}</router-link>
          </div>

          <div class="learning-nav-actions">
            <LocaleSwitcher />
            <button
              type="button"
              class="learning-icon-button"
              :title="isDark ? copy.nav.light : copy.nav.dark"
              @click="toggleTheme"
            >
              <Icon v-if="isDark" name="sun" size="sm" />
              <Icon v-else name="moon" size="sm" />
            </button>
            <router-link to="/login" class="learning-solid-button">
              {{ copy.nav.login }}
              <Icon name="arrowRight" size="xs" />
            </router-link>
          </div>
        </nav>
      </header>

      <main class="learning-main" :class="{ 'learning-main-app': isAuthenticated }">
        <section class="learning-hero" data-learning-reveal>
          <div class="learning-hero-copy">
            <span class="learning-eyebrow">{{ copy.hero.eyebrow }}</span>
            <h1>{{ copy.hero.title }}</h1>
            <p class="learning-hero-subtitle">{{ copy.hero.subtitle }}</p>
            <p class="learning-hero-description">{{ copy.hero.description }}</p>

            <div class="learning-hero-tags" aria-label="Public profile scope">
              <span v-for="tag in copy.hero.tags" :key="tag"><i></i>{{ tag }}</span>
            </div>

            <div class="learning-hero-actions">
              <a href="#learning-map" class="learning-primary-button">
                {{ copy.hero.primaryCta }}
                <Icon name="arrowDown" size="sm" />
              </a>
              <router-link to="/home" class="learning-secondary-button">
                {{ copy.hero.secondaryCta }}
                <Icon name="arrowRight" size="sm" />
              </router-link>
            </div>
          </div>

          <div
            ref="heroStageRef"
            class="learning-hero-visual"
            :aria-label="copy.hero.visualLabel"
            role="img"
            @pointermove="handlePointerMove"
            @pointerleave="resetPointer"
          >
            <div class="learning-visual-grid" aria-hidden="true"></div>
            <svg class="learning-constellation" viewBox="0 0 600 520" aria-hidden="true">
              <path d="M114 128L292 254L488 114M292 254L487 390M292 254L118 394" />
              <circle cx="114" cy="128" r="4" />
              <circle cx="488" cy="114" r="4" />
              <circle cx="118" cy="394" r="4" />
              <circle cx="487" cy="390" r="4" />
            </svg>

            <div class="learning-core" aria-hidden="true">
              <span class="learning-core-halo"></span>
              <strong>AI</strong>
              <small>LEARNING CORE</small>
            </div>

            <button
              v-for="node in copy.hero.nodes"
              :key="node.id"
              type="button"
              class="learning-bubble"
              :class="[`learning-bubble-${node.id}`, { 'is-active': selectedModuleId === node.id }]"
              @click="selectModule(node.id)"
            >
              <span class="learning-bubble-shine"></span>
              <strong>{{ node.label }}</strong>
              <small>{{ node.caption }}</small>
            </button>

            <div class="learning-visual-caption">
              <span><i></i>{{ copy.hero.visualStatus }}</span>
              <small>{{ copy.hero.visualHint }}</small>
            </div>
          </div>
        </section>

        <section class="learning-stat-strip" data-learning-reveal aria-label="Profile highlights">
          <div v-for="stat in copy.stats" :key="stat.label" class="learning-stat-card">
            <strong>{{ stat.value }}</strong>
            <span>{{ stat.label }}</span>
          </div>
        </section>

        <section id="learning-map" class="learning-section learning-map-section" data-learning-reveal>
          <div class="learning-section-heading">
            <span class="learning-section-index">{{ copy.map.eyebrow }}</span>
            <h2>{{ copy.map.title }}</h2>
            <p>{{ copy.map.description }}</p>
          </div>

          <div class="learning-map-layout">
            <div class="learning-module-list" role="tablist" :aria-label="copy.map.title">
              <button
                v-for="module in copy.modules"
                :key="module.id"
                type="button"
                class="learning-module-tab"
                :class="{ 'is-active': selectedModuleId === module.id }"
                role="tab"
                :aria-selected="selectedModuleId === module.id"
                @click="selectModule(module.id)"
              >
                <span class="learning-module-icon"><Icon :name="module.icon" size="md" /></span>
                <span>
                  <strong>{{ module.label }}</strong>
                  <small>{{ module.caption }}</small>
                </span>
                <Icon name="arrowRight" size="sm" />
              </button>
            </div>

            <Transition name="learning-detail" mode="out-in">
              <article v-if="activeModule" :key="activeModule.id" class="learning-module-detail" role="tabpanel">
                <div class="learning-detail-topline">
                  <span>{{ activeModule.code }}</span>
                  <span class="learning-detail-pulse"><i></i>{{ copy.map.activeLabel }}</span>
                </div>
                <div class="learning-detail-heading">
                  <span class="learning-detail-icon"><Icon :name="activeModule.icon" size="lg" /></span>
                  <div>
                    <h3>{{ activeModule.label }}</h3>
                    <p>{{ activeModule.summary }}</p>
                  </div>
                </div>
                <div class="learning-detail-tags">
                  <span v-for="tag in activeModule.tags" :key="tag">{{ tag }}</span>
                </div>
                <ul class="learning-detail-list">
                  <li v-for="item in activeModule.points" :key="item"><i></i>{{ item }}</li>
                </ul>
              </article>
            </Transition>
          </div>
        </section>

        <section class="learning-section learning-timeline-section" data-learning-reveal>
          <div class="learning-section-heading learning-section-heading-split">
            <div>
              <span class="learning-section-index">{{ copy.timeline.eyebrow }}</span>
              <h2>{{ copy.timeline.title }}</h2>
            </div>
            <p>{{ copy.timeline.description }}</p>
          </div>

          <div class="learning-timeline">
            <article v-for="(item, index) in copy.timeline.items" :key="item.title" class="learning-timeline-card">
              <div class="learning-timeline-marker"><span>0{{ index + 1 }}</span><i></i></div>
              <div class="learning-timeline-card-body">
                <div class="learning-card-meta"><span>{{ item.period }}</span><span>{{ item.role }}</span></div>
                <h3>{{ item.title }}</h3>
                <p>{{ item.description }}</p>
                <div class="learning-card-tags"><span v-for="tag in item.tags" :key="tag">{{ tag }}</span></div>
              </div>
            </article>
          </div>
        </section>

        <section class="learning-section learning-method-section" data-learning-reveal>
          <div class="learning-section-heading">
            <span class="learning-section-index">{{ copy.method.eyebrow }}</span>
            <h2>{{ copy.method.title }}</h2>
            <p>{{ copy.method.description }}</p>
          </div>

          <div class="learning-method-grid">
            <article v-for="(item, index) in copy.method.items" :key="item.title" class="learning-method-card">
              <span class="learning-method-number">0{{ index + 1 }}</span>
              <span class="learning-method-icon"><Icon :name="item.icon" size="md" /></span>
              <h3>{{ item.title }}</h3>
              <p>{{ item.description }}</p>
            </article>
          </div>
        </section>

        <section class="learning-contact-panel" data-learning-reveal>
          <div class="learning-privacy-card">
            <span class="learning-section-index">{{ copy.privacy.eyebrow }}</span>
            <h2>{{ copy.privacy.title }}</h2>
            <p>{{ copy.privacy.description }}</p>
            <div class="learning-privacy-list">
              <span v-for="item in copy.privacy.items" :key="item"><Icon name="checkCircle" size="sm" />{{ item }}</span>
            </div>
          </div>

          <div class="learning-contact-card">
            <div class="learning-contact-avatar">A</div>
            <div class="learning-contact-copy">
              <span class="learning-contact-label">{{ copy.contact.eyebrow }}</span>
              <h3>{{ copy.contact.name }}</h3>
              <p>{{ copy.contact.role }}</p>
            </div>
            <div class="learning-contact-links">
              <a :href="`mailto:${contact.email}`"><Icon name="mail" size="sm" /><span>{{ contact.email }}</span></a>
              <a :href="`tel:+86${contact.phone.replace(/\s+/g, '')}`"><Icon name="phone" size="sm" /><span>{{ contact.phone }}</span></a>
              <a :href="contact.github" target="_blank" rel="noopener noreferrer"><Icon name="link" size="sm" /><span>Mojo MVP / GitHub</span><Icon name="externalLink" size="xs" /></a>
            </div>
          </div>
        </section>
      </main>

      <footer v-if="!isAuthenticated" class="learning-footer">
        <span>&copy; {{ currentYear }} {{ siteName }}</span>
        <span>{{ copy.footer }}</span>
      </footer>
    </div>
  </component>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { gsap } from 'gsap'
import AppLayout from '@/components/layout/AppLayout.vue'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { brand } from '@/config/brand'
import { useAppStore, useAuthStore } from '@/stores'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import { sanitizeUrl } from '@/utils/url'
import { toggleThemeWithTransition } from '@/utils/themeTransition'

type LearningIcon = 'brain' | 'cpu' | 'database' | 'beaker' | 'server' | 'sparkles' | 'shield' | 'mail' | 'phone' | 'link' | 'checkCircle'
type ModuleId = 'agent' | 'workflow' | 'memory' | 'embodied'

interface LearningNode {
  id: ModuleId
  label: string
  caption: string
}

interface LearningModule {
  id: ModuleId
  code: string
  label: string
  caption: string
  icon: LearningIcon
  summary: string
  tags: string[]
  points: string[]
}

interface LearningCopy {
  nav: { home: string; learning: string; modelPlaza: string; login: string; light: string; dark: string }
  hero: {
    eyebrow: string
    title: string
    subtitle: string
    description: string
    tags: string[]
    primaryCta: string
    secondaryCta: string
    visualLabel: string
    visualStatus: string
    visualHint: string
    nodes: LearningNode[]
  }
  stats: Array<{ value: string; label: string }>
  map: { eyebrow: string; title: string; description: string; activeLabel: string }
  modules: LearningModule[]
  timeline: {
    eyebrow: string
    title: string
    description: string
    items: Array<{ period: string; role: string; title: string; description: string; tags: string[] }>
  }
  method: {
    eyebrow: string
    title: string
    description: string
    items: Array<{ icon: LearningIcon; title: string; description: string }>
  }
  privacy: { eyebrow: string; title: string; description: string; items: string[] }
  contact: { eyebrow: string; name: string; role: string }
  footer: string
}

const zhCopy: LearningCopy = {
  nav: { home: '首页', learning: 'AI 学习', modelPlaza: '模型广场', login: '登录', light: '切换浅色模式', dark: '切换深色模式' },
  hero: {
    eyebrow: 'AI LEARNING / PUBLIC KNOWLEDGE',
    title: '把经验，变成可复用的智能系统。',
    subtitle: '一份脱敏职业档案，拆解 AI Agent 产品与系统设计的思考路径。',
    description: '从语音交互、训练数据，到具身智能与 Multi-Agent 工作流，持续关注智能系统如何理解上下文、组织协作并完成任务。',
    tags: ['已授权公开摘要', '不含住址与出生信息', '可复用的方法论'],
    primaryCta: '浏览学习地图',
    secondaryCta: '返回首页',
    visualLabel: 'AI 学习能力关系图',
    visualStatus: 'PUBLIC KNOWLEDGE / READY',
    visualHint: '点击气泡查看模块',
    nodes: [
      { id: 'agent', label: 'Agent', caption: 'Architecture' },
      { id: 'workflow', label: 'Workflow', caption: 'AI-native' },
      { id: 'memory', label: 'Memory', caption: 'Feedback' },
      { id: 'embodied', label: 'Embodied', caption: 'Intelligence' },
    ],
  },
  stats: [
    { value: '8+', label: 'AI 产品与系统实践' },
    { value: '7', label: 'Mojo 核心团队' },
    { value: '15', label: '具身智能团队' },
    { value: '32', label: 'AI 数据运营团队' },
  ],
  map: { eyebrow: '01 / LEARNING MAP', title: '一张图，看懂能力如何连成系统。', description: '把简历里的项目经验整理成四个可学习、可讨论、可继续拆解的能力模块。点击左侧节点，查看对应的实践摘要。', activeLabel: 'ACTIVE MODULE' },
  modules: [
    { id: 'agent', code: 'MODULE / 01', label: 'Agent Architecture', caption: '从规划到执行', icon: 'brain', summary: '以主 Agent 负责规划与调度，让多个专业 Agent 围绕同一任务协作。', tags: ['Multi-Agent', 'Task planning', 'Orchestration'], points: ['从 0 到 1 设计 Mojo AIGC 智能体平台。', '用项目级 AGENTS、流程级 Skill、任务级 Rule 与 Prompt 模板组织执行。', '把复杂内容生产拆成可编排、可复用、可持续推进的流程。'] },
    { id: 'workflow', code: 'MODULE / 02', label: 'AI-native Workflow', caption: '让流程持续推进', icon: 'cpu', summary: '把调研、脚本、视觉、视频和质量复核组织成一条可追踪的 AI 产线。', tags: ['Workflow', 'Skill / Rule', 'Quality loop'], points: ['覆盖调研、策划、脚本、分镜、视觉生成、视频制作与交付。', '将纠错改进、伙伴训练与外部情报获取纳入运行机制。', '用反馈优化持续提升 Agent 的任务完成质量。'] },
    { id: 'memory', code: 'MODULE / 03', label: 'Memory & Feedback', caption: '让上下文可延续', icon: 'database', summary: '关注长流程任务中的记忆、状态保持和错误复盘，让系统不止完成一次调用。', tags: ['Memory', 'Feedback', 'Evaluation'], points: ['探索连续任务中的上下文延续和多 Agent 协作。', '将错误复盘、能力训练与信息更新变成可重复的机制。', '围绕数据评估体系，建立从输入到结果的质量反馈。'] },
    { id: 'embodied', code: 'MODULE / 04', label: 'Embodied Intelligence', caption: '从模型走向现场', icon: 'beaker', summary: '把视觉感知、任务规划和机器人本体放进真实仓储场景，验证 AI 如何落地。', tags: ['Robotics', 'Vision', 'Task planning'], points: ['发起“墨工”具身智能机器人项目，面向中小仓储现实任务。', '协调具身大脑、计算机视觉、多模态感知、机器人本体与导航资源。', '从产品定位、技术路线到商业模式，推进从能运行到能落地。'] },
  ],
  timeline: {
    eyebrow: '02 / PRACTICE TRAJECTORY',
    title: '三段经历，一条清晰的系统化路径。',
    description: '从“理解一句话”，到“完成一项任务”，再到“让一组 Agent 持续协作”，关注点逐步从交互走向系统。',
    items: [
      { period: '2026.06 - 至今', role: 'AI 产线负责人', title: 'Mojo AIGC 智能体平台', description: '从 0 到 1 设计并实现面向复杂内容生产的 AI Agent 系统，建立端到端 AIGC 生产流程。', tags: ['Multi-Agent', 'AIGC', 'MVP'] },
      { period: '2024.06 - 2026.04', role: '创始人', title: '“墨工”具身智能机器人', description: '面向中小仓库的搬运、入库、盘点与分拣等现实任务，组建跨学科研发团队并探索商业化路径。', tags: ['Embodied AI', 'Robotics', 'Team building'] },
      { period: '2018.02 - 2023.08', role: 'AI 产品 / 交互设计师', title: '晓悟智能助手', description: '围绕语音交互、自然语言理解与智能任务响应，参与 50 余个细分应用场景的产品化落地。', tags: ['Voice AI', 'NLU', 'Interaction'] },
    ],
  },
  method: {
    eyebrow: '03 / WORKING PRINCIPLES',
    title: '把方法讲清楚，系统才有机会复用。',
    description: '这不是一份简历复印件，而是一组可以继续讨论、验证和迁移的工作方法。',
    items: [
      { icon: 'server', title: '先搭系统，再堆能力', description: '先定义任务边界、角色分工和反馈闭环，再让模型能力进入流程。' },
      { icon: 'sparkles', title: '让经验变成机制', description: '把个人判断沉淀为 Skill、Rule、Prompt、数据评估与协作规范。' },
      { icon: 'shield', title: '在真实场景中验证', description: '用真实用户、真实任务和可衡量结果检验 AI 是否真正完成了工作。' },
    ],
  },
  privacy: { eyebrow: 'PRIVACY BOUNDARY', title: '公开的是能力，不是隐私。', description: '本页面只展示经授权的职业摘要与公开联系方式，不上传原始简历，也不展示出生年份、住址、身份证等敏感信息。', items: ['只保留职业经历、能力模块与项目方法', '联系方式单独标注为公开联系', '所有内容均为脱敏后的展示摘要'] },
  contact: { eyebrow: 'OPEN CONTACT', name: '谢剑浩 Ango', role: 'AI Agent 产品负责人 / AI 系统设计者' },
  footer: 'Public profile summary · Built with care for privacy',
}

const enCopy: LearningCopy = {
  nav: { home: 'Home', learning: 'AI Learning', modelPlaza: 'Model Plaza', login: 'Sign in', light: 'Switch to light mode', dark: 'Switch to dark mode' },
  hero: {
    eyebrow: 'AI LEARNING / PUBLIC KNOWLEDGE',
    title: 'Turn experience into reusable intelligent systems.',
    subtitle: 'A privacy-first public profile that maps the thinking behind AI Agent products and system design.',
    description: 'From voice interaction and training data to embodied intelligence and Multi-Agent workflows, the thread is consistent: understand context, organize collaboration, and complete the task.',
    tags: ['Authorized public summary', 'No address or birth data', 'Reusable principles'],
    primaryCta: 'Explore the map',
    secondaryCta: 'Back home',
    visualLabel: 'AI learning capability map',
    visualStatus: 'PUBLIC KNOWLEDGE / READY',
    visualHint: 'Select a bubble to explore',
    nodes: [
      { id: 'agent', label: 'Agent', caption: 'Architecture' },
      { id: 'workflow', label: 'Workflow', caption: 'AI-native' },
      { id: 'memory', label: 'Memory', caption: 'Feedback' },
      { id: 'embodied', label: 'Embodied', caption: 'Intelligence' },
    ],
  },
  stats: [
    { value: '8+', label: 'Years in AI products & systems' },
    { value: '7', label: 'Mojo core team' },
    { value: '15', label: 'Embodied AI team' },
    { value: '32', label: 'AI data operations team' },
  ],
  map: { eyebrow: '01 / LEARNING MAP', title: 'See how capabilities become a system.', description: 'The resume is distilled into four learnable modules that can be discussed, tested and decomposed further. Select a module to view its practice summary.', activeLabel: 'ACTIVE MODULE' },
  modules: [
    { id: 'agent', code: 'MODULE / 01', label: 'Agent Architecture', caption: 'Plan to execution', icon: 'brain', summary: 'Let a primary Agent plan and orchestrate work while specialist Agents collaborate around one task.', tags: ['Multi-Agent', 'Task planning', 'Orchestration'], points: ['Designed Mojo from 0 to 1 as an AIGC Agent platform.', 'Structured execution with project AGENTS, process Skills, task Rules and prompt templates.', 'Turned complex content production into an orchestrated, reusable and continuous workflow.'] },
    { id: 'workflow', code: 'MODULE / 02', label: 'AI-native Workflow', caption: 'Keep work moving', icon: 'cpu', summary: 'Organize research, scripting, visuals, video and quality review into a traceable AI production line.', tags: ['Workflow', 'Skill / Rule', 'Quality loop'], points: ['Covered research, planning, scripts, storyboards, visual generation, video and delivery.', 'Brought correction, partner training and external intelligence into the operating loop.', 'Used feedback to improve Agent task quality over time.'] },
    { id: 'memory', code: 'MODULE / 03', label: 'Memory & Feedback', caption: 'Carry context forward', icon: 'database', summary: 'Explore memory, state and error review in long-running tasks so the system does more than answer once.', tags: ['Memory', 'Feedback', 'Evaluation'], points: ['Explored context continuity and Multi-Agent collaboration across ongoing tasks.', 'Made error review, capability training and information updates repeatable mechanisms.', 'Built quality feedback from input to outcome around an evaluation mindset.'] },
    { id: 'embodied', code: 'MODULE / 04', label: 'Embodied Intelligence', caption: 'Bring models to the field', icon: 'beaker', summary: 'Put vision, task planning and robot bodies into real warehouse scenarios to test how AI lands in the world.', tags: ['Robotics', 'Vision', 'Task planning'], points: ['Started the “Mogong” embodied robot project for real SMB warehouse tasks.', 'Coordinated embodied brains, computer vision, multimodal perception, robotics and navigation.', 'Moved from product definition and technical route to business validation.'] },
  ],
  timeline: {
    eyebrow: '02 / PRACTICE TRAJECTORY',
    title: 'Three chapters, one systems path.',
    description: 'The focus moved from understanding one sentence, to completing one task, to helping a group of Agents collaborate continuously.',
    items: [
      { period: '2026.06 - present', role: 'AI production lead', title: 'Mojo AIGC Agent platform', description: 'Designed and built an AI Agent system for complex content production from 0 to 1, including an end-to-end AIGC workflow.', tags: ['Multi-Agent', 'AIGC', 'MVP'] },
      { period: '2024.06 - 2026.04', role: 'Founder', title: '“Mogong” embodied robot', description: 'Built a cross-disciplinary team and explored product and commercial paths for real SMB warehouse tasks.', tags: ['Embodied AI', 'Robotics', 'Team building'] },
      { period: '2018.02 - 2023.08', role: 'AI product / interaction designer', title: 'Xiaowu intelligent assistant', description: 'Worked on voice interaction, language understanding and task response across more than 50 product scenarios.', tags: ['Voice AI', 'NLU', 'Interaction'] },
    ],
  },
  method: {
    eyebrow: '03 / WORKING PRINCIPLES',
    title: 'Make the method clear, and the system can travel.',
    description: 'This is not a resume copy. It is a small set of methods that can be discussed, tested and transferred.',
    items: [
      { icon: 'server', title: 'Build the system first', description: 'Define task boundaries, roles and feedback loops before adding model capabilities.' },
      { icon: 'sparkles', title: 'Turn experience into mechanisms', description: 'Distill judgement into Skills, Rules, prompts, evaluation and collaboration norms.' },
      { icon: 'shield', title: 'Validate in real contexts', description: 'Use real users, real tasks and measurable outcomes to test whether AI completed the work.' },
    ],
  },
  privacy: { eyebrow: 'PRIVACY BOUNDARY', title: 'Share capability, not private data.', description: 'This page shows only an authorized professional summary and public contact routes. The original resume is not uploaded, and birth data, address and identity details are excluded.', items: ['Only professional experience and methods are included', 'Contact routes are explicitly marked public', 'All content is a redacted display summary'] },
  contact: { eyebrow: 'OPEN CONTACT', name: 'Jianhao Xie / Ango', role: 'AI Agent product lead / AI systems designer' },
  footer: 'Public profile summary · Built with care for privacy',
}

const contact = {
  email: 'sxmkwc@163.com',
  phone: '136 2687 8801',
  github: 'https://github.com/angolord/mojo-system',
}

const { locale } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const pageRef = ref<HTMLElement | null>(null)
const heroStageRef = ref<HTMLElement | null>(null)
const isDark = ref(document.documentElement.classList.contains('dark'))
const selectedModuleId = ref<ModuleId>('agent')
const copy = computed(() => locale.value.startsWith('zh') ? zhCopy : enCopy)
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || brand.name)
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const isAuthenticated = computed(() => authStore.isAuthenticated)
const showModelPlazaEntry = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))
const currentYear = new Date().getFullYear()
const activeModule = computed(() => copy.value.modules.find((module) => module.id === selectedModuleId.value) || copy.value.modules[0])
let motionContext: gsap.Context | null = null
let themeObserver: MutationObserver | null = null

function toggleTheme(event?: MouseEvent) {
  isDark.value = toggleThemeWithTransition(isDark.value, event)
}

function selectModule(moduleId: ModuleId) {
  selectedModuleId.value = moduleId
  document.getElementById('learning-map')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function handlePointerMove(event: PointerEvent) {
  if (event.pointerType === 'touch') return
  const stage = heroStageRef.value
  if (!stage) return
  const rect = stage.getBoundingClientRect()
  const x = (event.clientX - rect.left) / Math.max(rect.width, 1) - 0.5
  const y = (event.clientY - rect.top) / Math.max(rect.height, 1) - 0.5
  stage.style.setProperty('--learning-rotate-x', `${y * -3.8}deg`)
  stage.style.setProperty('--learning-rotate-y', `${x * 4.8}deg`)
  stage.style.setProperty('--learning-light-x', `${50 + x * 28}%`)
  stage.style.setProperty('--learning-light-y', `${45 + y * 24}%`)
}

function resetPointer() {
  const stage = heroStageRef.value
  if (!stage) return
  stage.style.setProperty('--learning-rotate-x', '0deg')
  stage.style.setProperty('--learning-rotate-y', '0deg')
  stage.style.setProperty('--learning-light-x', '50%')
  stage.style.setProperty('--learning-light-y', '45%')
}

function initializeMotion() {
  const root = pageRef.value
  if (!root) return
  const revealElements = root.querySelectorAll<HTMLElement>('[data-learning-reveal]')
  const reducedMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
  motionContext = gsap.context(() => {
    if (reducedMotion) {
      gsap.set(revealElements, { autoAlpha: 1, y: 0 })
      return
    }
    gsap.fromTo(revealElements, { autoAlpha: 0, y: 22 }, {
      autoAlpha: 1,
      y: 0,
      duration: 0.68,
      ease: 'power3.out',
      stagger: 0.08,
      clearProps: 'transform',
    })
  }, root)
}

onMounted(() => {
  authStore.checkAuth()
  void appStore.fetchPublicSettings()
  themeObserver = new MutationObserver(() => {
    isDark.value = document.documentElement.classList.contains('dark')
  })
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
  void nextTick(initializeMotion)
})

onBeforeUnmount(() => {
  motionContext?.revert()
  motionContext = null
  themeObserver?.disconnect()
  themeObserver = null
})
</script>

<style scoped>
.learning-layout { min-height: 100vh; }
.learning-page { --learning-border: color-mix(in srgb, var(--mr-border) 76%, transparent); --learning-border-strong: color-mix(in srgb, var(--mr-border-strong) 82%, transparent); position: relative; min-height: 100vh; overflow: clip; color: var(--mr-text); background: radial-gradient(circle at 78% 8%, color-mix(in srgb, var(--mr-primary) 10%, transparent), transparent 28rem), radial-gradient(circle at 12% 42%, color-mix(in srgb, var(--mr-secondary) 8%, transparent), transparent 32rem), var(--mr-canvas); font-family: "Noto Sans SC Variable", system-ui, sans-serif; }
.learning-page::before { position: absolute; inset: 0; background-image: linear-gradient(color-mix(in srgb, var(--mr-border) 30%, transparent) 1px, transparent 1px), linear-gradient(90deg, color-mix(in srgb, var(--mr-border) 30%, transparent) 1px, transparent 1px); background-size: 32px 32px; content: ''; opacity: 0.27; pointer-events: none; mask-image: linear-gradient(to bottom, black, transparent 76%); }
.learning-background { position: absolute; inset: 0; overflow: hidden; pointer-events: none; }
.learning-orb { position: absolute; display: block; border-radius: 50%; filter: blur(2px); opacity: 0.7; mix-blend-mode: multiply; animation: learning-orb-float 12s ease-in-out infinite alternate; }
:global(.dark) .learning-orb { mix-blend-mode: screen; opacity: 0.38; }
.learning-orb-one { top: 9%; right: 8%; width: 260px; height: 260px; background: radial-gradient(circle at 32% 30%, rgba(129, 140, 248, 0.5), rgba(99, 102, 241, 0.07) 68%, transparent 70%); }
.learning-orb-two { top: 38%; left: -90px; width: 250px; height: 250px; background: radial-gradient(circle at 60% 40%, rgba(34, 211, 238, 0.3), transparent 70%); animation-delay: -4s; }
.learning-orb-three { right: 18%; bottom: 12%; width: 190px; height: 190px; background: radial-gradient(circle at 34% 34%, rgba(244, 114, 182, 0.22), transparent 70%); animation-delay: -8s; }
.learning-header { position: sticky; top: 0; z-index: 20; border-bottom: 1px solid transparent; background: color-mix(in srgb, var(--mr-canvas) 76%, transparent); box-shadow: inset 0 -1px 0 color-mix(in srgb, var(--mr-border) 34%, transparent); backdrop-filter: blur(20px) saturate(145%); -webkit-backdrop-filter: blur(20px) saturate(145%); }
.learning-navbar { display: flex; min-height: 72px; align-items: center; gap: 26px; width: min(100% - 48px, 1180px); margin: 0 auto; }
.learning-brand { display: inline-flex; min-width: max-content; align-items: center; gap: 10px; color: var(--mr-text); font-size: 15px; font-weight: 700; }
.learning-brand-mark { display: grid; width: 32px; height: 32px; place-items: center; overflow: hidden; border: 1px solid color-mix(in srgb, var(--mr-primary) 35%, transparent); border-radius: 10px; background: linear-gradient(145deg, var(--mr-primary), var(--mr-secondary)); box-shadow: 0 9px 22px color-mix(in srgb, var(--mr-primary) 22%, transparent), inset 0 1px 0 rgba(255, 255, 255, 0.4); }
.learning-brand-mark img { width: 100%; height: 100%; object-fit: contain; }
.learning-nav-links { display: flex; align-items: center; gap: 4px; margin-left: auto; padding: 4px; border: 1px solid var(--learning-border); border-radius: 999px; background: color-mix(in srgb, var(--mr-surface) 62%, transparent); box-shadow: inset 0 1px 0 var(--glass-highlight); backdrop-filter: blur(12px); -webkit-backdrop-filter: blur(12px); }
.learning-nav-links a { min-height: 32px; padding: 0 12px; border-radius: 999px; color: var(--mr-text-muted); font-size: 12px; font-weight: 600; line-height: 32px; transition: color 180ms ease, background-color 180ms ease, transform 180ms ease; }
.learning-nav-links a:hover, .learning-nav-links a.is-active { color: var(--mr-text); background: var(--mr-surface-raised); box-shadow: 0 7px 17px color-mix(in srgb, var(--mr-primary) 10%, transparent); transform: translateY(-1px); }
.learning-nav-actions { display: flex; align-items: center; gap: 5px; }
.learning-icon-button { display: inline-grid; width: 36px; height: 36px; place-items: center; border: 1px solid transparent; border-radius: 10px; color: var(--mr-text-muted); transition: color 180ms ease, background-color 180ms ease, border-color 180ms ease, transform 180ms ease; }
.learning-icon-button:hover { border-color: var(--learning-border); color: var(--mr-text); background: var(--mr-surface); transform: translateY(-1px); }
.learning-solid-button, .learning-primary-button { position: relative; display: inline-flex; min-height: 40px; align-items: center; justify-content: center; gap: 7px; overflow: hidden; padding: 0 15px; border: 1px solid color-mix(in srgb, var(--mr-primary) 55%, transparent); border-radius: 999px; color: #fff; background: linear-gradient(135deg, var(--mr-primary), var(--mr-secondary)); box-shadow: 0 12px 25px color-mix(in srgb, var(--mr-primary) 22%, transparent), inset 0 1px 0 rgba(255, 255, 255, 0.35); font-size: 12px; font-weight: 700; transition: transform 180ms ease, box-shadow 180ms ease, filter 180ms ease; }
.learning-solid-button::after, .learning-primary-button::after { position: absolute; top: -35%; left: -28%; width: 22%; height: 170%; background: rgba(255, 255, 255, 0.35); content: ''; transform: skewX(-18deg) translateX(-220%); transition: transform 540ms cubic-bezier(0.16, 1, 0.3, 1); }
.learning-solid-button:hover::after, .learning-primary-button:hover::after { transform: skewX(-18deg) translateX(670%); }
.learning-solid-button:hover, .learning-primary-button:hover { box-shadow: 0 15px 30px color-mix(in srgb, var(--mr-primary) 30%, transparent), inset 0 1px 0 rgba(255, 255, 255, 0.45); filter: saturate(1.08); transform: translateY(-2px); }
.learning-main { position: relative; z-index: 1; width: min(100% - 48px, 1180px); margin: 0 auto; }
.learning-main-app { width: 100%; max-width: 1220px; padding: 0 4px; }
.learning-hero { display: grid; min-height: min(760px, calc(100vh - 72px)); grid-template-columns: minmax(0, 0.85fr) minmax(0, 1.15fr); align-items: center; gap: 54px; padding: 82px 0 92px; }
.learning-hero-copy { max-width: 550px; }
.learning-eyebrow, .learning-section-index { color: var(--mr-primary); font-size: 11px; font-weight: 800; letter-spacing: 0.16em; }
.learning-hero h1 { max-width: 620px; margin: 18px 0 0; color: var(--mr-text); font-size: clamp(48px, 6vw, 78px); font-weight: 760; letter-spacing: -0.065em; line-height: 1.02; text-wrap: balance; }
.learning-hero h1::first-line { background: linear-gradient(100deg, var(--mr-text) 15%, var(--mr-primary) 66%, var(--mr-secondary)); -webkit-background-clip: text; background-clip: text; color: transparent; }
.learning-hero-subtitle { max-width: 520px; margin: 25px 0 0; color: var(--mr-text); font-size: clamp(18px, 2vw, 23px); font-weight: 560; line-height: 1.45; }
.learning-hero-description { max-width: 510px; margin: 14px 0 0; color: var(--mr-text-muted); font-size: 14px; line-height: 1.8; }
.learning-hero-tags { display: flex; flex-wrap: wrap; gap: 10px 16px; margin-top: 24px; color: var(--mr-text-subtle); font-size: 11px; }
.learning-hero-tags span { display: inline-flex; align-items: center; gap: 7px; }
.learning-hero-tags i, .learning-visual-caption i { display: inline-block; width: 6px; height: 6px; border-radius: 50%; background: var(--mr-success); box-shadow: 0 0 0 4px color-mix(in srgb, var(--mr-success) 12%, transparent); }
.learning-hero-actions { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 28px; }
.learning-primary-button { min-height: 44px; padding: 0 18px; }
.learning-secondary-button { display: inline-flex; min-height: 44px; align-items: center; justify-content: center; gap: 7px; padding: 0 17px; border: 1px solid var(--learning-border-strong); border-radius: 999px; color: var(--mr-text); background: color-mix(in srgb, var(--mr-surface) 74%, transparent); box-shadow: inset 0 1px 0 var(--glass-highlight); font-size: 12px; font-weight: 650; transition: transform 180ms ease, border-color 180ms ease, background-color 180ms ease; }
.learning-secondary-button:hover { border-color: var(--mr-primary); background: var(--mr-surface); transform: translateY(-2px); }
.learning-hero-visual { --learning-rotate-x: 0deg; --learning-rotate-y: 0deg; --learning-light-x: 50%; --learning-light-y: 45%; position: relative; min-height: 560px; overflow: hidden; border: 1px solid color-mix(in srgb, var(--mr-primary) 26%, var(--mr-border-strong)); border-radius: 32px; background: radial-gradient(circle at var(--learning-light-x) var(--learning-light-y), color-mix(in srgb, var(--mr-primary) 15%, transparent), transparent 30%), color-mix(in srgb, var(--mr-surface) 56%, transparent); box-shadow: inset 0 1px 0 var(--glass-highlight), 0 30px 74px color-mix(in srgb, var(--mr-primary) 13%, transparent); backdrop-filter: blur(18px) saturate(135%); -webkit-backdrop-filter: blur(18px) saturate(135%); transform: perspective(1400px) rotateX(var(--learning-rotate-x)) rotateY(var(--learning-rotate-y)); transform-style: preserve-3d; transition: transform 600ms cubic-bezier(0.16, 1, 0.3, 1), border-color 220ms ease, box-shadow 220ms ease; }
.learning-hero-visual::before { position: absolute; inset: 16px; border: 1px solid color-mix(in srgb, var(--mr-primary) 17%, transparent); border-radius: 23px; content: ''; pointer-events: none; transform: translateZ(16px); }
.learning-hero-visual::after { position: absolute; inset: 13% 12%; border-right: 1px solid color-mix(in srgb, var(--mr-secondary) 20%, transparent); border-left: 1px solid color-mix(in srgb, var(--mr-primary) 20%, transparent); content: ''; pointer-events: none; transform: translateZ(8px); }
.learning-hero-visual:hover { border-color: color-mix(in srgb, var(--mr-primary) 52%, var(--mr-border-strong)); box-shadow: inset 0 1px 0 var(--glass-highlight), 0 36px 86px color-mix(in srgb, var(--mr-primary) 20%, transparent); }
.learning-visual-grid { position: absolute; inset: 0; background-image: linear-gradient(color-mix(in srgb, var(--mr-border) 38%, transparent) 1px, transparent 1px), linear-gradient(90deg, color-mix(in srgb, var(--mr-border) 38%, transparent) 1px, transparent 1px); background-size: 38px 38px; opacity: 0.48; mask-image: radial-gradient(circle at center, black 0%, transparent 73%); }
.learning-constellation { position: absolute; inset: 0; width: 100%; height: 100%; overflow: visible; pointer-events: none; }
.learning-constellation path { fill: none; stroke: color-mix(in srgb, var(--mr-primary) 44%, transparent); stroke-width: 1.2; stroke-dasharray: 4 7; animation: learning-dash 12s linear infinite; }
.learning-constellation circle { fill: var(--mr-secondary); filter: drop-shadow(0 0 7px color-mix(in srgb, var(--mr-secondary) 60%, transparent)); }
.learning-core { position: absolute; top: 50%; left: 50%; z-index: 3; display: grid; width: 168px; height: 168px; place-content: center; justify-items: center; border: 1px solid color-mix(in srgb, var(--mr-primary) 65%, transparent); border-radius: 50%; color: #fff; background: linear-gradient(145deg, color-mix(in srgb, var(--mr-primary) 82%, #111827), color-mix(in srgb, var(--mr-secondary) 64%, #111827)); box-shadow: 0 0 0 16px color-mix(in srgb, var(--mr-primary) 7%, transparent), 0 0 0 34px color-mix(in srgb, var(--mr-secondary) 5%, transparent), 0 26px 45px rgba(0, 0, 0, 0.23), inset 0 2px 0 rgba(255, 255, 255, 0.36); transform: translate(-50%, -50%) translateZ(72px); }
.learning-core::before, .learning-core::after { position: absolute; inset: -30px; border: 1px solid color-mix(in srgb, var(--mr-primary) 28%, transparent); border-radius: 50%; content: ''; animation: learning-core-ring 9s linear infinite; }
.learning-core::after { inset: -50px; border-color: color-mix(in srgb, var(--mr-secondary) 22%, transparent); animation-direction: reverse; animation-duration: 13s; }
.learning-core-halo { position: absolute; inset: -2px; border-radius: inherit; background: radial-gradient(circle at 30% 20%, rgba(255, 255, 255, 0.42), transparent 22%), radial-gradient(circle at 70% 72%, rgba(34, 211, 238, 0.5), transparent 38%); opacity: 0.64; }
.learning-core strong, .learning-core small { position: relative; z-index: 1; }
.learning-core strong { font-size: 44px; font-weight: 800; letter-spacing: -0.09em; }
.learning-core small { margin-top: 3px; color: rgba(255, 255, 255, 0.7); font: 700 8px ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.17em; }
.learning-bubble { position: absolute; z-index: 5; display: grid; width: 136px; min-height: 84px; place-content: center; justify-items: center; gap: 4px; overflow: hidden; padding: 13px 10px; border: 1px solid color-mix(in srgb, var(--mr-primary) 34%, var(--mr-border-strong)); border-radius: 24px; color: var(--mr-text); background: linear-gradient(145deg, color-mix(in srgb, var(--mr-surface-raised) 84%, transparent), color-mix(in srgb, var(--mr-primary) 10%, transparent)); box-shadow: inset 0 1px 0 var(--glass-highlight), 0 18px 30px rgba(17, 24, 39, 0.13); backdrop-filter: blur(13px) saturate(135%); -webkit-backdrop-filter: blur(13px) saturate(135%); transform: translate(-50%, -50%) translateZ(42px); animation: learning-bubble-float 6s ease-in-out infinite; transition: border-color 180ms ease, box-shadow 180ms ease, background-color 180ms ease, transform 180ms ease; }
.learning-bubble::before { position: absolute; inset: 0; border-radius: inherit; background: radial-gradient(circle at 18% 12%, rgba(255, 255, 255, 0.52), transparent 28%); content: ''; opacity: 0.6; pointer-events: none; }
.learning-bubble-shine { position: absolute; top: -60%; left: -35%; width: 38%; height: 220%; background: rgba(255, 255, 255, 0.22); transform: rotate(25deg) translateX(-180%); transition: transform 550ms ease; }
.learning-bubble:hover .learning-bubble-shine, .learning-bubble.is-active .learning-bubble-shine { transform: rotate(25deg) translateX(510%); }
.learning-bubble:hover, .learning-bubble.is-active { border-color: color-mix(in srgb, var(--mr-secondary) 66%, var(--mr-border-strong)); box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.45), 0 22px 40px color-mix(in srgb, var(--mr-primary) 25%, transparent), 0 0 0 5px color-mix(in srgb, var(--mr-secondary) 9%, transparent); transform: translate(-50%, -50%) translateZ(72px) scale(1.04); }
.learning-bubble strong, .learning-bubble small { position: relative; z-index: 1; }
.learning-bubble strong { font-size: 14px; font-weight: 750; }
.learning-bubble small { color: var(--mr-text-subtle); font: 700 9px ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.08em; text-transform: uppercase; }
.learning-bubble-agent { top: 24%; left: 19%; animation-delay: -1s; }
.learning-bubble-workflow { top: 22%; left: 81%; animation-delay: -3.5s; }
.learning-bubble-memory { top: 76%; left: 20%; animation-delay: -4.7s; }
.learning-bubble-embodied { top: 77%; left: 81%; animation-delay: -2.2s; }
.learning-visual-caption { position: absolute; right: 28px; bottom: 24px; left: 28px; z-index: 6; display: flex; align-items: center; justify-content: space-between; gap: 12px; color: var(--mr-text-subtle); font: 700 9px ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.1em; }
.learning-visual-caption span { display: inline-flex; align-items: center; gap: 8px; color: var(--mr-success); }
.learning-visual-caption small { letter-spacing: 0.02em; }
.learning-stat-strip { display: grid; grid-template-columns: repeat(4, 1fr); gap: 1px; overflow: hidden; border: 1px solid var(--learning-border); border-radius: 20px; background: var(--learning-border); box-shadow: 0 20px 44px color-mix(in srgb, var(--mr-primary) 7%, transparent); }
.learning-stat-card { display: grid; min-height: 112px; align-content: center; gap: 8px; padding: 20px 24px; background: color-mix(in srgb, var(--mr-surface) 78%, transparent); backdrop-filter: blur(14px); -webkit-backdrop-filter: blur(14px); }
.learning-stat-card strong { color: var(--mr-text); font-size: 34px; font-weight: 740; letter-spacing: -0.05em; }
.learning-stat-card span { color: var(--mr-text-muted); font-size: 11px; }
.learning-section { padding-top: 140px; scroll-margin-top: 96px; }
.learning-section-heading { max-width: 680px; }
.learning-section-heading h2, .learning-contact-card h3, .learning-privacy-card h2 { margin: 14px 0 0; color: var(--mr-text); font-size: clamp(32px, 4vw, 54px); font-weight: 720; letter-spacing: -0.055em; line-height: 1.08; }
.learning-section-heading > p, .learning-privacy-card > p { margin: 19px 0 0; color: var(--mr-text-muted); font-size: 14px; line-height: 1.8; }
.learning-map-layout { display: grid; grid-template-columns: minmax(260px, 0.72fr) minmax(0, 1.28fr); gap: 18px; margin-top: 46px; }
.learning-module-list { display: grid; align-content: start; gap: 8px; }
.learning-module-tab { display: grid; grid-template-columns: 42px minmax(0, 1fr) 18px; gap: 12px; align-items: center; min-height: 76px; padding: 12px 14px; border: 1px solid var(--learning-border); border-radius: 17px; color: var(--mr-text-muted); background: color-mix(in srgb, var(--mr-surface) 65%, transparent); text-align: left; box-shadow: inset 0 1px 0 var(--glass-highlight); transition: color 180ms ease, border-color 180ms ease, background-color 180ms ease, transform 180ms ease, box-shadow 180ms ease; }
.learning-module-tab:hover, .learning-module-tab.is-active { color: var(--mr-text); border-color: color-mix(in srgb, var(--mr-primary) 44%, var(--mr-border-strong)); background: linear-gradient(135deg, color-mix(in srgb, var(--mr-primary) 13%, var(--mr-surface)), color-mix(in srgb, var(--mr-secondary) 7%, var(--mr-surface))); box-shadow: inset 0 1px 0 var(--glass-highlight), 0 14px 26px color-mix(in srgb, var(--mr-primary) 10%, transparent); transform: translateX(3px); }
.learning-module-icon, .learning-method-icon { display: grid; width: 38px; height: 38px; place-items: center; border: 1px solid color-mix(in srgb, var(--mr-primary) 26%, transparent); border-radius: 12px; color: var(--mr-primary); background: color-mix(in srgb, var(--mr-primary) 11%, transparent); }
.learning-module-tab > span:nth-child(2) { display: grid; min-width: 0; gap: 4px; }
.learning-module-tab strong { overflow: hidden; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.learning-module-tab small { color: var(--mr-text-subtle); font-size: 10px; }
.learning-module-tab > svg { color: var(--mr-text-subtle); transition: transform 180ms ease, color 180ms ease; }
.learning-module-tab:hover > svg, .learning-module-tab.is-active > svg { color: var(--mr-primary); transform: translateX(3px); }
.learning-module-detail { position: relative; min-height: 330px; overflow: hidden; padding: 30px; border: 1px solid color-mix(in srgb, var(--mr-primary) 30%, var(--mr-border-strong)); border-radius: 24px; background: radial-gradient(circle at 85% 8%, color-mix(in srgb, var(--mr-secondary) 15%, transparent), transparent 18rem), color-mix(in srgb, var(--mr-surface) 72%, transparent); box-shadow: inset 0 1px 0 var(--glass-highlight), 0 24px 52px color-mix(in srgb, var(--mr-primary) 10%, transparent); backdrop-filter: blur(16px) saturate(130%); -webkit-backdrop-filter: blur(16px) saturate(130%); }
.learning-module-detail::before { position: absolute; right: -70px; bottom: -90px; width: 240px; height: 240px; border: 1px solid color-mix(in srgb, var(--mr-primary) 16%, transparent); border-radius: 50%; box-shadow: 0 0 0 24px color-mix(in srgb, var(--mr-primary) 5%, transparent), 0 0 0 48px color-mix(in srgb, var(--mr-secondary) 4%, transparent); content: ''; pointer-events: none; }
.learning-detail-topline { display: flex; align-items: center; justify-content: space-between; gap: 10px; color: var(--mr-text-subtle); font: 700 9px ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.12em; }
.learning-detail-pulse { display: inline-flex; align-items: center; gap: 7px; color: var(--mr-success); letter-spacing: 0; }
.learning-detail-pulse i { width: 5px; height: 5px; border-radius: 50%; background: currentColor; box-shadow: 0 0 0 4px color-mix(in srgb, currentColor 12%, transparent); }
.learning-detail-heading { position: relative; z-index: 1; display: flex; align-items: flex-start; gap: 16px; margin-top: 38px; }
.learning-detail-icon { display: grid; width: 54px; height: 54px; flex: 0 0 auto; place-items: center; border: 1px solid color-mix(in srgb, var(--mr-secondary) 44%, transparent); border-radius: 16px; color: var(--mr-secondary); background: linear-gradient(145deg, color-mix(in srgb, var(--mr-primary) 17%, transparent), color-mix(in srgb, var(--mr-secondary) 16%, transparent)); box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.16), 0 12px 25px color-mix(in srgb, var(--mr-primary) 13%, transparent); }
.learning-detail-heading h3 { margin: 0; color: var(--mr-text); font-size: 24px; font-weight: 700; letter-spacing: -0.035em; }
.learning-detail-heading p { max-width: 560px; margin: 8px 0 0; color: var(--mr-text-muted); font-size: 13px; line-height: 1.75; }
.learning-detail-tags, .learning-card-tags { display: flex; flex-wrap: wrap; gap: 6px; }
.learning-detail-tags { position: relative; z-index: 1; margin: 24px 0 0 70px; }
.learning-detail-tags span, .learning-card-tags span { padding: 6px 9px; border: 1px solid color-mix(in srgb, var(--mr-primary) 20%, var(--mr-border)); border-radius: 999px; color: var(--mr-primary); background: color-mix(in srgb, var(--mr-primary) 8%, transparent); font: 700 10px ui-monospace, SFMono-Regular, Menlo, monospace; }
.learning-detail-list { position: relative; z-index: 1; display: grid; gap: 10px; margin: 25px 0 0 70px; padding: 0; color: var(--mr-text-muted); font-size: 12px; line-height: 1.6; list-style: none; }
.learning-detail-list li { display: flex; align-items: flex-start; gap: 8px; }
.learning-detail-list i { width: 5px; height: 5px; flex: 0 0 auto; margin-top: 7px; border-radius: 50%; background: var(--mr-secondary); }
.learning-detail-enter-active, .learning-detail-leave-active { transition: opacity 180ms ease, transform 180ms ease; }
.learning-detail-enter-from { opacity: 0; transform: translateY(8px); }
.learning-detail-leave-to { opacity: 0; transform: translateY(-8px); }
.learning-section-heading-split { display: grid; grid-template-columns: minmax(0, 1fr) minmax(280px, 0.72fr); gap: 56px; align-items: end; max-width: none; }
.learning-timeline { display: grid; grid-template-columns: repeat(3, 1fr); gap: 1px; margin-top: 46px; background: var(--learning-border); }
.learning-timeline-card { min-height: 300px; padding: 26px; background: var(--mr-surface); }
.learning-timeline-marker { display: flex; align-items: center; gap: 12px; color: var(--mr-primary); font: 700 11px ui-monospace, SFMono-Regular, Menlo, monospace; }
.learning-timeline-marker i { display: block; height: 1px; flex: 1; background: color-mix(in srgb, var(--mr-primary) 28%, transparent); }
.learning-timeline-card-body { margin-top: 44px; }
.learning-card-meta { display: flex; flex-wrap: wrap; gap: 8px 14px; color: var(--mr-text-subtle); font: 700 10px ui-monospace, SFMono-Regular, Menlo, monospace; }
.learning-card-meta span:last-child { color: var(--mr-primary); }
.learning-timeline-card h3 { margin: 13px 0 0; color: var(--mr-text); font-size: 19px; font-weight: 700; letter-spacing: -0.03em; }
.learning-timeline-card p { margin: 12px 0 0; color: var(--mr-text-muted); font-size: 12px; line-height: 1.75; }
.learning-card-tags { margin-top: 22px; }
.learning-card-tags span { color: var(--mr-text-subtle); border-color: var(--learning-border); background: transparent; font-size: 9px; }
.learning-method-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 1px; margin-top: 46px; background: var(--learning-border); }
.learning-method-card { min-height: 230px; padding: 26px; background: var(--mr-surface); }
.learning-method-number { color: var(--mr-primary); font: 700 11px ui-monospace, SFMono-Regular, Menlo, monospace; }
.learning-method-icon { margin-top: 30px; }
.learning-method-card h3 { margin: 22px 0 0; color: var(--mr-text); font-size: 16px; font-weight: 700; }
.learning-method-card p { margin: 10px 0 0; color: var(--mr-text-muted); font-size: 12px; line-height: 1.7; }
.learning-contact-panel { display: grid; grid-template-columns: minmax(0, 1fr) minmax(360px, 0.9fr); gap: 18px; padding: 140px 0 100px; }
.learning-privacy-card, .learning-contact-card { position: relative; overflow: hidden; border: 1px solid var(--learning-border-strong); border-radius: 24px; background: color-mix(in srgb, var(--mr-surface) 78%, transparent); box-shadow: inset 0 1px 0 var(--glass-highlight), 0 24px 54px color-mix(in srgb, var(--mr-primary) 8%, transparent); backdrop-filter: blur(16px) saturate(130%); -webkit-backdrop-filter: blur(16px) saturate(130%); }
.learning-privacy-card { padding: 34px; }
.learning-privacy-card::after { position: absolute; right: -70px; top: -100px; width: 260px; height: 260px; border: 1px solid color-mix(in srgb, var(--mr-secondary) 16%, transparent); border-radius: 50%; box-shadow: 0 0 0 22px color-mix(in srgb, var(--mr-secondary) 5%, transparent); content: ''; }
.learning-privacy-list { position: relative; z-index: 1; display: grid; gap: 11px; margin-top: 28px; color: var(--mr-text-muted); font-size: 12px; }
.learning-privacy-list span { display: flex; align-items: center; gap: 9px; }
.learning-privacy-list svg { flex: 0 0 auto; color: var(--mr-success); }
.learning-contact-card { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: 18px; align-content: start; padding: 30px; background: linear-gradient(145deg, color-mix(in srgb, var(--mr-primary) 14%, var(--mr-surface)), color-mix(in srgb, var(--mr-secondary) 9%, var(--mr-surface))); }
.learning-contact-avatar { display: grid; width: 56px; height: 56px; place-items: center; border: 1px solid color-mix(in srgb, var(--mr-secondary) 42%, transparent); border-radius: 18px; color: #fff; background: linear-gradient(145deg, var(--mr-primary), var(--mr-secondary)); box-shadow: 0 14px 26px color-mix(in srgb, var(--mr-primary) 24%, transparent); font-size: 23px; font-weight: 800; }
.learning-contact-copy { min-width: 0; }
.learning-contact-label { color: var(--mr-secondary); font: 700 10px ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.12em; }
.learning-contact-card h3 { margin-top: 8px; font-size: 25px; }
.learning-contact-copy p { margin: 7px 0 0; color: var(--mr-text-muted); font-size: 12px; line-height: 1.6; }
.learning-contact-links { display: grid; grid-column: 1 / -1; gap: 8px; margin-top: 15px; }
.learning-contact-links a { display: flex; min-height: 42px; align-items: center; gap: 9px; padding: 0 12px; border: 1px solid var(--learning-border); border-radius: 12px; color: var(--mr-text-muted); background: color-mix(in srgb, var(--mr-surface) 48%, transparent); font-size: 11px; transition: color 180ms ease, border-color 180ms ease, background-color 180ms ease, transform 180ms ease; }
.learning-contact-links a span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.learning-contact-links a svg:last-child { margin-left: auto; }
.learning-contact-links a:hover { border-color: var(--mr-primary); color: var(--mr-text); background: var(--mr-surface); transform: translateX(2px); }
.learning-footer { position: relative; z-index: 1; display: flex; justify-content: space-between; gap: 20px; width: min(100% - 48px, 1180px); margin: 0 auto; padding: 20px 0 26px; border-top: 1px solid var(--learning-border); color: var(--mr-text-subtle); font-size: 10px; }

@keyframes learning-orb-float { from { transform: translate3d(-12px, 6px, 0) scale(0.96); } to { transform: translate3d(12px, -10px, 0) scale(1.04); } }
@keyframes learning-dash { to { stroke-dashoffset: -120; } }
@keyframes learning-core-ring { to { transform: rotate(360deg); } }
@keyframes learning-bubble-float { 0%, 100% { margin-top: 0; } 50% { margin-top: -9px; } }

@media (max-width: 980px) {
  .learning-navbar { gap: 14px; }
  .learning-nav-links { order: 3; width: 100%; justify-content: center; margin: 0 0 6px; overflow-x: auto; }
  .learning-navbar { flex-wrap: wrap; padding: 9px 0 4px; }
  .learning-hero { grid-template-columns: minmax(0, 0.86fr) minmax(0, 1.14fr); gap: 24px; }
  .learning-hero h1 { font-size: clamp(44px, 6vw, 66px); }
  .learning-hero-visual { min-height: 500px; }
  .learning-contact-panel { grid-template-columns: 1fr; }
}

@media (max-width: 780px) {
  .learning-main { width: min(100% - 32px, 680px); }
  .learning-main-app { width: 100%; padding: 0; }
  .learning-hero { display: block; min-height: auto; padding: 62px 0 54px; }
  .learning-hero-copy { max-width: 680px; }
  .learning-hero-visual { min-height: 500px; margin-top: 28px; }
  .learning-map-layout { grid-template-columns: 1fr; }
  .learning-module-list { grid-template-columns: repeat(2, 1fr); }
  .learning-module-tab { grid-template-columns: 35px minmax(0, 1fr); }
  .learning-module-tab > svg { display: none; }
  .learning-timeline { grid-template-columns: 1fr; }
  .learning-timeline-card { min-height: auto; }
  .learning-section-heading-split { display: block; }
  .learning-section-heading-split > p { max-width: 620px; margin-top: 18px; }
  .learning-method-grid { grid-template-columns: 1fr; }
  .learning-method-card { min-height: auto; }
  .learning-contact-panel { padding-top: 92px; }
}

@media (max-width: 560px) {
  .learning-navbar, .learning-main, .learning-footer { width: calc(100% - 28px); }
  .learning-brand > span:last-child { max-width: 110px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .learning-nav-actions { margin-left: auto; }
  .learning-solid-button { width: 38px; padding: 0; font-size: 0; }
  .learning-solid-button svg { width: 16px; height: 16px; }
  .learning-nav-links { justify-content: flex-start; }
  .learning-hero h1 { font-size: clamp(40px, 13vw, 56px); }
  .learning-hero-subtitle { font-size: 18px; }
  .learning-hero-actions { flex-direction: column; align-items: stretch; }
  .learning-primary-button, .learning-secondary-button { width: 100%; }
  .learning-hero-visual { min-height: 430px; border-radius: 24px; }
  .learning-core { width: 122px; height: 122px; }
  .learning-core strong { font-size: 34px; }
  .learning-core small { font-size: 7px; }
  .learning-bubble { width: 108px; min-height: 66px; border-radius: 18px; }
  .learning-bubble strong { font-size: 12px; }
  .learning-bubble small { font-size: 8px; }
  .learning-visual-caption { right: 18px; bottom: 17px; left: 18px; font-size: 8px; }
  .learning-visual-caption small { display: none; }
  .learning-stat-strip { grid-template-columns: repeat(2, 1fr); border-radius: 16px; }
  .learning-stat-card { min-height: 95px; padding: 16px; }
  .learning-stat-card strong { font-size: 28px; }
  .learning-section { padding-top: 90px; }
  .learning-section-heading h2, .learning-privacy-card h2 { font-size: 35px; }
  .learning-module-list { grid-template-columns: 1fr; }
  .learning-module-detail { min-height: auto; padding: 22px; }
  .learning-detail-heading { margin-top: 28px; }
  .learning-detail-heading h3 { font-size: 20px; }
  .learning-detail-tags, .learning-detail-list { margin-left: 0; }
  .learning-detail-tags { margin-top: 20px; }
  .learning-detail-list { margin-top: 20px; }
  .learning-privacy-card, .learning-contact-card { padding: 23px; border-radius: 20px; }
  .learning-contact-card { grid-template-columns: 48px minmax(0, 1fr); gap: 13px; }
  .learning-contact-avatar { width: 48px; height: 48px; border-radius: 15px; }
  .learning-contact-card h3 { font-size: 21px; }
  .learning-footer { align-items: flex-start; flex-direction: column; }
}

@media (prefers-reduced-motion: reduce) {
  .learning-page *, .learning-page *::before, .learning-page *::after { animation-duration: 1ms !important; animation-iteration-count: 1 !important; transition-duration: 1ms !important; scroll-behavior: auto !important; }
  .learning-hero-visual { transform: none; }
}
</style>
