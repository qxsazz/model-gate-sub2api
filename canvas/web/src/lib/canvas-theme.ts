export type CanvasColorTheme = "light" | "dark";
export type CanvasBackgroundMode = "dots" | "lines" | "blank";

export const canvasThemes = {
    light: {
        canvas: {
            background: "#fbfaf7",
            dot: "rgba(68,64,60,.28)",
            line: "rgba(68,64,60,.12)",
            selectionStroke: "#9c8344",
            selectionFill: "rgba(28,25,23,.06)",
        },
        node: {
            label: "#4a4740",
            fill: "#f5f1e7",
            panel: "#ffffff",
            stroke: "#d6cebd",
            activeStroke: "#9c8344",
            placeholder: "#8a8479",
            text: "#18191d",
            muted: "#746f65",
            faint: "#a8a29e",
        },
        toolbar: {
            panel: "rgba(251,250,247,.96)",
            border: "#d6cebd",
            item: "#4a4740",
            itemHover: "#f5f1e7",
            activeBg: "#f5f1e7",
            activeText: "#18191d",
        },
    },
    dark: {
        canvas: {
            background: "#111214",
            dot: "rgba(245,245,244,.24)",
            line: "rgba(245,245,244,.10)",
            selectionStroke: "#c9b477",
            selectionFill: "rgba(250,250,249,.10)",
        },
        node: {
            label: "#d6cebd",
            fill: "#18191d",
            panel: "#18191d",
            stroke: "#38352f",
            activeStroke: "#c9b477",
            placeholder: "#a8a29e",
            text: "#f5f5f4",
            muted: "#d6cebd",
            faint: "#746f65",
        },
        toolbar: {
            panel: "rgba(31,29,26,.96)",
            border: "#38352f",
            item: "#d6cebd",
            itemHover: "#18191d",
            activeBg: "#3a3631",
            activeText: "#f5f5f4",
        },
    },
} as const;

export type CanvasTheme = (typeof canvasThemes)[CanvasColorTheme];
