import type { CSSProperties, MouseEvent as ReactMouseEvent } from "react";
import { useEffect, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, App, Button, Empty, Input, Modal, Radio, Segmented, Select, Skeleton, Tag, Tooltip } from "antd";
import { ArrowLeft, Check, KeyRound, Link2, LoaderCircle, LogIn, Plus, RefreshCw } from "lucide-react";
import { useTranslation } from "react-i18next";

import { createModuRelayApiKey, loadModuRelayContext, loadModuRelayModels, maskModuRelayKey, ModuRelayApiError, moduRelayContextQueryKey, normalizedModuRelayBaseUrl } from "@/services/api/modurelay";
import { encodeChannelModel, guessCapability, modelOptionsFromChannels, useConfigStore } from "@/stores/use-config-store";

type ModuRelayBridgeActionsProps = {
    className: string;
    style?: CSSProperties;
    showBackLabel?: boolean;
};

type ConnectMode = "reuse" | "create";

function attachModels(baseUrl: string, modelNames: string[]) {
    if (!modelNames.length) return;
    const state = useConfigStore.getState();
    const channelIndex = state.config.channels.findIndex((channel) => normalizedModuRelayBaseUrl(channel.baseUrl) === normalizedModuRelayBaseUrl(baseUrl));
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
    const { message } = App.useApp();
    const { i18n, t } = useTranslation();
    const queryClient = useQueryClient();
    const [open, setOpen] = useState(false);
    const [connecting, setConnecting] = useState(false);
    const [mode, setMode] = useState<ConnectMode>("reuse");
    const [selectedKeyId, setSelectedKeyId] = useState<number | null>(null);
    const [keyName, setKeyName] = useState("Infinite Canvas");
    const [groupId, setGroupId] = useState<number | null>(null);
    const [validationVisible, setValidationVisible] = useState(false);
    const config = useConfigStore((state) => state.config);
    const importChannelCredentials = useConfigStore((state) => state.importChannelCredentials);
    const gatewayBaseUrl = window.location.origin;
    const token = localStorage.getItem("auth_token") || "";
    const gatewayChannel = useMemo(() => config.channels.find((channel) => normalizedModuRelayBaseUrl(channel.baseUrl) === normalizedModuRelayBaseUrl(gatewayBaseUrl)), [config.channels, gatewayBaseUrl]);
    const connected = Boolean(gatewayChannel?.apiKey.trim());
    const contextQuery = useQuery({
        queryKey: moduRelayContextQueryKey,
        queryFn: () => loadModuRelayContext(token),
        enabled: open && Boolean(token),
    });
    const groupById = useMemo(() => new Map((contextQuery.data?.groups || []).map((group) => [group.id, group])), [contextQuery.data?.groups]);
    const selectedKey = contextQuery.data?.usableKeys.find((key) => key.id === selectedKeyId);

    useEffect(() => {
        if (!open || !contextQuery.data) return;
        const usableKeys = contextQuery.data.usableKeys;
        setSelectedKeyId((current) => {
            if (current !== null && usableKeys.some((key) => key.id === current)) return current;
            return usableKeys.find((key) => key.key === gatewayChannel?.apiKey)?.id ?? usableKeys[0]?.id ?? null;
        });
        if (!usableKeys.length) setMode("create");
    }, [contextQuery.data, gatewayChannel?.apiKey, open]);

    const openConnection = () => {
        if (!token) {
            window.location.assign("/login?redirect=/canvas");
            return;
        }
        setMode("reuse");
        setSelectedKeyId(null);
        setKeyName("Infinite Canvas");
        setGroupId(null);
        setValidationVisible(false);
        setOpen(true);
    };

    const connect = async () => {
        setValidationVisible(true);
        if (mode === "reuse" && !selectedKey) return;
        if (mode === "create" && (!keyName.trim() || groupId === null)) return;

        setConnecting(true);
        try {
            const key = mode === "create" ? await createModuRelayApiKey(token, { name: keyName, groupId: groupId! }) : selectedKey!;
            importChannelCredentials({ baseUrl: gatewayBaseUrl, apiKey: key.key });
            const models = await loadModuRelayModels(key.key);
            attachModels(gatewayBaseUrl, models);
            if (mode === "create") void queryClient.invalidateQueries({ queryKey: moduRelayContextQueryKey });
            setOpen(false);
            message.success(t(models.length ? "modurelay.connectSuccess" : "modurelay.connectSuccessWithoutModels"));
        } catch (error) {
            const detail = error instanceof Error ? error.message : String(error);
            if (error instanceof ModuRelayApiError && error.status === 401) message.error(t("modurelay.sessionExpired"));
            else message.error(t("modurelay.connectFailed", { message: detail }));
        } finally {
            setConnecting(false);
        }
    };

    const returnToRelay = (event: ReactMouseEvent<HTMLAnchorElement>) => {
        event.preventDefault();
        window.dispatchEvent(new CustomEvent("modurelay-workspace-door", { detail: { direction: "to-relay", href: "/dashboard", replace: true } }));
    };

    const connectionLabel = t(!token ? "modurelay.signIn" : connected ? "modurelay.manageConnection" : "modurelay.connect");
    const ConnectionIcon = connecting ? LoaderCircle : !token ? LogIn : connected ? Check : Link2;
    const locale = i18n.resolvedLanguage === "zh-CN" ? "zh-CN" : "en-US";
    const formatAmount = (value: number) => new Intl.NumberFormat(locale, { style: "currency", currency: "USD", minimumFractionDigits: 2 }).format(value || 0);
    const formatDate = (value: string) => new Intl.DateTimeFormat(locale, { year: "numeric", month: "short", day: "numeric" }).format(new Date(value));
    const canSubmit = !contextQuery.isPending && !contextQuery.isError && (mode === "reuse" ? Boolean(selectedKey) : Boolean(keyName.trim() && groupId !== null && contextQuery.data?.groups.some((group) => group.id === groupId)));

    return (
        <>
            <Tooltip title={t("modurelay.back")} mouseEnterDelay={0.2}>
                <a href="/dashboard" onClick={returnToRelay} className={showBackLabel ? `${className} !w-auto min-w-fit gap-1 px-2 text-xs` : className} style={style} aria-label={t("modurelay.back")}>
                    <ArrowLeft className="size-4" />
                    {showBackLabel ? <span className="hidden whitespace-nowrap sm:inline">{t("modurelay.back")}</span> : null}
                </a>
            </Tooltip>
            <Tooltip title={connectionLabel} mouseEnterDelay={0.2}>
                <button type="button" className={className} style={style} onClick={openConnection} disabled={connecting} aria-label={connectionLabel} aria-busy={connecting}>
                    <ConnectionIcon className={connecting ? "size-4 animate-spin" : "size-4"} />
                </button>
            </Tooltip>

            <Modal
                title={t("modurelay.connection.title")}
                open={open}
                width={620}
                centered
                destroyOnHidden
                confirmLoading={connecting}
                okText={t(mode === "reuse" ? "modurelay.connection.connectSelected" : "modurelay.connection.createAndConnect")}
                cancelText={t("common.cancel")}
                okButtonProps={{ disabled: !canSubmit }}
                cancelButtonProps={{ disabled: connecting }}
                closable={!connecting}
                maskClosable={!connecting}
                onCancel={() => setOpen(false)}
                onOk={() => void connect()}
            >
                <div className="space-y-5 pt-2">
                    <Segmented
                        block
                        value={mode}
                        options={[
                            { value: "reuse", label: t("modurelay.connection.reuse", { count: contextQuery.data?.usableKeys.length || 0 }), icon: <KeyRound className="size-4" /> },
                            { value: "create", label: t("modurelay.connection.create"), icon: <Plus className="size-4" /> },
                        ]}
                        onChange={(value) => {
                            setMode(value as ConnectMode);
                            setValidationVisible(false);
                        }}
                    />

                    {contextQuery.isPending ? (
                        <div className="space-y-3" aria-live="polite">
                            <Skeleton active paragraph={{ rows: 3 }} />
                        </div>
                    ) : contextQuery.isError ? (
                        <Alert
                            type="error"
                            showIcon
                            message={t("modurelay.connection.loadFailed")}
                            description={contextQuery.error instanceof Error ? contextQuery.error.message : String(contextQuery.error)}
                            action={
                                <Button type="text" size="small" icon={<RefreshCw className="size-4" />} onClick={() => void contextQuery.refetch()}>
                                    {t("common.retry")}
                                </Button>
                            }
                        />
                    ) : mode === "reuse" ? (
                        contextQuery.data.usableKeys.length ? (
                            <Radio.Group className="w-full" value={selectedKeyId} onChange={(event) => setSelectedKeyId(event.target.value as number)}>
                                <div className="max-h-[360px] space-y-2 overflow-y-auto pr-1">
                                    {contextQuery.data.usableKeys.map((key) => {
                                        const group = key.group_id === null ? undefined : groupById.get(key.group_id);
                                        const current = key.key === gatewayChannel?.apiKey;
                                        return (
                                            <Radio
                                                key={key.id}
                                                value={key.id}
                                                aria-label={key.name}
                                                className={`!flex !items-start rounded-lg border border-border !p-3 transition-colors hover:bg-muted/60 [&>span:last-child]:min-w-0 [&>span:last-child]:flex-1 ${selectedKeyId === key.id ? "bg-muted/70" : "bg-background"}`}
                                            >
                                                <div className="min-w-0 flex-1">
                                                    <div className="flex min-w-0 flex-wrap items-center gap-2">
                                                        <span className="min-w-0 truncate text-sm font-medium text-foreground">{key.name}</span>
                                                        {current ? (
                                                            <Tag color="success" className="!m-0">
                                                                {t("modurelay.connection.current")}
                                                            </Tag>
                                                        ) : null}
                                                    </div>
                                                    <div className="mt-1 flex min-w-0 flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
                                                        <code className="max-w-full truncate">{maskModuRelayKey(key.key)}</code>
                                                        <span className="truncate">{group?.name || t("modurelay.connection.unknownGroup")}</span>
                                                        <span>{key.quota > 0 ? t("modurelay.connection.quota", { used: formatAmount(key.quota_used), total: formatAmount(key.quota) }) : t("modurelay.connection.unlimitedQuota")}</span>
                                                        <span>{key.expires_at ? t("modurelay.connection.expires", { date: formatDate(key.expires_at) }) : t("modurelay.connection.neverExpires")}</span>
                                                    </div>
                                                </div>
                                            </Radio>
                                        );
                                    })}
                                </div>
                            </Radio.Group>
                        ) : (
                            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("modurelay.connection.noReusableKeys")}>
                                <Button type="primary" icon={<Plus className="size-4" />} onClick={() => setMode("create")}>
                                    {t("modurelay.connection.create")}
                                </Button>
                            </Empty>
                        )
                    ) : (
                        <div className="space-y-4">
                            <div>
                                <label htmlFor="modurelay-key-name" className="mb-1.5 block text-sm font-medium text-foreground">
                                    {t("modurelay.connection.keyName")}
                                </label>
                                <Input
                                    id="modurelay-key-name"
                                    value={keyName}
                                    maxLength={100}
                                    status={validationVisible && !keyName.trim() ? "error" : undefined}
                                    onChange={(event) => setKeyName(event.target.value)}
                                    onBlur={() => setValidationVisible(true)}
                                />
                                {validationVisible && !keyName.trim() ? (
                                    <p className="mt-1 text-xs text-destructive" role="alert">
                                        {t("modurelay.connection.keyNameRequired")}
                                    </p>
                                ) : null}
                            </div>
                            <div>
                                <label htmlFor="modurelay-key-group" className="mb-1.5 block text-sm font-medium text-foreground">
                                    {t("modurelay.connection.group")}
                                </label>
                                <Select
                                    id="modurelay-key-group"
                                    className="w-full"
                                    value={groupId}
                                    placeholder={t("modurelay.connection.selectGroup")}
                                    status={validationVisible && groupId === null ? "error" : undefined}
                                    options={(contextQuery.data?.groups || []).map((group) => ({ value: group.id, label: `${group.name} · ${group.platform}` }))}
                                    onChange={(value) => setGroupId(value)}
                                    onBlur={() => setValidationVisible(true)}
                                />
                                {groupId === null ? <p className={`mt-1 text-xs ${validationVisible ? "text-destructive" : "text-muted-foreground"}`}>{t("modurelay.connection.groupRequired")}</p> : null}
                            </div>
                            {!contextQuery.data?.groups.length ? <Alert type="warning" showIcon message={t("modurelay.connection.noGroups")} /> : null}
                        </div>
                    )}
                </div>
            </Modal>
        </>
    );
}
