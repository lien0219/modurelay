import { App } from "antd";
import { useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";

const WARNING_RATIO = 0.8;
const CRITICAL_RATIO = 0.92;
const CHECK_INTERVAL_MS = 60_000;

type PressureLevel = "normal" | "warning" | "critical";

export function StoragePressureMonitor() {
    const { message } = App.useApp();
    const { t } = useTranslation();
    const lastLevelRef = useRef<PressureLevel>("normal");

    useEffect(() => {
        let disposed = false;
        const check = async () => {
            if (!navigator.storage?.estimate) return;
            try {
                const estimate = await navigator.storage.estimate();
                if (disposed || !estimate.usage || !estimate.quota) return;
                const ratio = estimate.usage / estimate.quota;
                const level: PressureLevel = ratio >= CRITICAL_RATIO ? "critical" : ratio >= WARNING_RATIO ? "warning" : "normal";
                if (level === lastLevelRef.current) return;
                lastLevelRef.current = level;
                if (level === "normal") return;
                const percent = Math.round(ratio * 100);
                if (level === "critical") message.error({ key: "infinite-canvas-storage-pressure", content: t("config.localStorage.criticalWarning", { percent }), duration: 8 });
                else message.warning({ key: "infinite-canvas-storage-pressure", content: t("config.localStorage.warning", { percent }), duration: 6 });
            } catch {
                // Advisory only; never block the workspace.
            }
        };
        const onVisibility = () => {
            if (document.visibilityState === "visible") void check();
        };
        void check();
        const timer = window.setInterval(() => void check(), CHECK_INTERVAL_MS);
        document.addEventListener("visibilitychange", onVisibility);
        return () => {
            disposed = true;
            window.clearInterval(timer);
            document.removeEventListener("visibilitychange", onVisibility);
        };
    }, [message, t]);

    return null;
}
