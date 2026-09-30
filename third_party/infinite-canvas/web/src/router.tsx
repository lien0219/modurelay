import { lazy, Suspense } from "react";
import { createBrowserRouter, Outlet } from "react-router-dom";
import { LoaderCircle } from "lucide-react";

import { AnalyticsTracker } from "@/components/layout/analytics-tracker";
import UserLayout from "@/layouts/user-layout";
import NotFound from "@/pages/not-found";
import i18n from "@/i18n";

const AssetsPage = lazy(() => import("@/pages/assets"));
const CanvasPage = lazy(() => import("@/pages/canvas"));
const CanvasProjectPage = lazy(() => import("@/pages/canvas/project"));
const ConfigPage = lazy(() => import("@/pages/config"));
const HomePage = lazy(() => import("@/pages/home"));
const ImagePage = lazy(() => import("@/pages/image"));
const PromptsPage = lazy(() => import("@/pages/prompts"));
const VideoPage = lazy(() => import("@/pages/video"));

export const router = createBrowserRouter(
    [
        {
            element: (
                <UserLayout>
                    <AnalyticsTracker />
                    <Suspense
                        fallback={
                            <main className="flex h-full items-center justify-center bg-background text-muted-foreground" role="status" aria-live="polite">
                                <LoaderCircle className="size-5 animate-spin" aria-hidden="true" />
                                <span className="sr-only">{i18n.t("prompts.loading")}</span>
                            </main>
                        }
                    >
                        <Outlet />
                    </Suspense>
                </UserLayout>
            ),
            children: [
                { path: "/", element: <HomePage /> },
                { path: "/image", element: <ImagePage /> },
                { path: "/video", element: <VideoPage /> },
                { path: "/assets", element: <AssetsPage /> },
                { path: "/prompts", element: <PromptsPage /> },
                { path: "/canvas", element: <CanvasPage /> },
                { path: "/canvas/:id", element: <CanvasProjectPage /> },
                { path: "/config", element: <ConfigPage /> },
            ],
        },
        { path: "*", element: <NotFound /> },
    ],
    { basename: import.meta.env.BASE_URL },
);
