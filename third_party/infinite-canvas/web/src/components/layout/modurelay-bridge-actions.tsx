import type { CSSProperties, MouseEvent as ReactMouseEvent } from "react";
import { useMemo, useState } from "react";
import { App, Tooltip } from "antd";
import { ArrowLeft, Check, Link2, LoaderCircle, LogIn } from "lucide-react";
import { useTranslation } from "react-i18next";

import { encodeChannelModel, guessCapability, modelOptionsFromChannels, useConfigStore } from "@/stores/use-config-store";

type ModuRelayBridgeActionsProps = {
    className: string;
    style?: CSSProperties;
    showBackLabel?: boolean;
};

type ApiKeyRecord = { key?: unknown };
type ApiEnvelope<T> = { code?: number; message?: string; data?: T };

function idempotencyKey() {
    return typeof crypto.randomUUID === "function" ? crypto.randomUUID() : `infinite-canvas-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

function normalizedBaseUrl(value: string) {
    return value.trim().replace(/\/+$/, "").replace(/\/v1$/i, "");
}

async function responseBody<T>(response: Response): Promise<T> {
    const payload = (await response.json().catch(() => null)) as ApiEnvelope<T> | T | null;
    const envelope = payload && typeof payload === "object" && "code" in payload ? (payload as ApiEnvelope<T>) : null;
    if (!response.ok || (envelope && envelope.code !== 0)) {
        throw new Error(envelope?.message || `HTTP ${response.status}`);
    }
    return (envelope ? envelope.data : payload) as T;
}

async function createCanvasApiKey(token: string) {
    const response = await fetch("/api/v1/keys", {
        method: "POST",
        credentials: "same-origin",
        headers: {
            Accept: "application/json",
            "Accept-Language": localStorage.getItem("sub2api_locale") || "zh",
            Authorization: `Bearer ${token}`,
            "Content-Type": "application/json",
            "Idempotency-Key": idempotencyKey(),
            "X-User-UI-Request": "1",
        },
        body: JSON.stringify({ name: "Infinite Canvas" }),
    });
    const record = await responseBody<ApiKeyRecord>(response);
    if (typeof record?.key !== "string" || !record.key.trim()) throw new Error("API key was not returned");
    return record.key;
}

async function loadGatewayModels(apiKey: string) {
    const response = await fetch("/v1/models", {
        credentials: "same-origin",
        headers: { Accept: "application/json", Authorization: `Bearer ${apiKey}` },
    });
    if (!response.ok) return [];
    const payload = (await response.json().catch(() => null)) as { data?: Array<{ id?: unknown }> } | null;
    return Array.from(new Set((payload?.data || []).map((item) => (typeof item.id === "string" ? item.id.trim() : "")).filter(Boolean)));
}

function attachModels(baseUrl: string, modelNames: string[]) {
    if (!modelNames.length) return;
    const state = useConfigStore.getState();
    const channelIndex = state.config.channels.findIndex((channel) => normalizedBaseUrl(channel.baseUrl) === normalizedBaseUrl(baseUrl));
    if (channelIndex < 0) return;

    const channel = state.config.channels[channelIndex];
    const models = modelNames.map((name) => ({ name, capability: guessCapability(name) }));
    const channels = state.config.channels.map((item, index) => (index === channelIndex ? { ...item, models } : item));
    state.updateConfig("channels", channels);
    state.updateConfig("models", modelOptionsFromChannels(channels));

    for (const capability of ["image", "video", "text", "audio"] as const) {
        const first = models.find((model) => model.capability === capability);
        if (first) state.updateConfig(`${capability}Model`, encodeChannelModel(channel.id, first.name));
    }
}

export function ModuRelayBridgeActions({ className, style, showBackLabel = false }: ModuRelayBridgeActionsProps) {
    const { message, modal } = App.useApp();
    const { t } = useTranslation();
    const [connecting, setConnecting] = useState(false);
    const config = useConfigStore((state) => state.config);
    const importChannelCredentials = useConfigStore((state) => state.importChannelCredentials);
    const openConfigDialog = useConfigStore((state) => state.openConfigDialog);
    const gatewayBaseUrl = window.location.origin;
    const token = localStorage.getItem("auth_token") || "";
    const connected = useMemo(() => config.channels.some((channel) => normalizedBaseUrl(channel.baseUrl) === normalizedBaseUrl(gatewayBaseUrl) && Boolean(channel.apiKey.trim())), [config.channels, gatewayBaseUrl]);

    const connect = () => {
        if (!token) {
            // Keep the Vue-owned /canvas route in the login redirect. After
            // authentication it performs a full-document handoff to the
            // embedded React app; Vue Router cannot resolve /infinite-canvas
            // as an in-app route.
            window.location.assign("/login?redirect=/canvas");
            return;
        }
        if (connected) {
            openConfigDialog(false, "channels");
            return;
        }

        modal.confirm({
            title: t("modurelay.confirmTitle"),
            content: t("modurelay.confirmDescription"),
            okText: t("modurelay.confirmAction"),
            cancelText: t("common.cancel"),
            centered: true,
            onOk: async () => {
                setConnecting(true);
                try {
                    const apiKey = await createCanvasApiKey(token);
                    importChannelCredentials({ baseUrl: gatewayBaseUrl, apiKey });
                    const models = await loadGatewayModels(apiKey);
                    attachModels(gatewayBaseUrl, models);
                    message.success(t(models.length ? "modurelay.connectSuccess" : "modurelay.connectSuccessWithoutModels"));
                } catch (error) {
                    const detail = error instanceof Error ? error.message : String(error);
                    if (/401|unauthorized|token/i.test(detail)) message.error(t("modurelay.sessionExpired"));
                    else message.error(t("modurelay.connectFailed", { message: detail }));
                    throw error;
                } finally {
                    setConnecting(false);
                }
            },
        });
    };

    const returnToRelay = (event: ReactMouseEvent<HTMLAnchorElement>) => {
        event.preventDefault();
        window.dispatchEvent(new CustomEvent("modurelay-workspace-door", { detail: { direction: "to-relay", href: "/dashboard" } }));
    };

    const connectionLabel = t(!token ? "modurelay.signIn" : connected ? "modurelay.connected" : "modurelay.connect");
    const ConnectionIcon = connecting ? LoaderCircle : !token ? LogIn : connected ? Check : Link2;

    return (
        <>
            <Tooltip title={t("modurelay.back")} mouseEnterDelay={0.2}>
                <a href="/dashboard" onClick={returnToRelay} className={showBackLabel ? `${className} !w-auto min-w-fit gap-1 px-2 text-xs` : className} style={style} aria-label={t("modurelay.back")}>
                    <ArrowLeft className="size-4" />
                    {showBackLabel ? <span className="hidden whitespace-nowrap sm:inline">{t("modurelay.back")}</span> : null}
                </a>
            </Tooltip>
            <Tooltip title={connectionLabel} mouseEnterDelay={0.2}>
                <button type="button" className={className} style={style} onClick={connect} disabled={connecting} aria-label={connectionLabel} aria-busy={connecting}>
                    <ConnectionIcon className={connecting ? "size-4 animate-spin" : "size-4"} />
                </button>
            </Tooltip>
        </>
    );
}
