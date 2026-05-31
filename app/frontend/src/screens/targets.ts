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
  el.innerHTML = `<h1>Where should it go?</h1><div id="t">Loading…</div>
    <button id="back">← Back</button>
    <button id="write" disabled>Write →</button>`;
  const t = el.querySelector("#t")!;
  const write = el.querySelector("#write") as HTMLButtonElement;
  write.disabled = !store.hasValidKey();
  if (!store.hasValidKey()) write.title = "Add at least one valid key first";
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
    for (const tt of targets) {
      if (!(tt.id in store.state.targets)) store.state.targets[tt.id] = tt.detected;
      const row = document.createElement("label");
      row.className = "trow";
      const cb = document.createElement("input");
      cb.type = "checkbox";
      cb.checked = store.state.targets[tt.id];
      cb.onchange = (e) => {
        store.state.targets[tt.id] = (e.target as HTMLInputElement).checked;
      };
      const name = document.createTextNode(
        ` ${LABEL[tt.id] ?? tt.id} ${tt.detected ? "(found)" : ""}`,
      );
      row.append(cb, name);
      t.appendChild(row);
    }
  });
  return el;
}
