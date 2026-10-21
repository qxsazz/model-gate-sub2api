import { useEffect, useState, type ReactNode } from "react";
import { usePromptSourceScheduler } from "@/hooks/use-prompt-source-scheduler";
import { usePromptSourceStore } from "@/stores/use-prompt-source-store";
function Scheduler() { usePromptSourceScheduler(); return null; }
export function ClientRootInit({ children }: { children: ReactNode }) {
    const [ready, setReady] = useState(usePromptSourceStore.persist.hasHydrated());
    useEffect(() => { const stop = usePromptSourceStore.persist.onFinishHydration(() => setReady(true)); if (usePromptSourceStore.persist.hasHydrated()) setReady(true); return stop; }, []);
    return <>{ready && <Scheduler />}{children}</>;
}
