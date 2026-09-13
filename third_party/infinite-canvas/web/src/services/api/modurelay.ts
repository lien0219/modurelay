export type ModuRelayUser = {
    id: number;
    username: string;
    email: string;
    role: "admin" | "user";
    balance: number;
    frozen_balance?: number;
    concurrency: number;
    status: "active" | "disabled";
};

export type ModuRelayGroup = {
    id: number;
    name: string;
    description?: string | null;
    platform: string;
    rate_multiplier: number;
    status: "active" | "inactive";
};

export type ModuRelayApiKey = {
    id: number;
    key: string;
    name: string;
    group_id: number | null;
    status: "active" | "inactive" | "quota_exhausted" | "expired";
    quota: number;
    quota_used: number;
    expires_at: string | null;
    current_concurrency: number;
};

type ApiEnvelope<T> = { code?: number; message?: string; data?: T };
type PaginatedResponse<T> = { items: T[]; total: number; page: number; page_size: number; pages: number };

export type ModuRelayContext = {
    user: ModuRelayUser;
    groups: ModuRelayGroup[];
    keys: ModuRelayApiKey[];
    usableKeys: ModuRelayApiKey[];
};

export const moduRelayContextQueryKey = ["modurelay", "account-context"] as const;

export class ModuRelayApiError extends Error {
    constructor(
        message: string,
        readonly status: number,
    ) {
        super(message);
        this.name = "ModuRelayApiError";
    }
}

function requestHeaders(token: string, json = false) {
    return {
        Accept: "application/json",
        "Accept-Language": localStorage.getItem("sub2api_locale") || "zh",
        Authorization: `Bearer ${token}`,
        ...(json ? { "Content-Type": "application/json" } : {}),
        "X-User-UI-Request": "1",
    };
}

async function responseBody<T>(response: Response): Promise<T> {
    const payload = (await response.json().catch(() => null)) as ApiEnvelope<T> | T | null;
    const envelope = payload && typeof payload === "object" && "code" in payload ? (payload as ApiEnvelope<T>) : null;
    if (!response.ok || (envelope && envelope.code !== 0)) {
        throw new ModuRelayApiError(envelope?.message || `HTTP ${response.status}`, response.status);
    }
    return (envelope ? envelope.data : payload) as T;
}

async function get<T>(path: string, token: string) {
    const response = await fetch(path, { credentials: "same-origin", headers: requestHeaders(token) });
    return responseBody<T>(response);
}

function idempotencyKey() {
    return typeof crypto.randomUUID === "function" ? crypto.randomUUID() : `infinite-canvas-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

async function listAllApiKeys(token: string) {
    const first = await get<PaginatedResponse<ModuRelayApiKey>>("/api/v1/keys?page=1&page_size=100", token);
    if (first.pages <= 1) return first.items;

    const remaining = await Promise.all(Array.from({ length: first.pages - 1 }, (_, index) => get<PaginatedResponse<ModuRelayApiKey>>(`/api/v1/keys?page=${index + 2}&page_size=100`, token)));
    return [first, ...remaining].flatMap((page) => page.items);
}

export function isUsableModuRelayKey(key: ModuRelayApiKey, groupIds: ReadonlySet<number>, now = Date.now()) {
    if (key.status !== "active" || !key.key.trim() || key.group_id === null || !groupIds.has(key.group_id)) return false;
    if (key.quota > 0 && key.quota_used >= key.quota) return false;
    if (!key.expires_at) return true;
    const expiresAt = Date.parse(key.expires_at);
    return Number.isFinite(expiresAt) && expiresAt > now;
}

export async function loadModuRelayContext(token: string): Promise<ModuRelayContext> {
    const [user, groups, keys] = await Promise.all([get<ModuRelayUser>("/api/v1/auth/me", token), get<ModuRelayGroup[]>("/api/v1/groups/available", token), listAllApiKeys(token)]);
    const activeGroups = groups.filter((group) => group.status === "active");
    const groupIds = new Set(activeGroups.map((group) => group.id));
    return { user, groups: activeGroups, keys, usableKeys: keys.filter((key) => isUsableModuRelayKey(key, groupIds)) };
}

export async function createModuRelayApiKey(token: string, input: { name: string; groupId: number }) {
    const response = await fetch("/api/v1/keys", {
        method: "POST",
        credentials: "same-origin",
        headers: { ...requestHeaders(token, true), "Idempotency-Key": idempotencyKey() },
        body: JSON.stringify({ name: input.name.trim(), group_id: input.groupId }),
    });
    const record = await responseBody<ModuRelayApiKey>(response);
    if (!record.key?.trim()) throw new ModuRelayApiError("API key was not returned", response.status);
    return record;
}

export async function loadModuRelayModels(apiKey: string) {
    const response = await fetch("/v1/models", {
        credentials: "same-origin",
        headers: { Accept: "application/json", Authorization: `Bearer ${apiKey}` },
    });
    if (!response.ok) return [];
    const payload = (await response.json().catch(() => null)) as { data?: Array<{ id?: unknown }> } | null;
    return Array.from(new Set((payload?.data || []).map((item) => (typeof item.id === "string" ? item.id.trim() : "")).filter(Boolean)));
}

export function maskModuRelayKey(value: string) {
    const key = value.trim();
    if (key.length <= 10) return `${key.slice(0, 3)}...${key.slice(-3)}`;
    return `${key.slice(0, 7)}...${key.slice(-4)}`;
}

export function normalizedModuRelayBaseUrl(value: string) {
    return value.trim().replace(/\/+$/, "").replace(/\/v1$/i, "");
}
