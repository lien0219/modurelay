import type { CSSProperties } from "react";
import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Alert, Button, Drawer, Empty, Skeleton, Tag, Tooltip } from "antd";
import { BadgeDollarSign, CircleUserRound, ExternalLink, Gauge, KeyRound, Layers3, RefreshCw, Snowflake } from "lucide-react";
import { useTranslation } from "react-i18next";

import { loadModuRelayContext, maskModuRelayKey, moduRelayContextQueryKey, normalizedModuRelayBaseUrl } from "@/services/api/modurelay";
import { useConfigStore } from "@/stores/use-config-store";

type ModuRelayAccountCenterProps = {
    className: string;
    style?: CSSProperties;
};

export function ModuRelayAccountCenter({ className, style }: ModuRelayAccountCenterProps) {
    const { i18n, t } = useTranslation();
    const [open, setOpen] = useState(false);
    const config = useConfigStore((state) => state.config);
    const token = localStorage.getItem("auth_token") || "";
    const gatewayBaseUrl = window.location.origin;
    const gatewayKey = useMemo(() => config.channels.find((channel) => normalizedModuRelayBaseUrl(channel.baseUrl) === normalizedModuRelayBaseUrl(gatewayBaseUrl))?.apiKey.trim() || "", [config.channels, gatewayBaseUrl]);
    const query = useQuery({
        queryKey: moduRelayContextQueryKey,
        queryFn: () => loadModuRelayContext(token),
        enabled: open && Boolean(token),
    });
    const locale = i18n.resolvedLanguage === "zh-CN" ? "zh-CN" : "en-US";
    const formatAmount = (value: number) => new Intl.NumberFormat(locale, { style: "currency", currency: "USD", minimumFractionDigits: 2 }).format(value || 0);
    const formatDate = (value: string) => new Intl.DateTimeFormat(locale, { year: "numeric", month: "short", day: "numeric" }).format(new Date(value));

    const showAccount = () => {
        if (!token) {
            window.location.assign("/login?redirect=/canvas");
            return;
        }
        setOpen(true);
    };

    const openKeyManagement = () => {
        window.dispatchEvent(new CustomEvent("modurelay-workspace-door", { detail: { direction: "to-relay", href: "/keys" } }));
    };

    return (
        <>
            <Tooltip title={t("modurelay.account.title")} mouseEnterDelay={0.2}>
                <button type="button" className={className} style={style} onClick={showAccount} aria-label={t("modurelay.account.title")}>
                    <CircleUserRound className="size-4" />
                </button>
            </Tooltip>

            <Drawer
                title={t("modurelay.account.title")}
                open={open}
                placement="right"
                width="min(440px, 100vw)"
                destroyOnHidden
                onClose={() => setOpen(false)}
                styles={{ body: { padding: 0 } }}
                extra={
                    <Tooltip title={t("modurelay.account.refresh")}>
                        <Button type="text" shape="circle" aria-label={t("modurelay.account.refresh")} icon={<RefreshCw className={`size-4 ${query.isFetching ? "animate-spin" : ""}`} />} disabled={query.isFetching} onClick={() => void query.refetch()} />
                    </Tooltip>
                }
            >
                <div className="flex h-full min-h-0 flex-col">
                    <div className="min-h-0 flex-1 overflow-y-auto p-5">
                        {query.isPending ? (
                            <div className="space-y-5" aria-live="polite">
                                <Skeleton active avatar paragraph={{ rows: 2 }} />
                                <Skeleton active paragraph={{ rows: 6 }} />
                            </div>
                        ) : query.isError ? (
                            <Alert
                                type="error"
                                showIcon
                                message={t("modurelay.account.loadFailed")}
                                description={query.error instanceof Error ? query.error.message : String(query.error)}
                                action={
                                    <Button type="text" size="small" onClick={() => void query.refetch()}>
                                        {t("common.retry")}
                                    </Button>
                                }
                            />
                        ) : (
                            <>
                                <section className="flex min-w-0 items-center gap-3 border-b border-border pb-5">
                                    <div className="flex size-11 shrink-0 items-center justify-center rounded-lg bg-muted text-base font-semibold text-foreground" aria-hidden="true">
                                        {(query.data.user.username.trim() || query.data.user.email.trim() || "M").slice(0, 1).toUpperCase()}
                                    </div>
                                    <div className="min-w-0 flex-1">
                                        <div className="flex min-w-0 flex-wrap items-center gap-2">
                                            <h2 className="min-w-0 truncate text-base font-semibold text-foreground">{query.data.user.username}</h2>
                                            <Tag className="!m-0">{t(`modurelay.account.roles.${query.data.user.role}`)}</Tag>
                                            <Tag color={query.data.user.status === "active" ? "success" : "error"} className="!m-0">
                                                {t(`modurelay.account.statuses.${query.data.user.status}`)}
                                            </Tag>
                                        </div>
                                        <p className="mt-1 break-all text-sm text-muted-foreground">{query.data.user.email}</p>
                                    </div>
                                </section>

                                <section className="grid grid-cols-2 border-b border-border py-2" aria-label={t("modurelay.account.summary")}>
                                    <Metric icon={<BadgeDollarSign className="size-4" />} label={t("modurelay.account.balance")} value={formatAmount(query.data.user.balance)} />
                                    <Metric icon={<Snowflake className="size-4" />} label={t("modurelay.account.frozenBalance")} value={formatAmount(query.data.user.frozen_balance || 0)} />
                                    <Metric icon={<Gauge className="size-4" />} label={t("modurelay.account.concurrency")} value={String(query.data.user.concurrency)} />
                                    <Metric icon={<Layers3 className="size-4" />} label={t("modurelay.account.availableGroups")} value={String(query.data.groups.length)} />
                                </section>

                                <section className="pt-5">
                                    <div className="mb-3 flex items-center justify-between gap-3">
                                        <h3 className="text-sm font-semibold text-foreground">{t("modurelay.account.availableKeys")}</h3>
                                        <span className="text-xs tabular-nums text-muted-foreground">{query.data.usableKeys.length}</span>
                                    </div>
                                    {query.data.usableKeys.length ? (
                                        <div className="divide-y divide-border border-y border-border">
                                            {query.data.usableKeys.map((key) => {
                                                const group = query.data.groups.find((item) => item.id === key.group_id);
                                                const current = key.key === gatewayKey;
                                                return (
                                                    <article key={key.id} className="py-3">
                                                        <div className="flex min-w-0 items-center gap-2">
                                                            <KeyRound className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
                                                            <span className="min-w-0 flex-1 truncate text-sm font-medium text-foreground">{key.name}</span>
                                                            {current ? (
                                                                <Tag color="success" className="!m-0 shrink-0">
                                                                    {t("modurelay.connection.current")}
                                                                </Tag>
                                                            ) : null}
                                                        </div>
                                                        <code className="mt-1.5 block truncate text-xs text-muted-foreground">{maskModuRelayKey(key.key)}</code>
                                                        <div className="mt-2 flex min-w-0 flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
                                                            <span className="truncate">{group?.name || t("modurelay.connection.unknownGroup")}</span>
                                                            <span>{key.quota > 0 ? t("modurelay.account.quota", { used: formatAmount(key.quota_used), total: formatAmount(key.quota) }) : t("modurelay.connection.unlimitedQuota")}</span>
                                                            <span>{key.expires_at ? t("modurelay.connection.expires", { date: formatDate(key.expires_at) }) : t("modurelay.connection.neverExpires")}</span>
                                                        </div>
                                                    </article>
                                                );
                                            })}
                                        </div>
                                    ) : (
                                        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("modurelay.account.noKeys")} className="py-8" />
                                    )}
                                </section>
                            </>
                        )}
                    </div>

                    <div className="shrink-0 border-t border-border bg-background p-4">
                        <Button block type="primary" icon={<ExternalLink className="size-4" />} onClick={openKeyManagement}>
                            {t("modurelay.account.manageKeys")}
                        </Button>
                    </div>
                </div>
            </Drawer>
        </>
    );
}

function Metric({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
    return (
        <div className="min-w-0 px-2 py-3 first:pl-0 even:pr-0">
            <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                <span aria-hidden="true">{icon}</span>
                <span className="truncate">{label}</span>
            </div>
            <div className="mt-1 truncate text-base font-semibold tabular-nums text-foreground" title={value}>
                {value}
            </div>
        </div>
    );
}
