import axios, { AxiosError, AxiosHeaders, type InternalAxiosRequestConfig } from "axios";

export type ManagedKey = { id: number; name: string; image: boolean; video: boolean };
export type HostedContext = { keys: ManagedKey[]; theme: "light" | "dark"; userId: number };
export let hosted: HostedContext;
const pending = new Map<string, { resolve: (value: any) => void; reject: (reason: Error) => void }>();

export function request<T = any>(path: string, method = "GET", body?: unknown, binary = false): Promise<T> {
    const id = crypto.randomUUID();
    return new Promise((resolve, reject) => {
        pending.set(id, { resolve, reject });
        window.parent.postMessage({ type: "mg-canvas-request", id, path, method, body, binary }, window.location.origin);
    });
}

export function notifyHost(action: string, value?: unknown) {
    window.parent.postMessage({ type: "mg-canvas-action", action, value }, window.location.origin);
}

export async function initializeHosted() {
    if (window.parent === window) throw new Error("请从 MODEL-GATE 的无限画布入口进入");
    window.addEventListener("message", (event) => {
        if (event.origin !== window.location.origin || event.source !== window.parent) return;
        const data = event.data;
        if (data?.type === "mg-canvas-response") {
            const item = pending.get(data.id);
            if (!item) return;
            pending.delete(data.id);
            if (data.error) item.reject(new Error(data.error));
            else item.resolve(data.value);
        }
        if (data?.type === "mg-canvas-refresh") window.dispatchEvent(new Event("mg-key-settings"));
        if (data?.type === "mg-canvas-theme") window.dispatchEvent(new CustomEvent("mg-theme", { detail: data.theme }));
    });
    hosted = await request<HostedContext>("context");
    // Every AI request uses the authenticated host broker. No API credential is passed to this frame.
    axios.defaults.adapter = async (config: InternalAxiosRequestConfig) => {
        const url = new URL(config.url || "", window.location.origin);
        const prefix = "/api/v1/canvas/gateway/";
        if (url.origin !== window.location.origin || !url.pathname.startsWith(prefix)) throw new Error("托管画布仅允许使用本站提供的模型接口");
        const path = url.pathname.slice("/api/v1".length).replace(/(\/gateway\/\d+)\/v1\//, "$1/");
        const isBinary = config.responseType === "blob";
        const result = await request<{ data: any; status: number; headers: Record<string, string> }>(path, (config.method || "GET").toUpperCase(), config.data instanceof FormData ? { form: Array.from(config.data.entries()) } : config.data, isBinary);
        const response = { ...result, config, statusText: String(result.status), headers: AxiosHeaders.from(result.headers) };
        if (result.status >= 400) throw new AxiosError(result.data?.message || result.data?.error?.message || "生成请求失败", String(result.status), config, null, response);
        return response;
    };
    // Generated private image URLs are read through the host rather than anonymous fetch.
    const originalFetch = window.fetch.bind(window);
    window.fetch = async (input, init) => {
        const raw = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
        const url = new URL(raw, window.location.origin);
        if (url.origin === window.location.origin && url.pathname.startsWith("/api/v1/canvas/assets/")) {
            const blob = await request<Blob>(url.pathname.slice("/api/v1".length), "GET", undefined, true);
            return new Response(blob, { status: 200, headers: { "Content-Type": blob.type } });
        }
        return originalFetch(input, init);
    };
}

const revisions = new Map<string, number>();
const writes = new Map<string, Promise<unknown>>();
let blocked = false;
export const cloudStateStorage = {
    async getItem(name: string): Promise<string | null> {
        let value: { value: string | null; revision: number };
        try { value = await request(`/canvas/state/${encodeURIComponent(name)}`); }
        catch (error) { blocked = true; notifyHost("save-error", (error as Error).message); throw error; }
        revisions.set(name, value.revision);
        return value.value;
    },
    async setItem(name: string, value: string) {
        if (blocked) throw new Error("云端保存已暂停，请刷新画布");
        const run = (writes.get(name) || Promise.resolve()).then(async () => {
            const result = await request<{ revision: number }>(`/canvas/state/${encodeURIComponent(name)}`, "PUT", { value, revision: revisions.get(name) || 0 });
            revisions.set(name, result.revision);
            notifyHost("saved");
        }).catch((error) => {
            blocked = true;
            notifyHost("save-error", error.message);
            throw error;
        });
        writes.set(name, run);
        await run;
    },
    async removeItem(name: string) { await cloudStateStorage.setItem(name, JSON.stringify({ state: {}, version: 0 })); },
};

// Media is not persisted in IndexedDB. Object URLs last only for this page session.
export const cloudMediaStorage = {
    async setItem(key: string, blob: Blob) {
        await request(`/canvas/storage/${encodeURIComponent(key)}`, "PUT", blob);
        return blob;
    },
    async getItem<T = Blob>(key: string): Promise<T | null> {
        try { return await request<T>(`/canvas/storage/${encodeURIComponent(key)}`, "GET", undefined, true); }
        catch (error) { notifyHost("media-error", (error as Error).message); return null; }
    },
    // Removing a node does not erase evidence before the seven-day retention deadline.
    async removeItem(_key: string) {},
    async iterate(_callback: (value: any, key: string) => void) {},
};
