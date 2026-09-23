import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { CANVAS_DOCUMENT_SCHEMA_VERSION, canvasAPI, type CanvasDocument, type CanvasProject } from '@/api/canvas'
import i18n from '@/i18n'

const emptyDocument = (): CanvasDocument => ({
  schema_version: CANVAS_DOCUMENT_SCHEMA_VERSION,
  nodes: [],
  edges: [],
  viewport: { x: 0, y: 0, zoom: 1 },
})

export const useCanvasStore = defineStore('canvas', () => {
  const projects = ref<CanvasProject[]>([])
  const project = ref<CanvasProject | null>(null)
  const loading = ref(false)
  const saving = ref(false)
  const error = ref('')
  let saveTimer: ReturnType<typeof setTimeout> | null = null
  let pendingSave: { title: string; document: CanvasDocument } | null = null
  let savePromise: Promise<void> | null = null

  const document = computed(() => project.value?.document || emptyDocument())

  async function list() {
    loading.value = true
    try { projects.value = await canvasAPI.listProjects(); error.value = '' }
    catch (err) { error.value = err instanceof Error ? err.message : i18n.global.t('canvas.failedToLoadProjects') }
    finally { loading.value = false }
  }

  async function open(id: number) {
    loading.value = true
    try { project.value = await canvasAPI.getProject(id); error.value = '' }
    catch (err) { error.value = err instanceof Error ? err.message : i18n.global.t('canvas.failedToOpenProject') }
    finally { loading.value = false }
  }

  async function create(title?: string) {
    const created = await canvasAPI.createProject(title)
    projects.value = [created, ...projects.value]
    project.value = created
    return created
  }

  async function save(title: string, nextDocument: CanvasDocument) {
    if (!project.value) return
    pendingSave = { title, document: nextDocument }
    if (savePromise) return savePromise

    saving.value = true
    savePromise = (async () => {
      while (pendingSave && project.value) {
        const next = pendingSave
        pendingSave = null
        try {
          project.value = await canvasAPI.saveProject(
            project.value.id,
            project.value.revision,
            next.title,
            next.document,
          )
          const index = projects.value.findIndex(item => item.id === project.value?.id)
          if (index >= 0 && project.value) projects.value[index] = project.value
          error.value = ''
        } catch (err) {
          error.value = err instanceof Error ? err.message : i18n.global.t('canvas.failedToSave')
          throw err
        }
      }
    })()

    try {
      await savePromise
    } finally {
      savePromise = null
      saving.value = false
    }
  }

  function scheduleSave(title: string, nextDocument: CanvasDocument) {
    if (saveTimer) clearTimeout(saveTimer)
    saveTimer = setTimeout(() => { void save(title, nextDocument) }, 900)
  }

  async function remove(id: number) {
    await canvasAPI.deleteProject(id)
    projects.value = projects.value.filter(item => item.id !== id)
    if (project.value?.id === id) project.value = null
  }

  return { projects, project, document, loading, saving, error, list, open, create, save, scheduleSave, remove }
})
