export type KeyState = {
  providerID: string;
  envVar: string;
  value: string;
  status: "" | "validating" | "ok" | "quota" | "invalid" | "unreachable" | "error";
};

export type WizardState = {
  step: number;            // 0..3
  useCase: string;
  priority: string;
  keys: Record<string, KeyState>;   // providerID -> KeyState
  targets: Record<string, boolean>; // target id -> selected
};

// "ok" and "quota" both mean the key works (quota is valid-but-exhausted).
const USABLE = new Set(["ok", "quota"]);

export function createStore() {
  const state: WizardState = {
    step: 0,
    useCase: "",
    priority: "",
    keys: {},
    targets: {},
  };

  return {
    state,
    setKeyStatus(
      providerID: string,
      envVar: string,
      value: string,
      status: KeyState["status"],
    ) {
      state.keys[providerID] = { providerID, envVar, value, status };
    },
    hasValidKey(): boolean {
      return Object.values(state.keys).some((k) => USABLE.has(k.status));
    },
    validKeys(): Record<string, string> {
      const out: Record<string, string> = {};
      for (const k of Object.values(state.keys)) {
        if (USABLE.has(k.status)) out[k.envVar] = k.value;
      }
      return out;
    },
    selectedTargets(): string[] {
      return Object.entries(state.targets)
        .filter(([, on]) => on)
        .map(([id]) => id);
    },
  };
}
