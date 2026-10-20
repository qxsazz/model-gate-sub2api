import { useEffect, useState } from "react";
import { Alert, App, Badge, Button, Drawer, Tag, Image } from "antd";
import { getCanvasTasks, refreshCanvasTasks, taskPending, taskError, getTaskImages, loadTaskImages } from "@/lib/canvas-tasks";
import { request } from "@/lib/model-gate";

export function GenerationTasks() {
    const { message, modal } = App.useApp();
    const [busy, setBusy] = useState("");
    const [hidden, setHidden] = useState<Set<string>>(new Set());
    async function act(id: string, remove: boolean) {
        setBusy(id);
        try {
            await request(`/canvas/tasks/${id}${remove ? "" : "/cancel"}`, remove ? "DELETE" : "POST");
            if (remove) setHidden(ids => new Set([...ids, id]));
            else message.info("已请求停止任务，状态将自动更新");
            window.dispatchEvent(new Event("mg-tasks-refresh"));
            return true;
        } catch (e) { message.error(e instanceof Error ? e.message : "操作失败，请重试"); return false; }
        finally { setBusy(""); }
    }
    function confirmAction(id: string, remove: boolean) {
        modal.confirm({
            title: remove ? "删除任务记录？" : "停止这个任务？",
            content: remove ? "从生成任务列表移除，已有作品和素材保留。" : "本站将取消请求；上游可能已经生成或计费，请确认结果后再重试。",
            okText: remove ? "确认删除" : "停止任务",
            cancelText: "取消",
            okButtonProps: { danger: remove },
            async onOk() { if (!await act(id, remove)) return Promise.reject(); },
        });
    }
    const [revision, setRevision] = useState(0),
        [open, setOpen] = useState(false),
        [error, setError] = useState("");
    useEffect(() => {
        let active = true;
        const load = () =>
            void refreshCanvasTasks()
                .then(() => {
                    if (active) {
                        setRevision((v) => v + 1);
                        setError("");
                    }
                })
                .catch(() => {
                    if (active) setError("任务状态暂时无法刷新，正在进行的任务仍保存在服务器");
                });
        load();
        const timer = setInterval(load, 2500);
        window.addEventListener("mg-tasks-refresh", load);
        return () => {
            active = false;
            clearInterval(timer);
            window.removeEventListener("mg-tasks-refresh", load);
        };
    }, []);
    void revision;
    const tasks = getCanvasTasks().filter(t => !t.deleted_at && !hidden.has(t.id)),
        pending = tasks.filter(taskPending).length;
    return (
        <>
            <Badge count={pending} size="small">
                <Button onClick={() => setOpen(true)}>生成任务</Button>
            </Badge>
            <Drawer title="生成任务" open={open} onClose={() => setOpen(false)} width={440}>
                {error && <Alert type="warning" message={error} />}
                <p className="mb-4 text-sm text-muted-foreground">切换栏目后任务继续执行，返回时自动恢复。同步生图最长执行 6 分钟，超时后停止连接并提示重试；素材保存 7 天。</p>
                {!tasks.length && <p>暂无生图任务</p>}
                {tasks.map((t) => (
                    <article key={t.id} className="mb-3 rounded-lg border border-border p-3">
                        <Tag>{t.response_status === 504 ? "已超时" : ({ queued: "排队中", running: "生成中", completed: "已完成", failed: "失败", interrupted: "已中断" } as Record<string, string>)[t.status]}</Tag>
                        <strong>{t.metadata.model}</strong>
                        <p className="mt-2 whitespace-pre-wrap break-words text-sm">{t.metadata.prompt}</p>
                        {t.status === "completed" && (
                            <div className="mt-2 flex flex-wrap gap-2">
                                {getTaskImages(t.id).map((image) => (
                                    <Image key={image.id} width={80} src={image.dataUrl} />
                                ))}
                                {!getTaskImages(t.id).length && (
                                    <Button
                                        size="small"
                                        onClick={() =>
                                            void loadTaskImages(t)
                                                .then(() => setRevision((v) => v + 1))
                                                .catch(() => setError("素材已删除、过期或暂时无法读取"))
                                        }
                                    >
                                        查看图片
                                    </Button>
                                )}
                            </div>
                        )}
                        <div className="mt-3">
                            <Button size="small" danger={!taskPending(t)} loading={busy === t.id} disabled={Boolean(busy)} onClick={() => confirmAction(t.id, !taskPending(t))}>
                                {taskPending(t) ? "停止任务" : "删除任务"}
                            </Button>
                        </div>
                        <p className="mt-2 text-xs text-muted-foreground">
                            {t.metadata.source === "canvas" ? "画布任务" : "生图工作台"} · {new Date(t.created_at).toLocaleString()}
                        </p>
                        {!taskPending(t) && t.status !== "completed" && (
                            <p role="alert" className="text-sm text-red-600">
                                {taskError(t)}
                            </p>
                        )}
                    </article>
                ))}
            </Drawer>
        </>
    );
}
