import { expect, test, mock } from "bun:test";

let currentTasks: any[] = [];
const stores = new Map<string, Map<string, any>>();
const store = (name: string) => {
    if (!stores.has(name)) stores.set(name, new Map());
    return stores.get(name)!;
};
let mediaFails = false;
const events = new EventTarget();
Object.defineProperty(globalThis, "window", { value: events, configurable: true });
mock.module("@/lib/model-gate", () => ({
    listHostedImageTasks: async () => ({ tasks: currentTasks }),
    createCloudLogStore: (name: string) => ({
        setItem: async (id: string, value: unknown) => { store(name).set(id, value); },
        iterate: async (callback: (value: unknown, id: string) => void) => { store(name).forEach(callback); },
    }),
}));
mock.module("@/services/image-storage", () => ({ uploadImage: async () => {
    if (mediaFails) throw new Error("temporary media download failure");
    return { url: "blob:preview", storageKey: "image:stored", width: 10, height: 10, bytes: 100, mimeType: "image/png" };
} }));
mock.module("@/stores/canvas/use-canvas-store", () => ({
    useCanvasStore: { getState: () => ({ projects: [], hydrated: false }) }, flushCanvasStore: async () => {},
}));
const { refreshCanvasTasks, retryTaskImages, saveWorkbenchLog } = await import("../src/lib/canvas-tasks");
function task(id: string, status = "completed", batchId = id, batchSize = 1) {
    return { id, status, metadata: { source: "image-workbench", batchId, batchSize, prompt: "landscape", model: "gpt-image-2.5" },
        created_at: "2026-10-24T03:40:19Z", updated_at: "2026-10-24T03:41:35Z", result: { data: [{ url: "/api/v1/canvas/assets/abcd-1234/content" }] } };
}
test("completed generation remains successful when reading its image fails", async () => {
    mediaFails = true;
    currentTasks = [task("download-failed")];
    await refreshCanvasTasks();
    expect(store("image_generation_logs").get("download-failed")).toMatchObject({ status: "success", successCount: 1, failCount: 0, images: [] });
});
test("reloading a completed image updates an already receipted history without resubmitting generation", async () => {
    mediaFails = true;
    currentTasks = [task("reload")];
    await refreshCanvasTasks();
    mediaFails = false;
    await retryTaskImages(currentTasks[0]);
    await refreshCanvasTasks();
    expect(store("image_generation_logs").get("reload").images).toHaveLength(1);
});
test("old false failure receipts are repaired while deleted histories stay deleted", async () => {
    mediaFails = false;
    currentTasks = [task("old-false-failure"), task("deleted")];
    store("task_receipts").set("old-false-failure", true);
    store("task_receipts").set("deleted", true);
    store("image_generation_logs").set("old-false-failure", { status: "failed", successCount: 0, failCount: 1, images: [], references: [{ storageKey: "image:reference" }] });
    await refreshCanvasTasks();
    expect(store("image_generation_logs").get("old-false-failure")).toMatchObject({ status: "success", successCount: 1, failCount: 0, references: [{ storageKey: "image:reference" }] });
    expect(store("image_generation_logs").has("deleted")).toBe(false);
});
test("mixed outcomes count server failures and wait for genuinely pending slots", async () => {
    mediaFails = true;
    currentTasks = [task("mixed-ok", "completed", "mixed", 2), task("mixed-fail", "failed", "mixed", 2)];
    await refreshCanvasTasks();
    expect(store("image_generation_logs").get("mixed")).toMatchObject({ successCount: 1, failCount: 1, status: "success" });
    currentTasks = [task("pending-ok", "completed", "pending", 2), task("pending-running", "running", "pending", 2)];
    await refreshCanvasTasks();
    expect(store("image_generation_logs").has("pending")).toBe(false);
});
test("a late media failure callback cannot overwrite server success; plugin logs still save", async () => {
    mediaFails = false;
    currentTasks = [task("callback-race")];
    await refreshCanvasTasks();
    await saveWorkbenchLog({ id: "callback-race", status: "failed", successCount: 0, failCount: 1, images: [] });
    expect(store("image_generation_logs").get("callback-race")).toMatchObject({ status: "success", successCount: 1, failCount: 0 });
    await saveWorkbenchLog({ id: "plugin", status: "success", successCount: 1 });
    expect(store("image_generation_logs").get("plugin").status).toBe("success");
});
