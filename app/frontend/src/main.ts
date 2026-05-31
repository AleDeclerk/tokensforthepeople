import { createStore } from "./store";
import { renderUseCase } from "./screens/usecase";
import { renderKeys } from "./screens/keys";
import { renderTargets } from "./screens/targets";
import { renderDone } from "./screens/done";

const store = createStore();
const root = document.getElementById("app")!;

export function render() {
  root.innerHTML = "";
  const screens = [renderUseCase, renderKeys, renderTargets, renderDone];
  root.appendChild(screens[store.state.step](store, render));
}

render();
