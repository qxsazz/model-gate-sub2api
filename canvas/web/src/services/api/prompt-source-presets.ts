import { nanoid } from "nanoid";

export type PromptSource = {
    id: string;
    name: string;
    url: string;
    homepage: string;
    enabled: boolean;
    builtIn: boolean;
};

const PROMPT_LIBRARY_BASE = new URL("/canvas-app/prompt-library/v1/", window.location.origin).href;

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
];

function curatedSource(collection: string, name: string, homepage: string): PromptSource {
    return { id: `model-gate-curated-${collection}`, name, url: `${PROMPT_LIBRARY_BASE}${collection}.json`, homepage, enabled: true, builtIn: true };
}

export function mergePromptSources(savedSources: PromptSource[]): PromptSource[] {
    const enabledById = new Map(savedSources.map(source => [source.id, source.enabled]));
    const previousBuiltIns = savedSources.filter(source => source.builtIn);
    const disabledAll = previousBuiltIns.length > 0 && previousBuiltIns.every(source => !source.enabled);
    const builtIn = DEFAULT_PROMPT_SOURCES.map(source => ({ ...source, enabled: enabledById.get(source.id) ?? (disabledAll ? false : source.enabled) }));
    const builtInIds = new Set(builtIn.map(source => source.id));
    const custom = savedSources.filter(source => !source.builtIn && !builtInIds.has(source.id)).map(source => createPromptSource(source));
    return [...builtIn, ...custom];
}
