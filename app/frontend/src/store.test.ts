import { describe, it, expect } from "vitest";
import { createStore } from "./store";

describe("store", () => {
  it("starts on step 0 with no valid keys", () => {
    const s = createStore();
    expect(s.state.step).toBe(0);
    expect(s.hasValidKey()).toBe(false);
  });

  it("records a valid key and reports it", () => {
    const s = createStore();
    s.setKeyStatus("gemini", "GEMINI_API_KEY", "AIza-x", "ok");
    expect(s.hasValidKey()).toBe(true);
    expect(s.validKeys()).toEqual({ GEMINI_API_KEY: "AIza-x" });
  });

  it("excludes invalid keys from validKeys", () => {
    const s = createStore();
    s.setKeyStatus("gemini", "GEMINI_API_KEY", "bad", "invalid");
    expect(s.hasValidKey()).toBe(false);
    expect(s.validKeys()).toEqual({});
  });

  it("treats quota as usable (valid but exhausted)", () => {
    const s = createStore();
    s.setKeyStatus("groq", "GROQ_API_KEY", "gsk-x", "quota");
    expect(s.hasValidKey()).toBe(true);
  });
});
