import { lazy, Suspense, type ReactNode } from "react";
import { LoaderCircle } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useLocation } from "react-router-dom";

import { useAgentStore } from "@/stores/use-agent-store";

const AppTopNav = lazy(() => import("@/components/layout/app-top-nav").then((module) => ({ default: module.AppTopNav })));
const AgentPanel = lazy(() => import("@/components/agent/agent-panel").then((module) => ({ default: module.AgentPanel })));

export default function UserLayout({ children }: { children: ReactNode }) {
    const { t } = useTranslation();
    const hideHeader = /^\/canvas\/[^/]+/.test(useLocation().pathname);
    const panelMounted = useAgentStore((state) => state.panelMounted);
    const panelWidth = useAgentStore((state) => state.width);

    return (
        <div className="flex h-dvh overflow-hidden bg-background text-foreground">
            <div className="flex min-w-0 flex-1 flex-col overflow-hidden">
                <Suspense fallback={hideHeader ? null : <header className="h-14 shrink-0 border-b border-stone-200 dark:border-stone-800" aria-hidden="true" />}>
                    <AppTopNav />
                </Suspense>
                <div className="min-h-0 flex-1 overflow-hidden">{children}</div>
            </div>
            {panelMounted ? (
                <Suspense
                    fallback={
                        <aside className="flex h-full shrink-0 items-center justify-center border-l bg-background text-muted-foreground" style={{ width: panelWidth }} role="status" aria-live="polite">
                            <LoaderCircle className="size-5 animate-spin" aria-hidden="true" />
                            <span className="sr-only">{t("prompts.loading")}</span>
                        </aside>
                    }
                >
                    <AgentPanel />
                </Suspense>
            ) : null}
        </div>
    );
}
