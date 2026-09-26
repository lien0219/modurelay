import axios from "axios";
import { nanoid } from "nanoid";

import i18n from "@/i18n";
import { dataUrlToFile, readFileAsDataUrl } from "@/lib/image-utils";
import { clampVideoSeconds, computeVideoSize, inferVideoRatio, parseVideoResolution } from "@/lib/media-size";
import { getMediaBlob, resolveMediaUrl, uploadMediaFile, type UploadedFile } from "@/services/file-storage";
import { imageToDataUrl } from "@/services/image-storage";
import { boolConfig, buildApiUrl, modelOptionName, resolveModelRequestConfig, resolveModelRequestProfile, resolveModelScript, withLocalProxy, type AiConfig, type ModelRequestProfile } from "@/stores/use-config-store";
import { runModelPlugin } from "./model-plugin";
import { providerAxios, providerFetch } from "./provider-transport";
import { videoTransportKind } from "./media-adapters";
import type { ReferenceImage } from "@/types/image";
import type { ReferenceAudio, ReferenceVideo } from "@/types/media";

type VideoResponse = {
    id?: string | number;
    request_id?: string | number;
    task_id?: string | number;
    status?: string;
    state?: string;
    task_status?: string;
    error?: { message?: string } | string;
    message?: string;
    url?: string;
    result_url?: string;
    video_url?: string;
    content?: { video_url?: string; url?: string } | null;
    data?: VideoResponse | null;
    result?: VideoResponse | null;
    output?: VideoResponse | null;
};
type ApiVideoResponse = VideoResponse | { code?: number | string; data?: VideoResponse | null; msg?: string; message?: string; error?: { message?: string } };
type ApiEnvelope<T> = T | { code?: number | string; data?: T | null; msg?: string; message?: string; error?: { message?: string } };
type RequestOptions = { signal?: AbortSignal; deadlineAt?: number };
type VideoMediaOptions = RequestOptions & { videos?: ReferenceVideo[]; audios?: ReferenceAudio[] };
const apiText = (key: string, options?: Record<string, unknown>) => i18n.t(`apiErrors.${key}`, options);

export const VIDEO_TASK_POLL_INTERVAL_MS = 2500;
export const VIDEO_TASK_POLL_TIMEOUT_MS = 60 * 60 * 1000;

export type VideoGenerationResult = { blob?: Blob; url?: string; mimeType?: string };
export type VideoGenerationTask = { id: string; provider: "openai" | "gemini" | "plugin"; model: string };
type GeminiInlineData = { bytesBase64Encoded: string; mimeType: string };
type GeminiVideoOperation = {
    name?: string;
    done?: boolean;
    error?: { message?: string };
    response?: { generateVideoResponse?: { generatedSamples?: Array<{ video?: { uri?: string } }> } };
};
export type VideoGenerationTaskState = { status: "pending" } | { status: "completed"; result: VideoGenerationResult } | { status: "failed"; error: string };

/** Results for scripted (plugin) video models, which run their own create+poll in one shot at task creation. */
const pluginVideoResults = new Map<string, VideoGenerationResult>();

function aiApiUrl(config: AiConfig, path: string) {
    return buildApiUrl(config.baseUrl, path);
}

function aiHeaders(config: AiConfig, contentType?: string) {
    return {
        Authorization: `Bearer ${config.apiKey}`,
        ...(contentType ? { "Content-Type": contentType } : {}),
    };
}

export async function requestVideoGeneration(config: AiConfig, prompt: string, references: ReferenceImage[] = [], options?: VideoMediaOptions): Promise<VideoGenerationResult> {
    return waitForVideoGenerationTask(config, await createVideoGenerationTask(config, prompt, references, options), options);
}

export async function waitForVideoGenerationTask(config: AiConfig, task: VideoGenerationTask, options?: RequestOptions): Promise<VideoGenerationResult> {
    const deadline = options?.deadlineAt || Date.now() + VIDEO_TASK_POLL_TIMEOUT_MS;
    for (;;) {
        if (options?.signal?.aborted) throw abortVideoRequest();
        const state = await pollVideoGenerationTask(config, task, options);
        if (state.status === "completed") return state.result;
        if (state.status === "failed") throw videoTaskFailed(state.error);
        if (Date.now() >= deadline) throw videoTaskTimeout();
        const pollInterval = task.id.startsWith("aistarslab:") ? 10_000 : VIDEO_TASK_POLL_INTERVAL_MS;
        await delay(pollInterval, options?.signal);
    }
}

export function isVideoTaskFailed(error: unknown) {
    return error instanceof Error && error.name === "VideoTaskFailed";
}

export function isVideoTaskTimeout(error: unknown) {
    return error instanceof Error && error.name === "VideoTaskTimeout";
}

function videoTaskFailed(message: string) {
    const error = new Error(message);
    error.name = "VideoTaskFailed";
    return error;
}

function videoTaskTimeout() {
    const error = new Error(apiText("videoTimeout", { provider: "" }));
    error.name = "VideoTaskTimeout";
    return error;
}

export async function createVideoGenerationTask(config: AiConfig, prompt: string, references: ReferenceImage[] = [], options?: VideoMediaOptions): Promise<VideoGenerationTask> {
    const selectedModel = (config.model || config.videoModel).trim();
    const requestConfig = resolveModelRequestConfig(config, selectedModel);
    const script = resolveModelScript(config, selectedModel);
    if (script) return createPluginVideoTask(requestConfig, selectedModel, script, prompt, references, options);
    assertVideoConfig(requestConfig, requestConfig.model);
    if (requestConfig.apiFormat === "gemini") return createGeminiVideoTask(requestConfig, selectedModel, prompt, references, options);
    return createOpenAIVideoTask(requestConfig, selectedModel, prompt, references, options);
}

export async function pollVideoGenerationTask(config: AiConfig, task: VideoGenerationTask, options?: RequestOptions): Promise<VideoGenerationTaskState> {
    if (task.provider === "plugin") {
        const result = pluginVideoResults.get(task.id);
        if (result) pluginVideoResults.delete(task.id);
        return result ? { status: "completed", result } : { status: "failed", error: apiText("pluginVideoExpired") };
    }
    const requestConfig = resolveModelRequestConfig(config, task.model);
    assertVideoConfig(requestConfig, requestConfig.model);
    if (task.provider === "gemini") return pollGeminiVideoTask(requestConfig, task, options);
    return pollOpenAIVideoTask(requestConfig, task, options);
}

async function createPluginVideoTask(config: AiConfig, model: string, script: string, prompt: string, references: ReferenceImage[], options?: VideoMediaOptions): Promise<VideoGenerationTask> {
    if (!config.baseUrl.trim()) throw new Error(apiText("baseUrlRequired"));
    if (!config.apiKey.trim()) throw new Error(apiText("apiKeyRequired"));
    const refs = await Promise.all(references.map((image) => imageToDataUrl(image)));
    const videos = await Promise.all((options?.videos || []).map((video) => referenceMediaToFile(video, "ref.mp4", "invalidReferenceVideo", options)));
    const audios = await Promise.all((options?.audios || []).map((audio) => referenceMediaToFile(audio, "ref.mp3", "invalidReferenceAudio", options)));
    const result = videoPluginResult(
        await runModelPlugin({
            capability: "video",
            script,
            config,
            prompt,
            images: refs,
            videos,
            audios,
            params: {
                seconds: normalizeVideoSeconds(config.videoSeconds),
                size: normalizeVideoSize(config.size, config.vquality),
                resolution: normalizeVideoResolution(config.vquality),
                ratio: videoAspectRatio(config.size),
                generateAudio: boolConfig(config.videoGenerateAudio, true),
                watermark: boolConfig(config.videoWatermark, false),
                mode: resolveVideoMode(config.videoMode, refs.length),
            },
            signal: options?.signal,
        }),
    );
    const id = nanoid();
    pluginVideoResults.set(id, result);
    return { id, provider: "plugin", model };
}

function videoPluginResult(result: unknown): VideoGenerationResult {
    if (result instanceof Blob) return { blob: result };
    if (typeof result === "string") return { url: result, mimeType: "video/mp4" };
    if (result && typeof result === "object") {
        const record = result as Record<string, unknown>;
        if (record.blob instanceof Blob) return { blob: record.blob };
        const url = [record.url, record.video_url, record.result_url].find((value) => typeof value === "string" && value) as string | undefined;
        if (url) return { url, mimeType: "video/mp4" };
    }
    throw new Error(apiText("scriptNoVideo"));
}

export async function storeGeneratedVideo(result: VideoGenerationResult, options?: RequestOptions): Promise<UploadedFile> {
    if (result.blob) return uploadMediaFile(result.blob, "video", options);
    if (result.url) {
        try {
            return await uploadMediaFile(result.url, "video", options);
        } catch (error) {
            if (options?.signal?.aborted || axios.isCancel(error) || (error instanceof Error && error.name === "AbortError")) throw error;
            // data:/blob: values are local ephemeral payloads. If local persistence
            // fails, returning the raw value would create a false "success" that
            // cannot reliably survive refresh, so surface the real failure.
            if (/^(data:|blob:)/i.test(result.url)) throw error;
            return { url: result.url, storageKey: "", bytes: 0, mimeType: result.mimeType || "video/mp4" };
        }
    }
    throw new Error(apiText("noPlayableVideo"));
}

async function createOpenAIVideoTask(config: AiConfig, model: string, prompt: string, references: ReferenceImage[], options?: VideoMediaOptions): Promise<VideoGenerationTask> {
    const upstreamModel = modelOptionName(model);
    const mode = resolveVideoMode(config.videoMode, references.length);
    const requestProfile = resolveModelRequestProfile(config, model);
    const transport = resolveVideoTransport(requestProfile, upstreamModel);

    try {
        let response: ApiVideoResponse;
        if (transport === "xai-json") {
            response = await postVideoJSON(
                config,
                await buildXaiVideoPayload(config, upstreamModel, prompt, references, mode),
                options,
            );
        } else if (transport === "compatible-json") {
            try {
                const payload = shouldUseAistarsLabVideoContract(requestProfile, config, upstreamModel)
                    ? await buildAistarsLabVideoPayload(config, upstreamModel, prompt, references, mode, options)
                    : await buildCompatibleVideoPayload(config, upstreamModel, prompt, references, mode, options);
                response = await postVideoJSON(config, payload, options);
            } catch (error) {
                if (!isVideoTransportNegotiationError(error)) throw error;
                response = await postVideoMultipart(config, upstreamModel, prompt, references, mode, options);
            }
        } else {
            try {
                response = await postVideoMultipart(config, upstreamModel, prompt, references, mode, options);
            } catch (error) {
                if (!isVideoTransportNegotiationError(error)) throw error;
                response = await postVideoJSON(
                    config,
                    await buildCompatibleVideoPayload(config, upstreamModel, prompt, references, mode, options),
                    options,
                );
            }
        }

        const created = unwrapVideoResponse(response);
        const taskId = videoTaskId(created);
        if (!taskId) {
            const directUrl = videoResultUrl(created);
            if (directUrl) {
                const id = nanoid();
                pluginVideoResults.set(id, await videoResultFromUrl(config, directUrl, options));
                return { id, provider: "plugin", model };
            }
            throw new Error(apiText("noVideoTaskId"));
        }
        return { id: taskId, provider: "openai", model };
    } catch (error) {
        throw videoRequestError(error, apiText("videoTaskCreateFailed"));
    }
}

async function postVideoJSON(config: AiConfig, body: Record<string, unknown>, options?: RequestOptions) {
    return (
        await providerAxios.post<ApiVideoResponse>(
            aiApiUrl(config, "/videos"),
            body,
            { headers: aiHeaders(config, "application/json"), signal: options?.signal },
        )
    ).data;
}

async function postVideoMultipart(
    config: AiConfig,
    model: string,
    prompt: string,
    references: ReferenceImage[],
    mode: "frames" | "reference",
    options?: VideoMediaOptions,
) {
    const images = await Promise.all(references.map(async (image) => dataUrlToFile({ ...image, dataUrl: await imageToDataUrl(image) })));
    const videos = await Promise.all((options?.videos || []).map((video) => referenceMediaToFile(video, "ref.mp4", "invalidReferenceVideo", options)));
    const audios = await Promise.all((options?.audios || []).map((audio) => referenceMediaToFile(audio, "ref.mp3", "invalidReferenceAudio", options)));
    const body = new FormData();
    body.append("model", model);
    body.append("prompt", prompt);
    body.append("seconds", normalizeVideoSeconds(config.videoSeconds));
    body.append("size", normalizeVideoSize(config.size, config.vquality) || "1280x720");
    body.append("resolution_name", normalizeVideoResolution(config.vquality));
    body.append("generate_audio", String(boolConfig(config.videoGenerateAudio, true)));
    body.append("watermark", String(boolConfig(config.videoWatermark, false)));
    body.append("mode", mode);
    if (mode === "frames") {
        if (images[0]) body.append("first_frame", images[0], "first.png");
        if (images[1]) body.append("last_frame", images[1], "last.png");
    } else {
        images.forEach((file) => body.append("image[]", file, "ref.png"));
    }
    videos.forEach((file) => body.append("video[]", file));
    audios.forEach((file) => body.append("audio[]", file));
    return (await providerAxios.post<ApiVideoResponse>(aiApiUrl(config, "/videos"), body, { headers: aiHeaders(config), signal: options?.signal })).data;
}

function resolveVideoTransport(profile: ModelRequestProfile, model: string) {
    switch (profile) {
        case "xai-json":
            return "xai-json" as const;
        case "compatible-json":
        case "aistars-json":
            return "compatible-json" as const;
        case "openai-multipart":
            return "multipart" as const;
        default:
            return videoTransportKind(model);
    }
}

function shouldUseAistarsLabVideoContract(profile: ModelRequestProfile, config: AiConfig, model: string) {
    if (profile === "aistars-json") return true;
    if (profile !== "auto") return false;
    const baseUrl = config.baseUrl.trim().toLowerCase();
    return /(^|\.)aistarslab\.com(?:\/|$)/.test(baseUrl.replace(/^https?:\/\//, ""));
}

async function buildAistarsLabVideoPayload(
    config: AiConfig,
    model: string,
    prompt: string,
    references: ReferenceImage[],
    mode: "frames" | "reference",
    options?: VideoMediaOptions,
): Promise<Record<string, unknown>> {
    const images = await Promise.all(references.map((image) => imageToDataUrl(image)));
    const videos: string[] = [];
    const audios: string[] = [];

    for (const video of options?.videos || []) {
        const file = await referenceMediaToFile(video, "ref.mp4", "invalidReferenceVideo", options);
        videos.push(await readFileAsDataUrl(file));
    }
    for (const audio of options?.audios || []) {
        const file = await referenceMediaToFile(audio, "ref.mp3", "invalidReferenceAudio", options);
        audios.push(await readFileAsDataUrl(file));
    }

    const metadata: Record<string, unknown> = {
        resolution: normalizeVideoResolution(config.vquality),
    };

    if (images.length > 0) metadata.images = images;
    if (videos.length > 0) metadata.videos = videos;
    if (audios.length > 0) metadata.audios = audios;

    // AistarsLab's OpenAI-compatible endpoint distinguishes task mode inside metadata.
    // frames2video requires exactly two images; one image must use image2video.
    if (mode === "frames" && images.length === 2) {
        metadata.mode_type = "frames2video";
    } else if (images.length > 0) {
        metadata.mode_type = "image2video";
    } else if (videos.length === 0 && audios.length === 0) {
        metadata.mode_type = "text2video";
    }

    return {
        model,
        prompt,
        seconds: normalizeVideoSeconds(config.videoSeconds),
        size: videoAspectRatio(config.size),
        n: 1,
        metadata,
    };
}

async function buildCompatibleVideoPayload(
    config: AiConfig,
    model: string,
    prompt: string,
    references: ReferenceImage[],
    mode: "frames" | "reference",
    options?: VideoMediaOptions,
): Promise<Record<string, unknown>> {
    const imageDataUrls = await Promise.all(references.map((image) => imageToDataUrl(image)));
    const media: Array<{ type: "first_frame" | "last_frame" | "reference_image" | "reference_video" | "reference_audio"; url: string }> = [];

    if (mode === "frames") {
        if (imageDataUrls[0]) media.push({ type: "first_frame", url: imageDataUrls[0] });
        if (imageDataUrls[1]) media.push({ type: "last_frame", url: imageDataUrls[1] });
    } else {
        imageDataUrls.forEach((url) => media.push({ type: "reference_image", url }));
    }

    for (const video of options?.videos || []) {
        const file = await referenceMediaToFile(video, "ref.mp4", "invalidReferenceVideo", options);
        media.push({ type: "reference_video", url: await readFileAsDataUrl(file) });
    }
    for (const audio of options?.audios || []) {
        const file = await referenceMediaToFile(audio, "ref.mp3", "invalidReferenceAudio", options);
        media.push({ type: "reference_audio", url: await readFileAsDataUrl(file) });
    }

    return {
        model,
        prompt,
        duration: Number(normalizeVideoSeconds(config.videoSeconds)) || 5,
        resolution: normalizeVideoResolution(config.vquality),
        aspect_ratio: videoAspectRatio(config.size),
        audio: boolConfig(config.videoGenerateAudio, true),
        generate_audio: boolConfig(config.videoGenerateAudio, true),
        watermark: boolConfig(config.videoWatermark, false),
        ...(media.length ? { media } : {}),
    };
}

async function buildXaiVideoPayload(
    config: AiConfig,
    model: string,
    prompt: string,
    references: ReferenceImage[],
    mode: "frames" | "reference",
): Promise<Record<string, unknown>> {
    const images = await Promise.all(references.map((image) => imageToDataUrl(image)));
    const payload: Record<string, unknown> = {
        model,
        prompt,
        duration: Number(normalizeVideoSeconds(config.videoSeconds)) || 5,
        resolution: normalizeVideoResolution(config.vquality),
        aspect_ratio: videoAspectRatio(config.size),
        generate_audio: boolConfig(config.videoGenerateAudio, true),
    };
    if (mode === "frames") {
        if (images[0]) payload.image = { url: images[0] };
        if (images[1]) payload.last_frame = { url: images[1] };
    } else if (images.length) {
        payload.reference_images = images.slice(0, 7).map((url) => ({ url }));
    }
    return payload;
}

function isVideoTransportNegotiationError(error: unknown) {
    if (!axios.isAxiosError(error)) return false;
    const status = error.response?.status || 0;
    if (status === 415) return true;
    if (status !== 400 && status !== 422) return false;
    const message = readApiErrorMessage(error.response?.data).toLowerCase();
    return [
        "content-type",
        "content type",
        "multipart",
        "form-data",
        "application/json",
        "json body",
        "request body",
        "body format",
        "media type",
    ].some((hint) => message.includes(hint));
}

async function pollOpenAIVideoTask(config: AiConfig, task: VideoGenerationTask, options?: RequestOptions): Promise<VideoGenerationTaskState> {
    try {
        const video = unwrapVideoResponse((await providerAxios.get<ApiVideoResponse>(aiApiUrl(config, `/videos/${task.id}`), { headers: aiHeaders(config), signal: options?.signal })).data);
        const url = videoResultUrl(video);
        if (url) return { status: "completed", result: await videoResultFromUrl(config, url, options) };

        const status = videoStatus(video);
        if (status === "completed" || status === "done" || status === "succeeded" || status === "success" || status === "finished") {
            try {
                const content = await providerAxios.get<Blob>(aiApiUrl(config, `/videos/${task.id}/content`), { headers: aiHeaders(config), responseType: "blob", signal: options?.signal });
                await assertVideoBlob(content.data);
                return { status: "completed", result: { blob: content.data } };
            } catch (error) {
                // Some compatible providers report a terminal task state before
                // the generated media is replicated to the content endpoint.
                // Keep polling instead of turning that short propagation window
                // into a false generation failure.
                if (isTransientVideoContentError(error)) return { status: "pending" };
                throw error;
            }
        }
        if (status === "failed" || status === "cancelled" || status === "canceled" || status === "expired" || status === "error" || status === "rejected") {
            return { status: "failed", error: readApiErrorMessage(video) || apiText("videoGenerationFailed") };
        }
        return { status: "pending" };
    } catch (error) {
        throw videoRequestError(error, apiText("videoTaskQueryFailed"));
    }
}

function isTransientVideoContentError(error: unknown) {
    if (!axios.isAxiosError(error)) return false;
    return [404, 409, 425, 429, 500, 502, 503, 504].includes(error.response?.status || 0);
}

async function videoResultFromUrl(config: AiConfig, url: string, options?: RequestOptions): Promise<VideoGenerationResult> {
    const resolvedUrl = resolveVideoResultUrl(config, url);
    if (/^(data:|blob:)/i.test(resolvedUrl)) {
        const response = await providerFetch(resolvedUrl, { signal: options?.signal });
        if (!response.ok) throw new Error(apiText("videoDownloadFailed"));
        const blob = await response.blob();
        await assertVideoBlob(blob);
        return { blob };
    }
    try {
        const response = await providerAxios.get<Blob>(withLocalProxy(resolvedUrl), { responseType: "blob", signal: options?.signal });
        await assertVideoBlob(response.data);
        return { blob: response.data };
    } catch (error) {
        if (axios.isCancel(error) || options?.signal?.aborted) throw error;
        return { url: resolvedUrl, mimeType: "video/mp4" };
    }
}

async function createGeminiVideoTask(config: AiConfig, model: string, prompt: string, references: ReferenceImage[], options?: VideoMediaOptions): Promise<VideoGenerationTask> {
    const images = await Promise.all(references.map((image) => imageToDataUrl(image)));
    const videos = await Promise.all((options?.videos || []).map((video) => referenceMediaToFile(video, "ref.mp4", "invalidReferenceVideo", options)));
    const audios = await Promise.all((options?.audios || []).map((audio) => referenceMediaToFile(audio, "ref.mp3", "invalidReferenceAudio", options)));
    const mode = resolveVideoMode(config.videoMode, images.length);
    const instance: Record<string, unknown> = { prompt };
    if (mode === "frames") {
        if (images[0]) instance.image = parseDataUrlInline(images[0]);
        if (images[1]) instance.lastFrame = parseDataUrlInline(images[1]);
    } else {
        instance.referenceImages = images.map((dataUrl) => ({ image: parseDataUrlInline(dataUrl), referenceType: "asset" }));
    }
    if (videos[0]) instance.video = await fileToGeminiInline(videos[0]);
    if (audios[0]) instance.audio = await fileToGeminiInline(audios[0]);
    try {
        const created = unwrapEnvelope(
            (
                await providerAxios.post<ApiEnvelope<GeminiVideoOperation>>(
                    geminiVideoUrl(config, model, "predictLongRunning"),
                    {
                        instances: [instance],
                        parameters: {
                            aspectRatio: videoAspectRatio(config.size),
                            durationSeconds: Number(normalizeVideoSeconds(config.videoSeconds)) || 8,
                            resolution: normalizeVideoResolution(config.vquality),
                            generateAudio: boolConfig(config.videoGenerateAudio, true),
                            addWatermark: boolConfig(config.videoWatermark, false),
                        },
                    },
                    { headers: geminiVideoHeaders(config), signal: options?.signal },
                )
            ).data,
            apiText("noVideoTask"),
        );
        if (!created.name) throw new Error(apiText("noVideoTaskId"));
        return { id: created.name, provider: "gemini", model };
    } catch (error) {
        throw videoRequestError(error, apiText("videoTaskCreateFailed"));
    }
}

async function pollGeminiVideoTask(config: AiConfig, task: VideoGenerationTask, options?: RequestOptions): Promise<VideoGenerationTaskState> {
    try {
        const state = unwrapEnvelope((await providerAxios.get<ApiEnvelope<GeminiVideoOperation>>(geminiOperationUrl(config, task.id), { headers: geminiVideoHeaders(config), signal: options?.signal })).data, apiText("videoTaskQueryFailed"));
        if (state.error) return { status: "failed", error: readApiErrorMessage(state.error.message) || apiText("videoGenerationFailed") };
        if (!state.done) return { status: "pending" };
        const uri = state.response?.generateVideoResponse?.generatedSamples?.[0]?.video?.uri;
        if (!uri) return { status: "failed", error: apiText("noPlayableVideo") };
        const url = uri.includes("key=") ? uri : `${uri}${uri.includes("?") ? "&" : "?"}key=${config.apiKey}`;
        return { status: "completed", result: await videoResultFromUrl(config, url, options) };
    } catch (error) {
        throw videoRequestError(error, apiText("videoTaskQueryFailed"));
    }
}

function assertVideoConfig(config: AiConfig, model: string) {
    if (!model) throw new Error(apiText("videoModelRequired"));
    if (!config.baseUrl.trim()) throw new Error(apiText("baseUrlRequired"));
    if (!config.apiKey.trim()) throw new Error(apiText("apiKeyRequired"));
}

function geminiVideoBaseUrl(config: Pick<AiConfig, "baseUrl">) {
    const normalizedBaseUrl = config.baseUrl.trim().replace(/\/+$/, "");
    const lowerBaseUrl = normalizedBaseUrl.toLowerCase();
    return lowerBaseUrl.endsWith("/v1") || lowerBaseUrl.endsWith("/v1beta") ? normalizedBaseUrl : `${normalizedBaseUrl}/v1beta`;
}

function geminiVideoUrl(config: Pick<AiConfig, "baseUrl">, model: string, action: string) {
    return withLocalProxy(`${geminiVideoBaseUrl(config)}/models/${encodeURIComponent(modelOptionName(model).replace(/^models\//, ""))}:${action}`);
}

function geminiOperationUrl(config: Pick<AiConfig, "baseUrl">, name: string) {
    return withLocalProxy(`${geminiVideoBaseUrl(config)}/${name.replace(/^\//, "")}`);
}

function geminiVideoHeaders(config: Pick<AiConfig, "apiKey">) {
    return { "x-goog-api-key": config.apiKey, "Content-Type": "application/json" };
}

function videoAspectRatio(size: string) {
    const ratio = inferVideoRatio(size);
    return ratio === "auto" ? "16:9" : ratio;
}

function parseDataUrlInline(dataUrl: string, fallbackType = "image/png"): GeminiInlineData {
    const match = dataUrl.match(/^data:([^;]+);base64,(.*)$/);
    return { bytesBase64Encoded: match?.[2] || "", mimeType: match?.[1] || fallbackType };
}

async function fileToGeminiInline(file: File): Promise<GeminiInlineData> {
    return parseDataUrlInline(await readFileAsDataUrl(file), file.type || "application/octet-stream");
}

async function referenceMediaToFile(item: { name: string; type?: string; url?: string; storageKey?: string }, fallbackName: string, errorKey: "invalidReferenceVideo" | "invalidReferenceAudio", options?: RequestOptions) {
    let blob = item.storageKey ? await getMediaBlob(item.storageKey) : null;
    if (!blob) {
        const url = item.storageKey ? await resolveMediaUrl(item.storageKey, item.url || "") : item.url || "";
        if (!url) throw new Error(apiText(errorKey));
        try {
            const response = await providerFetch(url, { signal: options?.signal });
            if (!response.ok) throw new Error(apiText(errorKey));
            blob = await response.blob();
        } catch (error) {
            if (options?.signal?.aborted || (error instanceof DOMException && error.name === "AbortError") || (error instanceof Error && error.name === "AbortError")) throw error;
            throw new Error(apiText(errorKey));
        }
    }
    if (!blob.size) throw new Error(apiText(errorKey));
    const expectedPrefix = errorKey === "invalidReferenceVideo" ? "video/" : "audio/";
    if (blob.type && !blob.type.startsWith(expectedPrefix) && !blob.type.includes("octet-stream")) throw new Error(apiText(errorKey));
    return new File([blob], item.name || fallbackName, { type: item.type || blob.type || "application/octet-stream" });
}

function normalizeVideoSeconds(value: string) {
    return clampVideoSeconds(value);
}

function resolveVideoMode(mode: string | undefined, imageCount: number) {
    if (mode === "reference" || imageCount > 2) return "reference";
    return "frames";
}

function normalizeVideoSize(value: string, resolution?: string) {
    if (value === "auto") return null;
    if (/^\d+x\d+$/.test(value || "")) return value;
    const ratio = inferVideoRatio(value || "16:9");
    if (ratio === "auto") return null;
    return computeVideoSize(resolution || "720", ratio);
}

function normalizeVideoResolution(value: string) {
    return `${parseVideoResolution(value)}p`;
}

function unwrapVideoResponse(payload: ApiVideoResponse) {
    return unwrapEnvelope(payload, apiText("noVideoTask"));
}

function unwrapEnvelope<T>(payload: ApiEnvelope<T>, emptyMessage: string): T {
    if (!payload) throw new Error(emptyMessage);
    if (typeof payload === "object" && "code" in payload && payload.code !== undefined) {
        if (payload.code !== 0 && payload.code !== "0") throw new Error(readApiErrorMessage(payload) || apiText("requestFailed"));
        if (!payload.data) throw new Error(emptyMessage);
        return payload.data;
    }
    return payload as T;
}

function videoTaskId(payload: VideoResponse | null | undefined): string {
    if (!payload) return "";
    for (const value of [payload.id, payload.request_id, payload.task_id]) {
        if ((typeof value === "string" || typeof value === "number") && String(value).trim()) return String(value).trim();
    }
    for (const nested of [payload.data, payload.result, payload.output]) {
        const id = videoTaskId(nested);
        if (id) return id;
    }
    return "";
}

function videoStatus(payload: VideoResponse) {
    return nestedVideoStatus(payload, 0);
}

function nestedVideoStatus(value: unknown, depth: number): string {
    if (depth > 5 || value == null || typeof value !== "object") return "";
    const record = value as Record<string, unknown>;
    for (const key of ["status", "state", "task_status", "taskStatus"]) {
        const candidate = record[key];
        if (typeof candidate === "string" && candidate.trim()) return candidate.trim().toLowerCase();
    }
    for (const key of ["data", "result", "output", "task"]) {
        if (!(key in record)) continue;
        const status = nestedVideoStatus(record[key], depth + 1);
        if (status) return status;
    }
    return "";
}

function videoResultUrl(payload: VideoResponse) {
    const direct = [payload.video_url, payload.result_url, payload.url, payload.content?.video_url, payload.content?.url].find(
        (url) => typeof url === "string" && isVideoUrlCandidate(url),
    );
    if (direct) return direct;
    return nestedVideoResultUrl(payload, 0);
}

function nestedVideoResultUrl(value: unknown, depth: number): string | undefined {
    if (depth > 5 || value == null) return undefined;
    if (typeof value === "string") return isVideoUrlCandidate(value) ? value : undefined;
    if (Array.isArray(value)) {
        for (const item of value) {
            const found = nestedVideoResultUrl(item, depth + 1);
            if (found) return found;
        }
        return undefined;
    }
    if (typeof value !== "object") return undefined;

    const record = value as Record<string, unknown>;
    for (const key of ["video_url", "result_url", "url"]) {
        const candidate = record[key];
        if (typeof candidate === "string" && isVideoUrlCandidate(candidate)) return candidate;
    }
    for (const key of ["output", "result", "data", "metadata", "media", "content", "videos", "items"]) {
        if (!(key in record)) continue;
        const found = nestedVideoResultUrl(record[key], depth + 1);
        if (found) return found;
    }
    return undefined;
}

function readApiErrorMessage(value: unknown): string {
    if (!value) return "";
    if (typeof value === "string") {
        try {
            const parsed = JSON.parse(value);
            const inner = readApiErrorMessage(parsed) || value;
            if (inner === value && typeof parsed === "object" && Object.keys(parsed).length === 0) return "";
            return inner;
        } catch {
            if (/<[a-z][\s\S]*>/i.test(value)) return apiText("htmlError", { preview: `${value.slice(0, 80)}...` });
            return value;
        }
    }
    if (typeof value !== "object") return "";
    const payload = value as { msg?: unknown; message?: unknown; error?: unknown; detail?: unknown; data?: unknown; result?: unknown; output?: unknown };
    // error may be a string or an object containing a message.
    const errorMsg = typeof payload.error === "string" ? payload.error : (payload.error as { message?: unknown })?.message;
    return readApiErrorMessage(payload.msg) ||
        readApiErrorMessage(payload.message) ||
        readApiErrorMessage(errorMsg) ||
        readApiErrorMessage(payload.detail) ||
        readApiErrorMessage(payload.data) ||
        readApiErrorMessage(payload.result) ||
        readApiErrorMessage(payload.output) ||
        "";
}

function abortVideoRequest() {
    const error = new Error(apiText("requestCanceled"));
    error.name = "AbortError";
    return error;
}

function videoRequestError(error: unknown, fallback: string) {
    if (axios.isCancel(error) || (error instanceof DOMException && error.name === "AbortError")) return abortVideoRequest();
    return new Error(readAxiosError(error, fallback));
}

function readAxiosError(error: unknown, fallback: string) {
    if (axios.isCancel(error)) return apiText("requestCanceled");
    if (axios.isAxiosError<{ error?: { message?: string }; msg?: string; message?: string; code?: number | string }>(error)) {
        if (!error.response && error.code === "ERR_NETWORK") return apiText("requestFailed");
        const responseData = error.response?.data;
        return readApiErrorMessage(responseData) || statusMessage(error.response?.status, fallback);
    }
    if (error instanceof DOMException && error.name === "AbortError") return apiText("requestCanceled");
    return error instanceof Error ? readApiErrorMessage(error.message) || error.message : fallback;
}

function statusMessage(status: number | undefined, fallback: string) {
    if (status === 401 || status === 403) return apiText("authenticationFailed");
    if (status === 429) return apiText("rateLimited");
    return status ? `${fallback}（${status}）` : fallback;
}

async function assertVideoBlob(blob: Blob) {
    if (!blob.type.includes("json")) return;
    let payload: { code?: number; msg?: string; error?: { message?: string } };
    try {
        payload = JSON.parse(await blob.text()) as { code?: number; msg?: string; error?: { message?: string } };
    } catch {
        return;
    }
    if (typeof payload.code === "number" && payload.code !== 0) throw new Error(readApiErrorMessage(payload) || apiText("videoDownloadFailed"));
    if (payload.error?.message) throw new Error(readApiErrorMessage(payload.error.message) || payload.error.message);
}

function isPublicMediaUrl(value: string) {
    return /^https?:\/\//i.test(value || "");
}

function isVideoUrlCandidate(value: string) {
    const candidate = value.trim();
    return isPublicMediaUrl(candidate) ||
        /^data:video\//i.test(candidate) ||
        /^blob:/i.test(candidate) ||
        /\.mp4(\?|#|$)/i.test(candidate) ||
        /^\.{0,2}\//.test(candidate);
}

function resolveVideoResultUrl(config: AiConfig, value: string) {
    const candidate = value.trim();
    if (!candidate || isPublicMediaUrl(candidate) || /^data:|^blob:/i.test(candidate)) return candidate;
    const baseUrl = config.baseUrl.trim();
    if (!baseUrl) return candidate;
    try {
        return new URL(candidate, baseUrl.endsWith("/") ? baseUrl : `${baseUrl}/`).toString();
    } catch {
        return candidate;
    }
}

function delay(ms: number, signal?: AbortSignal) {
    return new Promise<void>((resolve, reject) => {
        if (signal?.aborted) {
            reject(new DOMException("Aborted", "AbortError"));
            return;
        }
        const timer = setTimeout(resolve, ms);
        signal?.addEventListener(
            "abort",
            () => {
                clearTimeout(timer);
                reject(new DOMException("Aborted", "AbortError"));
            },
            { once: true },
        );
    });
}
