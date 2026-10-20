import { initializeHosted } from "./lib/model-gate";
initializeHosted().then(() => import("./hosted-entry")).catch((error) => {
    document.getElementById("root")!.textContent = error.message || "画布初始化失败，请返回主站重试";
});
