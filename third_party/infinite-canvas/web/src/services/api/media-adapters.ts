export type MediaModelFamily =
    | "grok"
    | "wan"
    | "seedance"
    | "kling"
    | "minimax"
    | "happyhorse"
    | "gemini"
    | "seedream"
    | "gpt-image"
    | "generic";

export type VideoTransportKind = "xai-json" | "compatible-json" | "multipart";

/**
 * Upstreams in channel pools often prefix the public model name with an internal
 * route id (for example "62:wan-3.0"). Adapter selection must ignore that
 * routing prefix while keeping the original model id in the actual request.
 */
export function normalizeRoutedModelName(model: string) {
    const value = model.trim().toLowerCase();
    const qualified = value.match(/^([a-z0-9_.-]+)\s*:\s*(.+)$/);
    if (!qualified) return value;

    // Match the backend VideoModelRef grammar. In particular, never interpret
    // URL schemes such as https:// as provider prefixes.
    const canonical = qualified[2].trim();
    return canonical.startsWith("/") ? value : canonical;
}

export function mediaModelFamily(model: string): MediaModelFamily {
    const value = normalizeRoutedModelName(model);
    if (/grok[-_.]?imagine[-_.]?video|^grok.*video/.test(value)) return "grok";
    if (/(?:^|[-_.])wan[-_.]?\d/.test(value) || value.startsWith("wan")) return "wan";
    if (value.includes("seedance")) return "seedance";
    if (value.includes("kling")) return "kling";
    if (value.includes("minimax") || value.includes("hailuo")) return "minimax";
    if (value.includes("happyhorse")) return "happyhorse";
    if (value.includes("seedream")) return "seedream";
    if (value.includes("gpt-image")) return "gpt-image";
    if (value.includes("gemini")) return "gemini";
    return "generic";
}

/**
 * Known modern video families are JSON-first. Unknown OpenAI-compatible video
 * models stay multipart-first for backwards compatibility and can negotiate
 * JSON after a 415 response.
 */
export function videoTransportKind(model: string): VideoTransportKind {
    const family = mediaModelFamily(model);
    if (family === "grok") return "xai-json";
    if (family === "wan" || family === "seedance" || family === "kling" || family === "minimax" || family === "happyhorse") return "compatible-json";
    return "multipart";
}

export function isKnownVideoFamily(model: string) {
    return videoTransportKind(model) !== "multipart";
}

/**
 * These image families are commonly exposed by aggregators through JSON image
 * generation APIs even when reference images are supplied. We still try the
 * standard OpenAI multipart edit endpoint first and only negotiate JSON after
 * an unsupported-media response.
 */
export function isJsonReferenceImageFamily(model: string) {
    const family = mediaModelFamily(model);
    return family === "seedream" || family === "gemini";
}

export function isGrokVideoFamily(model: string) {
    return mediaModelFamily(model) === "grok";
}
