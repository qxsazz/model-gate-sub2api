// Server task state is authoritative. Loading a completed image is not generation.
export type RecoveryTask = {
    id: string;
    status: string;
    created_at: string;
    metadata: { batchSize?: number; slot?: number };
};
export const IMAGE_BATCH_WAIT_MS = 6 * 60_000;
export function recoveredImagePhase(task: RecoveryTask, hasImage: boolean) {
    if (hasImage) return "success" as const;
    if (task.status === "queued" || task.status === "running") return "pending" as const;
    return task.status === "completed" ? "unavailable" as const : "failed" as const;
}
export function recoveredImageSlots<T extends RecoveryTask>(group: T[], now = Date.now()) {
    const startedAt = Math.min(...group.map(task => Date.parse(task.created_at)));
    const requested = Number(group[0]?.metadata.batchSize) || group.length;
    const count = Math.max(1, Math.min(10, requested));
    let slots = Array.from({ length: count }, (_, index) => group.find(task => (task.metadata.slot || 0) === index));
    // Older single-image retries sometimes retained their original grid index.
    if (group.length === count && new Set(slots.filter(Boolean)).size !== count) {
        slots = [...group].sort((a, b) => (a.metadata.slot || 0) - (b.metadata.slot || 0));
    }
    return { slots, startedAt, expired: now - startedAt >= IMAGE_BATCH_WAIT_MS };
}
