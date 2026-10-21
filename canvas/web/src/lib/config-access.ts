const adminTabs = new Set(["local-proxy", "webdav", "prompt-sources"]);

export function canAccessConfigTab(key: string, isAdmin: boolean) {
    return isAdmin || !adminTabs.has(key);
}

export function accessibleConfigTab<T extends string>(key: T, isAdmin: boolean): T | "channels" {
    return canAccessConfigTab(key, isAdmin) ? key : "channels";
}
