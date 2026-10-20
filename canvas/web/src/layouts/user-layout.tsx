import { useEffect, useState, type ReactNode } from "react";
import { Button, Select, Alert, App } from "antd";
import { NavLink, useLocation } from "react-router-dom";
import { useConfigStore, guessCapability, modelOptionsFromChannels, type ModelChannel } from "@/stores/use-config-store";
import { request, hosted, notifyHost } from "@/lib/model-gate";

export default function UserLayout({ children }: { children: ReactNode }) {
    const { pathname } = useLocation();
    const { message } = App.useApp();
    const config = useConfigStore((state) => state.config);
    const update = useConfigStore((state) => state.updateConfig);
    const [ready, setReady] = useState(false);
    const [error, setError] = useState("");
    const [loading, setLoading] = useState(false);
    async function load() {
        setLoading(true); setError("");
        try {
            const { keys } = await request<{ keys: typeof hosted.keys }>("/canvas/capabilities");
            const channels: ModelChannel[] = [];
            for (const key of keys) {
                const result = await request<{ data: { data?: Array<{ id: string }> }; status: number }>(`/canvas/gateway/${key.id}/models`);
                if (result.status >= 400) continue;
                const models = (result.data.data || []).map(({ id }) => ({ name: id, capability: guessCapability(id) })).filter((model) => (model.capability === "image" && key.image) || (model.capability === "video" && key.video));
                if (models.length) channels.push({ id: String(key.id), name: key.name, baseUrl: `${location.origin}/api/v1/canvas/gateway/${key.id}/v1`, apiKey: "managed-by-model-gate", apiFormat: "openai", models });
            }
            const options = modelOptionsFromChannels(channels);
            const previous = useConfigStore.getState().config;
            const first = (kind: string) => channels.flatMap((channel) => channel.models.filter((m) => m.capability === kind).map((m) => `${channel.id}::${m.name}`))[0] || "";
            const imageModel = options.includes(previous.imageModel) ? previous.imageModel : first("image");
            const videoModel = options.includes(previous.videoModel) ? previous.videoModel : first("video");
            useConfigStore.setState({ config: { ...previous, channels, models: options, imageModel, videoModel, model: imageModel, textModel: "", audioModel: "", proxyEnabled: false, channelMode: "remote" } });
            setReady(true);
        } catch (e) { setError((e as Error).message); }
        finally { setLoading(false); }
    }
    useEffect(() => {
        void load();
        const onSettings = () => { void load(); };
        const onError = (event: PromiseRejectionEvent) => { if (event.reason?.message) { event.preventDefault(); message.error(event.reason.message); } };
        window.addEventListener("mg-key-settings", onSettings);
        window.addEventListener("unhandledrejection", onError);
        return () => { window.removeEventListener("mg-key-settings", onSettings); window.removeEventListener("unhandledrejection", onError); };
    }, []);
    const options = (kind: string) => config.channels.flatMap((channel) => channel.models.filter((m) => m.capability === kind).map((m) => ({ value: `${channel.id}::${m.name}`, label: `${m.name} · ${channel.name}` })));
    return <div className="flex h-dvh min-w-0 flex-col bg-background text-foreground">
        <header className="flex flex-wrap items-center gap-3 border-b border-border px-4 py-3">
            <strong style={{ color: "#9c8344" }}>MODEL-GATE · 无限画布</strong>

            <Select aria-label="生图模型和Key" placeholder="选择生图模型 / Key" style={{ minWidth: 220 }} options={options("image")} value={config.imageModel || undefined} onChange={(value) => { update("imageModel", value); update("model", value); }} />
            <Select aria-label="生视频模型和Key" placeholder="选择生视频模型 / Key" style={{ minWidth: 220 }} options={options("video")} value={config.videoModel || undefined} onChange={(value) => update("videoModel", value)} />
            <Button loading={loading} onClick={() => void load()}>刷新 Key</Button>
        </header>
        <nav aria-label="创作功能" className="flex shrink-0 flex-wrap gap-2 border-b border-border px-4 py-2">
            {[["/image", "生图工作台"], ["/video", "视频工作台"], ["/canvas", "我的画布"], ["/assets", "素材库"]].map(([to, label]) => <NavLink key={to} to={to} className="rounded-md px-4 py-2 text-sm font-medium" style={({ isActive }) => ({ background: isActive ? "var(--primary)" : "transparent", color: isActive ? "var(--primary-foreground)" : "var(--foreground)" })}>{label}</NavLink>)}
        </nav>
        <div className="px-4 py-2 text-xs opacity-75">作品按账号云端保存；图片及视频素材保存 7 天，到期自动删除，请及时下载。授权管理员可进行事后内容审核。画布引擎：basketikun / infinite-canvas（MIT）。</div>
        {error && <Alert type="error" message={error} action={<Button onClick={() => void load()}>重试</Button>} />}
        {ready && pathname !== "/video" && !options("image").length && <Alert type="info" message="暂无可用生图 Key 或图片模型，请创建对应 Key，或检查分组权限。" action={<Button onClick={() => notifyHost("create-key", "image")}>创建生图 Key</Button>} />}
        {ready && pathname !== "/image" && !options("video").length && <Alert type="info" message="暂无可用生视频 Key 或视频模型，请创建对应 Key，或检查分组权限。" action={<Button onClick={() => notifyHost("create-key", "video")}>创建生视频 Key</Button>} />}
        <div className="min-h-0 flex-1 overflow-hidden">{ready ? children : <div className="p-8">正在连接云端画布…</div>}</div>
    </div>;
}
