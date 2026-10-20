import { listHostedImageTasks, createCloudLogStore } from "./model-gate";
import { uploadImage } from "@/services/image-storage";
import { useCanvasStore, flushCanvasStore } from "@/stores/canvas/use-canvas-store";
import type { CanvasNodeData, CanvasNodeImage } from "@/types/canvas";

export type TaskImage = { id: string; dataUrl: string; storageKey?: string; durationMs: number; width: number; height: number; bytes: number; mimeType?: string };
export type CanvasTask = {
    id: string;
    deleted_at?: string;
    key_id: number;
    path: string;
    status: string;
    metadata: {
        source?: string;
        projectId?: string;
        nodeId?: string;
        imageId?: string;
        batchId?: string;
        batchSize?: number;
        slot?: number;
        model?: string;
        prompt?: string;
        config?: { model?: string; imageModel?: string; quality?: string; size?: string; count?: string };
    };
    response_status: number;
    result: unknown;
    error: string;
    created_at: string;
    updated_at: string;
};
let tasks: CanvasTask[] = [];
const images = new Map<string, TaskImage[]>();
const mediaErrors = new Map<string, string>();
const receipts = createCloudLogStore("task_receipts");
const logs = createCloudLogStore("image_generation_logs");
let receiptIds: Set<string> | undefined;
let loading = false;
export const getCanvasTasks = () => tasks;
export const getTaskImages = (id: string) => images.get(id) || [];
export const getTaskMediaError = (id: string) => mediaErrors.get(id);
export const taskPending = (t: CanvasTask) => t.status === "queued" || t.status === "running";
export function taskError(t: CanvasTask) {
    const r = t.result as { message?: string; error?: { message?: string } } | null;
    return mediaErrors.get(t.id) || r?.error?.message || r?.message || t.error || "生图失败";
}
function imageURLs(value: unknown): string[] {
    const found = new Set<string>();
    function visit(v: unknown) {
        if (typeof v === "string" && /^\/api\/v1\/canvas\/assets\/[a-f0-9-]+\/content$/.test(v)) found.add(v);
        else if (Array.isArray(v)) v.forEach(visit);
        else if (v && typeof v === "object") Object.values(v).forEach(visit);
    }
    visit(value);
    return [...found];
}
export function applyCanvasTasks(nodes: CanvasNodeData[], projectId: string): CanvasNodeData[] {
    let changed = false;
    const next = nodes.map((node) => {
        let current = node;
        for (const task of tasks) {
            const m = task.metadata,
                key = m.imageId || "primary";
            if (m.source !== "canvas" || m.projectId !== projectId || m.nodeId !== node.id || node.metadata?.generationTaskIds?.[key] !== task.id) continue;
            const image = getTaskImages(task.id)[0];
            const status = taskPending(task) || (task.status === "completed" && !image && !mediaErrors.has(task.id)) ? "loading" : image ? "success" : "error";
            const metadata = { ...current.metadata };
            if (m.imageId && metadata.images?.length) {
                metadata.images = metadata.images.map((item) =>
                    item.id === m.imageId
                        ? ({
                              ...item,
                              status,
                              errorDetails: status === "error" ? taskError(task) : undefined,
                              ...(image
                                  ? {
                                        content: image.dataUrl,
                                        storageKey: image.storageKey,
                                        naturalWidth: image.width,
                                        naturalHeight: image.height,
                                        bytes: image.bytes,
                                        mimeType: image.mimeType || "image/png",
                                    }
                                  : {}),
                          } as CanvasNodeImage)
                        : item,
                );
                const primary = metadata.images.find((item) => item.id === metadata.primaryImageId) || metadata.images.find((item) => item.status === "success");
                metadata.status = metadata.images.some((item) => item.status === "loading")
                    ? "loading"
                    : metadata.images.some((item) => item.status === "success")
                      ? "success"
                      : "error";
                if (primary?.content)
                    Object.assign(metadata, {
                        content: primary.content,
                        storageKey: primary.storageKey,
                        naturalWidth: primary.naturalWidth,
                        naturalHeight: primary.naturalHeight,
                        bytes: primary.bytes,
                        mimeType: primary.mimeType,
                    });
            } else
                Object.assign(metadata, {
                    status,
                    errorDetails: status === "error" ? taskError(task) : undefined,
                    ...(image
                        ? {
                              content: image.dataUrl,
                              storageKey: image.storageKey,
                              naturalWidth: image.width,
                              naturalHeight: image.height,
                              bytes: image.bytes,
                              mimeType: image.mimeType,
                          }
                        : {}),
                });
            if (status !== "loading") {
                metadata.generationTaskIds = { ...metadata.generationTaskIds };
                delete metadata.generationTaskIds[key];
            }
            if (JSON.stringify(current.metadata) !== JSON.stringify(metadata)) {
                current = { ...current, metadata };
                changed = true;
            }
        }
        return current;
    });
    return changed ? next : nodes;
}
export async function loadTaskImages(t: CanvasTask) {
    if (images.has(t.id)) return;
    const values = await Promise.all(
        imageURLs(t.result).map(async (url, index) => {
            const stored = await uploadImage(url);
            return {
                id: `${t.id}-${index}`,
                dataUrl: stored.url,
                storageKey: stored.storageKey,
                width: stored.width,
                height: stored.height,
                bytes: stored.bytes,
                mimeType: stored.mimeType,
                durationMs: Date.parse(t.updated_at) - Date.parse(t.created_at),
            };
        }),
    );
    images.set(t.id, values);
    if (!values.length) mediaErrors.set(t.id, "任务未返回图片");
}
export async function refreshCanvasTasks() {
    if (loading) return;
    loading = true;
    try {
        const value = await listHostedImageTasks<CanvasTask>();
        tasks = value.tasks;
        receiptIds ||= new Set<string>();
        await receipts.iterate((_value, id) => receiptIds!.add(id));
        const latestWorkbench = tasks.find((t) => t.metadata.source !== "canvas");
        const latestBatch = latestWorkbench?.metadata.batchId || latestWorkbench?.id;
        for (const t of tasks) {
            const project = useCanvasStore.getState().projects.find((p) => p.id === t.metadata.projectId);
            const referenced = project?.nodes.some((n) => n.metadata?.generationTaskIds?.[t.metadata.imageId || "primary"] === t.id);
            const id = t.metadata.batchId || t.id;
            if ((t.metadata.source === "canvas" && !referenced) || (t.metadata.source !== "canvas" && receiptIds.has(id) && id !== latestBatch)) continue;
            if (t.status !== "completed" || images.has(t.id) || mediaErrors.has(t.id)) continue;
            try {
                await loadTaskImages(t);
            } catch (error) {
                if (/410|403|404|已删除|过期|屏蔽/.test(String(error))) mediaErrors.set(t.id, "素材已删除、过期或被屏蔽");
            }
        }
        const groups = new Map<string, CanvasTask[]>();
        for (const t of tasks.filter((t) => t.metadata.source !== "canvas")) {
            const id = t.metadata.batchId || t.id;
            groups.set(id, [...(groups.get(id) || []), t]);
        }
        for (const [id, group] of groups) {
            if (
                receiptIds.has(id) ||
                group.some(taskPending) ||
                group.length < (group[0].metadata.batchSize || 1) ||
                group.some((t) => t.status === "completed" && !images.has(t.id) && !mediaErrors.has(t.id))
            )
                continue;
            const meta = group[0].metadata;
            const output = group.flatMap((t) => getTaskImages(t.id));
            const count = meta.batchSize || group.length;
            const created = Date.parse(group.map((t) => t.created_at).sort()[0]);
            const model = meta.model || "";
            const prompt = meta.prompt || "";
            const config = { model, imageModel: model, quality: "auto", size: "auto", ...meta.config, count: String(count) };
            await logs.setItem(id, {
                id,
                createdAt: created,
                title: prompt.slice(0, 12),
                prompt,
                time: new Date(created).toLocaleString(),
                model,
                config,
                references: [],
                durationMs: Math.max(...group.map(task=>Date.parse(task.updated_at))) - created,
                successCount: output.length,
                failCount: count - output.length,
                imageCount: count,
                size: config.size,
                quality: config.quality,
                status: output.length ? "success" : "failed",
                images: output.map((image) => ({ ...image, dataUrl: "" })),
            });
            await receipts.setItem(id, true);
            receiptIds.add(id);
        }
        const state = useCanvasStore.getState();
        if (state.hydrated)
            for (const project of state.projects) {
                if (location.pathname === `/canvas-app/canvas/${project.id}`) continue;
                const nodes = applyCanvasTasks(project.nodes, project.id);
                if (nodes !== project.nodes) state.updateProject(project.id, { nodes });
            }
        await flushCanvasStore();
        window.dispatchEvent(new Event("mg-tasks-updated"));
    } finally {
        loading = false;
    }
}
