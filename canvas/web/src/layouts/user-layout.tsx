import { useEffect, useState, type ReactNode } from "react";
import { Button, Alert, App } from "antd";
import { NavLink, useLocation } from "react-router-dom";
import { useConfigStore } from "@/stores/use-config-store";
import { notifyHost } from "@/lib/model-gate";
import { refreshManagedModels } from "@/lib/managed-models";

const pages = [["/", "首页"], ["/image", "生图工作台"], ["/video", "视频工作台"], ["/canvas", "我的画布"], ["/assets", "素材库"], ["/prompts", "提示词库"], ["/config", "配置"]];
export default function UserLayout({ children }: { children: ReactNode }) {
    const { pathname } = useLocation();
    const { message } = App.useApp();
    const config = useConfigStore((state) => state.config);
    const [ready, setReady] = useState(false);
    const [error, setError] = useState("");
    async function load() {
        setError("");
        try {
            if (!useConfigStore.persist.hasHydrated()) await new Promise<void>((resolve) => { const stop = useConfigStore.persist.onFinishHydration(() => { stop(); resolve(); }); if (useConfigStore.persist.hasHydrated()) { stop(); resolve(); } });
            const warnings = await refreshManagedModels();
            if (warnings.length) message.warning(warnings.join("；"));
            setReady(true);
        } catch (e) { setError((e as Error).message); }
    }
    useEffect(() => {
        void load();
        const onSettings = () => { void load(); };
        const onError = (event: PromiseRejectionEvent) => { if (event.reason?.message) { event.preventDefault(); message.error(event.reason.message); } };
        window.addEventListener("mg-key-settings", onSettings);
        window.addEventListener("unhandledrejection", onError);
        return () => { window.removeEventListener("mg-key-settings", onSettings); window.removeEventListener("unhandledrejection", onError); };
    }, []);
    const has = (kind: string) => config.channels.some((c) => c.models.some((m) => m.capability === kind));
    return <div className="flex h-dvh min-w-0 flex-col bg-background text-foreground">
        <nav aria-label="创作功能" className="flex shrink-0 flex-wrap gap-1 border-b border-border p-2 sm:px-3">
            {pages.map(([to, label]) => <NavLink key={to} to={to} end={to === "/"} className="rounded-md px-3 py-2 text-sm font-medium transition-colors" style={({ isActive }) => ({ background: isActive ? "var(--accent)" : "transparent", color: isActive ? "var(--primary)" : "var(--foreground)", boxShadow: isActive ? "inset 0 -2px var(--primary)" : undefined })}>{label}</NavLink>)}
        </nav>
        {error && <Alert type="error" message={error} action={<Button onClick={() => void load()}>重试</Button>} />}
        {ready && pathname === "/image" && !has("image") && <Alert type="info" message="暂无可用生图 Key 或图片模型，请创建对应 Key，或检查分组权限。" action={<Button onClick={() => notifyHost("create-key", "image")}>创建生图 Key</Button>} />}
        {ready && pathname === "/video" && !has("video") && <Alert type="info" message="暂无可用生视频 Key 或视频模型，请创建对应 Key，或检查分组权限。" action={<Button onClick={() => notifyHost("create-key", "video")}>创建生视频 Key</Button>} />}
        <div className="min-h-0 flex-1 overflow-hidden">{ready ? children : <div className="p-8">正在加载工作台…</div>}</div>
    </div>;
}
