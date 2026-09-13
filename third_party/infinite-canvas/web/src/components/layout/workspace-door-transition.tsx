import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

type DoorDirection = "to-canvas" | "to-relay";

const ARRIVAL_KEY = "modurelay-workspace-door";

export function WorkspaceDoorTransition() {
    const { t } = useTranslation();
    const rootRef = useRef<HTMLDivElement>(null);
    const leftRef = useRef<HTMLDivElement>(null);
    const rightRef = useRef<HTMLDivElement>(null);
    const [active, setActive] = useState(false);
    const [direction, setDirection] = useState<DoorDirection>("to-canvas");
    const [opening, setOpening] = useState(false);

    useEffect(() => {
        const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
        const animateOpen = (nextDirection: DoorDirection) => {
            setDirection(nextDirection);
            setOpening(false);
            setActive(true);
            requestAnimationFrame(() => {
                const root = rootRef.current;
                const left = leftRef.current;
                const right = rightRef.current;
                if (!root || !left || !right) return;
                root.classList.toggle("is-reduced-motion", reduceMotion);
                setOpening(true);
                window.setTimeout(() => setActive(false), reduceMotion ? 20 : 400);
            });
        };

        const stored = sessionStorage.getItem(ARRIVAL_KEY);
        if (stored === "to-canvas" || stored === "to-relay") {
            sessionStorage.removeItem(ARRIVAL_KEY);
            animateOpen(stored);
        } else {
            animateOpen("to-canvas");
        }

        const handleDoor = (event: Event) => {
            const detail = (event as CustomEvent<{ direction?: DoorDirection; href?: string }>).detail || {};
            if (detail.direction !== "to-relay") return;
            setDirection("to-relay");
            setOpening(true);
            setActive(true);
            requestAnimationFrame(() => {
                const root = rootRef.current;
                const left = leftRef.current;
                const right = rightRef.current;
                if (!root || !left || !right) return;
                root.classList.toggle("is-reduced-motion", reduceMotion);
                setOpening(false);
                window.setTimeout(
                    () => {
                        sessionStorage.setItem(ARRIVAL_KEY, "to-relay");
                        window.location.assign(detail.href || "/dashboard");
                    },
                    reduceMotion ? 20 : 400,
                );
            });
        };

        window.addEventListener("modurelay-workspace-door", handleDoor);
        return () => window.removeEventListener("modurelay-workspace-door", handleDoor);
    }, []);

    if (!active) return null;
    return (
        <div ref={rootRef} className="workspace-door-transition" role="status" aria-live="polite">
            <div ref={leftRef} className={`workspace-door-transition__panel workspace-door-transition__panel--left${opening ? " is-opening" : ""}`} aria-hidden="true" />
            <div ref={rightRef} className={`workspace-door-transition__panel workspace-door-transition__panel--right${opening ? " is-opening" : ""}`} aria-hidden="true" />
            <span className="workspace-door-transition__label">{t(direction === "to-relay" ? "modurelay.returning" : "modurelay.entering")}</span>
        </div>
    );
}
