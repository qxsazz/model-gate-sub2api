import { create } from "zustand";
import { hosted } from "@/lib/model-gate";
import { persist } from "zustand/middleware";

export type ThemeName = "light" | "dark";

type ThemeStore = {
    theme: ThemeName;
    setTheme: (theme: ThemeName) => void;
};

export const useThemeStore = create<ThemeStore>()(
    persist(
        (set) => ({
            theme: hosted.theme,
            setTheme: (theme) => set({ theme }),
        }),
        { name: "infinite-canvas:theme_store", storage: { getItem: () => null, setItem: () => {}, removeItem: () => {} } },
    ),
);

window.addEventListener("mg-theme", (event) => useThemeStore.getState().setTheme((event as CustomEvent).detail));
