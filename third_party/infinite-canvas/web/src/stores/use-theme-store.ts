import { create } from "zustand";
import { persist } from "zustand/middleware";

export type ThemeName = "light" | "dark";

const MODURELAY_THEME_STORAGE_KEY = "theme";

function modurelayTheme(): ThemeName | null {
    const value = localStorage.getItem(MODURELAY_THEME_STORAGE_KEY);
    return value === "light" || value === "dark" ? value : null;
}

type ThemeStore = {
    theme: ThemeName;
    setTheme: (theme: ThemeName) => void;
};

export const useThemeStore = create<ThemeStore>()(
    persist(
        (set) => ({
            theme: modurelayTheme() || "dark",
            setTheme: (theme) => {
                localStorage.setItem(MODURELAY_THEME_STORAGE_KEY, theme);
                set({ theme });
            },
        }),
        {
            name: "infinite-canvas:theme_store",
            merge: (persisted, current) => ({
                ...current,
                ...(persisted as Partial<ThemeStore>),
                theme: modurelayTheme() || (persisted as Partial<ThemeStore>)?.theme || current.theme,
            }),
        },
    ),
);
