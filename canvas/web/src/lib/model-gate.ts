import axios, { AxiosError, AxiosHeaders, type InternalAxiosRequestConfig } from "axios";

export type ManagedKey = { id: number; name: string; image: boolean; video: boolean; text?: boolean; audio?: boolean; formats?: Array<"openai" | "gemini"> };
export type HostedContext = { keys: ManagedKey[]; theme: "light" | "dark"; userId: number };
export let hosted: HostedContext;
const pending = new Map<string, { resolve: (value: any) => void; reject: (reason: Error) => void }>();

export function request<T = any>(path: string, method = "GET", body?: unknown, binary = false, taskMeta?: string): Promise<T> {
    const id = crypto.randomUUID();
    return new Promise((resolve, reject) => {
        pending.set(id, { resolve, reject });
        window.parent.postMessage({ type: "mg-canvas-request", id, path, method, body, binary, taskMeta }, window.location.origin);
    });
}

let taskCache: { tasks: unknown[] } | undefined;
let taskCacheUntil = 0;
let taskCacheRequest: Promise<{ tasks: unknown[] }> | undefined;
// All active slots and the task drawer share one poll, independent of image count.
export async function listHostedImageTasks<T>(): Promise<{ tasks: T[] }> {
    if (taskCache && Date.now() < taskCacheUntil) return taskCache as {tasks:T[]};
    taskCacheRequest ||= request<{tasks:unknown[]}>("/canvas/tasks").then(value=>{
        taskCache=value;taskCacheUntil=Date.now()+2500;return value;
    }).finally(()=>{taskCacheRequest=undefined});
    return await taskCacheRequest as {tasks:T[]};
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
        const path = url.pathname.slice("/api/v1".length).replace(/(\/gateway\/\d+)\/v1\//, "$1/").replace(/(\/gateway\/\d+\/gemini)\/v1beta\//, "$1/");
        const isBinary = config.responseType === "blob";
        let result: { data: any; status: number; headers: Record<string, string> };
        const imageTask = path.match(/^\/canvas\/gateway\/(\d+)(\/(?:images\/(?:generations|edits)|gemini\/models\/[A-Za-z0-9._-]+:generateContent))$/);
        const body = config.data instanceof FormData ? { form: Array.from(config.data.entries()) } : config.data;
        if (imageTask && config.method?.toUpperCase() === "POST" && (imageTask[2].startsWith("/images/") || config.headers.has("X-Canvas-Task-Meta") || typeof config.data === "string" && config.data.includes('"IMAGE"'))) {
            const meta = String(config.headers.get("X-Canvas-Task-Meta") || encodeURIComponent(JSON.stringify({ source: "image-workbench" })));
            const context = JSON.parse(decodeURIComponent(meta));
            const id = context.id || crypto.randomUUID();
            await request(`/canvas/tasks/${id}/${imageTask[1]}${imageTask[2]}`, "PUT", body, false, meta);
            taskCacheUntil=0;
            window.dispatchEvent(new Event("mg-tasks-refresh"));
            for (;;) {
                if (config.signal?.aborted) { await request(`/canvas/tasks/${id}/cancel`,"POST"); throw new DOMException("已请求停止任务，可在任务列表中查看状态", "AbortError"); }
                let list: {tasks:Array<{id:string;status:string;response_status:number;result:any;error:string}>};
                try {list=await listHostedImageTasks();} catch {await new Promise(resolve=>setTimeout(resolve,5000));continue;}
                const task=list.tasks.find(t=>t.id===id) || await request<{id:string;status:string;response_status:number;result:any;error:string}>(`/canvas/tasks/${id}`);
                if (task.status !== "queued" && task.status !== "running") {
                    result = { data: task.result || { message: task.error }, status: task.response_status || 503, headers: { "content-type": "application/json" } };
                    if (task.status !== "completed" && result.status < 400) result.status = 503;
                    break;
                }
                await new Promise((resolve) => setTimeout(resolve, 1000));
            }
        } else result = await request(path, (config.method || "GET").toUpperCase(), body, isBinary);
        const response = { ...result, config, statusText: String(result.status), headers: AxiosHeaders.from(result.headers) };
        if (result.status >= 400) throw new AxiosError(result.data?.message || result.data?.error?.message || "生成请求失败", String(result.status), config, null, response);
        return response;
    };
    // Generated private image URLs are read through the host rather than anonymous fetch.
    const originalFetch = window.fetch.bind(window);
    window.fetch = async (input, init) => {
        const raw = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
        const url = new URL(raw, window.location.origin);
        if (url.origin === window.location.origin && url.pathname.startsWith("/api/v1/canvas/gateway/")) {
            const path = url.pathname.slice("/api/v1".length).replace(/(\/gateway\/\d+)\/v1\//, "$1/").replace(/(\/gateway\/\d+\/gemini)\/v1beta\//, "$1/");
            const result = await request<{ data: unknown; status: number; headers: Record<string, string> }>(path, init?.method || "GET", init?.body);
            return new Response(typeof result.data === "string" ? result.data : JSON.stringify(result.data), { status: result.status, headers: result.headers });
        }
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
    async flush() { await Promise.all(writes.values()); },
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

// Histories contain metadata and private storage references; media stays in the seven-day archive.
// Serialize changes per history so simultaneous completions cannot overwrite one another.
type CloudLogStore = { setItem<T>(id: string, value: T): Promise<void>; removeItem(id: string): Promise<void>; clear(): Promise<void>; iterate<T, U>(callback: (value: T, key: string) => U): Promise<void> };
const logStores = new Map<string, CloudLogStore>();
export function createCloudLogStore(namespace: string): CloudLogStore {
    const name = `infinite-canvas:${namespace}`;
    const existing = logStores.get(namespace);
    if (existing) return existing;
    let queue: Promise<unknown> = Promise.resolve();
    let records: Record<string, unknown> | undefined;
    let loading: Promise<Record<string, unknown>> | undefined;
    async function load() {
        if (records) return records;
        loading ||= cloudStateStorage.getItem(name).then((value) => { records = JSON.parse(value || "{}"); return records!; });
        return loading;
    }
    function change(update: (values: Record<string, unknown>) => void) {
        const operation = queue.then(async () => {
            const values = await load();
            update(values);
            await cloudStateStorage.setItem(name, JSON.stringify(values));
        });
        queue = operation;
        return operation;
    }
    const store: CloudLogStore = {
        setItem<T>(id: string, value: T) { return change((values) => { values[id] = value; }); },
        removeItem(id: string) { return change((values) => { delete values[id]; }); },
        clear() { return change((values) => { for (const id of Object.keys(values)) delete values[id]; }); },
        async iterate<T, U>(callback: (value: T, key: string) => U) {
            await queue;
            for (const [key, value] of Object.entries(await load())) callback(value as T, key);
        },
    };
    logStores.set(namespace, store);
    return store;
}
