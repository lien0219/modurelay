import i18n from "@/i18n";

const AUTH_TOKEN_KEY = "auth_token";
const AUTH_USER_KEY = "auth_user";
const REFRESH_TOKEN_KEY = "refresh_token";
const TOKEN_EXPIRES_AT_KEY = "token_expires_at";
const TOKEN_REFRESH_LOCK_NAME = "sub2api-auth-token-refresh";
const TOKEN_REFRESH_BUFFER_MS = 60_000;

type RefreshTokenResponse = {
    access_token: string;
    refresh_token: string;
    expires_in: number;
};

type ApiEnvelope<T> = {
    code?: number;
    message?: string;
    data?: T;
};

export type ModuRelaySession = {
    accessToken: string;
    userID: string | null;
};

let inFlightRefresh: { userID: string | null; promise: Promise<string> } | null = null;

function currentUserID() {
    const rawUser = localStorage.getItem(AUTH_USER_KEY);
    if (!rawUser) return null;
    try {
        const id = (JSON.parse(rawUser) as { id?: unknown }).id;
        return typeof id === "string" || typeof id === "number" ? String(id) : null;
    } catch {
        return null;
    }
}

function currentAccessToken() {
    return localStorage.getItem(AUTH_TOKEN_KEY)?.trim() || "";
}

function sessionRequiredError() {
    return new Error(i18n.t("apiErrors.modurelayProviderSessionRequired"));
}

async function refreshUnderLock(failedAccessToken: string, expectedUserID: string | null) {
    if (currentUserID() !== expectedUserID) throw sessionRequiredError();

    const accessToken = currentAccessToken();
    if (accessToken && accessToken !== failedAccessToken) return accessToken;

    const refreshToken = localStorage.getItem(REFRESH_TOKEN_KEY)?.trim() || "";
    if (!refreshToken) throw sessionRequiredError();

    const response = await globalThis.fetch("/api/v1/auth/refresh", {
        method: "POST",
        credentials: "same-origin",
        headers: { Accept: "application/json", "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: refreshToken }),
    });
    const payload = (await response.json().catch(() => null)) as ApiEnvelope<RefreshTokenResponse> | null;
    const tokens = payload?.data;
    const nextAccessToken = tokens?.access_token?.trim() || "";
    const nextRefreshToken = tokens?.refresh_token?.trim() || "";
    const expiresIn = Number(tokens?.expires_in);
    if (
        !response.ok ||
        payload?.code !== 0 ||
        !nextAccessToken ||
        !nextRefreshToken ||
        !Number.isFinite(expiresIn) ||
        expiresIn <= 0
    ) {
        throw new Error(payload?.message || i18n.t("apiErrors.modurelayProviderSessionRequired"));
    }

    if (currentUserID() !== expectedUserID || localStorage.getItem(REFRESH_TOKEN_KEY) !== refreshToken) {
        const latestAccessToken = currentAccessToken();
        if (currentUserID() === expectedUserID && latestAccessToken && latestAccessToken !== accessToken) return latestAccessToken;
        throw sessionRequiredError();
    }

    localStorage.setItem(AUTH_TOKEN_KEY, nextAccessToken);
    localStorage.setItem(TOKEN_EXPIRES_AT_KEY, String(Date.now() + expiresIn * 1000));
    localStorage.setItem(REFRESH_TOKEN_KEY, nextRefreshToken);
    return nextAccessToken;
}

export async function getModuRelaySession(): Promise<ModuRelaySession> {
    const accessToken = currentAccessToken();
    if (!accessToken) throw sessionRequiredError();

    const userID = currentUserID();
    const rawExpiresAt = localStorage.getItem(TOKEN_EXPIRES_AT_KEY);
    const expiresAt = Number(rawExpiresAt);
    if (rawExpiresAt && Number.isFinite(expiresAt) && expiresAt <= Date.now() + TOKEN_REFRESH_BUFFER_MS) {
        return { accessToken: await refreshModuRelaySession(accessToken, userID), userID };
    }
    return { accessToken, userID };
}

export function refreshModuRelaySession(failedAccessToken: string, userID: string | null): Promise<string> {
    if (inFlightRefresh?.userID === userID) return inFlightRefresh.promise;

    const refresh = () => refreshUnderLock(failedAccessToken, userID);
    const promise: Promise<string> = typeof navigator !== "undefined" && navigator.locks
        ? Promise.resolve(navigator.locks.request(TOKEN_REFRESH_LOCK_NAME, refresh))
        : refresh();
    const pending = { userID, promise };
    inFlightRefresh = pending;
    void promise.then(
        () => { if (inFlightRefresh === pending) inFlightRefresh = null; },
        () => { if (inFlightRefresh === pending) inFlightRefresh = null; },
    );
    return promise;
}

async function responsePayload(value: unknown) {
    let payload = value;
    if (typeof Blob !== "undefined" && value instanceof Blob) {
        payload = await value.text().catch(() => "");
    }
    if (typeof payload === "string") {
        try {
            payload = JSON.parse(payload);
        } catch {
            return null;
        }
    }
    return payload && typeof payload === "object" ? payload as { code?: unknown } : null;
}

export async function isModuRelaySessionExpiredPayload(value: unknown) {
    return (await responsePayload(value))?.code === "TOKEN_EXPIRED";
}

export async function isModuRelaySessionExpiredResponse(response: Response) {
    if (response.status !== 401) return false;
    return isModuRelaySessionExpiredPayload(await response.clone().json().catch(() => null));
}
