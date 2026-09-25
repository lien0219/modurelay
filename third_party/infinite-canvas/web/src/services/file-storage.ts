import localforage from "localforage";
import { nanoid } from "nanoid";

import { providerFetch } from "@/services/api/provider-transport";

export type UploadedFile = { url: string; storageKey: string; bytes: number; mimeType: string; width?: number; height?: number; durationMs?: number };
type MediaReadOptions = { signal?: AbortSignal };

const MEDIA_METADATA_TIMEOUT_MS = 10_000;
const store = localforage.createInstance({ name: "infinite-canvas", storeName: "media_files" });
const videoLogStore = localforage.createInstance({ name: "infinite-canvas", storeName: "video_generation_logs" });
const objectUrls = new Map<string, string>();

export async function uploadMediaFile(input: string | Blob, prefix = "file", options?: MediaReadOptions): Promise<UploadedFile> {
    throwIfAborted(options?.signal);
    let blob: Blob;
    if (typeof input === "string") {
        const response = await providerFetch(input, { signal: options?.signal });
        if (!response.ok) throw new Error(`Media download failed (${response.status})`);
        blob = await response.blob();
    } else {
        blob = input;
    }
    throwIfAborted(options?.signal);
    validateStoredMediaBlob(blob, prefix);
    const storageKey = `${prefix}:${nanoid()}`;
    await store.setItem(storageKey, blob);
    throwIfAborted(options?.signal);
    const url = URL.createObjectURL(blob);
    objectUrls.set(storageKey, url);
    const meta = blob.type.startsWith("video/") ? await readVideoMeta(url) : blob.type.startsWith("audio/") ? await readAudioMeta(url) : {};
    return { url, storageKey, bytes: blob.size, mimeType: blob.type || "application/octet-stream", ...meta };
}

export async function resolveMediaUrl(storageKey?: string, fallback = "") {
    if (!storageKey) return fallback;
    const cached = objectUrls.get(storageKey);
    if (cached) return cached;
    const blob = await store.getItem<Blob>(storageKey);
    if (!blob) return fallback;
    const url = URL.createObjectURL(blob);
    objectUrls.set(storageKey, url);
    return url;
}

export async function getMediaBlob(storageKey: string) {
    return store.getItem<Blob>(storageKey);
}

export async function setMediaBlob(storageKey: string, blob: Blob) {
    await store.setItem(storageKey, blob);
    const previousUrl = objectUrls.get(storageKey);
    const url = URL.createObjectURL(blob);
    objectUrls.set(storageKey, url);
    if (previousUrl && previousUrl !== url) URL.revokeObjectURL(previousUrl);
    return url;
}

export async function deleteStoredMedia(keys: Iterable<string>) {
    await Promise.all(
        Array.from(new Set(keys)).map(async (key) => {
            const url = objectUrls.get(key);
            if (url) URL.revokeObjectURL(url);
            objectUrls.delete(key);
            await store.removeItem(key);
        }),
    );
}

export async function cleanupUnusedMedia(usedData: unknown) {
    const usedKeys = collectMediaStorageKeys(usedData);
    await videoLogStore.iterate((value) => {
        collectMediaStorageKeys(value, usedKeys);
    });
    const unused: string[] = [];
    await store.iterate((_value, key) => {
        if (!usedKeys.has(key)) unused.push(key);
    });
    await deleteStoredMedia(unused);
}

export function collectMediaStorageKeys(value: unknown, keys = new Set<string>()) {
    if (!value || typeof value !== "object") return keys;
    if ("storageKey" in value && typeof value.storageKey === "string" && value.storageKey.includes(":")) keys.add(value.storageKey);
    Object.values(value).forEach((item) => (Array.isArray(item) ? item.forEach((child) => collectMediaStorageKeys(child, keys)) : collectMediaStorageKeys(item, keys)));
    return keys;
}

function validateStoredMediaBlob(blob: Blob, prefix: string) {
    if (!blob.size) throw new Error("Media response is empty");
    const mimeType = blob.type.toLowerCase();
    if (!mimeType || mimeType.includes("octet-stream")) return;
    if (prefix.startsWith("video") && !mimeType.startsWith("video/")) throw new Error("Video response is not playable media");
    if (prefix.startsWith("audio") && !mimeType.startsWith("audio/")) throw new Error("Audio response is not playable media");
}

function readVideoMeta(url: string) {
    return new Promise<{ width: number; height: number; durationMs?: number }>((resolve) => {
        const video = document.createElement("video");
        video.preload = "metadata";
        let settled = false;
        const done = () => {
            if (settled) return;
            settled = true;
            window.clearTimeout(timer);
            const result = { width: video.videoWidth || 1280, height: video.videoHeight || 720, durationMs: Number.isFinite(video.duration) ? Math.round(video.duration * 1000) : undefined };
            video.onloadedmetadata = null;
            video.onerror = null;
            video.removeAttribute("src");
            video.load();
            resolve(result);
        };
        const timer = window.setTimeout(done, MEDIA_METADATA_TIMEOUT_MS);
        video.onloadedmetadata = done;
        video.onerror = done;
        video.src = url;
    });
}

function readAudioMeta(url: string) {
    return new Promise<{ durationMs?: number }>((resolve) => {
        const audio = document.createElement("audio");
        audio.preload = "metadata";
        let settled = false;
        const done = () => {
            if (settled) return;
            settled = true;
            window.clearTimeout(timer);
            const result = { durationMs: Number.isFinite(audio.duration) ? Math.round(audio.duration * 1000) : undefined };
            audio.onloadedmetadata = null;
            audio.onerror = null;
            audio.removeAttribute("src");
            audio.load();
            resolve(result);
        };
        const timer = window.setTimeout(done, MEDIA_METADATA_TIMEOUT_MS);
        audio.onloadedmetadata = done;
        audio.onerror = done;
        audio.src = url;
    });
}

function throwIfAborted(signal?: AbortSignal) {
    if (!signal?.aborted) return;
    const error = signal.reason instanceof Error ? signal.reason : new Error("Request canceled");
    if (error.name === "Error") error.name = "AbortError";
    throw error;
}
