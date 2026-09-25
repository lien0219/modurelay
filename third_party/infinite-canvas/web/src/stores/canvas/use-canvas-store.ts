import { create } from "zustand";
import { persist, type PersistStorage, type StorageValue } from "zustand/middleware";

import { nanoid } from "nanoid";
import i18n from "@/i18n";
import { localForageStorage } from "@/lib/localforage-storage";
import type { CanvasBackgroundMode } from "@/lib/canvas-theme";
import type { CanvasAssistantSession, CanvasConnection, CanvasNodeData, ViewportTransform } from "@/types/canvas";

export type CanvasProject = {
    id: string;
    title: string;
    createdAt: string;
    updatedAt: string;
    nodes: CanvasNodeData[];
    connections: CanvasConnection[];
    chatSessions: CanvasAssistantSession[];
    activeChatId: string | null;
    backgroundMode: CanvasBackgroundMode;
    showImageInfo: boolean;
    viewport: ViewportTransform;
};

export type CanvasDeletedProject = {
    id: string;
    deletedAt: string;
};

type CanvasStore = {
    hydrated: boolean;
    projects: CanvasProject[];
    deletedProjects: CanvasDeletedProject[];
    createProject: (title?: string) => string;
    importProject: (project: Partial<CanvasProject>) => string;
    openProject: (id: string) => CanvasProject | null;
    renameProject: (id: string, title: string) => void;
    deleteProjects: (ids: string[]) => void;
    replaceProjects: (projects: CanvasProject[], deletedProjects?: CanvasDeletedProject[]) => void;
    updateProject: (id: string, patch: Partial<Pick<CanvasProject, "nodes" | "connections" | "chatSessions" | "activeChatId" | "backgroundMode" | "showImageInfo" | "viewport">>) => void;
};

const initialViewport: ViewportTransform = { x: 0, y: 0, k: 1 };
const CANVAS_STORE_KEY = "infinite-canvas:canvas_store";
const CANVAS_STORE_JOURNAL_KEY = `${CANVAS_STORE_KEY}:pending`;
type PersistedCanvasState = Pick<CanvasStore, "projects" | "deletedProjects">;
let saveTimer: ReturnType<typeof setTimeout> | null = null;
let queuedPersistState: PersistedCanvasState | null = null;
let pendingPersistWrite: { name: string; value: string } | null = null;

async function flushCanvasPersist() {
    if (saveTimer) {
        clearTimeout(saveTimer);
        saveTimer = null;
    }
    const pending = pendingPersistWrite;
    if (!pending) return;
    pendingPersistWrite = null;
    await localForageStorage.setItem(pending.name, pending.value);
    clearCanvasPersistJournal(pending);
}

function persistCanvasJournal() {
    if (typeof window === "undefined" || !pendingPersistWrite) return;
    try {
        window.localStorage.setItem(CANVAS_STORE_JOURNAL_KEY, JSON.stringify(pendingPersistWrite));
    } catch {
        // Best-effort crash/reload journal only; IndexedDB remains the primary store.
    }
}

function readCanvasPersistJournal(name: string) {
    if (typeof window === "undefined") return null;
    try {
        const raw = window.localStorage.getItem(CANVAS_STORE_JOURNAL_KEY);
        if (!raw) return null;
        const journal = JSON.parse(raw) as { name?: string; value?: string };
        return journal.name === name && typeof journal.value === "string" ? { name, value: journal.value } : null;
    } catch {
        return null;
    }
}

function clearCanvasPersistJournal(expected: { name: string; value: string }) {
    if (typeof window === "undefined") return;
    try {
        const current = readCanvasPersistJournal(expected.name);
        if (current?.value === expected.value) window.localStorage.removeItem(CANVAS_STORE_JOURNAL_KEY);
    } catch {
        // Ignore journal cleanup failures.
    }
}

if (typeof window !== "undefined") {
    const flush = () => {
        persistCanvasJournal();
        void flushCanvasPersist();
    };
    window.addEventListener("pagehide", flush);
    document.addEventListener("visibilitychange", () => {
        if (document.visibilityState === "hidden") flush();
    });
}

const canvasStorage: PersistStorage<CanvasStore> = {
    getItem: async (name) => {
        const journal = readCanvasPersistJournal(name);
        const value = journal?.value || (await localForageStorage.getItem(name));
        if (!value) return null;
        const parsed = JSON.parse(value) as StorageValue<CanvasStore>;
        queuedPersistState = parsed.state as PersistedCanvasState;
        if (journal) {
            await localForageStorage.setItem(name, journal.value);
            clearCanvasPersistJournal(journal);
        }
        return parsed;
    },
    setItem: (name, value) => {
        const nextState = value.state as PersistedCanvasState;
        if (queuedPersistState && queuedPersistState.projects === nextState.projects && queuedPersistState.deletedProjects === nextState.deletedProjects) return;
        queuedPersistState = nextState;
        pendingPersistWrite = { name, value: JSON.stringify(value) };
        if (saveTimer) clearTimeout(saveTimer);
        saveTimer = setTimeout(() => {
            void flushCanvasPersist();
        }, 400);
    },
    removeItem: async (name) => {
        if (pendingPersistWrite?.name === name) pendingPersistWrite = null;
        if (typeof window !== "undefined") {
            const journal = readCanvasPersistJournal(name);
            if (journal) window.localStorage.removeItem(CANVAS_STORE_JOURNAL_KEY);
        }
        await localForageStorage.removeItem(name);
    },
};

export const useCanvasStore = create<CanvasStore>()(
    persist(
        (set, get) => ({
            hydrated: false,
            projects: [],
            deletedProjects: [],
            createProject: (title = i18n.t("canvas.project.untitled")) => {
                const now = new Date().toISOString();
                const id = nanoid();
                const project: CanvasProject = {
                    id,
                    title,
                    createdAt: now,
                    updatedAt: now,
                    nodes: [],
                    connections: [],
                    chatSessions: [],
                    activeChatId: null,
                    backgroundMode: "lines",
                    showImageInfo: false,
                    viewport: initialViewport,
                };
                set((state) => ({ projects: [project, ...state.projects] }));
                return id;
            },
            importProject: (source) => {
                const now = new Date().toISOString();
                const project: CanvasProject = {
                    id: nanoid(),
                    title: source.title || i18n.t("canvas.project.imported"),
                    createdAt: source.createdAt || now,
                    updatedAt: now,
                    nodes: source.nodes || [],
                    connections: source.connections || [],
                    chatSessions: source.chatSessions || [],
                    activeChatId: source.activeChatId || null,
                    backgroundMode: source.backgroundMode || "lines",
                    showImageInfo: source.showImageInfo || false,
                    viewport: source.viewport || initialViewport,
                };
                set((state) => ({ projects: [project, ...state.projects] }));
                return project.id;
            },
            openProject: (id) => {
                return get().projects.find((item) => item.id === id) || null;
            },
            renameProject: (id, title) =>
                set((state) => ({
                    projects: state.projects.map((project) => (project.id === id ? { ...project, title: title.trim() || project.title, updatedAt: new Date().toISOString() } : project)),
                })),
            deleteProjects: (ids) =>
                set((state) => {
                    const now = new Date().toISOString();
                    const removing = new Set(ids);
                    const projects = state.projects.filter((project) => !removing.has(project.id));
                    const deletedProjects = [...state.deletedProjects.filter((item) => !removing.has(item.id)), ...ids.map((id) => ({ id, deletedAt: now }))];
                    return { projects, deletedProjects };
                }),
            replaceProjects: (projects, deletedProjects = []) => set({ projects, deletedProjects }),
            updateProject: (id, patch) =>
                set((state) => ({
                    projects: state.projects.map((project) => (project.id === id ? { ...project, ...patch, updatedAt: new Date().toISOString() } : project)),
                })),
        }),
        {
            name: CANVAS_STORE_KEY,
            storage: canvasStorage,
            partialize: (state) =>
                ({
                    projects: state.projects,
                    deletedProjects: state.deletedProjects,
                }) as StorageValue<CanvasStore>["state"],
            onRehydrateStorage: () => () => {
                useCanvasStore.setState({ hydrated: true });
            },
        },
    ),
);
