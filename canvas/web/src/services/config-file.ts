import { saveAs } from "file-saver";

import i18n from "@/i18n";
import { useConfigStore, type AiConfig, type WebdavSyncConfig } from "@/stores/use-config-store";
import { usePromptSourceStore, type PromptSourceSchedule } from "@/stores/use-prompt-source-store";
import type { PromptSource } from "@/services/api/prompt-source-presets";

type AppConfigFile = {
    app: "infinite-canvas";
    version: 1;
    exportedAt: string;
    config: AiConfig;
    webdav: WebdavSyncConfig;
    promptSources: {
        sources: PromptSource[];
        schedule: PromptSourceSchedule;
    };
};

export function exportAppConfig() {
    const { config, webdav } = useConfigStore.getState();
    const { sources, schedule } = usePromptSourceStore.getState();
    const data: AppConfigFile = { app: "infinite-canvas", version: 1, exportedAt: new Date().toISOString(), config: { ...config, baseUrl: "", apiKey: "", proxyEnabled: false, channels: config.channels.map((channel) => ({ ...channel, baseUrl: "", apiKey: "", models: channel.models.map(({ name, capability }) => ({ name, capability })) })) }, webdav: { ...webdav, password: "" }, promptSources: { sources, schedule } };
    saveAs(new Blob([JSON.stringify(data, null, 2)], { type: "application/json;charset=utf-8" }), "infinite-canvas-config.json");
}

export async function importAppConfig(file: File) {
    let data: AppConfigFile;
    try {
        data = JSON.parse(await file.text()) as AppConfigFile;
    } catch {
        throw new Error(i18n.t("config.invalidFile"));
    }
    if (data.app !== "infinite-canvas" || data.version !== 1 || !data.config || !data.webdav || !data.promptSources) throw new Error(i18n.t("config.invalidFile"));
    const current = useConfigStore.getState().config;
    const incoming = Object.fromEntries(Object.entries(data.config).filter(([key, value]) => key in current && typeof value === typeof current[key as keyof AiConfig]));
    for (const key of ["model", "imageModel", "videoModel", "textModel", "audioModel"]) { if (typeof incoming[key] === "string" && !current.models.includes(incoming[key] as string)) delete incoming[key]; }
    useConfigStore.setState({ config: { ...current, ...incoming, baseUrl: "", apiKey: "", channels: current.channels, models: current.models, proxyEnabled: false, channelMode: "local" }, webdav: data.webdav });
    usePromptSourceStore.setState(data.promptSources);
}
