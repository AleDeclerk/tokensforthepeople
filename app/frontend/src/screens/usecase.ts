type Store = ReturnType<typeof import("../store").createStore>;

const USE_CASES = [
  ["coding-agent", "Coding assistant"],
  ["general-chat", "General chat"],
  ["agentic", "Agents / tools"],
  ["rag", "Documents / RAG"],
];
const PRIORITIES = [
  ["quality", "Best quality"],
  ["latency", "Fastest"],
  ["balanced", "Balanced"],
  ["privacy", "Most private"],
];

export function renderUseCase(store: Store, render: () => void): HTMLElement {
  const el = document.createElement("section");
  el.innerHTML = `<h1>Free LLM tokens for the rest of us</h1>
    <p>Set up free AI in your editor in about a minute.</p>
    <h2>What will you use it for?</h2>
    <div id="uc"></div><h2>What matters most?</h2><div id="pr"></div>
    <button id="next" disabled>Next →</button>`;
  const uc = el.querySelector("#uc")!,
    pr = el.querySelector("#pr")!;
  const next = el.querySelector("#next") as HTMLButtonElement;
  const refresh = () => {
    next.disabled = !(store.state.useCase && store.state.priority);
  };
  for (const [id, label] of USE_CASES) {
    const b = document.createElement("button");
    b.textContent = label;
    b.className = store.state.useCase === id ? "sel" : "";
    b.onclick = () => {
      store.state.useCase = id;
      render();
    };
    uc.appendChild(b);
  }
  for (const [id, label] of PRIORITIES) {
    const b = document.createElement("button");
    b.textContent = label;
    b.className = store.state.priority === id ? "sel" : "";
    b.onclick = () => {
      store.state.priority = id;
      render();
    };
    pr.appendChild(b);
  }
  next.onclick = () => {
    store.state.step = 1;
    render();
  };
  refresh();
  return el;
}
