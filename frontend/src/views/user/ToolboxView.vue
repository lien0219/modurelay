<template>
  <AppLayout :enable-onboarding="false">
    <main class="toolbox-page">
      <div class="toolbox-shell">
        <template v-if="isHome">
          <section class="toolbox-hero">
            <div class="toolbox-hero__main">
              <p class="toolbox-eyebrow">MODURELAY / {{ t('nav.tools') }}</p>
              <h1 class="toolbox-title">{{ t('tools.title') }}</h1>
              <p class="toolbox-description">{{ t('tools.description') }}</p>

              <label class="toolbox-search">
                <span class="sr-only">{{ t('tools.searchPlaceholder') }}</span>
                <Icon name="search" size="md" class="toolbox-search__icon" aria-hidden="true" />
                <input
                  ref="searchInputRef"
                  v-model="search"
                  type="search"
                  :placeholder="t('tools.searchPlaceholder')"
                  autocomplete="off"
                />
                <button
                  v-if="search"
                  type="button"
                  class="toolbox-search__clear"
                  :aria-label="t('tools.clearSearch')"
                  :title="t('tools.clearSearch')"
                  @click="clearSearch"
                >
                  <Icon name="x" size="sm" aria-hidden="true" />
                </button>
                <kbd v-else class="toolbox-search__shortcut">Ctrl K</kbd>
              </label>

              <div class="toolbox-filter-strip" role="tablist" :aria-label="t('tools.allCategories')">
                <button
                  v-for="category in categories"
                  :key="category"
                  type="button"
                  class="tool-filter"
                  :class="{ 'tool-filter-active': selectedCategory === category }"
                  :aria-selected="selectedCategory === category"
                  role="tab"
                  @click="selectedCategory = category"
                >
                  {{ category === 'all' ? t('tools.allCategories') : t('tools.categories.' + category) }}
                </button>
              </div>
            </div>

            <aside class="toolbox-hero__preview" aria-label="tools preview">
              <div class="toolbox-preview-art" aria-hidden="true">
                <span class="toolbox-preview-card toolbox-preview-card--back"></span>
                <span class="toolbox-preview-card toolbox-preview-card--middle"></span>
                <span class="toolbox-preview-card toolbox-preview-card--front">
                  <Icon name="cube" size="xl" />
                </span>
              </div>
              <div class="toolbox-preview-copy">
                <strong>{{ t('tools.heroCardTitle') }}</strong>
                <p>{{ t('tools.heroCardDescription') }}</p>
                <ul>
                  <li><Icon name="checkCircle" size="sm" aria-hidden="true" />{{ t('tools.heroPointLocal') }}</li>
                  <li><Icon name="checkCircle" size="sm" aria-hidden="true" />{{ t('tools.heroPointUnified') }}</li>
                  <li><Icon name="checkCircle" size="sm" aria-hidden="true" />{{ t('tools.heroPointCoverage') }}</li>
                </ul>
              </div>
              <span class="toolbox-preview-label" aria-hidden="true">LOCAL WORKBENCH</span>
            </aside>
          </section>

          <section class="toolbox-catalog" aria-live="polite">
            <div class="toolbox-catalog__header">
              <div>
                <p class="toolbox-catalog__eyebrow">{{ selectedCategory === 'all' ? t('tools.allCategories') : t('tools.categories.' + selectedCategory) }}</p>
                <h2>{{ t('tools.toolCount', { count: displayedTools.length }) }}</h2>
              </div>

              <div class="toolbox-view-controls">
                <label class="toolbox-sort">
                  <span class="sr-only">{{ t('tools.sortLabel') }}</span>
                  <Icon name="sort" size="sm" aria-hidden="true" />
                  <select v-model="sortMode" :aria-label="t('tools.sortLabel')">
                    <option value="default">{{ t('tools.sortDefault') }}</option>
                    <option value="name">{{ t('tools.sortName') }}</option>
                  </select>
                </label>
                <div class="toolbox-view-toggle" :aria-label="t('tools.viewMode')" role="group">
                  <button
                    type="button"
                    :class="{ 'is-active': viewMode === 'grid' }"
                    :aria-pressed="viewMode === 'grid'"
                    :title="t('tools.gridView')"
                    @click="viewMode = 'grid'"
                  >
                    <Icon name="grid" size="sm" aria-hidden="true" />
                    <span class="sr-only">{{ t('tools.gridView') }}</span>
                  </button>
                  <button
                    type="button"
                    :class="{ 'is-active': viewMode === 'list' }"
                    :aria-pressed="viewMode === 'list'"
                    :title="t('tools.listView')"
                    @click="viewMode = 'list'"
                  >
                    <Icon name="menu" size="sm" aria-hidden="true" />
                    <span class="sr-only">{{ t('tools.listView') }}</span>
                  </button>
                </div>
              </div>
            </div>

            <div
              v-if="displayedTools.length"
              class="toolbox-grid"
              :class="{ 'toolbox-grid--list': viewMode === 'list' }"
            >
              <RouterLink
                v-for="tool in displayedTools"
                :key="tool.id"
                :to="tool.route"
                class="tool-card group"
              >
                <span class="tool-card-icon" :class="'tool-card-icon--' + tool.category" aria-hidden="true">
                  <Icon :name="tool.icon" size="md" />
                </span>

                <span class="tool-card-copy">
                  <strong>{{ t(tool.titleKey) }}</strong>
                  <span class="tool-card-description">{{ t(tool.descriptionKey) }}</span>
                  <span class="tool-card-tags" aria-hidden="true">
                    <span>{{ t('tools.categories.' + tool.category) }}</span>
                    <span>{{ toolTechLabel(tool.id) }}</span>
                    <span v-if="tool.category === 'security'">{{ t('tools.privateTag') }}</span>
                  </span>
                </span>

                <span class="tool-card-arrow" aria-hidden="true">
                  <Icon name="arrowRight" size="sm" />
                </span>
              </RouterLink>
            </div>

            <div v-else class="tool-empty-state" role="status">
              <div class="tool-empty-state__art" aria-hidden="true">
                <span class="tool-empty-state__box"><Icon name="cube" size="xl" /></span>
                <span class="tool-empty-state__search"><Icon name="search" size="md" /></span>
              </div>
              <strong>{{ t('tools.noTools') }}</strong>
              <p>{{ t('tools.noToolsFor', { query: search || t('tools.allCategories') }) }}</p>
              <span class="tool-empty-state__label">{{ t('tools.tryKeywords') }}</span>
              <div class="tool-empty-state__suggestions">
                <button v-for="suggestion in searchSuggestions" :key="suggestion" type="button" @click="search = suggestion">
                  {{ suggestion }}
                </button>
              </div>
              <div class="tool-empty-state__actions">
                <button type="button" class="tool-empty-action tool-empty-action--secondary" @click="clearSearch">
                  <Icon name="x" size="sm" aria-hidden="true" />
                  {{ t('tools.clearSearch') }}
                </button>
                <button type="button" class="tool-empty-action tool-empty-action--primary" @click="showAllTools">
                  <Icon name="grid" size="sm" aria-hidden="true" />
                  {{ t('tools.viewAllTools') }}
                </button>
              </div>
            </div>
          </section>
        </template>

        <template v-else-if="selectedTool">
          <div class="tool-workspace-shell">
            <RouterLink to="/tools" class="tool-back-link">
              <Icon name="arrowLeft" size="sm" aria-hidden="true" />
              {{ t('tools.backToTools') }}
            </RouterLink>
            <Suspense>
              <ToolWorkspace :tool="selectedTool.id" />
              <template #fallback>
                <section class="tool-loading" aria-live="polite">{{ t('tools.loading') }}</section>
              </template>
            </Suspense>
          </div>
        </template>

        <template v-else>
          <section class="tool-invalid-route" role="alert">
            <span class="tool-invalid-route__icon"><Icon name="exclamationCircle" size="lg" aria-hidden="true" /></span>
            <h1>{{ t('tools.title') }}</h1>
            <p>{{ t('tools.noInput') }}</p>
            <RouterLink to="/tools" class="btn btn-primary">{{ t('tools.backToTools') }}</RouterLink>
          </section>
        </template>
      </div>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent, onBeforeUnmount, onMounted, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { TOOL_REGISTRY, getToolDefinition, type ToolCategory, type ToolId } from '@/features/tools/registry'

const { t } = useI18n()
const route = useRoute()
const search = ref('')
const searchInputRef = ref<HTMLInputElement | null>(null)
const selectedCategory = ref<'all' | ToolCategory>('all')
const sortMode = ref<'default' | 'name'>('default')
const viewMode = ref<'grid' | 'list'>('grid')
const categories: readonly ('all' | ToolCategory)[] = ['all', 'security', 'encoding', 'developer', 'api', 'network', 'text']
const searchSuggestions = ['API', 'JSON', 'HMAC', 'Base64', 'UUID'] as const

const toolId = computed(() => typeof route.params.toolId === 'string' ? route.params.toolId : '')
const selectedTool = computed(() => getToolDefinition(toolId.value))
const ToolWorkspace = defineAsyncComponent(() => selectedTool.value?.lazyComponent() ?? import('@/features/tools/ToolWorkspace.vue'))
const isHome = computed(() => !toolId.value)

const filteredTools = computed(() => {
  const query = search.value.trim().toLocaleLowerCase()
  return TOOL_REGISTRY.filter((tool) => {
    if (selectedCategory.value !== 'all' && tool.category !== selectedCategory.value) return false
    if (!query) return true
    return (tool.id + ' ' + t(tool.titleKey) + ' ' + t(tool.descriptionKey)).toLocaleLowerCase().includes(query)
  })
})

const displayedTools = computed(() => {
  const tools = [...filteredTools.value]
  if (sortMode.value === 'name') {
    tools.sort((a, b) => t(a.titleKey).localeCompare(t(b.titleKey)))
  }
  return tools
})

const techLabels: Partial<Record<ToolId, string>> = {
  totp: 'TOTP',
  password: 'CSPRNG',
  uuid: 'UUID',
  json: 'JSON',
  base64: 'Base64',
  url: 'URL',
  timestamp: 'Unix',
  jwt: 'JWT',
  hash: 'SHA',
  hmac: 'HMAC',
  regex: 'RegExp',
  'http-status': 'HTTP',
  'user-agent': 'UA',
  'ip-cidr': 'CIDR',
  'api-builder': 'API',
  'curl-converter': 'cURL',
  'json-to-struct': 'Struct',
  cron: 'Cron',
  diff: 'Diff',
  markdown: 'Markdown',
  qrcode: 'QR',
}

function toolTechLabel(id: ToolId): string {
  return techLabels[id] || t('tools.localTag')
}

function clearSearch(): void {
  search.value = ''
  searchInputRef.value?.focus()
}

function showAllTools(): void {
  search.value = ''
  selectedCategory.value = 'all'
  sortMode.value = 'default'
  searchInputRef.value?.focus()
}

function handleShortcut(event: KeyboardEvent): void {
  if ((event.ctrlKey || event.metaKey) && event.key.toLocaleLowerCase() === 'k') {
    event.preventDefault()
    searchInputRef.value?.focus()
  }
}

onMounted(() => window.addEventListener('keydown', handleShortcut))
onBeforeUnmount(() => window.removeEventListener('keydown', handleShortcut))
</script>

<style scoped>
.toolbox-page {
  min-height: calc(100dvh - 4rem);
  color: var(--color-text-primary);
}

.toolbox-shell {
  width: min(1460px, calc(100% - 40px));
  margin-inline: auto;
  padding: 28px 0 48px;
}

.toolbox-hero {
  display: grid;
  grid-template-columns: minmax(0, 1.45fr) minmax(360px, .75fr);
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-radius: 18px;
  background: var(--color-surface);
  box-shadow: var(--shadow-sm);
}

.toolbox-hero__main {
  position: relative;
  min-width: 0;
  padding: 28px;
}

.toolbox-hero__main::after {
  position: absolute;
  top: -150px;
  right: -110px;
  width: 330px;
  height: 330px;
  border: 1px solid var(--color-primary-border);
  border-radius: 50%;
  content: '';
  opacity: .45;
  pointer-events: none;
}

.toolbox-eyebrow,
.toolbox-catalog__eyebrow {
  margin: 0;
  color: var(--color-primary);
  font-size: .75rem;
  font-weight: 750;
  letter-spacing: .09em;
  text-transform: uppercase;
}

.toolbox-title {
  margin: .55rem 0 0;
  font-size: clamp(1.9rem, 3vw, 2.45rem);
  font-weight: 700;
  line-height: 1.15;
  letter-spacing: -.035em;
}

.toolbox-description {
  max-width: 46rem;
  margin: .65rem 0 0;
  color: var(--color-text-secondary);
  font-size: .9rem;
  line-height: 1.65;
}

.toolbox-search {
  position: relative;
  z-index: 1;
  display: block;
  max-width: 46rem;
  margin-top: 1.35rem;
}

.toolbox-search input {
  width: 100%;
  min-height: 46px;
  padding: 0 78px 0 42px;
  border: 1px solid var(--color-border-strong);
  border-radius: 12px;
  outline: none;
  background: var(--color-surface-raised);
  color: var(--color-text-primary);
  box-shadow: var(--shadow-xs);
  font: inherit;
  font-size: .875rem;
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    box-shadow var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard);
}

.toolbox-search input:focus {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 3px var(--color-primary-ring);
}

.toolbox-search input::-webkit-search-cancel-button {
  display: none;
}

.toolbox-search__icon {
  position: absolute;
  z-index: 2;
  top: 50%;
  left: 14px;
  color: var(--color-text-muted);
  transform: translateY(-50%);
  pointer-events: none;
}

.toolbox-search__clear {
  position: absolute;
  z-index: 2;
  top: 50%;
  right: 8px;
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border: 0;
  border-radius: 9px;
  background: transparent;
  color: var(--color-text-muted);
  cursor: pointer;
  transform: translateY(-50%);
}

.toolbox-search__clear:hover {
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.toolbox-search__clear:focus-visible,
.tool-filter:focus-visible,
.toolbox-view-toggle button:focus-visible,
.toolbox-sort select:focus-visible,
.tool-empty-state button:focus-visible,
.tool-card:focus-visible,
.tool-back-link:focus-visible {
  outline: 2px solid var(--color-primary-ring);
  outline-offset: 2px;
}

.toolbox-search__shortcut {
  position: absolute;
  top: 50%;
  right: 10px;
  padding: 4px 7px;
  border: 1px solid var(--color-border);
  border-radius: 7px;
  background: var(--color-surface-soft);
  color: var(--color-text-muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: .68rem;
  transform: translateY(-50%);
}

.toolbox-filter-strip {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 1rem;
}

.tool-filter {
  min-height: 36px;
  padding: 0 13px;
  border: 1px solid var(--color-border);
  border-radius: 999px;
  background: var(--color-surface);
  color: var(--color-text-secondary);
  cursor: pointer;
  font: inherit;
  font-size: .8rem;
  font-weight: 600;
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard),
    color var(--motion-fast) var(--ease-standard),
    transform var(--motion-fast) var(--ease-standard);
}

.tool-filter:hover {
  border-color: var(--color-primary-border);
  color: var(--color-text-primary);
  transform: translateY(-1px);
}

.tool-filter-active {
  border-color: var(--color-primary);
  background: var(--color-primary);
  color: var(--color-surface);
  box-shadow: var(--shadow-xs);
}

.tool-filter-active:hover {
  color: var(--color-surface);
}

.toolbox-hero__preview {
  position: relative;
  display: grid;
  grid-template-columns: 132px minmax(0, 1fr);
  align-items: center;
  gap: 18px;
  min-width: 0;
  padding: 24px 24px 28px;
  overflow: hidden;
  border-left: 1px solid var(--color-border-subtle);
  background: var(--color-surface-soft);
}

.toolbox-preview-art {
  position: relative;
  width: 132px;
  height: 146px;
  min-width: 132px;
  justify-self: center;
  overflow: visible;
}

.toolbox-preview-card {
  position: absolute;
  display: grid;
  width: 82px;
  height: 94px;
  place-items: center;
  border: 1px solid var(--color-primary-border);
  border-radius: 17px;
  background: var(--color-surface);
  color: var(--color-primary);
  box-shadow: var(--shadow-sm);
}

.toolbox-preview-card--back {
  top: 10px;
  left: 5px;
  opacity: .38;
}

.toolbox-preview-card--middle {
  top: 24px;
  left: 25px;
  opacity: .62;
}

.toolbox-preview-card--front {
  top: 38px;
  left: 45px;
}

.toolbox-preview-copy {
  position: relative;
  z-index: 1;
  min-width: 0;
  overflow-wrap: anywhere;
}

.toolbox-preview-copy strong {
  display: block;
  max-width: 100%;
  font-size: .95rem;
  font-weight: 700;
  line-height: 1.4;
}

.toolbox-preview-copy p {
  max-width: 100%;
  margin: .4rem 0 0;
  color: var(--color-text-muted);
  font-size: .78rem;
  line-height: 1.55;
}

.toolbox-preview-copy ul {
  display: grid;
  gap: 7px;
  margin: .85rem 0 0;
  padding: 0;
  list-style: none;
}

.toolbox-preview-copy li {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  gap: 7px;
  color: var(--color-text-secondary);
  font-size: .75rem;
  line-height: 1.45;
}

.toolbox-preview-copy li svg {
  color: var(--color-primary);
}

.toolbox-preview-label {
  position: absolute;
  right: 18px;
  bottom: 14px;
  color: var(--color-text-disabled);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: .63rem;
  letter-spacing: .12em;
}

.toolbox-catalog {
  margin-top: 22px;
}

.toolbox-catalog__header {
  display: flex;
  min-height: 44px;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 12px;
}

.toolbox-catalog__header h2 {
  margin: .22rem 0 0;
  font-size: 1.05rem;
  font-weight: 700;
}

.toolbox-view-controls {
  display: flex;
  align-items: center;
  gap: 8px;
}

.toolbox-sort {
  position: relative;
  display: flex;
  min-height: 36px;
  align-items: center;
  gap: 6px;
  padding-left: 10px;
  border: 1px solid var(--color-border);
  border-radius: 10px;
  background: var(--color-surface);
  color: var(--color-text-muted);
}

.toolbox-sort select {
  min-height: 34px;
  padding: 0 26px 0 2px;
  border: 0;
  outline: none;
  appearance: auto;
  background: transparent;
  color: var(--color-text-secondary);
  font: inherit;
  font-size: .78rem;
  cursor: pointer;
}

.toolbox-view-toggle {
  display: flex;
  padding: 3px;
  border: 1px solid var(--color-border);
  border-radius: 10px;
  background: var(--color-surface);
}

.toolbox-view-toggle button {
  display: grid;
  width: 30px;
  height: 30px;
  place-items: center;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: var(--color-text-muted);
  cursor: pointer;
}

.toolbox-view-toggle button.is-active {
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.toolbox-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.toolbox-grid--list {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.tool-card {
  display: grid;
  min-width: 0;
  min-height: 118px;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: start;
  gap: 12px;
  padding: 14px;
  border: 1px solid var(--color-border);
  border-radius: 14px;
  background: var(--color-surface);
  box-shadow: var(--shadow-xs);
  color: inherit;
  text-decoration: none;
  transition:
    border-color var(--motion-base) var(--ease-standard),
    background-color var(--motion-base) var(--ease-standard),
    box-shadow var(--motion-base) var(--ease-standard),
    transform var(--motion-base) var(--ease-standard);
}

.tool-card:hover {
  border-color: var(--color-primary-border);
  background: var(--color-surface-raised);
  box-shadow: var(--shadow-sm);
  transform: translateY(-2px);
}

.toolbox-grid--list .tool-card {
  min-height: 104px;
}

.tool-card-icon {
  display: inline-flex;
  width: 38px;
  height: 38px;
  flex: 0 0 38px;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--color-primary-border);
  border-radius: 11px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.tool-card-icon--api,
.tool-card-icon--network {
  border-color: color-mix(in srgb, var(--color-accent) 28%, var(--color-border));
  background: var(--color-accent-soft);
  color: var(--color-accent);
}

.tool-card-copy {
  display: block;
  min-width: 0;
}

.tool-card-copy strong {
  display: block;
  color: var(--color-text-primary);
  font-size: .88rem;
  font-weight: 680;
  line-height: 1.35;
}

.tool-card-description {
  display: -webkit-box;
  min-height: 2.45rem;
  margin-top: 4px;
  overflow: hidden;
  color: var(--color-text-muted);
  font-size: .75rem;
  line-height: 1.48;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.tool-card-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  margin-top: 8px;
}

.tool-card-tags > span {
  display: inline-flex;
  min-height: 21px;
  align-items: center;
  padding: 0 7px;
  border-radius: 999px;
  background: var(--color-surface-soft);
  color: var(--color-text-muted);
  font-size: .65rem;
  line-height: 1;
  white-space: nowrap;
}

.tool-card-arrow {
  display: inline-flex;
  padding-top: 4px;
  color: var(--color-text-muted);
  transition:
    color var(--motion-fast) var(--ease-standard),
    transform var(--motion-fast) var(--ease-standard);
}

.tool-card:hover .tool-card-arrow {
  color: var(--color-primary);
  transform: translateX(2px);
}

.tool-empty-state {
  display: grid;
  width: min(520px, 100%);
  min-height: 390px;
  justify-items: center;
  align-content: center;
  margin: 28px auto 0;
  padding: 34px;
  border: 1px solid var(--color-border);
  border-radius: 18px;
  background: var(--color-surface);
  box-shadow: var(--shadow-sm);
  text-align: center;
}

.tool-empty-state__art {
  position: relative;
  width: 104px;
  height: 86px;
  margin-bottom: 14px;
}

.tool-empty-state__box {
  position: absolute;
  top: 8px;
  left: 14px;
  display: grid;
  width: 72px;
  height: 72px;
  place-items: center;
  border: 1px solid var(--color-primary-border);
  border-radius: 18px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.tool-empty-state__search {
  position: absolute;
  right: 2px;
  bottom: 0;
  display: grid;
  width: 42px;
  height: 42px;
  place-items: center;
  border: 1px solid var(--color-border);
  border-radius: 50%;
  background: var(--color-surface-raised);
  color: var(--color-primary);
  box-shadow: var(--shadow-sm);
}

.tool-empty-state > strong {
  color: var(--color-text-primary);
  font-size: 1.15rem;
  font-weight: 700;
}

.tool-empty-state > p {
  max-width: 390px;
  margin: .55rem 0 0;
  color: var(--color-text-muted);
  font-size: .82rem;
  line-height: 1.6;
}

.tool-empty-state__label {
  margin-top: 1.15rem;
  color: var(--color-text-disabled);
  font-size: .7rem;
}

.tool-empty-state__suggestions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 7px;
  margin-top: .65rem;
}

.tool-empty-state__suggestions button {
  min-height: 32px;
  padding: 0 12px;
  border: 0;
  border-radius: 999px;
  background: var(--color-surface-soft);
  color: var(--color-text-secondary);
  cursor: pointer;
  font: inherit;
  font-size: .75rem;
}

.tool-empty-state__suggestions button:hover {
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.tool-empty-state__actions {
  display: flex;
  gap: 9px;
  margin-top: 1.25rem;
}

.tool-empty-action {
  display: inline-flex;
  min-height: 38px;
  align-items: center;
  justify-content: center;
  gap: 7px;
  padding: 0 15px;
  border-radius: 10px;
  cursor: pointer;
  font: inherit;
  font-size: .78rem;
  font-weight: 650;
}

.tool-empty-action--secondary {
  border: 1px solid var(--color-border);
  background: var(--color-surface);
  color: var(--color-text-secondary);
}

.tool-empty-action--primary {
  border: 1px solid var(--color-primary);
  background: var(--color-primary);
  color: var(--color-surface);
}

.tool-workspace-shell {
  display: grid;
  gap: 14px;
}

.tool-back-link {
  display: inline-flex;
  width: fit-content;
  min-height: 36px;
  align-items: center;
  gap: .45rem;
  color: var(--color-primary);
  font-size: .82rem;
  font-weight: 650;
  text-decoration: none;
}

.tool-loading,
.tool-invalid-route {
  display: grid;
  min-height: 320px;
  place-items: center;
  padding: 32px;
  border: 1px solid var(--color-border);
  border-radius: 16px;
  background: var(--color-surface);
  color: var(--color-text-muted);
  text-align: center;
}

.tool-invalid-route {
  justify-items: center;
  align-content: center;
  gap: 10px;
}

.tool-invalid-route__icon {
  color: var(--color-warning);
}

.tool-invalid-route h1 {
  margin: 0;
  color: var(--color-text-primary);
  font-size: 1.4rem;
  font-weight: 700;
}

.tool-invalid-route p {
  margin: 0;
  color: var(--color-text-secondary);
}

@media (max-width: 1220px) {
  .toolbox-hero {
    grid-template-columns: minmax(0, 1.3fr) minmax(330px, .7fr);
  }

  .toolbox-hero__main {
    padding: 24px;
  }

  .toolbox-hero__preview {
    grid-template-columns: 108px minmax(0, 1fr);
    gap: 14px;
    padding: 22px 20px 26px;
  }

  .toolbox-preview-art {
    width: 108px;
    height: 126px;
    min-width: 108px;
  }

  .toolbox-preview-card {
    width: 70px;
    height: 80px;
    border-radius: 15px;
  }

  .toolbox-preview-card--back {
    top: 8px;
    left: 2px;
  }

  .toolbox-preview-card--middle {
    top: 20px;
    left: 18px;
  }

  .toolbox-preview-card--front {
    top: 32px;
    left: 34px;
  }

  .toolbox-preview-copy strong {
    font-size: .9rem;
  }

  .toolbox-preview-copy p {
    font-size: .75rem;
  }

  .toolbox-preview-copy li {
    gap: 6px;
    font-size: .72rem;
  }

  .toolbox-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .toolbox-grid--list {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 980px) {
  .toolbox-shell {
    width: min(100% - 28px, 900px);
  }

  .toolbox-hero {
    grid-template-columns: 1fr;
  }

  .toolbox-hero__preview {
    grid-template-columns: 118px minmax(0, 1fr);
    min-height: 190px;
    gap: 20px;
    padding: 22px 28px 28px;
    border-top: 1px solid var(--color-border-subtle);
    border-left: 0;
  }

  .toolbox-preview-art {
    width: 118px;
    height: 132px;
    min-width: 118px;
  }

  .toolbox-preview-card {
    width: 76px;
    height: 86px;
  }

  .toolbox-preview-card--front {
    left: 38px;
  }

  .toolbox-grid,
  .toolbox-grid--list {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 760px) {
  .toolbox-hero__preview {
    grid-template-columns: 94px minmax(0, 1fr);
    gap: 16px;
    min-height: 172px;
    padding: 20px 22px 26px;
  }

  .toolbox-preview-art {
    width: 94px;
    height: 112px;
    min-width: 94px;
  }

  .toolbox-preview-card {
    width: 62px;
    height: 70px;
    border-radius: 14px;
  }

  .toolbox-preview-card--back {
    top: 6px;
    left: 0;
  }

  .toolbox-preview-card--middle {
    top: 16px;
    left: 14px;
  }

  .toolbox-preview-card--front {
    top: 26px;
    left: 28px;
  }

  .toolbox-preview-label {
    right: 14px;
    bottom: 10px;
    font-size: .58rem;
  }
}

@media (max-width: 640px) {
  .toolbox-shell {
    width: min(100% - 20px, 560px);
    padding-top: 18px;
  }

  .toolbox-hero {
    border-radius: 14px;
  }

  .toolbox-hero__main {
    padding: 20px;
  }

  .toolbox-hero__preview {
    display: none;
  }

  .toolbox-search input {
    padding-right: 46px;
  }

  .toolbox-search__shortcut {
    display: none;
  }

  .toolbox-filter-strip {
    gap: 7px;
  }

  .tool-filter {
    min-height: 40px;
  }

  .toolbox-catalog__header {
    align-items: flex-start;
    flex-direction: column;
  }

  .toolbox-view-controls {
    width: 100%;
    justify-content: space-between;
  }

  .toolbox-grid,
  .toolbox-grid--list {
    grid-template-columns: 1fr;
  }

  .tool-card {
    min-height: 112px;
  }

  .tool-empty-state {
    min-height: 340px;
    padding: 26px 18px;
  }

  .tool-empty-state__actions {
    width: 100%;
    flex-direction: column;
  }

  .tool-empty-action {
    width: 100%;
    min-height: 44px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .tool-card,
  .tool-card-arrow,
  .tool-filter,
  .toolbox-search input {
    transition-duration: .01ms;
  }

  .tool-card:hover,
  .tool-filter:hover {
    transform: none;
  }
}
</style>
