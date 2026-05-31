import { ListProviders, ValidateKey, OpenURL } from "../../wailsjs/go/main/API";
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

  ListProviders().then((providers) => {
    rows.innerHTML = "";
    for (const p of providers) {
      const row = document.createElement("div");
      row.className = "keyrow";
      const known = store.state.keys[p.id];
      // Build via DOM, not innerHTML: the key value and provider display must
      // never be interpolated into an HTML string (XSS / breakage on quotes).
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
      rows.appendChild(row);
    }
  });
  return el;
}
