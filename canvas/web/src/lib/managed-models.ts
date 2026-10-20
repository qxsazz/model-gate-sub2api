import { hosted, request } from "@/lib/model-gate";
import { useConfigStore, guessCapability, modelOptionsFromChannels, type ModelChannel, type ModelCapability } from "@/stores/use-config-store";

export async function refreshManagedModels() {
    const { keys } = await request<{ keys: typeof hosted.keys }>("/canvas/capabilities");
    const previous = useConfigStore.getState().config;
    const channels: ModelChannel[] = [];
    const failures: string[] = [];
    await Promise.all(keys.map(async (key) => {
        for (const format of key.formats || ["openai"]) {
            try {
                const native = format === "gemini";
                const result = await request<{ data: { data?: Array<{ id: string }>; models?: Array<{ name: string; supportedGenerationMethods?: string[] }> }; status: number }>(`/canvas/gateway/${key.id}/${native ? "gemini/" : ""}models`);
                if (result.status >= 400) throw new Error(`${key.name} · ${format}: 无法获取模型`);
                const names = native ? (result.data.models || []).filter((m) => !m.supportedGenerationMethods?.length || m.supportedGenerationMethods.includes("generateContent")).map((m) => m.name.replace(/^models\//, "")) : (result.data.data || []).map((m) => m.id);
                const id = native ? `${key.id}-gemini` : String(key.id);
                const saved = previous.channels.find((channel) => channel.id === id);
                const selected = saved?.modelsConfigured ? saved.models.map((m) => m.name) : names;
                const models = selected.map((name) => ({ name, capability: saved?.models.find((model) => model.name === name)?.capability || guessCapability(name) })).filter((model) => (model.capability === "image" && key.image) || (model.capability === "video" && key.video) || (model.capability === "audio" && key.audio) || (model.capability === "text" && key.text !== false));
                channels.push({ id, name: `${key.name} · ${native ? "Google 原生" : "OpenAI"}`, baseUrl: `${location.origin}/api/v1/canvas/gateway/${key.id}/${native ? "gemini/v1beta" : "v1"}`, apiKey: "managed-by-model-gate", apiFormat: format, modelsConfigured: saved?.modelsConfigured, models });
            } catch (error) {
                failures.push((error as Error).message);
                const native = format === "gemini";
                const id = native ? `${key.id}-gemini` : String(key.id);
                const saved = previous.channels.find((channel) => channel.id === id);
                channels.push({ id, name: `${key.name} · ${native ? "Google 原生" : "OpenAI"}`, baseUrl: `${location.origin}/api/v1/canvas/gateway/${key.id}/${native ? "gemini/v1beta" : "v1"}`, apiKey: "managed-by-model-gate", apiFormat: format, modelsConfigured: saved?.modelsConfigured, models: saved?.models || [] });
            }
        }
    }));
    channels.sort((a, b) => a.id.localeCompare(b.id, undefined, { numeric: true }));
    const options = modelOptionsFromChannels(channels);
    const choose = (kind: ModelCapability, selected: string) => channels.some((c) => c.models.some((m) => m.capability === kind && `${c.id}::${m.name}` === selected)) ? selected : channels.flatMap((c) => c.models.filter((m) => m.capability === kind).map((m) => `${c.id}::${m.name}`))[0] || "";
    const imageModel = choose("image", previous.imageModel);
    useConfigStore.setState({ config: { ...previous, channels, models: options, imageModel, videoModel: choose("video", previous.videoModel), textModel: choose("text", previous.textModel), audioModel: choose("audio", previous.audioModel), model: imageModel, proxyEnabled: false, channelMode: "local" } });
    return failures;
}
