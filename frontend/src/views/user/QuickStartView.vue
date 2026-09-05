<template>
  <AppLayout :enable-onboarding="false">
    <main class="quick-start-page mx-auto max-w-6xl space-y-6">
      <header class="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div class="min-w-0">
          <p class="text-sm font-medium text-primary-600 dark:text-primary-300">{{ t('quickStart.eyebrow') }}</p>
          <h1 class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{{ t('quickStart.title') }}</h1>
          <p class="mt-1.5 max-w-2xl text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t('quickStart.description') }}</p>
        </div>
        <RouterLink to="/keys" class="btn btn-primary inline-flex h-11 shrink-0 items-center justify-center gap-2 self-start">
          <Icon name="key" size="sm" />
          {{ t('quickStart.openKeys') }}
        </RouterLink>
      </header>

      <section class="rounded-xl border border-primary-200 bg-primary-50/70 p-4 dark:border-primary-900/60 dark:bg-primary-950/20" aria-labelledby="quick-start-notice-title">
        <div class="flex items-start gap-3">
          <Icon name="infoCircle" size="md" class="mt-0.5 shrink-0 text-primary-600 dark:text-primary-300" />
          <div class="min-w-0 text-sm leading-6 text-primary-900 dark:text-primary-100">
            <h2 id="quick-start-notice-title" class="font-semibold">{{ t('quickStart.noticeTitle') }}</h2>
            <p class="mt-0.5 text-primary-800/80 dark:text-primary-200/80">{{ t('quickStart.noticeDescription') }}</p>
          </div>
        </div>
      </section>

      <section aria-labelledby="quick-start-steps-title">
        <div class="mb-3 flex flex-wrap items-end justify-between gap-3">
          <div>
            <h2 id="quick-start-steps-title" class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('quickStart.stepsTitle') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('quickStart.stepsDescription') }}</p>
          </div>
          <span class="inline-flex items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400">
            <Icon name="clock" size="sm" />
            {{ t('quickStart.timeEstimate') }}
          </span>
        </div>
        <div class="grid gap-4 lg:grid-cols-4">
          <article
            v-for="(step, index) in steps"
            :key="step.id"
            class="flex min-w-0 flex-col rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-800"
          >
            <div class="flex items-start gap-3">
              <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary-50 text-sm font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-200">{{ String(index + 1).padStart(2, '0') }}</span>
              <div class="min-w-0">
                <Icon :name="step.icon" size="sm" class="text-primary-600 dark:text-primary-300" />
                <h3 class="mt-2 text-base font-semibold text-gray-900 dark:text-white">{{ t(`quickStart.steps.${step.id}.title`) }}</h3>
              </div>
            </div>
            <p class="mt-3 flex-1 text-sm leading-6 text-gray-600 dark:text-gray-300">{{ t(`quickStart.steps.${step.id}.description`) }}</p>
            <RouterLink
              v-if="step.route"
              :to="step.route"
              class="mt-4 inline-flex min-h-10 items-center gap-1.5 text-sm font-medium text-primary-600 hover:text-primary-700 dark:text-primary-300 dark:hover:text-primary-200"
            >
              {{ t(`quickStart.steps.${step.id}.action`) }}
              <Icon name="arrowRight" size="sm" />
            </RouterLink>
            <a
              v-else
              :href="step.anchor"
              class="mt-4 inline-flex min-h-10 items-center gap-1.5 text-sm font-medium text-primary-600 hover:text-primary-700 dark:text-primary-300 dark:hover:text-primary-200"
            >
              {{ t(`quickStart.steps.${step.id}.action`) }}
              <Icon name="arrowRight" size="sm" />
            </a>
          </article>
        </div>
      </section>

      <section id="client-guides" aria-labelledby="quick-start-guides-title">
        <div class="mb-3">
          <h2 id="quick-start-guides-title" class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('quickStart.guides.title') }}</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('quickStart.guides.description') }}</p>
        </div>
        <div class="grid gap-4 lg:grid-cols-2">
          <article id="codex-guide" class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800">
            <div class="p-5 sm:p-6">
              <div class="flex items-start gap-3">
                <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-gray-900 text-white dark:bg-gray-100 dark:text-gray-900"><Icon name="terminal" size="sm" /></span>
                <div class="min-w-0">
                  <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('quickStart.guides.codex.title') }}</h3>
                  <p class="mt-1 text-sm leading-6 text-gray-600 dark:text-gray-300">{{ t('quickStart.guides.codex.description') }}</p>
                </div>
              </div>
              <dl class="mt-5 divide-y divide-gray-100 border-y border-gray-100 text-sm dark:divide-dark-700 dark:border-dark-700">
                <div class="grid gap-1 py-3 sm:grid-cols-[8rem_minmax(0,1fr)] sm:gap-3">
                  <dt class="font-medium text-gray-700 dark:text-gray-200">{{ t('quickStart.guides.codex.configFile') }}</dt>
                  <dd class="min-w-0 space-y-1 text-gray-600 dark:text-gray-300">
                    <code class="block break-all font-mono text-xs">{{ t('quickStart.guides.codex.configPathUnix') }}</code>
                    <code class="block break-all font-mono text-xs">{{ t('quickStart.guides.codex.configPathWindows') }}</code>
                  </dd>
                </div>
                <div class="grid gap-1 py-3 sm:grid-cols-[8rem_minmax(0,1fr)] sm:gap-3">
                  <dt class="font-medium text-gray-700 dark:text-gray-200">{{ t('quickStart.guides.codex.secretSource') }}</dt>
                  <dd class="break-words text-gray-600 dark:text-gray-300">{{ t('quickStart.guides.codex.secretSourceValue') }}</dd>
                </div>
              </dl>
              <ol class="mt-4 space-y-3 text-sm leading-6 text-gray-600 dark:text-gray-300">
                <li v-for="index in 4" :key="index" class="flex gap-2.5">
                  <span class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary-50 text-xs font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-200">{{ index }}</span>
                  <span>{{ t(`quickStart.guides.codex.steps.${index}`) }}</span>
                </li>
              </ol>
              <div class="mt-5">
                <p class="text-sm font-medium text-gray-800 dark:text-gray-100">{{ t('quickStart.guides.codex.configExample') }}</p>
                <pre class="mt-2 overflow-x-auto rounded-lg bg-gray-950 p-4 text-xs leading-6 text-gray-100"><code>{{ codexConfigSnippet }}</code></pre>
              </div>
              <div class="mt-4">
                <p class="text-sm font-medium text-gray-800 dark:text-gray-100">{{ t('quickStart.guides.codex.envExample') }}</p>
                <pre class="mt-2 overflow-x-auto rounded-lg bg-gray-950 p-4 text-xs leading-6 text-gray-100"><code>{{ codexEnvironmentSnippet }}</code></pre>
              </div>
              <RouterLink to="/keys" class="mt-5 inline-flex min-h-10 items-center gap-1.5 text-sm font-medium text-primary-600 hover:text-primary-700 dark:text-primary-300 dark:hover:text-primary-200">
                {{ t('quickStart.guides.openKeyConfig') }}
                <Icon name="arrowRight" size="sm" />
              </RouterLink>
            </div>
          </article>

          <article id="ccswitch-guide" class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800">
            <div class="p-5 sm:p-6">
              <div class="flex items-start gap-3">
                <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-200"><Icon name="grid" size="sm" /></span>
                <div class="min-w-0">
                  <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('quickStart.guides.ccswitch.title') }}</h3>
                  <p class="mt-1 text-sm leading-6 text-gray-600 dark:text-gray-300">{{ t('quickStart.guides.ccswitch.description') }}</p>
                </div>
              </div>
              <dl class="mt-5 divide-y divide-gray-100 border-y border-gray-100 text-sm dark:divide-dark-700 dark:border-dark-700">
                <div class="grid gap-1 py-3 sm:grid-cols-[8rem_minmax(0,1fr)] sm:gap-3">
                  <dt class="font-medium text-gray-700 dark:text-gray-200">{{ t('quickStart.guides.ccswitch.configLocation') }}</dt>
                  <dd class="break-words text-gray-600 dark:text-gray-300">{{ t('quickStart.guides.ccswitch.configLocationValue') }}</dd>
                </div>
                <div class="grid gap-1 py-3 sm:grid-cols-[8rem_minmax(0,1fr)] sm:gap-3">
                  <dt class="font-medium text-gray-700 dark:text-gray-200">{{ t('quickStart.guides.ccswitch.configFile') }}</dt>
                  <dd class="break-words text-gray-600 dark:text-gray-300">{{ t('quickStart.guides.ccswitch.configFileValue') }}</dd>
                </div>
              </dl>
              <ol class="mt-4 space-y-3 text-sm leading-6 text-gray-600 dark:text-gray-300">
                <li v-for="index in 4" :key="index" class="flex gap-2.5">
                  <span class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary-50 text-xs font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-200">{{ index }}</span>
                  <span>{{ t(`quickStart.guides.ccswitch.steps.${index}`) }}</span>
                </li>
              </ol>
              <div class="mt-5">
                <p class="text-sm font-medium text-gray-800 dark:text-gray-100">{{ t('quickStart.guides.ccswitch.manualFields') }}</p>
                <dl class="mt-2 divide-y divide-gray-100 border-y border-gray-100 text-sm dark:divide-dark-700 dark:border-dark-700">
                  <div v-for="field in ccSwitchFields" :key="field.id" class="grid gap-1 py-2.5 sm:grid-cols-[7rem_minmax(0,1fr)] sm:gap-3">
                    <dt class="font-medium text-gray-700 dark:text-gray-200">{{ t(`quickStart.guides.ccswitch.fields.${field.id}`) }}</dt>
                    <dd class="min-w-0 break-all font-mono text-xs leading-5 text-gray-600 dark:text-gray-300">{{ field.value }}</dd>
                  </div>
                </dl>
              </div>
              <RouterLink to="/keys" class="mt-5 inline-flex min-h-10 items-center gap-1.5 text-sm font-medium text-primary-600 hover:text-primary-700 dark:text-primary-300 dark:hover:text-primary-200">
                {{ t('quickStart.guides.openCcSwitchImport') }}
                <Icon name="arrowRight" size="sm" />
              </RouterLink>
            </div>
          </article>
        </div>
      </section>

      <section class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-800 sm:p-6" aria-labelledby="quick-start-code-title">
        <div class="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
          <div class="min-w-0">
            <p class="text-xs font-semibold uppercase tracking-wide text-primary-600 dark:text-primary-300">{{ t('quickStart.code.eyebrow') }}</p>
            <h2 id="quick-start-code-title" class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ t('quickStart.code.title') }}</h2>
            <p class="mt-1 text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t('quickStart.code.description') }}</p>
          </div>
          <div class="min-w-0 text-left sm:text-right">
            <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('quickStart.code.endpoint') }}</span>
            <code class="mt-1 block break-all font-mono text-sm text-gray-800 dark:text-gray-200">{{ apiBase }}</code>
          </div>
        </div>

        <div class="mt-5 flex flex-wrap items-center justify-between gap-3">
          <div class="flex flex-wrap gap-1 rounded-lg bg-gray-100 p-1 dark:bg-dark-900" role="tablist" :aria-label="t('quickStart.code.title')">
            <button
              v-for="snippet in snippets"
              :key="snippet.id"
              type="button"
              class="min-h-9 rounded-md px-3 text-sm font-medium"
              :class="activeSnippetId === snippet.id ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white' : 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'"
              role="tab"
              :aria-selected="activeSnippetId === snippet.id"
              @click="activeSnippetId = snippet.id"
            >
              {{ t(`quickStart.code.tabs.${snippet.id}`) }}
            </button>
          </div>
          <button type="button" class="btn btn-secondary inline-flex min-h-10 items-center gap-2" @click="copyActiveSnippet">
            <Icon :name="copied ? 'check' : 'copy'" size="sm" />
            {{ copied ? t('quickStart.code.copied') : t('quickStart.code.copy') }}
          </button>
        </div>
        <pre class="mt-4 overflow-x-auto rounded-lg bg-gray-950 p-4 text-sm leading-6 text-gray-100 shadow-inner"><code>{{ activeSnippet.code }}</code></pre>
      </section>

      <section id="model-lookup" class="rounded-xl border border-primary-200 bg-primary-50/40 p-5 shadow-sm dark:border-primary-900/60 dark:bg-primary-950/10 sm:p-6" aria-labelledby="quick-start-lookup-title">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div class="min-w-0">
            <p class="text-xs font-semibold uppercase tracking-wide text-primary-600 dark:text-primary-300">{{ t('quickStart.lookup.eyebrow') }}</p>
            <h2 id="quick-start-lookup-title" class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ t('quickStart.lookup.title') }}</h2>
            <p class="mt-1 max-w-3xl text-sm leading-6 text-gray-600 dark:text-gray-300">{{ t('quickStart.lookup.description') }}</p>
          </div>
          <Icon name="search" size="lg" class="shrink-0 text-primary-500 dark:text-primary-300" />
        </div>

        <form class="mt-5" @submit.prevent="queryModels">
          <div class="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] lg:items-end">
            <div class="min-w-0">
              <label for="quick-start-api-url" class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-200">{{ t('quickStart.lookup.urlLabel') }}</label>
              <input id="quick-start-api-url" v-model="lookupBaseUrl" type="url" inputmode="url" autocomplete="url" class="input h-10 w-full font-mono" :placeholder="apiBase" aria-describedby="quick-start-api-url-hint" @input="lookupUrlTouched = true" />
            </div>
            <div class="min-w-0">
              <label for="quick-start-api-key" class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-200">{{ t('quickStart.lookup.keyLabel') }}</label>
              <input id="quick-start-api-key" v-model="lookupApiKey" type="password" autocomplete="off" class="input h-10 w-full font-mono" :placeholder="t('quickStart.lookup.keyPlaceholder')" aria-describedby="quick-start-api-key-hint" />
            </div>
            <button type="submit" class="btn btn-primary inline-flex h-10 items-center justify-center gap-2 lg:self-end" :disabled="modelLookupState === 'loading'">
              <Icon :name="modelLookupState === 'loading' ? 'refresh' : 'search'" size="sm" :class="modelLookupState === 'loading' ? 'animate-spin' : ''" />
              {{ modelLookupState === 'loading' ? t('quickStart.lookup.loading') : t('quickStart.lookup.submit') }}
            </button>
          </div>
          <div class="mt-2 grid gap-1.5 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] lg:gap-4">
            <p id="quick-start-api-url-hint" class="text-xs text-gray-500 dark:text-gray-400">{{ t('quickStart.lookup.urlHint') }}</p>
            <p id="quick-start-api-key-hint" class="text-xs text-gray-500 dark:text-gray-400">{{ t('quickStart.lookup.keyHint') }}</p>
            <span class="hidden lg:block" aria-hidden="true" />
          </div>
        </form>

        <p class="mt-4 break-all font-mono text-xs text-gray-500 dark:text-gray-400">{{ t('quickStart.lookup.endpointLabel') }}: {{ modelsEndpoint }}</p>
        <div v-if="modelLookupState === 'ready'" class="mt-4 rounded-lg border border-emerald-200 bg-emerald-50 p-4 dark:border-emerald-900/60 dark:bg-emerald-950/20" role="status" aria-live="polite">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <p class="text-sm font-semibold text-emerald-800 dark:text-emerald-200">{{ t('quickStart.lookup.success', { count: modelIds.length }) }}</p>
            <span class="text-xs text-emerald-700 dark:text-emerald-300">{{ t('quickStart.lookup.keyNotStored') }}</span>
          </div>
          <div class="mt-3 flex flex-wrap gap-2">
            <button
              v-for="model in modelIds"
              :key="model"
              type="button"
              class="group inline-flex max-w-full items-center gap-1.5 rounded-md border border-emerald-200 bg-white px-2.5 py-1.5 text-left font-mono text-xs text-emerald-800 transition-[border-color,background-color] duration-150 hover:border-emerald-400 hover:bg-emerald-50 dark:border-emerald-900/70 dark:bg-dark-800 dark:text-emerald-200 dark:hover:bg-emerald-950/30"
              :title="t('quickStart.lookup.copyModel')"
              @click="copyModel(model)"
            >
              <span class="break-all">{{ model }}</span>
              <Icon name="copy" size="xs" class="shrink-0 opacity-60 group-hover:opacity-100" />
            </button>
          </div>
        </div>
        <p v-else-if="modelLookupState === 'error'" class="mt-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm leading-6 text-red-700 dark:border-red-900/60 dark:bg-red-950/20 dark:text-red-200" role="alert">{{ modelLookupError }}</p>
      </section>

      <section class="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]" aria-labelledby="quick-start-links-title">
        <div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-800 sm:p-6">
          <h2 id="quick-start-links-title" class="text-base font-semibold text-gray-900 dark:text-white">{{ t('quickStart.links.title') }}</h2>
          <div class="mt-4 grid gap-2 sm:grid-cols-2">
            <RouterLink v-for="link in quickLinks" :key="link.id" :to="link.route" class="group flex min-h-12 items-center gap-3 rounded-lg border border-gray-200 px-3 py-2.5 transition-[border-color,background-color] duration-200 hover:border-primary-300 hover:bg-primary-50/60 dark:border-dark-700 dark:hover:border-primary-700 dark:hover:bg-primary-900/20">
              <span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300"><Icon :name="link.icon" size="sm" /></span>
              <span class="min-w-0"><strong class="block truncate text-sm font-medium text-gray-800 group-hover:text-primary-700 dark:text-gray-100 dark:group-hover:text-primary-200">{{ t(`quickStart.links.${link.id}.title`) }}</strong><small class="mt-0.5 block truncate text-xs text-gray-500 dark:text-gray-400">{{ t(`quickStart.links.${link.id}.description`) }}</small></span>
              <Icon name="arrowRight" size="sm" class="ml-auto shrink-0 text-gray-400 group-hover:text-primary-600" />
            </RouterLink>
          </div>
        </div>

        <div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-800 sm:p-6">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('quickStart.faq.title') }}</h2>
          <div class="mt-2 divide-y divide-gray-100 dark:divide-dark-700">
            <details v-for="item in faqItems" :key="item" class="group py-3 first:pt-1 last:pb-1">
              <summary class="flex cursor-pointer list-none items-center justify-between gap-3 text-sm font-medium text-gray-800 marker:hidden dark:text-gray-100">
                <span>{{ t(`quickStart.faq.items.${item}.question`) }}</span>
                <Icon name="chevronDown" size="sm" class="shrink-0 text-gray-400 transition-transform duration-200 group-open:rotate-180" />
              </summary>
              <p class="mt-2 pr-6 text-sm leading-6 text-gray-600 dark:text-gray-300">{{ t(`quickStart.faq.items.${item}.answer`) }}</p>
            </details>
          </div>
        </div>
      </section>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { useAppStore } from '@/stores/app'

type SnippetId = 'curl' | 'javascript' | 'python'
type ModelLookupState = 'idle' | 'loading' | 'ready' | 'error'

const { t } = useI18n()
const { copied, copyToClipboard } = useClipboard()
const appStore = useAppStore()
const activeSnippetId = ref<SnippetId>('curl')
const apiBase = ref(resolveApiBaseUrl())
const lookupBaseUrl = ref(apiBase.value)
const lookupUrlTouched = ref(false)
const lookupApiKey = ref('')
const modelLookupState = ref<ModelLookupState>('idle')
const modelLookupError = ref('')
const modelIds = ref<string[]>([])

const steps = [
  { id: 'apiKey', route: '/keys', icon: 'key' as const },
  { id: 'connection', anchor: '#model-lookup', icon: 'link' as const },
  { id: 'codex', anchor: '#codex-guide', icon: 'terminal' as const },
  { id: 'ccswitch', anchor: '#ccswitch-guide', icon: 'grid' as const },
]

const quickLinks = [
  { id: 'apiKeys', route: '/keys', icon: 'key' as const },
  { id: 'learning', route: '/ai-learning', icon: 'book' as const },
  { id: 'usage', route: '/usage', icon: 'chart' as const },
  { id: 'lookup', route: '/quick-start#model-lookup', icon: 'search' as const },
]

const codexConfigSnippet = computed(() => `model = "your-model-id"
model_provider = "modurelay"

[model_providers.modurelay]
name = "ModuRelay"
base_url = "${apiBase.value}"
env_key = "MODURELAY_API_KEY"
wire_api = "responses"
requires_openai_auth = false
supports_websockets = false`)

const codexEnvironmentSnippet = `# macOS / Linux
export MODURELAY_API_KEY="<your-api-key>"

# Windows PowerShell
$env:MODURELAY_API_KEY="<your-api-key>"`

const ccSwitchFields = computed(() => [
  { id: 'name', value: 'ModuRelay' },
  { id: 'client', value: t('quickStart.guides.ccswitch.fieldValues.client') },
  { id: 'endpoint', value: apiBase.value },
  { id: 'key', value: t('quickStart.guides.ccswitch.fieldValues.key') },
  { id: 'model', value: t('quickStart.guides.ccswitch.fieldValues.model') },
])

const faqItems = ['key', 'endpoint', 'lookup'] as const

const snippets = computed(() => [
  {
    id: 'curl' as const,
    code: [
      `curl ${apiBase.value}/chat/completions`,
      '  -H "Authorization: Bearer $MODURELAY_API_KEY"',
      '  -H "Content-Type: application/json"',
      `  -d '{"model":"your-model-id","messages":[{"role":"user","content":"Hello"}]}'`,
    ].join('\n'),
  },
  {
    id: 'javascript' as const,
    code: `const response = await fetch('${apiBase.value}/chat/completions', {\n  method: 'POST',\n  headers: {\n    Authorization: \`Bearer \${process.env.MODURELAY_API_KEY}\`,\n    'Content-Type': 'application/json',\n  },\n  body: JSON.stringify({\n    model: 'your-model-id',\n    messages: [{ role: 'user', content: 'Hello' }],\n  }),\n})`,
  },
  {
    id: 'python' as const,
    code: `from openai import OpenAI\n\nclient = OpenAI(\n    api_key="$MODURELAY_API_KEY",\n    base_url="${apiBase.value}",\n)\nresponse = client.chat.completions.create(\n    model="your-model-id",\n    messages=[{"role": "user", "content": "Hello"}],\n)`,
  },
])

const activeSnippet = computed(() => snippets.value.find(snippet => snippet.id === activeSnippetId.value) || snippets.value[0])
const normalizedLookupBase = computed(() => normalizeBaseUrl(lookupBaseUrl.value))
const modelsEndpoint = computed(() => `${normalizedLookupBase.value}/models`)

function normalizeBaseUrl(value: string): string {
  const trimmed = value.trim().replace(/\/+$/, '')
  if (!trimmed) return apiBase.value
  return /\/v1$/i.test(trimmed) ? trimmed : `${trimmed}/v1`
}

function resolveApiBaseUrl(configured = ''): string {
  const fallback = typeof window !== 'undefined' ? window.location.origin : ''
  const value = configured.trim() || fallback
  if (!value) return '/v1'
  const trimmed = value.replace(/\/+$/, '')
  return /\/v1$/i.test(trimmed) ? trimmed : `${trimmed}/v1`
}

function endpointForRequest(): string {
  const endpoint = modelsEndpoint.value
  if (/^https?:\/\//i.test(endpoint)) return endpoint
  if (typeof window !== 'undefined') return new URL(endpoint, window.location.origin).toString()
  return endpoint
}

function extractModelIds(payload: unknown): string[] {
  if (!payload || typeof payload !== 'object') return []
  const value = payload as { data?: unknown; models?: unknown }
  const entries = Array.isArray(value.data) ? value.data : Array.isArray(value.models) ? value.models : []
  return Array.from(new Set(entries
    .map((entry) => {
      if (typeof entry === 'string') return entry
      if (!entry || typeof entry !== 'object') return ''
      const item = entry as { id?: unknown; slug?: unknown; model?: unknown }
      return typeof item.id === 'string' ? item.id : typeof item.slug === 'string' ? item.slug : typeof item.model === 'string' ? item.model : ''
    })
    .map(model => model.trim())
    .filter(Boolean)))
}

async function queryModels(): Promise<void> {
  const key = lookupApiKey.value.trim()
  if (!key) {
    modelLookupState.value = 'error'
    modelLookupError.value = t('quickStart.lookup.errors.keyRequired')
    return
  }

  modelLookupState.value = 'loading'
  modelLookupError.value = ''
  modelIds.value = []
  try {
    const response = await fetch(endpointForRequest(), {
      method: 'GET',
      headers: { Accept: 'application/json', Authorization: `Bearer ${key}` },
      cache: 'no-store',
    })
    if (!response.ok) {
      modelLookupState.value = 'error'
      modelLookupError.value = response.status === 401 || response.status === 403
        ? t('quickStart.lookup.errors.unauthorized')
        : t('quickStart.lookup.errors.http', { status: response.status })
      return
    }
    const ids = extractModelIds(await response.json())
    if (!ids.length) {
      modelLookupState.value = 'error'
      modelLookupError.value = t('quickStart.lookup.errors.empty')
      return
    }
    modelIds.value = ids
    modelLookupState.value = 'ready'
  } catch {
    modelLookupState.value = 'error'
    modelLookupError.value = t('quickStart.lookup.errors.network')
  }
}

async function copyActiveSnippet(): Promise<void> {
  await copyToClipboard(activeSnippet.value.code, t('quickStart.code.copied'))
}

async function copyModel(model: string): Promise<void> {
  await copyToClipboard(model, t('quickStart.lookup.modelCopied'))
}

onMounted(async () => {
  const previousDefault = apiBase.value
  const settings = await appStore.fetchPublicSettings()
  const resolved = resolveApiBaseUrl(settings?.api_base_url || appStore.apiBaseUrl)
  apiBase.value = resolved
  if (!lookupUrlTouched.value || lookupBaseUrl.value === previousDefault) {
    lookupBaseUrl.value = resolved
  }
})
</script>
