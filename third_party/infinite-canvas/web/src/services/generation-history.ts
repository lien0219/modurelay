import localforage from "localforage";

import { cleanupUnusedMedia } from "@/services/file-storage";
import { cleanupUnusedImages } from "@/services/image-storage";

export const IMAGE_GENERATION_LOG_LIMIT = 300;
export const VIDEO_GENERATION_LOG_LIMIT = 300;

type StoredGenerationLog = {
    id?: string;
    createdAt?: number;
    status?: string;
};

type LogStore = ReturnType<typeof localforage.createInstance>;

const imageLogStore = localforage.createInstance({ name: "infinite-canvas", storeName: "image_generation_logs" });
const videoLogStore = localforage.createInstance({ name: "infinite-canvas", storeName: "video_generation_logs" });

export async function pruneImageGenerationHistory() {
    return pruneLogStore(imageLogStore, IMAGE_GENERATION_LOG_LIMIT, () => true);
}

export async function pruneVideoGenerationHistory() {
    return pruneLogStore(videoLogStore, VIDEO_GENERATION_LOG_LIMIT, (log) => log.status !== "pending");
}

export async function compactGenerationStorage() {
    const [imageLogsRemoved, videoLogsRemoved] = await Promise.all([pruneImageGenerationHistory(), pruneVideoGenerationHistory()]);
    const [{ useCanvasStore }, { useAssetStore }] = await Promise.all([import("@/stores/canvas/use-canvas-store"), import("@/stores/use-asset-store")]);
    const usedData = {
        projects: useCanvasStore.getState().projects,
        assets: useAssetStore.getState().assets,
    };
    await Promise.all([cleanupUnusedImages(usedData), cleanupUnusedMedia(usedData)]);
    return { imageLogsRemoved, videoLogsRemoved, totalRemoved: imageLogsRemoved + videoLogsRemoved };
}

async function pruneLogStore(store: LogStore, limit: number, eligible: (log: StoredGenerationLog) => boolean) {
    const records: Array<{ key: string; createdAt: number; log: StoredGenerationLog }> = [];
    await store.iterate<StoredGenerationLog, void>((value, key) => {
        records.push({ key, createdAt: Number(value?.createdAt) || 0, log: value || {} });
    });
    const removable = records.filter((item) => eligible(item.log)).sort((a, b) => b.createdAt - a.createdAt).slice(limit);
    await Promise.all(removable.map((item) => store.removeItem(item.key)));
    return removable.length;
}
