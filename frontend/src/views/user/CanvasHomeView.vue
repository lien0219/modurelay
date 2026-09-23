<template>
  <CanvasWorkspaceNav>
    <div class="canvas-home-page">
      <main id="canvas-workspace-content" class="canvas-home" tabindex="-1">
      <section class="canvas-home__hero" aria-labelledby="canvas-home-title">
        <div class="canvas-home__eyebrow">
          <span aria-hidden="true"><Icon name="sparkles" size="sm" /></span>
          {{ t('canvas.home.eyebrow') }}
        </div>
        <h1 id="canvas-home-title">{{ t('canvas.home.title') }}</h1>
        <p>{{ t('canvas.home.description') }}</p>

        <div class="canvas-home__hero-actions">
          <button
            type="button"
            class="canvas-home__primary-action"
            data-testid="canvas-start"
            :disabled="creatingMode !== null"
            :aria-busy="creatingMode === 'starter' || undefined"
            @click="createProject('starter')"
          >
            <Icon name="sparkles" size="sm" aria-hidden="true" />
            {{ creatingMode === 'starter' ? t('canvas.home.creating') : t('canvas.home.startUsing') }}
            <Icon name="arrowRight" size="sm" aria-hidden="true" />
          </button>
          <button
            v-if="latestProject"
            type="button"
            class="canvas-home__secondary-action"
            data-testid="canvas-open-latest"
            :disabled="creatingMode !== null"
            @click="openProject(latestProject.id)"
          >
            <Icon name="clock" size="sm" aria-hidden="true" />
            {{ t('canvas.home.continueLatest') }}
          </button>
          <button
            v-else
            type="button"
            class="canvas-home__secondary-action"
            data-testid="canvas-create-blank"
            :disabled="creatingMode !== null"
            :aria-busy="creatingMode === 'blank' || undefined"
            @click="createProject('blank')"
          >
            <Icon name="plus" size="sm" aria-hidden="true" />
            {{ t('canvas.home.blankCanvas') }}
          </button>
        </div>

        <button
          type="button"
          class="canvas-home__starter-preview"
          :aria-label="t('canvas.home.useStarterTemplate')"
          :disabled="creatingMode !== null"
          @click="createProject('starter')"
        >
          <span class="canvas-home__starter-label">
            <span>
              <Icon name="sparkles" size="sm" aria-hidden="true" />
              {{ t('canvas.home.starterTemplate') }}
            </span>
            <span>{{ t('canvas.home.starterPath') }} <Icon name="arrowRight" size="xs" aria-hidden="true" /></span>
          </span>
          <CanvasProjectPreview :document="starterDocument" />
        </button>
      </section>

      <section id="canvas-prompts" class="canvas-home__collection" aria-labelledby="canvas-prompts-title">
        <header class="canvas-home__collection-header">
          <div>
            <span class="canvas-home__collection-icon" aria-hidden="true"><Icon name="document" size="sm" /></span>
            <div>
              <h2 id="canvas-prompts-title">{{ t('canvas.home.promptLibraryTitle') }}</h2>
              <p>{{ t('canvas.home.promptLibraryDescription') }}</p>
            </div>
          </div>
          <span>{{ t('canvas.home.promptCount', { count: promptRecords.length }) }}</span>
        </header>

        <div v-if="promptRecords.length" class="canvas-home__prompt-grid">
          <article v-for="promptRecord in promptRecords" :key="promptRecord.id" class="canvas-home__prompt-card">
            <p>{{ promptRecord.prompt }}</p>
            <footer>
              <RouterLink :to="editorLocation(promptRecord.projectId)">
                {{ promptRecord.projectTitle }}
              </RouterLink>
              <button
                type="button"
                :aria-label="t('canvas.home.copyPromptNamed', { title: promptRecord.projectTitle })"
                :title="t('canvas.home.copyPrompt')"
                @click="copyPrompt(promptRecord.prompt)"
              >
                <Icon name="copy" size="sm" aria-hidden="true" />
              </button>
            </footer>
          </article>
        </div>
        <div v-else class="canvas-home__inline-empty">
          <span aria-hidden="true"><Icon name="document" size="lg" /></span>
          <div>
            <h3>{{ t('canvas.home.emptyPromptsTitle') }}</h3>
            <p>{{ t('canvas.home.emptyPromptsDescription') }}</p>
          </div>
          <button type="button" :disabled="creatingMode !== null" @click="createProject('starter')">
            {{ t('canvas.home.createPromptWorkflow') }}
          </button>
        </div>
      </section>

      <section id="canvas-assets" class="canvas-home__collection" aria-labelledby="canvas-assets-title">
        <header class="canvas-home__collection-header">
          <div>
            <span class="canvas-home__collection-icon is-accent" aria-hidden="true"><Icon name="inbox" size="sm" /></span>
            <div>
              <h2 id="canvas-assets-title">{{ t('canvas.home.assetLibraryTitle') }}</h2>
              <p>{{ t('canvas.home.assetLibraryDescription') }}</p>
            </div>
          </div>
          <span>{{ t('canvas.home.assetCount', { count: assetRecords.length }) }}</span>
        </header>

        <div v-if="assetRecords.length" class="canvas-home__asset-grid">
          <article v-for="asset in assetRecords" :key="asset.id" class="canvas-home__asset-card">
            <RouterLink :to="editorLocation(asset.projectId)" :aria-label="t('canvas.home.openAssetNamed', { title: asset.label })">
              <span class="canvas-home__asset-preview">
                <img
                  v-if="assetPreviewURLs[asset.assetId]"
                  :src="assetPreviewURLs[asset.assetId]"
                  :alt="asset.label"
                  loading="lazy"
                  @error="markAssetUnavailable(asset.assetId)"
                />
                <Icon v-else name="inbox" size="lg" aria-hidden="true" />
              </span>
              <span class="canvas-home__asset-body">
                <strong>{{ asset.label }}</strong>
                <small>{{ asset.projectTitle }}</small>
              </span>
            </RouterLink>
          </article>
        </div>
        <div v-else class="canvas-home__inline-empty">
          <span aria-hidden="true"><Icon name="inbox" size="lg" /></span>
          <div>
            <h3>{{ t('canvas.home.emptyAssetsTitle') }}</h3>
            <p>{{ t('canvas.home.emptyAssetsDescription') }}</p>
          </div>
          <RouterLink :to="{ name: 'BatchImageGuide' }">{{ t('canvas.home.openImageWorkbench') }}</RouterLink>
        </div>
      </section>

      <section id="canvas-library" class="canvas-home__library" aria-labelledby="canvas-library-title">
        <header class="canvas-home__library-header">
          <div>
            <div class="canvas-home__section-title-row">
              <h2 id="canvas-library-title">{{ t('canvas.home.myCanvases') }}</h2>
              <span>{{ t('canvas.home.projectCount', { count: store.projects.length }) }}</span>
            </div>
            <p>{{ t('canvas.home.libraryDescription') }}</p>
          </div>

          <div class="canvas-home__library-actions">
            <label class="canvas-home__search">
              <span class="sr-only">{{ t('canvas.home.searchProjects') }}</span>
              <Icon name="search" size="sm" aria-hidden="true" />
              <input
                v-model="searchQuery"
                type="search"
                :placeholder="t('canvas.home.searchProjects')"
              />
            </label>
            <button
              type="button"
              class="canvas-home__new-button"
              :disabled="creatingMode !== null"
              @click="createProject('blank')"
            >
              <Icon name="plus" size="sm" aria-hidden="true" />
              {{ t('canvas.home.newCanvas') }}
            </button>
          </div>
        </header>

        <div v-if="visibleError" class="canvas-home__error" role="alert">
          <Icon name="exclamationCircle" size="sm" aria-hidden="true" />
          <span>{{ visibleError }}</span>
          <button type="button" @click="loadProjects">{{ t('canvas.home.retry') }}</button>
        </div>

        <div
          v-if="store.loading && !store.projects.length"
          class="canvas-home__project-grid"
          role="status"
          :aria-label="t('canvas.home.loadingProjects')"
        >
          <div v-for="index in 3" :key="index" class="canvas-home__skeleton" aria-hidden="true">
            <span></span><i></i><i></i>
          </div>
        </div>

        <div v-else-if="filteredProjects.length" class="canvas-home__project-grid" :aria-busy="store.loading || undefined">
          <article
            v-for="project in filteredProjects"
            :key="project.id"
            class="canvas-home__project-card"
            :data-project-id="project.id"
          >
            <button
              type="button"
              class="canvas-home__project-open"
              :aria-label="t('canvas.home.openProjectNamed', { title: project.title })"
              @click="openProject(project.id)"
            >
              <CanvasProjectPreview :document="project.document" />
              <span class="canvas-home__project-body">
                <strong>{{ project.title }}</strong>
                <small>{{ projectSummary(project) }}</small>
                <span>
                  {{ t('canvas.home.updatedAt', { time: formatProjectDate(project) }) }}
                  <Icon name="arrowRight" size="xs" aria-hidden="true" />
                </span>
              </span>
            </button>

            <div class="canvas-home__project-actions">
              <button
                type="button"
                :title="t('canvas.home.openInNewTab')"
                :aria-label="t('canvas.home.openProjectInNewTabNamed', { title: project.title })"
                @click="openProjectInNewTab(project.id)"
              >
                <Icon name="externalLink" size="sm" aria-hidden="true" />
              </button>
              <button
                type="button"
                class="is-danger"
                :title="t('canvas.deleteProject')"
                :aria-label="t('canvas.home.deleteProjectNamed', { title: project.title })"
                :disabled="deletingProjectId !== null"
                @click="deleteCandidate = project"
              >
                <Icon name="trash" size="sm" aria-hidden="true" />
              </button>
            </div>
          </article>
        </div>

        <div v-else-if="store.projects.length" class="canvas-home__empty-state">
          <span aria-hidden="true"><Icon name="search" size="lg" /></span>
          <h3>{{ t('canvas.home.noResultsTitle') }}</h3>
          <p>{{ t('canvas.home.noResultsDescription') }}</p>
          <button type="button" @click="searchQuery = ''">{{ t('canvas.home.clearSearch') }}</button>
        </div>

        <div v-else class="canvas-home__empty-state">
          <span aria-hidden="true"><Icon name="grid" size="lg" /></span>
          <h3>{{ t('canvas.home.emptyLibraryTitle') }}</h3>
          <p>{{ t('canvas.home.emptyLibraryDescription') }}</p>
          <button type="button" :disabled="creatingMode !== null" @click="createProject('starter')">
            <Icon name="sparkles" size="sm" aria-hidden="true" />
            {{ t('canvas.home.startUsing') }}
          </button>
        </div>
      </section>

      <ConfirmDialog
        :show="Boolean(deleteCandidate)"
        :title="t('canvas.home.deleteDialogTitle')"
        :message="t('canvas.deleteProjectConfirm', { title: deleteCandidate?.title || '' })"
        :confirm-text="t('canvas.deleteProject')"
        :confirming="deletingProjectId !== null"
        danger
        @confirm="confirmDeleteProject"
        @cancel="deleteCandidate = null"
      />
      </main>
    </div>
  </CanvasWorkspaceNav>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { canvasAPI, type CanvasProject } from '@/api/canvas'
import CanvasProjectPreview from '@/components/canvas/CanvasProjectPreview.vue'
import CanvasWorkspaceNav from '@/components/canvas/CanvasWorkspaceNav.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { useCanvasStore } from '@/stores/canvas'
import { createStarterCanvasDocument } from '@/utils/canvasGraph'

type CreationMode = 'starter' | 'blank'

const store = useCanvasStore()
const router = useRouter()
const route = useRoute()
const { t, locale } = useI18n()
const { copyToClipboard } = useClipboard()
const searchQuery = ref('')
const creatingMode = ref<CreationMode | null>(null)
const deletingProjectId = ref<number | null>(null)
const deleteCandidate = ref<CanvasProject | null>(null)
const actionError = ref('')
const assetPreviewURLs = ref<Record<number, string>>({})

const starterDocument = computed(() => createStarterCanvasDocument({
  text: t('canvas.textNodeLabel'),
  config: t('canvas.configNodeLabel'),
  image: t('canvas.imageNodeLabel'),
}))

const sortedProjects = computed(() => [...store.projects].sort((left, right) => {
  const leftTime = Date.parse(left.last_opened_at || left.updated_at)
  const rightTime = Date.parse(right.last_opened_at || right.updated_at)
  return rightTime - leftTime || right.id - left.id
}))

const latestProject = computed(() => sortedProjects.value[0])
const normalizedSearch = computed(() => searchQuery.value.trim().toLocaleLowerCase(locale.value))
const filteredProjects = computed(() => {
  if (!normalizedSearch.value) return sortedProjects.value
  return sortedProjects.value.filter(project => (
    project.title.toLocaleLowerCase(locale.value).includes(normalizedSearch.value)
  ))
})
const visibleError = computed(() => actionError.value || store.error)
const promptRecords = computed(() => {
  const seen = new Set<string>()
  return sortedProjects.value.flatMap(project => project.document.nodes.flatMap(node => {
    if (node.type !== 'prompt' && node.type !== 'text') return []
    const prompt = String(node.type === 'text' ? node.data?.content : node.data?.prompt || '').trim()
    const dedupeKey = prompt.toLocaleLowerCase(locale.value)
    if (!prompt || seen.has(dedupeKey)) return []
    seen.add(dedupeKey)
    return [{ id: `${project.id}-${node.id}`, projectId: project.id, projectTitle: project.title, prompt }]
  })).slice(0, 12)
})
const assetRecords = computed(() => {
  const seen = new Set<number>()
  return sortedProjects.value.flatMap(project => project.document.nodes.flatMap(node => {
    if (node.type !== 'reference' && node.type !== 'image') return []
    const assetId = Number(node.data?.assetId)
    if (!Number.isSafeInteger(assetId) || assetId <= 0 || seen.has(assetId)) return []
    seen.add(assetId)
    return [{
      id: `${project.id}-${node.id}`,
      assetId,
      projectId: project.id,
      projectTitle: project.title,
      label: String(node.data?.fileName || node.data?.label || t('canvas.imageNodeLabel')),
    }]
  })).slice(0, 12)
})

function editorLocation(projectId: number) {
  return { name: 'CanvasEditor', query: { project: String(projectId) } }
}

async function loadProjects() {
  actionError.value = ''
  await store.list()
  if (!store.error) void loadAssetPreviews()
}

async function loadAssetPreviews() {
  await Promise.all(assetRecords.value.map(async asset => {
    if (assetPreviewURLs.value[asset.assetId]) return
    try {
      const url = await canvasAPI.assetURL(asset.assetId)
      assetPreviewURLs.value = { ...assetPreviewURLs.value, [asset.assetId]: url }
    } catch {
      // A stale or deleted asset remains discoverable through its source canvas.
    }
  }))
}

function markAssetUnavailable(assetId: number) {
  const next = { ...assetPreviewURLs.value }
  delete next[assetId]
  assetPreviewURLs.value = next
}

function copyPrompt(prompt: string) {
  void copyToClipboard(prompt, t('canvas.home.promptCopied'))
}

async function scrollToCanvasHash(hash: string) {
  if (!hash) return
  await nextTick()
  await new Promise<void>(resolve => {
    requestAnimationFrame(() => requestAnimationFrame(() => resolve()))
  })
  document.querySelector(hash)?.scrollIntoView({ block: 'start' })
}

function openProject(projectId: number) {
  void router.push(editorLocation(projectId))
}

function openProjectInNewTab(projectId: number) {
  const href = router.resolve(editorLocation(projectId)).href
  window.open(href, '_blank', 'noopener,noreferrer')
}

async function createProject(mode: CreationMode) {
  if (creatingMode.value) return
  creatingMode.value = mode
  actionError.value = ''
  try {
    const created = await store.create(t('canvas.untitled'))
    if (mode === 'starter') {
      await store.save(created.title, starterDocument.value)
    }
    await router.push(editorLocation(created.id))
  } catch (error) {
    actionError.value = error instanceof Error ? error.message : t('canvas.failedToCreateProject')
  } finally {
    creatingMode.value = null
  }
}

async function confirmDeleteProject() {
  const candidate = deleteCandidate.value
  if (!candidate || deletingProjectId.value !== null) return
  deletingProjectId.value = candidate.id
  actionError.value = ''
  try {
    await store.remove(candidate.id)
    deleteCandidate.value = null
  } catch (error) {
    actionError.value = error instanceof Error ? error.message : t('canvas.failedToDeleteProject')
  } finally {
    deletingProjectId.value = null
  }
}

function projectSummary(project: CanvasProject) {
  return t('canvas.projectStats', {
    nodes: project.document.nodes.length,
    edges: project.document.edges.length,
  })
}

function formatProjectDate(project: CanvasProject) {
  const value = new Date(project.last_opened_at || project.updated_at)
  if (Number.isNaN(value.getTime())) return t('canvas.home.unknownDate')
  return new Intl.DateTimeFormat(locale.value, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(value)
}

onMounted(() => {
  void loadProjects()
  void scrollToCanvasHash(route.hash)
})

watch(() => route.hash, hash => { void scrollToCanvasHash(hash) })
</script>

<style scoped>
.canvas-home-page {
  min-height: calc(100dvh - 4rem);
  background: var(--color-bg);
}

.canvas-home {
  width: min(1240px, calc(100% - 48px));
  min-width: 0;
  margin: 0 auto;
  color: var(--color-text-primary);
}

.canvas-home:focus { outline: none; }

.canvas-home__hero {
  display: flex;
  min-height: 500px;
  align-items: center;
  flex-direction: column;
  justify-content: center;
  padding: 34px 20px 44px;
  text-align: center;
}

.canvas-home__eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--color-text-secondary);
  font-size: 13px;
  font-weight: 650;
}

.canvas-home__eyebrow > span {
  display: grid;
  width: 30px;
  height: 30px;
  place-items: center;
  border: 1px solid var(--color-primary-border);
  border-radius: 8px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.canvas-home__hero h1 {
  margin: 18px 0 0;
  font-size: 44px;
  font-weight: 750;
  letter-spacing: 0;
  line-height: 1.12;
}

.canvas-home__hero > p {
  width: min(600px, 100%);
  margin: 16px auto 0;
  color: var(--color-text-secondary);
  font-size: 15px;
  line-height: 1.7;
}

.canvas-home__hero-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 10px;
  margin-top: 24px;
}

.canvas-home__primary-action,
.canvas-home__secondary-action,
.canvas-home__new-button,
.canvas-home__empty-state button,
.canvas-home__inline-empty button,
.canvas-home__inline-empty a {
  display: inline-flex;
  min-height: 40px;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 0 16px;
  border: 1px solid transparent;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 650;
  cursor: pointer;
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard),
    color var(--motion-fast) var(--ease-standard),
    box-shadow var(--motion-fast) var(--ease-standard),
    transform var(--motion-fast) var(--ease-standard);
}

.canvas-home__primary-action,
.canvas-home__new-button,
.canvas-home__empty-state button,
.canvas-home__inline-empty button {
  background: var(--color-primary);
  color: var(--color-text-on-primary, white);
}

.canvas-home__primary-action:hover,
.canvas-home__new-button:hover,
.canvas-home__empty-state button:hover,
.canvas-home__inline-empty button:hover {
  background: var(--color-primary-hover);
  transform: translateY(-1px);
}

.canvas-home__secondary-action {
  border-color: var(--color-border);
  background: var(--color-surface);
  color: var(--color-text-primary);
  box-shadow: var(--shadow-xs);
}

.canvas-home__secondary-action:hover {
  border-color: var(--color-primary-border);
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.canvas-home__primary-action:disabled,
.canvas-home__secondary-action:disabled,
.canvas-home__new-button:disabled,
.canvas-home__starter-preview:disabled,
.canvas-home__empty-state button:disabled,
.canvas-home__inline-empty button:disabled {
  cursor: not-allowed;
  opacity: 0.55;
  transform: none;
}

.canvas-home__starter-preview {
  width: min(560px, 100%);
  margin-top: 26px;
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  background: var(--color-surface);
  box-shadow: var(--shadow-sm);
  color: var(--color-text-primary);
  cursor: pointer;
  text-align: left;
  transition:
    border-color var(--motion-base) var(--ease-standard),
    box-shadow var(--motion-base) var(--ease-standard),
    transform var(--motion-base) var(--ease-standard);
}

.canvas-home__starter-preview:hover {
  border-color: var(--color-primary-border);
  box-shadow: var(--shadow-md);
  transform: translateY(-2px);
}

.canvas-home__starter-label {
  display: flex;
  min-width: 0;
  min-height: 42px;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 0 14px;
  border-bottom: 1px solid var(--color-border-subtle);
  font-size: 12px;
}

.canvas-home__starter-label > span {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 7px;
}

.canvas-home__starter-label > span:first-child { color: var(--color-text-primary); font-weight: 650; }
.canvas-home__starter-label > span:last-child { color: var(--color-text-muted); white-space: nowrap; }
.canvas-home__starter-label > span:first-child :deep(svg) { color: var(--color-primary); }

.canvas-home__collection,
.canvas-home__library {
  scroll-margin-top: calc(4rem + 76px);
  padding: 28px 0 56px;
  border-top: 1px solid var(--color-border-subtle);
}

.canvas-home__collection-header {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  margin-bottom: 18px;
}

.canvas-home__collection-header > div {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  gap: 11px;
}

.canvas-home__collection-header h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
  letter-spacing: 0;
}

.canvas-home__collection-header p {
  margin: 5px 0 0;
  color: var(--color-text-muted);
  font-size: 13px;
  line-height: 1.55;
}

.canvas-home__collection-header > span {
  flex: 0 0 auto;
  padding: 4px 8px;
  border-radius: 999px;
  background: var(--color-surface-soft);
  color: var(--color-text-muted);
  font-size: 12px;
  white-space: nowrap;
}

.canvas-home__collection-icon {
  display: grid;
  width: 34px;
  height: 34px;
  flex: 0 0 auto;
  place-items: center;
  border: 1px solid var(--color-primary-border);
  border-radius: 8px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.canvas-home__collection-icon.is-accent {
  border-color: color-mix(in srgb, var(--color-accent) 36%, var(--color-border));
  background: var(--color-accent-soft);
  color: var(--color-accent);
}

.canvas-home__prompt-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
}

.canvas-home__prompt-card {
  display: flex;
  min-width: 0;
  min-height: 176px;
  flex-direction: column;
  justify-content: space-between;
  padding: 16px;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  background: var(--color-surface);
  box-shadow: var(--shadow-xs);
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    box-shadow var(--motion-fast) var(--ease-standard),
    transform var(--motion-fast) var(--ease-standard);
}

.canvas-home__prompt-card:hover {
  border-color: var(--color-primary-border);
  box-shadow: var(--shadow-sm);
  transform: translateY(-1px);
}

.canvas-home__prompt-card > p {
  display: -webkit-box;
  margin: 0;
  overflow: hidden;
  color: var(--color-text-secondary);
  font-size: 13px;
  line-height: 1.65;
  overflow-wrap: anywhere;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 5;
}

.canvas-home__prompt-card footer {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 16px;
  padding-top: 12px;
  border-top: 1px solid var(--color-border-subtle);
}

.canvas-home__prompt-card footer a {
  min-width: 0;
  overflow: hidden;
  color: var(--color-primary);
  font-size: 12px;
  font-weight: 650;
  text-decoration: none;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.canvas-home__prompt-card footer button {
  display: grid;
  width: 34px;
  height: 34px;
  flex: 0 0 auto;
  place-items: center;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  background: var(--color-surface-soft);
  color: var(--color-text-secondary);
  cursor: pointer;
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard),
    color var(--motion-fast) var(--ease-standard);
}

.canvas-home__prompt-card footer button:hover {
  border-color: var(--color-primary-border);
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.canvas-home__asset-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
}

.canvas-home__asset-card {
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  background: var(--color-surface);
  box-shadow: var(--shadow-xs);
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    box-shadow var(--motion-fast) var(--ease-standard),
    transform var(--motion-fast) var(--ease-standard);
}

.canvas-home__asset-card:hover {
  border-color: var(--color-primary-border);
  box-shadow: var(--shadow-sm);
  transform: translateY(-1px);
}

.canvas-home__asset-card > a {
  display: block;
  color: inherit;
  text-decoration: none;
}

.canvas-home__asset-preview {
  display: grid;
  aspect-ratio: 4 / 3;
  place-items: center;
  overflow: hidden;
  background: var(--color-bg-subtle);
  color: var(--color-text-muted);
}

.canvas-home__asset-preview img { display: block; width: 100%; height: 100%; object-fit: cover; }
.canvas-home__asset-body { display: block; min-width: 0; padding: 12px 13px 13px; }
.canvas-home__asset-body strong,
.canvas-home__asset-body small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.canvas-home__asset-body strong { font-size: 13px; font-weight: 700; }
.canvas-home__asset-body small { margin-top: 4px; color: var(--color-text-muted); font-size: 12px; }

.canvas-home__inline-empty {
  display: flex;
  min-height: 112px;
  align-items: center;
  gap: 14px;
  padding: 18px;
  border: 1px dashed var(--color-border-strong);
  border-radius: 8px;
  background: var(--color-surface-soft);
}

.canvas-home__inline-empty > span {
  display: grid;
  width: 44px;
  height: 44px;
  flex: 0 0 auto;
  place-items: center;
  border-radius: 8px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.canvas-home__inline-empty > div { min-width: 0; flex: 1; }
.canvas-home__inline-empty h3 { margin: 0; font-size: 15px; font-weight: 700; }
.canvas-home__inline-empty p { margin: 5px 0 0; color: var(--color-text-muted); font-size: 12px; line-height: 1.55; }
.canvas-home__inline-empty a { flex: 0 0 auto; border-color: var(--color-border); background: var(--color-surface); color: var(--color-primary); text-decoration: none; }
.canvas-home__inline-empty a:hover { border-color: var(--color-primary-border); background: var(--color-primary-soft); }

.canvas-home__library-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 20px;
}

.canvas-home__library-header > div:first-child { min-width: 0; }
.canvas-home__section-title-row { display: flex; align-items: center; gap: 10px; }
.canvas-home__section-title-row h2 { margin: 0; font-size: 20px; font-weight: 700; letter-spacing: 0; }
.canvas-home__section-title-row span { padding: 3px 8px; border-radius: 999px; background: var(--color-surface-soft); color: var(--color-text-muted); font-size: 12px; white-space: nowrap; }
.canvas-home__library-header p { margin: 6px 0 0; color: var(--color-text-muted); font-size: 13px; }

.canvas-home__library-actions {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 10px;
}

.canvas-home__search {
  display: flex;
  width: 250px;
  height: 40px;
  align-items: center;
  gap: 8px;
  padding: 0 11px;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  background: var(--color-surface);
  color: var(--color-text-muted);
  box-shadow: var(--shadow-xs);
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    box-shadow var(--motion-fast) var(--ease-standard);
}

.canvas-home__search:focus-within {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 3px var(--color-primary-ring);
}

.canvas-home__search input {
  width: 100%;
  min-width: 0;
  border: 0;
  background: transparent;
  color: var(--color-text-primary);
  font: inherit;
  font-size: 13px;
  outline: none;
}

.canvas-home__search input::placeholder { color: var(--color-text-muted); }

.canvas-home__error {
  display: flex;
  align-items: center;
  gap: 9px;
  margin-bottom: 18px;
  padding: 11px 13px;
  border: 1px solid color-mix(in srgb, var(--color-danger) 34%, var(--color-border));
  border-radius: 8px;
  background: color-mix(in srgb, var(--color-danger) 7%, var(--color-surface));
  color: var(--color-danger);
  font-size: 13px;
}

.canvas-home__error span { min-width: 0; flex: 1; overflow-wrap: anywhere; }
.canvas-home__error button { border: 0; background: transparent; color: currentColor; cursor: pointer; font: inherit; font-weight: 700; }

.canvas-home__project-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}

.canvas-home__project-card {
  position: relative;
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  background: var(--color-surface);
  box-shadow: var(--shadow-xs);
  transition:
    border-color var(--motion-base) var(--ease-standard),
    box-shadow var(--motion-base) var(--ease-standard),
    transform var(--motion-base) var(--ease-standard);
}

.canvas-home__project-card:hover {
  border-color: var(--color-primary-border);
  box-shadow: var(--shadow-md);
  transform: translateY(-2px);
}

.canvas-home__project-open {
  display: block;
  width: 100%;
  border: 0;
  background: transparent;
  color: inherit;
  cursor: pointer;
  text-align: left;
}

.canvas-home__project-body {
  display: block;
  min-width: 0;
  padding: 14px 16px 15px;
}

.canvas-home__project-body strong,
.canvas-home__project-body small,
.canvas-home__project-body > span { display: flex; min-width: 0; align-items: center; }
.canvas-home__project-body strong { padding-right: 72px; overflow: hidden; font-size: 14px; font-weight: 700; text-overflow: ellipsis; white-space: nowrap; }
.canvas-home__project-body small { margin-top: 5px; color: var(--color-text-muted); font-size: 12px; }
.canvas-home__project-body > span { justify-content: space-between; gap: 12px; margin-top: 12px; color: var(--color-text-secondary); font-size: 12px; }
.canvas-home__project-body > span :deep(svg) { flex: 0 0 auto; color: var(--color-primary); }

.canvas-home__project-actions {
  position: absolute;
  z-index: 2;
  top: 10px;
  right: 10px;
  display: flex;
  gap: 4px;
  padding: 3px;
  border: 1px solid var(--glass-border);
  border-radius: 8px;
  background: var(--glass-bg-strong);
  box-shadow: var(--shadow-xs);
  backdrop-filter: blur(14px);
}

.canvas-home__project-actions button {
  display: grid;
  width: 32px;
  height: 32px;
  place-items: center;
  border: 1px solid transparent;
  border-radius: 7px;
  background: transparent;
  color: var(--color-text-secondary);
  cursor: pointer;
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard),
    color var(--motion-fast) var(--ease-standard);
}

.canvas-home__project-actions button:hover { border-color: var(--color-primary-border); background: var(--color-primary-soft); color: var(--color-primary); }
.canvas-home__project-actions button.is-danger:hover { border-color: color-mix(in srgb, var(--color-danger) 40%, var(--color-border)); color: var(--color-danger); }
.canvas-home__project-actions button:disabled { cursor: not-allowed; opacity: 0.5; }

.canvas-home__empty-state {
  display: flex;
  min-height: 260px;
  align-items: center;
  flex-direction: column;
  justify-content: center;
  padding: 36px 20px;
  border: 1px dashed var(--color-border-strong);
  border-radius: 8px;
  background: var(--color-surface-soft);
  text-align: center;
}

.canvas-home__empty-state > span {
  display: grid;
  width: 48px;
  height: 48px;
  place-items: center;
  border: 1px solid var(--color-primary-border);
  border-radius: 10px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.canvas-home__empty-state h3 { margin: 16px 0 0; font-size: 16px; font-weight: 700; }
.canvas-home__empty-state p { max-width: 430px; margin: 7px 0 18px; color: var(--color-text-muted); font-size: 13px; line-height: 1.6; }

.canvas-home__skeleton {
  min-height: 260px;
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  background: var(--color-surface);
}

.canvas-home__skeleton span,
.canvas-home__skeleton i {
  display: block;
  background: var(--color-surface-soft);
  animation: canvas-home-pulse 1.25s ease-in-out infinite alternate;
}

.canvas-home__skeleton span { height: 180px; }
.canvas-home__skeleton i { width: 60%; height: 12px; margin: 15px 16px 0; border-radius: 4px; }
.canvas-home__skeleton i:last-child { width: 38%; margin-top: 9px; }

.canvas-home__primary-action:focus-visible,
.canvas-home__secondary-action:focus-visible,
.canvas-home__new-button:focus-visible,
.canvas-home__starter-preview:focus-visible,
.canvas-home__project-open:focus-visible,
.canvas-home__project-actions button:focus-visible,
.canvas-home__empty-state button:focus-visible,
.canvas-home__inline-empty button:focus-visible,
.canvas-home__inline-empty a:focus-visible,
.canvas-home__prompt-card footer a:focus-visible,
.canvas-home__prompt-card footer button:focus-visible,
.canvas-home__asset-card > a:focus-visible,
.canvas-home__error button:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}

@keyframes canvas-home-pulse {
  from { opacity: 0.52; }
  to { opacity: 1; }
}

@media (max-width: 1024px) {
  .canvas-home__prompt-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .canvas-home__asset-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .canvas-home__project-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}

@media (max-width: 760px) {
  .canvas-home__hero { min-height: 0; padding: 28px 0 38px; }
  .canvas-home__hero h1 { margin-top: 15px; font-size: 34px; }
  .canvas-home__hero > p { font-size: 14px; }
  .canvas-home__hero-actions { width: 100%; }
  .canvas-home__primary-action,
  .canvas-home__secondary-action { min-height: 44px; }
  .canvas-home__library { padding-bottom: 40px; }
  .canvas-home__asset-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .canvas-home__library-header { align-items: stretch; flex-direction: column; gap: 16px; }
  .canvas-home__library-actions { width: 100%; }
  .canvas-home__search { width: auto; min-width: 0; flex: 1; height: 44px; }
  .canvas-home__new-button { min-height: 44px; padding: 0 13px; }
  .canvas-home__project-grid { grid-template-columns: 1fr; }
  .canvas-home__project-actions button { width: 36px; height: 36px; }
}

@media (max-width: 430px) {
  .canvas-home { width: min(100% - 32px, 1240px); }
  .canvas-home__hero h1 { font-size: 30px; }
  .canvas-home__hero-actions { flex-direction: column; }
  .canvas-home__primary-action,
  .canvas-home__secondary-action { width: 100%; }
  .canvas-home__starter-label > span:last-child { display: none; }
  .canvas-home__library-actions { align-items: stretch; flex-direction: column; }
  .canvas-home__new-button { width: 100%; }
  .canvas-home__collection-header { align-items: flex-start; }
  .canvas-home__collection-header > span { display: none; }
  .canvas-home__prompt-grid,
  .canvas-home__asset-grid { grid-template-columns: 1fr; }
  .canvas-home__inline-empty { align-items: stretch; flex-direction: column; }
  .canvas-home__inline-empty button,
  .canvas-home__inline-empty a { width: 100%; min-height: 44px; }
}

@media (prefers-reduced-motion: reduce) {
  .canvas-home__primary-action,
  .canvas-home__secondary-action,
  .canvas-home__new-button,
  .canvas-home__starter-preview,
  .canvas-home__project-card,
  .canvas-home__prompt-card,
  .canvas-home__asset-card { transition-duration: 0.01ms; }
  .canvas-home__skeleton span,
  .canvas-home__skeleton i { animation: none; }
}
</style>
