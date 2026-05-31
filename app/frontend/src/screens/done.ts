import { WriteConfigs, OpenPath } from "../../wailsjs/go/main/API";
type Store = ReturnType<typeof import("../store").createStore>;

function line(text: string, cls = ""): HTMLParagraphElement {
  const p = document.createElement("p");
  if (cls) p.className = cls;
  p.textContent = text;
  return p;
}

export function renderDone(store: Store, _render: () => void): HTMLElement {
  const el = document.createElement("section");
  el.innerHTML = `<h1>Setting up…</h1><div id="out"></div>`;
  const out = el.querySelector("#out")!;
  WriteConfigs({
    useCase: store.state.useCase,
    priority: store.state.priority,
    keys: store.validKeys(),
    targets: store.selectedTargets(),
  }).then((res) => {
    out.innerHTML = "";
    if (res.error) {
      out.appendChild(line(res.error, "err"));
      return;
    }
    out.appendChild(line("✓ Keys saved securely (chmod 600)"));
    for (const tr of res.targets) {
      out.appendChild(
        tr.ok
          ? line(`✓ ${tr.target} configured`)
          : line(`✗ ${tr.target}: ${tr.err}`, "err"),
      );
    }
    out.appendChild(line("Reopen your editor and you're on free LLMs."));
    const folder = document.createElement("button");
    folder.textContent = "Open folder";
    folder.onclick = () => OpenPath(res.keysPath.replace(/\/keys\.env$/, ""));
    out.appendChild(folder);
  });
  return el;
}
