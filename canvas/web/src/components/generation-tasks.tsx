import { useEffect, useState } from "react";
import { Alert, Badge, Button, Drawer, Tag, Image } from "antd";
import { getCanvasTasks, refreshCanvasTasks, taskPending, taskError, getTaskImages, loadTaskImages } from "@/lib/canvas-tasks";
export function GenerationTasks() {
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
    const tasks = getCanvasTasks(),
        pending = tasks.filter(taskPending).length;
    return (
        <>
            <Badge count={pending} size="small">
                <Button onClick={() => setOpen(true)}>生成任务</Button>
            </Badge>
            <Drawer title="生成任务" open={open} onClose={() => setOpen(false)} width={440}>
                {error && <Alert type="warning" message={error} />}
                <p className="mb-4 text-sm text-muted-foreground">切换栏目后任务继续执行，返回时自动恢复；素材保存 7 天。</p>
                {!tasks.length && <p>暂无生图任务</p>}
                {tasks.map((t) => (
                    <article key={t.id} className="mb-3 rounded-lg border border-border p-3">
                        <Tag>{({ queued: "排队中", running: "生成中", completed: "已完成", failed: "失败", interrupted: "已中断" } as Record<string, string>)[t.status]}</Tag>
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
