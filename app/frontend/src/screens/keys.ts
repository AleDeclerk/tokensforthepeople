import { ProvidersForPlan, ValidateKey, OpenURL } from "../../wailsjs/go/main/API";
import type { main } from "../../wailsjs/go/models";
type Store = ReturnType<typeof import("../store").createStore>;

export function renderKeys(store: Store, render: () => void): HTMLElement {
  const el = document.createElement("section");
  el.innerHTML = `<h1>Paste your keys</h1><p>You only need ONE to start.</p>
    <div id="rows">Loading…</div>
    <button id="back">← Back</button><button id="next">Next →</button>`;
  const rows = el.querySelector("#rows")!;
  (el.querySelector("#back") as HTMLButtonElement).onclick = () => {
    store.state.step = 0;
    render();
  };
  (el.querySelector("#next") as HTMLButtonElement).onclick = () => {
    store.state.step = 2;
    render();
  };

  // Build one provider row. DOM-built (not innerHTML): the pasted key value and
  // provider display must never be interpolated into an HTML string (XSS).
  function makeRow(p: main.ProviderInfo): HTMLElement {
    const row = document.createElement("div");
    row.className = "keyrow";
    const known = store.state.keys[p.id];
    const label = document.createElement("label");
    label.textContent = p.display + (p.easiest ? " · easiest" : "");
    const input = document.createElement("input");
    input.type = "password";
    input.placeholder = "paste key";
    input.value = known?.value ?? "";
    const chip = document.createElement("span");
    chip.className = "chip";
    chip.textContent = known?.status ?? "";
    const link = document.createElement("button");
    link.className = "link";
    link.textContent = "Get a key ↗";
    link.onclick = () => OpenURL(p.signupURL);
    row.append(label, input, chip, link);
    let timer: number | undefined;
    input.oninput = () => {
      const val = input.value.trim();
      clearTimeout(timer);
      if (!val) {
        store.setKeyStatus(p.id, p.envVar, "", "");
        chip.textContent = "";
        return;
      }
      chip.textContent = "validating…";
      timer = window.setTimeout(async () => {
        const res = await ValidateKey(p.id, val);
        store.setKeyStatus(p.id, p.envVar, val, res.status as any);
        chip.textContent = res.status;
      }, 600);
    };
    return row;
  }

  function group(title: string, providers: main.ProviderInfo[], cls: string) {
    if (!providers.length) return;
    const heading = document.createElement("h2");
    heading.className = cls;
    heading.textContent = title;
    rows.appendChild(heading);
    for (const p of providers) rows.appendChild(makeRow(p));
  }

  ProvidersForPlan(store.state.useCase, store.state.priority).then((providers) => {
    rows.innerHTML = "";
    const recommended = providers.filter((p) => p.recommended);
    const others = providers.filter((p) => !p.recommended);
    group("Recommended for your setup", recommended, "recommended");
    group("Other providers (optional — improves fallback)", others, "");
  });
  return el;
}
