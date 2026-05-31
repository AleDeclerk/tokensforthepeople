# Plan: expand free-tier providers + chat-probe validation + chain-aware keys screen

**Branch:** `feat/more-providers` (off `main` @ fd3efed, after the desktop-app merge)
**Date:** 2026-05-31

## Goal

Add four more genuinely-free LLM API providers, wire them into the routing
decision matrix so they are actually used, add a POST chat-probe validation
mode for the two whose key cannot be validated with a cheap GET, and make the
keys screen show which providers the chosen use-case/priority chain actually
needs (Option C).

## Decisions already taken (do not re-litigate)

- **Add:** Mistral, NVIDIA NIM, GitHub Models, Z.ai/GLM. Keep all 5 existing.
- **Routing:** wire the new providers into `decisionMatrix` chains, not just as
  pasteable keys.
- **Validation:** add a POST chat-probe mode. Use it only where a cheap GET is
  not reliable. Existing GET stays for the 5 current providers.
- **Keys screen (Option C):** highlight/prioritize the providers the chosen
  chain uses; the rest render under an "optional / improves fallback" group.

## Research outcome (model IDs, endpoints) — verified May 2026

LiteLLM-canonical model strings and probe data:

| Provider | Coding model | General model | Validation | Probe model (native id) | Endpoint |
|----------|--------------|---------------|------------|-------------------------|----------|
| Mistral | `mistral/codestral-latest` | `mistral/mistral-small-latest` | **GET** | — | `GET https://api.mistral.ai/v1/models` |
| NVIDIA NIM | `nvidia_nim/qwen/qwen3-coder-480b-a35b-instruct` | `nvidia_nim/meta/llama-3.1-8b-instruct` | **GET** | — | `GET https://integrate.api.nvidia.com/v1/models` |
| GitHub Models | `github/openai/gpt-4.1` | `github/openai/gpt-4.1-mini` | **POST probe** | `openai/gpt-4.1-mini` | `POST https://models.github.ai/inference/chat/completions` |
| Z.ai/GLM | `zai/glm-4.5-flash` | `zai/glm-4.5-flash` | **POST probe** | `glm-4.5-flash` | `POST https://api.z.ai/api/paas/v4/chat/completions` |

All four authenticate with `Authorization: Bearer <key>`. Probe body (OpenAI-compatible, all four):
```json
{ "model": "<native-id>", "messages": [{"role":"user","content":"hi"}], "max_tokens": 1 }
```

**Flags to verify at integration (do NOT assume):**
1. GitHub Models LiteLLM string form `github/openai/gpt-4.1-mini` (double namespace) — LiteLLM docs are stale; confirm against installed LiteLLM, but the routing matrix only stores the string, so this is a data nit, not a code risk.
2. `zai/glm-4.6` is **paid** — only `glm-4.5-flash` is free. Use flash in both slots.
3. Mistral free Codestral key may route through `codestral.mistral.ai`, not
   `api.mistral.ai`. Validation uses `GET /v1/models` on `api.mistral.ai`, which
   validates a standard La Plateforme key regardless of the Codestral host quirk.

Signup URLs / env vars:
- Mistral: `https://console.mistral.ai/api-keys` · `MISTRAL_API_KEY`
- NVIDIA NIM: `https://build.nvidia.com/` · `NVIDIA_API_KEY`
- GitHub Models: `https://github.com/settings/tokens` (fine-grained PAT, `models:read`) · `GITHUB_API_KEY`
- Z.ai/GLM: `https://z.ai/manage-apikey/apikey-list` · `ZAI_API_KEY`

---

## Task 1: routing — provider constants + matrix wiring

**Files:** `internal/routing/routing.go`, `internal/routing/routing_test.go`

- [ ] Add constants: `ProviderMistral "mistral"`, `ProviderNVIDIA "nvidia"`,
      `ProviderGitHub "github"`, `ProviderZAI "zai"`. Add each to
      `isKnownProvider`-style coverage if such a helper exists (grep first).
- [ ] **TDD:** write tests first asserting the new models appear in the right
      chains, then edit the matrix. Predicted failure: chain slices don't
      contain the new provider entries.
- [ ] Matrix edits (append as fallback tail unless noted; keep Gemini-first
      where it already leads):
  - `(coding, quality)`: insert `{ProviderMistral, "mistral/codestral-latest"}`
    after Gemini; append `{ProviderGitHub, "github/openai/gpt-4.1"}` and
    `{ProviderNVIDIA, "nvidia_nim/qwen/qwen3-coder-480b-a35b-instruct"}`.
  - `(coding, balanced)`: append `{ProviderZAI, "zai/glm-4.5-flash"}`,
    `{ProviderMistral, "mistral/codestral-latest"}`.
  - `(coding, latency)`: append `{ProviderZAI, "zai/glm-4.5-flash"}` (flash is fast).
  - `(coding, privacy)`: **no cloud additions** — privacy chains stay local/Ollama.
  - `(general-chat, quality)`: append `{ProviderMistral, "mistral/mistral-small-latest"}`,
    `{ProviderGitHub, "github/openai/gpt-4.1-mini"}`.
  - `(general-chat, latency)`: append `{ProviderNVIDIA, "nvidia_nim/meta/llama-3.1-8b-instruct"}`.
  - `(general-chat, balanced)`: append `{ProviderZAI, "zai/glm-4.5-flash"}`.
  - `(general-chat, privacy)`: no cloud additions.
- [ ] Agentic & RAG are **hardcoded inline chains** in `BuildChain` (lines
      63–78), not matrix rows. For **agentic**, append
      `{ProviderGitHub, "github/openai/gpt-4.1"}` (strong tool calling) after the
      existing leaders. Leave RAG unless a clear win.
- [ ] **MANDATORY — sync `docs/wizard.md`.** `BuildChain`'s contract says every
      matrix change must update `docs/wizard.md` and the routing tests pin those
      rows (`routing_test.go:9`: "Each test pins one row … in docs/wizard.md").
      Update the doc's table in the same commit or tests fail. Read both first to
      learn the exact format.
- [ ] Commit: `feat(routing): add Mistral, NVIDIA, GitHub, Z.ai to chains`

## Task 2: providers — table entries + validation-mode metadata

**Files:** `internal/providers/providers.go`, `internal/providers/providers_test.go`

- [ ] Extend `Provider` with validation-mode metadata (additive, the 5 existing
      keep zero-value GET behavior):
  ```go
  type ValidateMode int
  const ( ValidateGET ValidateMode = iota; ValidateChatProbe )
  // new fields on Provider:
  Validate   ValidateMode
  ProbeModel string // native model id (no LiteLLM prefix); used when Validate==ValidateChatProbe
  ```
  For probe-mode providers, `Endpoint` holds the **chat-completions** URL.
- [ ] Append the 4 providers to `All` (order = most-useful-first; put Mistral and
      GitHub Models high since they are low-friction, Z.ai near coding value,
      NVIDIA after). Existing 5 stay first only if that preserves intent — choose
      a deliberate order and assert it in a test.
  - Mistral: `Validate: ValidateGET`, `Endpoint: https://api.mistral.ai/v1/models`, `AuthBearer`.
  - NVIDIA: `Validate: ValidateGET`, `Endpoint: https://integrate.api.nvidia.com/v1/models`, `AuthBearer`.
  - GitHub: `Validate: ValidateChatProbe`, `Endpoint: https://models.github.ai/inference/chat/completions`, `ProbeModel: "openai/gpt-4.1-mini"`, `AuthBearer`.
  - Z.ai: `Validate: ValidateChatProbe`, `Endpoint: https://api.z.ai/api/paas/v4/chat/completions`, `ProbeModel: "glm-4.5-flash"`, `AuthBearer`.
- [ ] Test: every provider in `All` has non-empty Display/SignupURL/EnvVar/Endpoint;
      every `ValidateChatProbe` provider has a non-empty `ProbeModel`; every
      `ValidateGET` provider has an empty `ProbeModel`.
- [ ] Commit: `feat(providers): add four free-tier providers with validation modes`

## Task 3: validation — POST chat-probe branch

**Files:** `internal/validation/validation.go`, `internal/validation/validation_test.go`

- [ ] **TDD:** add tests using `httptest.Server` that assert:
  - probe-mode provider + 200 → `StatusOK`, and the request was `POST` with a
    JSON body containing the `ProbeModel` and `max_tokens:1`.
  - 401 → `StatusInvalid`; 429 → `StatusQuotaExceeded`; 5xx → `StatusNetworkError`.
  - GET-mode providers still issue a GET (existing tests stay green).
- [ ] In `Ping`, branch on `p.Validate`:
  - `ValidateGET` → current code path (unchanged).
  - `ValidateChatProbe` → build a `POST` to `p.Endpoint` with
    `Content-Type: application/json` and body
    `{"model": p.ProbeModel, "messages":[{"role":"user","content":"hi"}], "max_tokens":1}`,
    `Authorization: Bearer <key>`. Reuse `classify(resp.StatusCode)`.
  - Keep the empty-key short-circuit and the timeout/context handling.
- [ ] Predicted failure before impl: probe tests fail because `Ping` always GETs.
- [ ] Commit: `feat(validation): add POST chat-probe validation mode`

## Task 4: binding — chain-aware provider list for the keys screen (Option C backend)

**Files:** `app/api.go`, `app/api_test.go`

- [ ] Add a method:
  ```go
  // ProvidersForPlan returns every provider, ordered with the ones used by the
  // (useCase, priority) chain first, each flagged Recommended. Callers that
  // pass an unknown pair get all providers with Recommended=false.
  func (a *API) ProvidersForPlan(useCase, priority string) []ProviderInfo
  ```
  Add `Recommended bool \`json:"recommended"\`` to `ProviderInfo`.
  Compute the recommended set from `routing.BuildChain(...)` provider ids; order
  recommended-first, preserving `providers.All` order within each group.
- [ ] Test: for `(coding, quality)` the recommended set includes gemini + the
      providers wired in Task 1; ordering puts recommended first; unknown pair →
      all `Recommended:false`.
- [ ] Keep existing `ListProviders` (used as a fallback / not breaking).
- [ ] Commit: `feat(app): expose chain-aware provider list to the frontend`

## Task 5: frontend — keys screen highlights recommended providers (Option C)

**Files:** `app/frontend/src/screens/keys.ts`

- [ ] Call `ProvidersForPlan(store.state.useCase, store.state.priority)` instead of
      `ListProviders()`.
- [ ] Render two groups: **"Recommended for your setup"** (recommended=true) and a
      collapsed-ish **"Other providers (optional, improves fallback)"** group.
      Keep the DOM-built rows (no innerHTML for key values — unchanged XSS rule).
- [ ] Add a `.recommended` style accent in `style.css`.
- [ ] `wails build` regenerates bindings; confirm `ProvidersForPlan` shows up in
      `wailsjs/go/main/API.d.ts`.
- [ ] Commit: `feat(app): keys screen highlights chain-relevant providers`

## Task 6: docs

**Files:** `README.md` (free-tier matrix), any `references/*matrix*`, `ROADMAP.md`

- [ ] Update the free-tier matrix table to list all 9 providers.
- [ ] Note the GitHub Models PAT requirement and the Z.ai email-only signup.
- [ ] Commit: `docs: document the four new free-tier providers`

## Final verification

- [ ] `go test -race -count=1 ./cmd/... ./internal/...` all ok
- [ ] `cd app/frontend && npm test` pass
- [ ] `cd app && wails build` → exit 0, `ProvidersForPlan` in generated bindings
- [ ] `gofmt -l` clean
- [ ] CI green on PR (ci.yml + app.yml)
- [ ] Manual GUI smoke: pick coding/quality, confirm Mistral/GitHub show under
      "Recommended"; paste a real GitHub PAT and confirm the chat-probe returns
      `ok` (not a false `ok` from a public endpoint).

## Notes / risks

- The chat-probe burns ~1 token per validation on GitHub Models and Z.ai. That is
  intentional and acceptable (GET is not trustworthy for those two).
- Do not wire `zai/glm-4.6` (paid) or NVIDIA "Mistral Nemotron" (unverified id).
- Privacy chains intentionally get no cloud providers.
