import type { StateStorage } from "zustand/middleware";
import { cloudStateStorage } from "./model-gate";
export const localForageStorage: StateStorage = cloudStateStorage;
