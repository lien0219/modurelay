import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { workspaceAPI, type Project, type Workspace } from '@/api/workspace'

const STORAGE_KEY = 'modurelay.workspace-selection'

interface SelectionPreference {
  workspaceId?: number
  projectId?: number
}

function readPreference(): SelectionPreference {
  try {
    const parsed = JSON.parse(localStorage.getItem(STORAGE_KEY) || '{}') as SelectionPreference
    return {
      workspaceId: Number.isInteger(parsed.workspaceId) ? parsed.workspaceId : undefined,
      projectId: Number.isInteger(parsed.projectId) ? parsed.projectId : undefined,
    }
  } catch {
    return {}
  }
}

function writePreference(value: SelectionPreference): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(value))
  } catch {
    // Local storage is a preference only; tenant admission stays server-side.
  }
}

function isRequestCancellation(error: unknown): boolean {
  return Boolean(error && typeof error === 'object' && 'code' in error && (error as { code?: unknown }).code === 'ERR_CANCELED')
}

export const useWorkspaceStore = defineStore('workspace', () => {
  const workspaces = ref<Workspace[]>([])
  const projects = ref<Project[]>([])
  const selectedWorkspaceId = ref<number | null>(null)
  const selectedProjectId = ref<number | null>(null)
  const permissions = ref<string[]>([])
  const loading = ref(false)
  const projectsLoading = ref(false)
  const error = ref<unknown>(null)
  const generation = ref(0)
  let controller: AbortController | null = null
  let projectController: AbortController | null = null
  let projectGeneration = 0

  const selectedWorkspace = computed(() => workspaces.value.find(item => item.id === selectedWorkspaceId.value) || null)
  const selectedProject = computed(() => projects.value.find(item => item.id === selectedProjectId.value) || null)
  const hasTenant = computed(() => selectedWorkspaceId.value !== null && selectedProjectId.value !== null)

  function can(permission: string): boolean {
    return permissions.value.includes(permission)
  }

  function persistSelection(): void {
    writePreference({ workspaceId: selectedWorkspaceId.value ?? undefined, projectId: selectedProjectId.value ?? undefined })
  }

  function chooseWorkspace(preferredId?: number): Workspace | null {
    if (preferredId && workspaces.value.some(item => item.id === preferredId)) {
      return workspaces.value.find(item => item.id === preferredId) || null
    }
    return workspaces.value.find(item => item.type === 'personal' && item.status === 'active')
      || workspaces.value.find(item => item.status === 'active')
      || workspaces.value[0]
      || null
  }

  function setWorkspace(workspace: Workspace | null): void {
    selectedWorkspaceId.value = workspace?.id ?? null
    permissions.value = workspace?.permissions || []
    projects.value = []
    selectedProjectId.value = null
    persistSelection()
  }

  async function loadProjects(workspaceId = selectedWorkspaceId.value, preferredProjectId?: number): Promise<void> {
    const currentProjectGeneration = ++projectGeneration
    projectController?.abort()
    if (!workspaceId) { projectsLoading.value = false; return }
    const currentGeneration = generation.value
    projectController = new AbortController()
    projectsLoading.value = true
    try {
      const page = await workspaceAPI.listProjects(workspaceId, { signal: projectController.signal })
      if (currentGeneration !== generation.value || currentProjectGeneration !== projectGeneration || selectedWorkspaceId.value !== workspaceId) return
      projects.value = page.items || []
      const preferred = preferredProjectId && projects.value.some(item => item.id === preferredProjectId)
        ? preferredProjectId
        : projects.value.find(item => item.is_default && item.status === 'active')?.id
          || projects.value.find(item => item.status === 'active')?.id
          || projects.value[0]?.id
          || null
      selectedProjectId.value = preferred
      persistSelection()
    } catch (cause) {
      if (!isRequestCancellation(cause) && currentGeneration === generation.value && currentProjectGeneration === projectGeneration) {
        projects.value = []
        selectedProjectId.value = null
        error.value = cause
      }
    } finally {
      if (currentGeneration === generation.value && currentProjectGeneration === projectGeneration) projectsLoading.value = false
    }
  }

  async function loadWorkspaces(): Promise<void> {
    controller?.abort()
    const currentGeneration = ++generation.value
    controller = new AbortController()
    loading.value = true
    error.value = null
    try {
      const page = await workspaceAPI.listWorkspaces({ signal: controller.signal })
      if (currentGeneration !== generation.value) return
      workspaces.value = page.items || []
      const preference = readPreference()
      const workspace = chooseWorkspace(preference.workspaceId)
      setWorkspace(workspace)
      await loadProjects(workspace?.id ?? null, preference.projectId)
    } catch (cause) {
      if (!isRequestCancellation(cause) && currentGeneration === generation.value) {
        workspaces.value = []
        projects.value = []
        selectedWorkspaceId.value = null
        selectedProjectId.value = null
        permissions.value = []
        error.value = cause
      }
    } finally {
      if (currentGeneration === generation.value) loading.value = false
    }
  }

  async function selectWorkspace(workspaceId: number): Promise<void> {
    const workspace = workspaces.value.find(item => item.id === workspaceId)
    if (!workspace) {
      await loadWorkspaces()
      return
    }
    ++generation.value
    setWorkspace(workspace)
    await loadProjects(workspaceId)
  }

  async function selectProject(projectId: number): Promise<void> {
    if (!projects.value.some(item => item.id === projectId)) return
    selectedProjectId.value = projectId
    persistSelection()
  }

  async function refreshWorkspace(workspaceId = selectedWorkspaceId.value): Promise<void> {
    if (!workspaceId) return
    try {
      const workspace = await workspaceAPI.getWorkspace(workspaceId)
      const index = workspaces.value.findIndex(item => item.id === workspaceId)
      if (index >= 0) workspaces.value[index] = workspace
      if (workspaceId === selectedWorkspaceId.value) permissions.value = workspace.permissions || []
    } catch (cause) {
      error.value = cause
      if ((cause as { status?: number })?.status && [403, 404].includes((cause as { status: number }).status)) {
        workspaces.value = workspaces.value.filter(item => item.id !== workspaceId)
        if (workspaceId === selectedWorkspaceId.value) {
          ++generation.value
          const fallback = chooseWorkspace()
          setWorkspace(fallback)
          await loadProjects(fallback?.id ?? null)
        }
      }
    }
  }

  function reset(): void {
    ++generation.value
    ++projectGeneration
    controller?.abort()
    projectController?.abort()
    controller = null
    projectController = null
    workspaces.value = []
    projects.value = []
    selectedWorkspaceId.value = null
    selectedProjectId.value = null
    permissions.value = []
    loading.value = false
    projectsLoading.value = false
    error.value = null
    try { localStorage.removeItem(STORAGE_KEY) } catch { /* preference cleanup is best effort */ }
  }

  return {
    workspaces,
    projects,
    selectedWorkspaceId,
    selectedProjectId,
    selectedWorkspace,
    selectedProject,
    permissions,
    hasTenant,
    loading,
    projectsLoading,
    error,
    can,
    loadWorkspaces,
    loadProjects,
    selectWorkspace,
    selectProject,
    refreshWorkspace,
    reset,
  }
})
