import { test, expect, mock, beforeEach } from "bun:test";

let imageAllowed = true;
let fetchedNames = ["gemini-3.1-flash-lite-image", "gemini-3.1-flash-image-preview", "gemini-nano-banana-2.1", "gemini-3-pro-image-preview"];
Object.defineProperty(globalThis, "location", { value: { origin: "https://model-gate.test" }, configurable: true });
mock.module("@/i18n", () => ({ default: { t: (key: string) => key } }));
mock.module("@/lib/model-gate", () => ({
    hosted: { keys: [] },
    cloudStateStorage: { getItem: async () => null, setItem: async () => {}, removeItem: async () => {} },
    request: async (path: string) =>
        path === "/canvas/capabilities" ? { keys: [{ id: 479, name: "Gemini image", image: imageAllowed, video: false, text: true, formats: ["gemini"] }] } : { status: 200, data: { models: fetchedNames.map((name) => ({ name: "models/" + name })) } },
}));
const { useConfigStore, defaultConfig } = await import("../src/stores/use-config-store.ts");
const { refreshManagedModels } = await import("../src/lib/managed-models.ts");
await useConfigStore.persist.rehydrate();
beforeEach(() => {
    imageAllowed = true;
    fetchedNames = ["gemini-3.1-flash-lite-image", "gemini-3.1-flash-image-preview", "gemini-nano-banana-2.1", "gemini-3-pro-image-preview"];
    useConfigStore.setState({ config: { ...defaultConfig, channels: [], imageModel: "", models: [] } });
});
test("new canvas discovers all four image models from normalized Google catalogue", async () => {
    expect(await refreshManagedModels()).toEqual([]);
    const channel = useConfigStore.getState().config.channels[0];
    expect(channel.models.map((model) => model.name)).toEqual(fetchedNames);
    expect(channel.models.every((model) => model.capability === "image")).toBe(true);
    expect(useConfigStore.getState().config.imageModel).toBe("479-gemini::" + fetchedNames[0]);
});
test("explicitly saved model selections and overrides remain intact", async () => {
    useConfigStore.setState({
        config: {
            ...defaultConfig,
            channels: [{ id: "479-gemini", name: "saved", baseUrl: "", apiKey: "", apiFormat: "gemini", modelsConfigured: true, models: [{ name: "gemini-nano-banana-2.1", capability: "image" }] }],
            imageModel: "479-gemini::gemini-nano-banana-2.1",
        },
    });
    expect(await refreshManagedModels()).toEqual([]);
    expect(useConfigStore.getState().config.channels[0].models).toEqual([{ name: "gemini-nano-banana-2.1", capability: "image" }]);
    expect(useConfigStore.getState().config.imageModel).toBe("479-gemini::gemini-nano-banana-2.1");
});
test("image-disabled groups do not gain usable image models", async () => {
    imageAllowed = false;
    expect(await refreshManagedModels()).toEqual([]);
    expect(useConfigStore.getState().config.channels[0].models).toEqual([]);
    expect(useConfigStore.getState().config.imageModel).toBe("");
});
