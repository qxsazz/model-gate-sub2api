import { useTranslation } from "react-i18next";

import { useSearchParams } from "react-router-dom";
import type { ConfigTabKey } from "@/stores/use-config-store";
import { AppConfigPanel } from "@/components/layout/app-config-modal";

export default function ConfigPage() {
    const [params] = useSearchParams();
    const requestedTab = params.get("tab");
    const initialTab = (["channels", "local-proxy", "preferences", "prompt-sources", "webdav", "local-storage"].includes(requestedTab || "") ? requestedTab : "channels") as ConfigTabKey;
    const { t } = useTranslation();

    return (
        <main className="h-full overflow-y-auto bg-background">
            <div className="mx-auto max-w-6xl px-6 py-6">
                <div className="mb-5">
                    <h1 className="text-xl font-semibold text-stone-950 dark:text-stone-100">{t("config.title")}</h1>
                    <p className="mt-1 text-sm text-stone-500">{t("config.description")}</p>
                </div>
                <AppConfigPanel initialTab={initialTab} />
                <p className="mt-8 border-t border-border pt-4 text-xs text-muted-foreground">画布基于 <a href="https://github.com/basketikun/infinite-canvas" target="_blank" rel="noreferrer">basketikun / infinite-canvas</a>（MIT）。媒体按账号私有保存 7 天，授权管理员可进行事后审核。</p>
            </div>
        </main>
    );
}
