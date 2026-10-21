import { nanoid } from "nanoid";

import type { PromptSource } from "./prompt-source-migration";
export type { PromptSource } from "./prompt-source-migration";

const PROMPT_LIBRARY_BASE = new URL("/canvas-app/prompt-library/v2/", window.location.origin).href;

export function createPromptSource(source?: Partial<PromptSource>): PromptSource {
    return {
        id: source?.id?.trim() || nanoid(),
        name: source?.name?.trim() || "",
        url: source?.url?.trim() || "",
        homepage: source?.homepage?.trim() || "",
        enabled: source?.enabled ?? true,
        builtIn: source?.builtIn ?? false,
    };
}

export const DEFAULT_PROMPT_SOURCES: PromptSource[] = [
    curatedSource("products", "精选 · 产品与美食", "https://github.com/JeremyGDM/awesome-ai-product-photography-prompts"),
    curatedSource("photography", "精选 · 人像与风景", "https://github.com/Pixmind-io/awesome-gpt-image-2-prompts/tree/master/prompts/01-cinematic-portraits"),
    curatedSource("design", "精选 · 海报与插画", "https://github.com/Pixmind-io/awesome-gpt-image-2-prompts/tree/master/prompts/09-poster-graphic"),
    curatedSource("cinema-portraits", "电影 · 人物特写", "https://github.com/Pixmind-io/awesome-gpt-image-2-prompts/tree/master/prompts/01-cinematic-portraits"),
    curatedSource("cinema-scenes", "电影 · 场景叙事", "https://github.com/Pixmind-io/awesome-gpt-image-2-prompts/tree/master/prompts/03-nature-landscapes"),
    curatedSource("cinema-worlds", "电影 · 动作与奇幻", "https://github.com/Pixmind-io/awesome-gpt-image-2-prompts/tree/master/prompts/04-fantasy-illustration"),
];

function curatedSource(collection: string, name: string, homepage: string): PromptSource {
    return { id: `model-gate-curated-${collection}`, name, url: `${PROMPT_LIBRARY_BASE}${collection}.json`, homepage, enabled: true, builtIn: true };
}
