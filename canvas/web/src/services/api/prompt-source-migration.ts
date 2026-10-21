export type PromptSource = {
    id: string;
    name: string;
    url: string;
    homepage: string;
    enabled: boolean;
    builtIn: boolean;
};

export function mergePromptSources(savedSources: PromptSource[], defaultSources: PromptSource[]): PromptSource[] {
    const enabledById = new Map(savedSources.map(source => [source.id, source.enabled]));
    const previousBuiltIns = savedSources.filter(source => source.builtIn);
    const disabledAll = previousBuiltIns.length > 0 && previousBuiltIns.every(source => !source.enabled);
    const builtIn = defaultSources.map(source => ({ ...source, enabled: enabledById.get(source.id) ?? (disabledAll ? false : source.enabled) }));
    const builtInIds = new Set(builtIn.map(source => source.id));
    const custom = savedSources.filter(source => !source.builtIn && !builtInIds.has(source.id)).map(source => ({ ...source }));
    return [...builtIn, ...custom];
}
