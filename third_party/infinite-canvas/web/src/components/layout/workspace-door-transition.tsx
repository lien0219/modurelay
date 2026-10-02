import { useEffect, useRef, useState } from "react";

type DoorDirection = "to-canvas" | "to-relay";

const ARRIVAL_KEY = "modurelay-workspace-door";
const DOCUMENT_DEPARTURE_KEY = "modurelay-workspace-document-departure";
const OPEN_DURATION_MS = 460;
const CLOSED_PAINT_FRAMES = 2;

export function WorkspaceDoorTransition() {
    const timerRef = useRef<number | null>(null);
    const frameRef = useRef<number | null>(null);
    const arrivalPendingRef = useRef(false);
    const [active, setActive] = useState(false);
    const [opening, setOpening] = useState(false);

    useEffect(() => {
        const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
        const clearScheduledWork = () => {
            if (timerRef.current !== null) window.clearTimeout(timerRef.current);
            if (frameRef.current !== null) window.cancelAnimationFrame(frameRef.current);
            timerRef.current = null;
            frameRef.current = null;
        };
        const schedule = (callback: () => void) => {
            if (timerRef.current !== null) window.clearTimeout(timerRef.current);
            timerRef.current = window.setTimeout(callback, reduceMotion ? 20 : OPEN_DURATION_MS);
        };
        const animateOpen = () => {
            clearScheduledWork();
            setOpening(false);
            setActive(true);
            let frames = reduceMotion ? 1 : CLOSED_PAINT_FRAMES;
            const waitForClosedPaint = () => {
                frameRef.current = window.requestAnimationFrame(() => {
                    frameRef.current = null;
                    frames -= 1;
                    if (frames > 0) {
                        waitForClosedPaint();
                        return;
                    }
                    setOpening(true);
                    schedule(() => {
                        timerRef.current = null;
                        arrivalPendingRef.current = false;
                        setActive(false);
                    });
                });
            };
            waitForClosedPaint();
        };

        const arrivedFromRelay = () => {
            const fromRelay = sessionStorage.getItem(DOCUMENT_DEPARTURE_KEY) === "relay";
            sessionStorage.removeItem(DOCUMENT_DEPARTURE_KEY);
            return fromRelay;
        };
        const handlePageHide = () => {
            sessionStorage.setItem(DOCUMENT_DEPARTURE_KEY, "canvas");
        };

        const stored = sessionStorage.getItem(ARRIVAL_KEY);
        const fromRelay = arrivedFromRelay();
        if (stored === "to-canvas" || stored === "to-relay" || fromRelay || arrivalPendingRef.current) {
            sessionStorage.removeItem(ARRIVAL_KEY);
            arrivalPendingRef.current = true;
            animateOpen();
        }

        const handleDoor = (event: Event) => {
            const detail = (event as CustomEvent<{ direction?: DoorDirection; href?: string; replace?: boolean }>).detail || {};
            if (detail.direction !== "to-relay") return;
            clearScheduledWork();
            setOpening(true);
            setActive(true);
            frameRef.current = window.requestAnimationFrame(() => {
                frameRef.current = null;
                setOpening(false);
                schedule(() => {
                    timerRef.current = null;
                    sessionStorage.setItem(ARRIVAL_KEY, "to-relay");
                    const href = detail.href || "/dashboard";
                    if (detail.replace) window.location.replace(href);
                    else window.location.assign(href);
                });
            });
        };

        const handlePageShow = (event: PageTransitionEvent) => {
            if (!event.persisted) return;
            if (arrivedFromRelay()) {
                animateOpen();
                return;
            }
            clearScheduledWork();
            sessionStorage.removeItem(ARRIVAL_KEY);
            setOpening(false);
            setActive(false);
        };

        window.addEventListener("modurelay-workspace-door", handleDoor);
        window.addEventListener("pagehide", handlePageHide);
        window.addEventListener("pageshow", handlePageShow);
        return () => {
            clearScheduledWork();
            window.removeEventListener("modurelay-workspace-door", handleDoor);
            window.removeEventListener("pagehide", handlePageHide);
            window.removeEventListener("pageshow", handlePageShow);
        };
    }, []);

    if (!active) return null;
    return (
        <div className="workspace-door-transition" aria-hidden="true">
            <div className={`workspace-door-transition__panel workspace-door-transition__panel--left${opening ? " is-opening" : ""}`} />
            <div className={`workspace-door-transition__panel workspace-door-transition__panel--right${opening ? " is-opening" : ""}`} />
        </div>
    );
}
