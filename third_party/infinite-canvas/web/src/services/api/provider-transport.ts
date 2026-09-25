import axios, { AxiosHeaders, type InternalAxiosRequestConfig } from "axios";

import i18n from "@/i18n";
import { normalizeLocalProxyUrl, useConfigStore, withLocalProxy } from "@/stores/use-config-store";

const MODURELAY_PROVIDER_PROXY_URL = "/api/v1/canvas/upstream";
const MODURELAY_TARGET_HEADER = "X-Canvas-Upstream-URL";
const MODURELAY_PROVIDER_AUTH_HEADER = "X-Canvas-Provider-Authorization";

function integratedRequestUrl(rawUrl: string) {
    if (typeof window === "undefined" || import.meta.env.VITE_MODURELAY_INTEGRATION !== "true") return null;
    let targetUrl = rawUrl.trim();
    const { proxyEnabled, proxyUrl } = useConfigStore.getState().config;
    if (proxyEnabled) {
        const localProxyBase = normalizeLocalProxyUrl(proxyUrl);
        if (localProxyBase && targetUrl.startsWith(`${localProxyBase}/`)) {
            targetUrl = targetUrl.slice(localProxyBase.length + 1);
        }
    }
    return targetUrl;
}

function integratedProviderTarget(rawUrl: string) {
    const targetUrl = integratedRequestUrl(rawUrl);
    if (targetUrl === null) return null;
    try {
        const target = new URL(targetUrl, window.location.href);
        if (!/^https?:$/.test(target.protocol) || target.origin === window.location.origin) return null;
        return target.href;
    } catch {
        return null;
    }
}

function requireModuRelaySession() {
    const token = localStorage.getItem("auth_token")?.trim() || "";
    if (!token) throw new Error(i18n.t("apiErrors.modurelayProviderSessionRequired"));
    return token;
}

function prepareAxiosProxyRequest(config: InternalAxiosRequestConfig) {
    const requestUrl = integratedRequestUrl(String(config.url || ""));
    if (requestUrl === null) return config;
    config.url = requestUrl;
    const targetUrl = integratedProviderTarget(axios.getUri(config));
    if (!targetUrl) return config;

    const headers = AxiosHeaders.from(config.headers as AxiosHeaders);
    const providerAuthorization = headers.get("Authorization");
    headers.delete("Authorization");
    if (typeof providerAuthorization === "string" && providerAuthorization.trim()) {
        headers.set(MODURELAY_PROVIDER_AUTH_HEADER, providerAuthorization);
    }
    headers.set(MODURELAY_TARGET_HEADER, targetUrl);
    headers.set("Authorization", `Bearer ${requireModuRelaySession()}`);
    headers.set("X-User-UI-Request", "1");
    config.url = MODURELAY_PROVIDER_PROXY_URL;
    config.baseURL = undefined;
    config.params = undefined;
    config.paramsSerializer = undefined;
    config.headers = headers;
    return config;
}

export const providerAxios = axios.create();

providerAxios.interceptors.request.use((config) => prepareAxiosProxyRequest(config));

export function providerFetch(input: RequestInfo | URL, init?: RequestInit) {
    const sourceRequest = typeof Request !== "undefined" && input instanceof Request ? input : null;
    const originalUrl = sourceRequest?.url || String(input);
    const signal = init?.signal || sourceRequest?.signal;
    if (/^data:/i.test(originalUrl)) return dataUrlResponse(originalUrl, signal);
    const proxiedUrl = withLocalProxy(originalUrl);
    const requestUrl = integratedRequestUrl(proxiedUrl) ?? proxiedUrl;
    const targetUrl = integratedProviderTarget(requestUrl);

    const headers = new Headers(sourceRequest?.headers);
    new Headers(init?.headers).forEach((value, key) => headers.set(key, value));
    let finalUrl = requestUrl;
    if (targetUrl) {
        const providerAuthorization = headers.get("Authorization");
        headers.delete("Authorization");
        if (providerAuthorization?.trim()) headers.set(MODURELAY_PROVIDER_AUTH_HEADER, providerAuthorization);
        headers.set(MODURELAY_TARGET_HEADER, targetUrl);
        headers.set("Authorization", `Bearer ${requireModuRelaySession()}`);
        headers.set("X-User-UI-Request", "1");
        finalUrl = MODURELAY_PROVIDER_PROXY_URL;
    }

    if (sourceRequest) {
        return globalThis.fetch(new Request(finalUrl, sourceRequest), { ...init, headers });
    }
    return globalThis.fetch(finalUrl, { ...init, headers });
}


function dataUrlResponse(value: string, signal?: AbortSignal | null) {
    if (signal?.aborted) return Promise.reject(abortReason(signal));
    const commaIndex = value.indexOf(",");
    if (commaIndex <= 5) return Promise.reject(new TypeError("Invalid data URL"));
    const metadata = value.slice(5, commaIndex);
    const payload = value.slice(commaIndex + 1);
    const parts = metadata.split(";");
    const mimeType = parts[0] || "text/plain";
    try {
        let blob: Blob;
        if (parts.includes("base64")) {
            const binary = atob(payload.replace(/\s/g, ""));
            const bytes = new Uint8Array(binary.length);
            for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index);
            blob = new Blob([bytes], { type: mimeType });
        } else {
            blob = new Blob([decodeURIComponent(payload)], { type: mimeType });
        }
        return Promise.resolve(new Response(blob, { status: 200, headers: { "Content-Type": mimeType } }));
    } catch {
        return Promise.reject(new TypeError("Invalid data URL"));
    }
}

function abortReason(signal: AbortSignal) {
    return signal.reason instanceof Error ? signal.reason : new DOMException("Aborted", "AbortError");
}
