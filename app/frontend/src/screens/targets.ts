import { DetectTargets } from "../../wailsjs/go/main/API";
type Store = ReturnType<typeof import("../store").createStore>;

const LABEL: Record<string, string> = {
  continue: "Continue.dev",
  aider: "Aider",
  cline: "Cline",
  litellm: "LiteLLM proxy",
};

export function renderTargets(store: Store, render: () => void): HTMLElement {
  const el = document.createElement("section");
  el.innerHTML = `<h1>Where should it go?</h1>
    <p class="hint">Optional. Your keys are saved either way — pick a tool below
    only if you want it auto-configured.</p>
    <div id="t">Loading…</div>
    <button id="back">← Back</button>
    <button id="write" disabled>Save keys →</button>`;
  const t = el.querySelector("#t")!;
  const write = el.querySelector("#write") as HTMLButtonElement;

  // The keys are always written; targets are optional extras. So Write is gated
  // only on having a valid key, and its label reflects what will happen: just
  // saving keys, or also writing tool configs.
  const refreshWrite = () => {
    write.disabled = !store.hasValidKey();
    write.title = write.disabled ? "Add at least one valid key first" : "";
    write.textContent =
      store.selectedTargets().length > 0 ? "Write configs →" : "Save keys →";
  };
  refreshWrite();

  (el.querySelector("#back") as HTMLButtonElement).onclick = () => {
    store.state.step = 1;
    render();
  };
  write.onclick = () => {
    store.state.step = 3;
    render();
  };

  DetectTargets().then((targets) => {
    t.innerHTML = "";
    const anyDetected = targets.some((tt) => tt.detected);
    if (!anyDetected) {
      const note = document.createElement("p");
      note.className = "hint";
      note.textContent =
        "No supported tools detected on this machine — that's fine, just save your keys.";
      t.appendChild(note);
    }
    for (const tt of targets) {
      if (!(tt.id in store.state.targets)) store.state.targets[tt.id] = tt.detected;
      const row = document.createElement("label");
      row.className = "trow";
      const cb = document.createElement("input");
      cb.type = "checkbox";
      cb.checked = store.state.targets[tt.id];
      cb.onchange = (e) => {
        store.state.targets[tt.id] = (e.target as HTMLInputElement).checked;
        refreshWrite();
      };
      const name = document.createTextNode(
        ` ${LABEL[tt.id] ?? tt.id} ${tt.detected ? "(found)" : ""}`,
      );
      row.append(cb, name);
      t.appendChild(row);
    }
    refreshWrite();
  });
  return el;
}
