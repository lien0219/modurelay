<template>
  <component
    :is="isAuthenticated ? AppLayout : 'div'"
    class="learning-layout"
    v-bind="isAuthenticated ? { enableOnboarding: false } : {}"
  >
    <div ref="pageRef" class="learning-page">
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
            <div class="learning-hero-badge" :aria-label="copy.hero.badgeLabel">
              <span class="learning-hero-badge-icon"><Icon name="sparkles" size="sm" /></span>
              <strong>{{ copy.hero.badgeLabel }}</strong>
            </div>
            <span class="learning-eyebrow">{{ copy.hero.eyebrow }}</span>
            <h1 :aria-label="copy.hero.title">
              <span v-for="(line, index) in copy.hero.titleLines" :key="line" :class="{ 'learning-hero-title-accent': index === 0 }">{{ line }}</span>
            </h1>
            <p class="learning-hero-subtitle">{{ copy.hero.subtitle }}</p>
            <p class="learning-hero-description">{{ copy.hero.description }}</p>

            <div class="learning-hero-tags" aria-label="Public profile scope">
              <span v-for="tag in copy.hero.tags" :key="tag"><i></i>{{ tag }}</span>
            </div>

            <div v-if="availableResources.length > 1" class="learning-resource-switcher" :aria-label="copy.resourceLabel">
              <span class="learning-resource-switcher-label">{{ copy.resourceLabel }}</span>
              <div class="learning-resource-options" role="listbox" :aria-label="copy.resourceLabel">
                <button
                  v-for="resource in availableResources"
                  :key="resource.id"
                  type="button"
                  :class="{ 'is-active': selectedResourceId === resource.id }"
                  :aria-selected="selectedResourceId === resource.id"
                  role="option"
                  @click="selectResource(resource.id)"
                >
                  <span>{{ resource.label }}</span>
                  <small>{{ resource.kindLabel }}</small>
                </button>
              </div>
            </div>

            <div class="learning-hero-actions">
              <a href="#learning-map" class="learning-primary-button">
                {{ copy.hero.primaryCta }}
                <Icon name="arrowDown" size="sm" />
              </a>
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

        <section class="learning-stat-strip" data-learning-reveal aria-label="Learning resource highlights">
          <div v-for="stat in activeResourceStats" :key="stat.label" class="learning-stat-card">
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
                v-for="module in activeResourceModules"
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

        <section class="learning-section learning-experience-section" data-learning-reveal>
          <div class="learning-section-heading learning-section-heading-split">
            <div>
              <span class="learning-section-index">{{ copy.experience.eyebrow }}</span>
              <h2>{{ copy.experience.title }}</h2>
            </div>
            <p>{{ copy.experience.description }}</p>
          </div>

          <div class="learning-experience-grid">
            <article v-for="(item, index) in activeResourceExperience.items" :key="item.title" class="learning-experience-card">
              <div class="learning-experience-topline">
                <span class="learning-experience-number">0{{ index + 1 }}</span>
                <span class="learning-experience-role">{{ item.role }}</span>
              </div>
              <h3>{{ item.title }}</h3>
              <p>{{ item.description }}</p>
              <div class="learning-card-tags"><span v-for="tag in item.tags" :key="tag">{{ tag }}</span></div>
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

        <section v-if="activeContact" class="learning-contact-panel" data-learning-reveal>
          <div class="learning-contact-card">
            <div class="learning-contact-intro">
              <div class="learning-contact-avatar">{{ activeContact.initials }}</div>
              <div class="learning-contact-copy">
                <span class="learning-contact-label">{{ copy.contact.eyebrow }}</span>
                <h3>{{ activeContact.name }}</h3>
                <p>{{ activeContact.role }}</p>
              </div>
            </div>
            <div class="learning-contact-links">
              <div v-if="activeContact.email" class="learning-contact-link">
                <a :href="`mailto:${activeContact.email}`"><Icon name="mail" size="sm" /><span>{{ activeContact.email }}</span></a>
                <CopyButton :text="activeContact.email" class="learning-contact-copy-button" />
              </div>
              <div v-if="activeContact.phone" class="learning-contact-link">
                <a :href="`tel:${activeContact.phone.replace(/[^\d+]/g, '')}`"><Icon name="phone" size="sm" /><span>{{ activeContact.phone }}</span></a>
                <CopyButton :text="activeContact.phone" class="learning-contact-copy-button" />
              </div>
              <div v-if="activeContact.github" class="learning-contact-link">
                <a :href="activeContact.github" target="_blank" rel="noopener noreferrer"><Icon name="link" size="sm" /><span>{{ activeContact.githubLabel || 'GitHub' }}</span><Icon name="externalLink" size="xs" /></a>
                <CopyButton :text="activeContact.github" class="learning-contact-copy-button" />
              </div>
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
import CopyButton from '@/components/common/CopyButton.vue'
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

interface LearningContact {
  initials: string
  name: string
  role: string
  email?: string
  phone?: string
  github?: string
  githubLabel?: string
}

interface LearningCopy {
  resourceLabel: string
  nav: { home: string; learning: string; modelPlaza: string; login: string; light: string; dark: string }
  hero: {
    eyebrow: string
    title: string
    titleLines: string[]
    badgeLabel: string
    subtitle: string
    description: string
    tags: string[]
    primaryCta: string
    visualLabel: string
    visualStatus: string
    visualHint: string
    nodes: LearningNode[]
  }
  stats: Array<{ value: string; label: string }>
  map: { eyebrow: string; title: string; description: string; activeLabel: string }
  modules: LearningModule[]
  experience: {
    eyebrow: string
    title: string
    description: string
    items: Array<{ role: string; title: string; description: string; tags: string[] }>
  }
  method: {
    eyebrow: string
    title: string
    description: string
    items: Array<{ icon: LearningIcon; title: string; description: string }>
  }
  contact: { eyebrow: string; name: string; role: string }
  footer: string
}

interface LearningResource {
  id: string
  kind: 'profile' | 'course' | 'document' | 'collection'
  label: string
  kindLabel: string
  stats: LearningCopy['stats']
  modules: LearningModule[]
  experience: LearningCopy['experience']
  contact?: LearningContact
}

const zhCopy: LearningCopy = {
  resourceLabel: '学习资源',
  nav: { home: '首页', learning: 'AI 学习', modelPlaza: '模型广场', login: '登录', light: '切换浅色模式', dark: '切换深色模式' },
  hero: {
    eyebrow: 'AI LEARNING / PUBLIC KNOWLEDGE',
    title: '让经验真正帮到人',
    titleLines: ['让经验真', '正帮到人'],
    badgeLabel: '合作讲师',
    subtitle: '面向老师与技术专家的 AI 学习空间',
    description: '把多年 AI 产品、Agent 系统与具身智能实践，整理成看得懂的案例、学得会的方法和做得出的练习。',
    tags: ['公开职业摘要', '隐私信息已排除', '案例与方法可复用'],
    primaryCta: '浏览学习地图',
    visualLabel: 'AI 学习能力关系图',
    visualStatus: '学习内容已整理',
    visualHint: '点击任意主题开始了解',
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
  map: { eyebrow: '01 / LEARNING MAP', title: '从理解原理到做出结果', description: '围绕四个主题，逐步建立从概念理解、任务拆解到系统落地的能力。每个主题都包含真实项目中的判断、方法和可练习的切入点。', activeLabel: '当前主题' },
  modules: [
    { id: 'agent', code: '核心能力', label: 'Agent 设计', caption: '把问题拆清楚', icon: 'brain', summary: '学会把一个复杂目标拆成角色、步骤和协作关系，让 AI 从会回答走向能完成任务。', tags: ['任务拆解', '多 Agent 协作', '执行闭环'], points: ['如何设计主 Agent 与专业 Agent 的分工。', '如何把任务拆成可执行的步骤与检查点。', '如何把一次成功沉淀成下一次可复用的方法。'] },
    { id: 'workflow', code: '工作方法', label: 'AI 工作流', caption: '让过程稳定复用', icon: 'cpu', summary: '把调研、创作、审核和交付串成清晰流程，让 AI 能在真实工作里持续产出。', tags: ['流程设计', '工具协作', '持续改进'], points: ['从需求到交付，明确每一步的输入与结果。', '用工具、规则和模板减少重复沟通。', '用复盘让流程越用越顺。'] },
    { id: 'memory', code: '持续改进', label: '记忆与反馈', caption: '让系统持续变好', icon: 'database', summary: '让 AI 记住重要上下文，也能从错误和反馈中调整下一次行动。', tags: ['上下文', '反馈机制', '结果评估'], points: ['保留长期任务真正需要的上下文。', '把错误复盘变成可执行的改进动作。', '用清晰的评估标准判断结果好不好。'] },
    { id: 'embodied', code: '现场实践', label: '具身智能', caption: '把能力带到现场', icon: 'beaker', summary: '从视觉、规划到机器人执行，理解 AI 如何在真实环境中感知、判断并完成动作。', tags: ['真实场景', '视觉与规划', '产品落地'], points: ['用真实仓储任务验证 AI 能否落地。', '理解感知、规划与执行之间的配合。', '从产品目标出发平衡技术与实际价值。'] },
  ],
  experience: {
    eyebrow: '02 / PROJECT EXPERIENCE',
    title: '三类真实项目，三种能力落点',
    description: '这些项目不是时间线，而是三种可迁移的实践：把产品做出来，把复杂流程跑起来，把技术带到真实场景。',
    items: [
      { role: 'AI 产线负责人', title: 'Mojo AIGC 智能体平台', description: '把调研、脚本、视觉与交付组织成一条可协作的 AI 生产流程，让复杂内容工作从灵感走向稳定交付。', tags: ['内容生产', 'Agent 协作'] },
      { role: '项目发起与产品负责人', title: '“墨工”具身智能机器人', description: '面向中小仓储的搬运、入库、盘点和分拣，把多模态感知、任务规划与机器人本体放进真实场景验证。', tags: ['具身智能', '真实场景'] },
      { role: 'AI 产品与交互设计', title: '晓悟智能助手', description: '围绕语音交互、自然语言理解和任务响应，推动 AI 能力进入 50 余个可使用的产品场景。', tags: ['语音交互', '产品化'] },
    ],
  },
  method: {
    eyebrow: '03 / WORKING PRINCIPLES',
    title: '把会做的事，变成会教的方法',
    description: '适合老师，也适合技术专家：先理解，再练习，最后用真实结果验证。',
    items: [
      { icon: 'server', title: '讲清楚', description: '把复杂概念拆成容易理解的例子、步骤和练习。' },
      { icon: 'sparkles', title: '做出来', description: '用 Agent、工作流和工具把方法放进真实任务。' },
      { icon: 'shield', title: '复盘提升', description: '根据结果和反馈调整方法，让能力可以持续进步。' },
    ],
  },
  contact: { eyebrow: 'OPEN CONTACT', name: '谢剑浩 Ango', role: 'AI Agent 产品负责人 / AI 系统设计者' },
  footer: 'Public profile summary · Built with care for privacy',
}

const enCopy: LearningCopy = {
  nav: { home: 'Home', learning: 'AI Learning', modelPlaza: 'Model Plaza', login: 'Sign in', light: 'Switch to light mode', dark: 'Switch to dark mode' },
  resourceLabel: 'Learning resources',
  hero: {
    eyebrow: 'AI LEARNING / PUBLIC KNOWLEDGE',
    title: 'Make experience useful to people',
    titleLines: ['Make experience useful to people'],
    badgeLabel: 'Collaborating instructor',
    subtitle: 'An AI learning space for teachers and technical experts',
    description: 'Real AI product, Agent system and embodied intelligence practice, shaped into clear cases, practical methods and exercises you can use.',
    tags: ['Public professional summary', 'Private data excluded', 'Reusable cases and methods'],
    primaryCta: 'Explore the map',
    visualLabel: 'AI learning capability map',
    visualStatus: 'LEARNING CONTENT READY',
    visualHint: 'Select a topic to begin',
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
  map: { eyebrow: '01 / LEARNING MAP', title: 'From understanding principles to making results', description: 'Four connected topics build capability step by step, from understanding concepts and breaking down tasks to putting a system into the real world. Each topic starts with a judgement, method or practice from an actual project.', activeLabel: 'CURRENT TOPIC' },
  modules: [
    { id: 'agent', code: 'Core capability', label: 'Agent design', caption: 'Break the problem down', icon: 'brain', summary: 'Break a complex goal into roles, steps and collaboration so AI moves from giving answers to completing useful work.', tags: ['Task breakdown', 'Agent collaboration', 'Execution loop'], points: ['Design a lead Agent and specialist Agents with clear responsibilities.', 'Turn a goal into executable steps and checkpoints.', 'Capture a successful approach so it can be reused next time.'] },
    { id: 'workflow', code: 'Working method', label: 'AI workflow', caption: 'Make the process repeatable', icon: 'cpu', summary: 'Connect research, creation, review and delivery into a clear process that keeps producing in real work.', tags: ['Process design', 'Tool collaboration', 'Continuous improvement'], points: ['Define the input and outcome for every step from request to delivery.', 'Use tools, rules and templates to remove repetitive communication.', 'Use review to make the workflow smoother each time.'] },
    { id: 'memory', code: 'Continuous improvement', label: 'Memory & feedback', caption: 'Help the system improve', icon: 'database', summary: 'Keep the context that matters and turn mistakes and feedback into better next actions.', tags: ['Context', 'Feedback', 'Outcome review'], points: ['Keep the context that an ongoing task actually needs.', 'Turn error review into a concrete improvement action.', 'Use clear criteria to decide whether the result is good.'] },
    { id: 'embodied', code: 'Field practice', label: 'Embodied intelligence', caption: 'Bring ability to the field', icon: 'beaker', summary: 'See how AI senses, decides and acts in a real environment through vision, planning and robotics.', tags: ['Real-world use', 'Vision and planning', 'Product delivery'], points: ['Test whether AI can help with real warehouse work.', 'Understand how perception, planning and execution work together.', 'Balance technical ambition with user value and product reality.'] },
  ],
  experience: {
    eyebrow: '02 / PROJECT EXPERIENCE',
    title: 'Three real projects three kinds of capability',
    description: 'Three transferable ways of working: bring a product to life, make a complex process run, and take technology into the field.',
    items: [
      { role: 'AI production lead', title: 'Mojo AIGC Agent platform', description: 'Organized research, scripts, visuals and delivery into a collaborative AI production process, helping complex content work move from ideas to steady delivery.', tags: ['Content production', 'Agent collaboration'] },
      { role: 'Founder and product lead', title: 'Mogong embodied robot', description: 'Put multimodal perception, task planning and robotics into real SMB warehouse work such as handling, stocking, counting and sorting.', tags: ['Embodied AI', 'Real-world use'] },
      { role: 'AI product and interaction design', title: 'Xiaowu intelligent assistant', description: 'Moved voice interaction, language understanding and task response into more than 50 usable product scenarios.', tags: ['Voice interaction', 'Product delivery'] },
    ],
  },
  method: {
    eyebrow: '03 / WORKING PRINCIPLES',
    title: 'Turn what you can do into what you can teach',
    description: 'Useful for teachers and technical experts: understand first, practise next, then use a real result to verify the method.',
    items: [
      { icon: 'server', title: 'Make it clear', description: 'Turn a complex idea into examples, steps and practice people can understand.' },
      { icon: 'sparkles', title: 'Make it work', description: 'Put the method into real tasks with Agents, workflows and tools.' },
      { icon: 'shield', title: 'Review and improve', description: 'Use outcomes and feedback to refine the method and keep the capability growing.' },
    ],
  },
  contact: { eyebrow: 'OPEN CONTACT', name: 'Jianhao Xie / Ango', role: 'AI Agent product lead / AI systems designer' },
  footer: 'Public profile summary · Built with care for privacy',
}

const publicContact = {
  email: 'sxmkwc@163.com',
  phone: '136 2687 8801',
  github: 'https://github.com/angolord/mojo-system',
}

const zhResources: LearningResource[] = [
  {
    id: 'ango-profile',
    kind: 'profile',
    label: '谢剑浩 Ango',
    kindLabel: '个人实践',
    stats: zhCopy.stats,
    modules: zhCopy.modules,
    experience: zhCopy.experience,
    contact: { initials: 'A', name: zhCopy.contact.name, role: zhCopy.contact.role, ...publicContact, githubLabel: 'Mojo MVP / GitHub' },
  },
]

const enResources: LearningResource[] = [
  {
    id: 'ango-profile',
    kind: 'profile',
    label: 'Jianhao Xie / Ango',
    kindLabel: 'Professional practice',
    stats: enCopy.stats,
    modules: enCopy.modules,
    experience: enCopy.experience,
    contact: { initials: 'A', name: enCopy.contact.name, role: enCopy.contact.role, ...publicContact, githubLabel: 'Mojo MVP / GitHub' },
  },
]

const { locale } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const pageRef = ref<HTMLElement | null>(null)
const heroStageRef = ref<HTMLElement | null>(null)
const isDark = ref(document.documentElement.classList.contains('dark'))
const selectedResourceId = ref('ango-profile')
const selectedModuleId = ref<ModuleId>('agent')
const copy = computed(() => locale.value.startsWith('zh') ? zhCopy : enCopy)
const availableResources = computed(() => locale.value.startsWith('zh') ? zhResources : enResources)
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || brand.name)
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const isAuthenticated = computed(() => authStore.isAuthenticated)
const showModelPlazaEntry = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))
const currentYear = new Date().getFullYear()
const activeResource = computed(() => availableResources.value.find((resource) => resource.id === selectedResourceId.value) || availableResources.value[0])
const activeResourceStats = computed(() => activeResource.value?.stats ?? copy.value.stats)
const activeResourceModules = computed(() => activeResource.value?.modules ?? copy.value.modules)
const activeResourceExperience = computed(() => activeResource.value?.experience ?? copy.value.experience)
const activeContact = computed(() => activeResource.value?.contact)
const activeModule = computed(() => activeResourceModules.value.find((module) => module.id === selectedModuleId.value) || activeResourceModules.value[0])
let motionContext: gsap.Context | null = null
let themeObserver: MutationObserver | null = null

function toggleTheme(event?: MouseEvent) {
  isDark.value = toggleThemeWithTransition(isDark.value, event)
}

function selectModule(moduleId: ModuleId) {
  selectedModuleId.value = moduleId
  document.getElementById('learning-map')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function selectResource(resourceId: string) {
  selectedResourceId.value = resourceId
  selectedModuleId.value = activeResourceModules.value[0]?.id || 'agent'
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
.learning-page { --learning-border: color-mix(in srgb, var(--mr-border) 76%, transparent); --learning-border-strong: color-mix(in srgb, var(--mr-border-strong) 82%, transparent); position: relative; min-height: 100vh; overflow: clip; color: var(--mr-text); background-color: var(--mr-canvas); font-family: "Noto Sans SC Variable", system-ui, sans-serif; }
.learning-header { position: sticky; top: 0; z-index: 20; border-bottom: 1px solid transparent; background: var(--mr-surface); box-shadow: inset 0 -1px 0 color-mix(in srgb, var(--mr-border) 34%, transparent); }
.learning-navbar { display: flex; min-height: 72px; align-items: center; gap: 26px; width: min(100% - 48px, 1180px); margin: 0 auto; }
.learning-brand { display: inline-flex; min-width: max-content; align-items: center; gap: 10px; color: var(--mr-text); font-size: 15px; font-weight: 700; }
.learning-brand-mark { display: grid; width: 32px; height: 32px; place-items: center; overflow: hidden; border: 1px solid color-mix(in srgb, var(--mr-primary) 35%, transparent); border-radius: 10px; background: linear-gradient(145deg, var(--mr-primary), var(--mr-secondary)); box-shadow: 0 9px 22px color-mix(in srgb, var(--mr-primary) 22%, transparent), inset 0 1px 0 rgba(255, 255, 255, 0.4); }
.learning-brand-mark img { width: 100%; height: 100%; object-fit: contain; }
.learning-nav-links { display: flex; align-items: center; gap: 4px; margin-left: auto; padding: 4px; border: 1px solid var(--learning-border); border-radius: 999px; background: var(--mr-surface-raised); box-shadow: inset 0 1px 0 var(--glass-highlight); }
.learning-nav-links a { display: inline-flex; min-height: 32px; align-items: center; justify-content: center; padding: 0 12px; border-radius: 999px; color: var(--mr-text-muted); font-size: 12px; font-weight: 600; line-height: 1; white-space: nowrap; transition: color 180ms ease, background-color 180ms ease, transform 180ms ease; }
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
.learning-hero-badge { display: flex; width: fit-content; max-width: 100%; align-items: center; gap: 9px; min-height: 34px; margin-bottom: 14px; padding: 0 13px 0 7px; border: 1px solid color-mix(in srgb, var(--mr-primary) 40%, var(--learning-border)); border-radius: 999px; color: var(--mr-primary); background: color-mix(in srgb, var(--mr-primary) 10%, var(--mr-surface)); box-shadow: inset 0 1px 0 var(--glass-highlight), 0 9px 20px color-mix(in srgb, var(--mr-primary) 9%, transparent); font-size: 12px; font-weight: 750; }
.learning-hero-badge-icon { display: inline-grid; width: 22px; height: 22px; place-items: center; border-radius: 50%; color: #fff; background: var(--mr-primary); }
.learning-eyebrow, .learning-section-index { color: var(--mr-primary); font-size: 11px; font-weight: 800; letter-spacing: 0.16em; }
.learning-hero h1 { display: block; max-width: 620px; margin: 18px 0 0; color: var(--mr-text); font-size: clamp(48px, 6vw, 78px); font-weight: 760; letter-spacing: 0; line-height: 1.2; overflow-wrap: normal; }
.learning-hero h1 > span { display: block; }
.learning-hero-title-accent { background: linear-gradient(100deg, var(--mr-text) 15%, var(--mr-primary) 66%, var(--mr-secondary)); -webkit-background-clip: text; background-clip: text; color: transparent; }
.learning-hero-subtitle { max-width: 520px; margin: 25px 0 0; color: var(--mr-text); font-size: clamp(18px, 2vw, 23px); font-weight: 560; line-height: 1.45; }
.learning-hero-description { max-width: 510px; margin: 14px 0 0; color: var(--mr-text-muted); font-size: 14px; line-height: 1.8; }
.learning-hero-tags { display: flex; flex-wrap: wrap; gap: 10px 16px; margin-top: 24px; color: var(--mr-text-subtle); font-size: 11px; }
.learning-hero-tags span { display: inline-flex; align-items: center; gap: 7px; }
.learning-hero-tags i, .learning-visual-caption i { display: inline-block; width: 6px; height: 6px; border-radius: 50%; background: var(--mr-success); box-shadow: 0 0 0 4px color-mix(in srgb, var(--mr-success) 12%, transparent); }
.learning-resource-switcher { display: grid; gap: 9px; margin-top: 22px; }
.learning-resource-switcher-label { color: var(--mr-text-subtle); font-size: 10px; font-weight: 750; letter-spacing: 0.08em; text-transform: uppercase; }
.learning-resource-options { display: flex; flex-wrap: wrap; gap: 7px; }
.learning-resource-options button { display: inline-flex; align-items: center; gap: 8px; min-height: 34px; padding: 0 11px; border: 1px solid var(--learning-border); border-radius: 999px; color: var(--mr-text-muted); background: color-mix(in srgb, var(--mr-surface) 54%, transparent); font-size: 11px; transition: color 180ms ease, border-color 180ms ease, background-color 180ms ease, transform 180ms ease; }
.learning-resource-options button small { color: var(--mr-text-subtle); font-size: 9px; }
.learning-resource-options button:hover, .learning-resource-options button.is-active { border-color: color-mix(in srgb, var(--mr-primary) 46%, var(--learning-border-strong)); color: var(--mr-text); background: color-mix(in srgb, var(--mr-primary) 10%, var(--mr-surface)); transform: translateY(-1px); }
.learning-hero-actions { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 28px; }
.learning-primary-button { min-height: 44px; padding: 0 18px; }
.learning-hero-visual { --learning-rotate-x: 0deg; --learning-rotate-y: 0deg; --learning-light-x: 50%; --learning-light-y: 45%; position: relative; min-height: 560px; overflow: hidden; border: 1px solid color-mix(in srgb, var(--mr-primary) 26%, var(--mr-border-strong)); border-radius: 32px; background: radial-gradient(circle at var(--learning-light-x) var(--learning-light-y), color-mix(in srgb, var(--mr-primary) 15%, transparent), transparent 30%), color-mix(in srgb, var(--mr-surface) 94%, transparent); box-shadow: inset 0 1px 0 var(--glass-highlight), 0 30px 74px color-mix(in srgb, var(--mr-primary) 13%, transparent); transform: perspective(1400px) rotateX(var(--learning-rotate-x)) rotateY(var(--learning-rotate-y)); transform-style: preserve-3d; transition: transform 600ms cubic-bezier(0.16, 1, 0.3, 1), border-color 220ms ease, box-shadow 220ms ease; }
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
.learning-bubble { position: absolute; z-index: 5; display: grid; width: 136px; min-height: 84px; place-content: center; justify-items: center; gap: 4px; overflow: hidden; padding: 13px 10px; border: 1px solid color-mix(in srgb, var(--mr-primary) 34%, var(--mr-border-strong)); border-radius: 24px; color: var(--mr-text); background: linear-gradient(145deg, var(--mr-surface-raised), color-mix(in srgb, var(--mr-primary) 10%, var(--mr-surface-raised))); box-shadow: inset 0 1px 0 var(--glass-highlight), 0 18px 30px rgba(17, 24, 39, 0.13); transform: translate(-50%, -50%) translateZ(42px); animation: learning-bubble-float 6s ease-in-out infinite; transition: border-color 180ms ease, box-shadow 180ms ease, background-color 180ms ease, transform 180ms ease; }
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
.learning-stat-card { display: grid; min-height: 112px; align-content: center; gap: 8px; padding: 20px 24px; background: var(--mr-surface); }
.learning-stat-card strong { color: var(--mr-text); font-size: 34px; font-weight: 740; letter-spacing: -0.05em; }
.learning-stat-card span { color: var(--mr-text-muted); font-size: 11px; }
.learning-section { padding-top: 140px; scroll-margin-top: 96px; }
.learning-section-heading { max-width: 680px; }
.learning-section-heading h2, .learning-contact-card h3 { margin: 14px 0 0; color: var(--mr-text); font-size: clamp(32px, 4vw, 54px); font-weight: 720; letter-spacing: -0.055em; line-height: 1.08; }
.learning-section-heading > p { margin: 19px 0 0; color: var(--mr-text-muted); font-size: 14px; line-height: 1.8; }
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
.learning-module-detail { position: relative; min-height: 330px; overflow: hidden; padding: 30px; border: 1px solid color-mix(in srgb, var(--mr-primary) 30%, var(--mr-border-strong)); border-radius: 24px; background: radial-gradient(circle at 85% 8%, color-mix(in srgb, var(--mr-secondary) 15%, transparent), transparent 18rem), color-mix(in srgb, var(--mr-surface) 96%, transparent); box-shadow: inset 0 1px 0 var(--glass-highlight), 0 24px 52px color-mix(in srgb, var(--mr-primary) 10%, transparent); }
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
.learning-experience-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 14px; margin-top: 46px; }
.learning-experience-card { position: relative; min-height: 300px; overflow: hidden; padding: 27px; border: 1px solid var(--learning-border); border-radius: 22px; background: linear-gradient(145deg, color-mix(in srgb, var(--mr-surface-raised) 82%, transparent), color-mix(in srgb, var(--mr-primary) 6%, var(--mr-surface))); box-shadow: inset 0 1px 0 var(--glass-highlight), 0 18px 36px color-mix(in srgb, var(--mr-primary) 7%, transparent); transition: transform 220ms ease, border-color 220ms ease, box-shadow 220ms ease; }
.learning-experience-card::before { position: absolute; top: 0; right: 24px; left: 24px; height: 2px; background: linear-gradient(90deg, var(--mr-primary), var(--mr-secondary), transparent); content: ''; opacity: 0.7; }
.learning-experience-card::after { position: absolute; right: -72px; bottom: -92px; width: 220px; height: 220px; border: 1px solid color-mix(in srgb, var(--mr-primary) 12%, transparent); border-radius: 50%; box-shadow: 0 0 0 20px color-mix(in srgb, var(--mr-secondary) 4%, transparent); content: ''; pointer-events: none; }
.learning-experience-card:hover { border-color: color-mix(in srgb, var(--mr-primary) 42%, var(--learning-border)); box-shadow: inset 0 1px 0 var(--glass-highlight), 0 24px 44px color-mix(in srgb, var(--mr-primary) 13%, transparent); transform: translateY(-5px); }
.learning-experience-topline { position: relative; z-index: 1; display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.learning-experience-number { color: var(--mr-primary); font: 700 11px ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.12em; }
.learning-experience-role { color: var(--mr-text-subtle); font-size: 11px; font-weight: 650; text-align: right; }
.learning-experience-card h3 { position: relative; z-index: 1; margin: 54px 0 0; color: var(--mr-text); font-size: 20px; font-weight: 720; letter-spacing: -0.035em; }
.learning-experience-card p { position: relative; z-index: 1; min-height: 86px; margin: 13px 0 0; color: var(--mr-text-muted); font-size: 12px; line-height: 1.78; }
.learning-card-tags { position: relative; z-index: 1; margin-top: 22px; }
.learning-card-tags span { color: var(--mr-text-subtle); border-color: var(--learning-border); background: transparent; font-size: 9px; }
.learning-method-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 1px; margin-top: 46px; background: var(--learning-border); }
.learning-method-card { min-height: 230px; padding: 26px; background: var(--mr-surface); }
.learning-method-number { color: var(--mr-primary); font: 700 11px ui-monospace, SFMono-Regular, Menlo, monospace; }
.learning-method-icon { margin-top: 30px; }
.learning-method-card h3 { margin: 22px 0 0; color: var(--mr-text); font-size: 16px; font-weight: 700; }
.learning-method-card p { margin: 10px 0 0; color: var(--mr-text-muted); font-size: 12px; line-height: 1.7; }
.learning-contact-panel { padding: 112px 0 92px; }
.learning-contact-card { position: relative; display: grid; width: 100%; grid-template-columns: minmax(280px, 0.72fr) minmax(0, 1.28fr); gap: 36px; align-items: center; overflow: hidden; padding: 34px 36px; border: 1px solid var(--learning-border-strong); border-radius: 24px; background: linear-gradient(145deg, color-mix(in srgb, var(--mr-primary) 14%, var(--mr-surface)), color-mix(in srgb, var(--mr-secondary) 9%, var(--mr-surface))); box-shadow: inset 0 1px 0 var(--glass-highlight), 0 24px 54px color-mix(in srgb, var(--mr-primary) 8%, transparent); }
.learning-contact-card::after { position: absolute; right: -72px; top: -114px; width: 250px; height: 250px; border: 1px solid color-mix(in srgb, var(--mr-secondary) 18%, transparent); border-radius: 50%; box-shadow: 0 0 0 24px color-mix(in srgb, var(--mr-secondary) 5%, transparent); content: ''; pointer-events: none; }
.learning-contact-intro { position: relative; z-index: 1; display: flex; align-items: flex-start; gap: 18px; }
.learning-contact-avatar { display: grid; width: 56px; height: 56px; place-items: center; border: 1px solid color-mix(in srgb, var(--mr-secondary) 42%, transparent); border-radius: 18px; color: #fff; background: linear-gradient(145deg, var(--mr-primary), var(--mr-secondary)); box-shadow: 0 14px 26px color-mix(in srgb, var(--mr-primary) 24%, transparent); font-size: 23px; font-weight: 800; }
.learning-contact-copy { min-width: 0; }
.learning-contact-label { color: var(--mr-secondary); font: 700 10px ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: 0.12em; }
.learning-contact-card h3 { margin-top: 8px; font-size: 25px; }
.learning-contact-copy p { margin: 7px 0 0; color: var(--mr-text-muted); font-size: 12px; line-height: 1.6; }
.learning-contact-links { position: relative; z-index: 1; display: grid; gap: 8px; }
.learning-contact-link { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 6px; align-items: center; }
.learning-contact-link > a { display: flex; min-height: 42px; min-width: 0; align-items: center; gap: 9px; padding: 0 12px; border: 1px solid var(--learning-border); border-radius: 12px; color: var(--mr-text-muted); background: color-mix(in srgb, var(--mr-surface) 48%, transparent); font-size: 11px; transition: color 180ms ease, border-color 180ms ease, background-color 180ms ease, transform 180ms ease; }
.learning-contact-link > a span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.learning-contact-link > a svg:last-child { margin-left: auto; }
.learning-contact-link > a:hover { border-color: var(--mr-primary); color: var(--mr-text); background: var(--mr-surface); transform: translateX(2px); }
.learning-contact-copy-button { display: inline-grid; width: 34px; height: 34px; place-items: center; border: 1px solid var(--learning-border); border-radius: 10px; color: var(--mr-text-muted); background: color-mix(in srgb, var(--mr-surface) 52%, transparent); }
.learning-contact-copy-button:hover { border-color: var(--mr-primary); color: var(--mr-primary); background: var(--mr-surface); }
.learning-footer { position: relative; z-index: 1; display: flex; justify-content: space-between; gap: 20px; width: min(100% - 48px, 1180px); margin: 0 auto; padding: 20px 0 26px; border-top: 1px solid var(--learning-border); color: var(--mr-text-subtle); font-size: 10px; }

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
  .learning-experience-grid { grid-template-columns: 1fr; }
  .learning-experience-card { min-height: auto; }
  .learning-experience-card p { min-height: auto; }
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
  .learning-primary-button { width: 100%; }
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
  .learning-section-heading h2 { font-size: 35px; }
  .learning-module-list { grid-template-columns: 1fr; }
  .learning-module-detail { min-height: auto; padding: 22px; }
  .learning-detail-heading { margin-top: 28px; }
  .learning-detail-heading h3 { font-size: 20px; }
  .learning-detail-tags, .learning-detail-list { margin-left: 0; }
  .learning-detail-tags { margin-top: 20px; }
  .learning-detail-list { margin-top: 20px; }
  .learning-contact-card { grid-template-columns: 1fr; gap: 22px; width: 100%; padding: 23px; border-radius: 20px; }
  .learning-contact-intro { gap: 13px; }
  .learning-contact-avatar { width: 48px; height: 48px; border-radius: 15px; }
  .learning-contact-card h3 { font-size: 21px; }
  .learning-footer { align-items: flex-start; flex-direction: column; }
}

@media (prefers-reduced-motion: reduce) {
  .learning-page *, .learning-page *::before, .learning-page *::after { animation-duration: 1ms !important; animation-iteration-count: 1 !important; transition-duration: 1ms !important; scroll-behavior: auto !important; }
  .learning-hero-visual { transform: none; }
}
</style>
