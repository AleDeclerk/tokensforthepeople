# t4p wizard — design spec

> Status: design draft. No code yet. Reviewing this doc decides the product.

## Goal

A user runs `t4p init`. ~60 seconds later they have:

- Validated free-tier API keys saved to `~/.config/t4p/keys.env` (chmod 600).
- A routing config matched to their use case.
- Config files dropped into their target tool(s) — Cline, Continue, Aider, or
  a local LiteLLM proxy — so the next time they open the editor, free LLMs
  just work.

The wizard is the product. Routing logic and matrix curation are the moat.

---

## The questions (5 screens)

### Screen 1 — What are you building?

```
┌─ tokensforthepeople ────────────────────────────────────────┐
│                                                             │
│  What's your main use case?                                 │
│                                                             │
│  > Code editor / coding agent  (Cline, Continue, Aider)     │
│    General chat                (ChatGPT-like UI)            │
│    Agentic workflows           (LangChain/CrewAI/custom)    │
│    RAG / Q&A over docs                                      │
│    Other / advanced            (show all providers raw)     │
│                                                             │
│  [↑↓] move   [enter] select   [q] quit                      │
└─────────────────────────────────────────────────────────────┘
```

Single-select. Drives 80% of the routing decisions.

### Screen 2 — What matters more?

```
  Latency vs. quality?

  > Quality          (slower, smarter — Gemini Flash first)
    Latency          (snappy autocomplete — Groq first)
    Balanced         (Gemini → Groq fallback)
    Privacy first    (avoid US providers — DeepSeek/Qwen first)
```

Single-select. Picks the **primary** provider in the fallback chain.

### Screen 3 — Which keys do you already have?

```
  Tick what you've got, leave the rest blank.

  [x] Gemini       (https://aistudio.google.com/apikey)
  [ ] Groq         (https://console.groq.com/keys)
  [x] OpenRouter   (https://openrouter.ai/keys)
  [ ] Ollama Cloud (https://ollama.com/settings/keys)
  [ ] Cerebras     (https://cloud.cerebras.ai/platform/api-keys)

  Items left unchecked open the signup URL in your browser if you say yes
  on the next screen.

  [space] toggle   [enter] continue
```

Multi-select. For checked items, screen 3b prompts for the key (paste, hidden
input, live validation with a single `say hi` request). For unchecked items,
screen 3c asks "open browser to sign up? [y/N]".

### Screen 4 — Where do you want this wired?

```
  Where should I write the config?

  [x] Cline           (~/Library/Application Support/Code/User/...
                       /globalStorage/saoudrizwan.claude-dev/settings/...)
  [x] Continue.dev    (~/.continue/config.json)
  [ ] Aider           (~/.aider.conf.yml)
  [ ] LiteLLM proxy   (~/.t4p/litellm_config.yaml + serve cmd)
  [ ] Just give me a .env, I'll wire it myself

  [space] toggle   [enter] write files
```

Multi-select. The wizard detects which tools are installed and pre-checks
those (Cline → check for VSCode extension dir; Aider → `which aider`).

### Screen 5 — Confirm

```
  Ready to write:

    ~/.config/t4p/keys.env                         600  new
    ~/.config/t4p/routing.yaml                     644  new
    ~/Library/.../cline/settings.json              644  overwrite ⚠
    ~/.continue/config.json                        644  new

  Routing for "coding agent" + "quality":
    1. gemini-2.5-flash         (1500 req/day free)
    2. openrouter/deepseek-v4   (fallback on quota)
    3. groq/llama-3.3-70b       (last resort, fast)

  [y] write   [d] show diff   [n] cancel
```

If a target file exists, show a unified diff before overwriting. Always back
up to `<file>.t4p.bak` before writing.

---

## Routing decision matrix

The wizard maps `(use_case, priority)` → ordered provider list. This is the
opinionated knowledge that LiteLLM-by-itself doesn't give you.

| Use case        | Priority    | 1st                            | 2nd                                 | 3rd                          |
|-----------------|-------------|--------------------------------|-------------------------------------|------------------------------|
| Coding agent    | Quality     | gemini-2.5-flash               | openrouter:deepseek-v4-flash:free   | groq:llama-3.3-70b           |
| Coding agent    | Latency     | groq:llama-3.3-70b             | cerebras:llama-3.3-70b (if key)     | gemini-2.5-flash             |
| Coding agent    | Privacy     | openrouter:qwen-2.5-coder:free | openrouter:deepseek-v4-flash:free   | ollama-cloud:qwen-397b       |
| Coding agent    | Balanced    | gemini-2.5-flash               | groq:llama-3.3-70b                  | openrouter:deepseek-v4:free  |
| General chat    | Quality     | gemini-2.5-flash               | openrouter:llama-3.3-70b:free       | groq:llama-3.3-70b           |
| General chat    | Latency     | groq:llama-3.1-8b-instant      | groq:llama-3.3-70b                  | gemini-2.5-flash             |
| Agentic         | (tool use)  | gemini-2.5-flash               | groq:llama-3.3-70b                  | openrouter:deepseek-v4:free  |
| RAG             | (long ctx)  | gemini-2.5-flash               | openrouter:deepseek-v4:free         | groq:llama-3.3-70b           |

Rationale, per row, lives in `docs/routing-rationale.md` (separate doc — every
ordering needs a justification anchored to a real provider quirk).

---

## Output file specs

### `~/.config/t4p/keys.env` (chmod 600)

```bash
# Written by t4p init on 2026-05-26. Do not commit.
GEMINI_API_KEY=AIza...
OPENROUTER_API_KEY=sk-or-v1-...
GROQ_API_KEY=gsk_...
```

### `~/.config/t4p/routing.yaml`

```yaml
version: 1
use_case: coding-agent
priority: quality
chain:
  - provider: gemini
    model: gemini-2.5-flash
  - provider: openrouter
    model: deepseek/deepseek-v4-flash:free
  - provider: groq
    model: llama-3.3-70b-versatile
fallback_on:
  - quota_exceeded
  - 5xx
  - timeout_30s
```

This is the source of truth. Every downstream config is rendered from it,
so editing it and re-running `t4p apply` updates all the targets atomically.

### Cline settings (sample)

```json
{
  "apiProvider": "openrouter",
  "openRouterApiKey": "${OPENROUTER_API_KEY}",
  "openRouterModelId": "deepseek/deepseek-v4-flash:free"
}
```

Cline does not have native multi-provider fallback. We pick a single model
that matches the user's priority and (if they opted into LiteLLM proxy)
point Cline at `http://localhost:4000` instead so the proxy handles routing.

### Continue.dev `config.json` (sample)

```json
{
  "models": [
    {
      "title": "Gemini 2.5 Flash (free)",
      "provider": "gemini",
      "model": "gemini-2.5-flash",
      "apiKey": "${GEMINI_API_KEY}"
    },
    {
      "title": "DeepSeek V4 free (OpenRouter)",
      "provider": "openrouter",
      "model": "deepseek/deepseek-v4-flash:free",
      "apiKey": "${OPENROUTER_API_KEY}"
    }
  ]
}
```

Continue supports listing multiple models — the user picks per chat.

### LiteLLM proxy config (sample)

```yaml
model_list:
  - model_name: smart
    litellm_params:
      model: gemini/gemini-2.5-flash
      api_key: os.environ/GEMINI_API_KEY
  - model_name: smart
    litellm_params:
      model: openrouter/deepseek/deepseek-v4-flash:free
      api_key: os.environ/OPENROUTER_API_KEY
  - model_name: smart
    litellm_params:
      model: groq/llama-3.3-70b-versatile
      api_key: os.environ/GROQ_API_KEY

router_settings:
  routing_strategy: simple-shuffle
  num_retries: 1
  fallbacks:
    - smart: [smart]   # cycles through the duplicates above
```

---

## Validation logic

Every key entered triggers a live `t4p ping <provider>` before continuing:

| Provider    | Validation call                                                       |
|-------------|-----------------------------------------------------------------------|
| Gemini      | `GET /v1beta/models?key=...` (lists models, ~50ms)                    |
| Groq        | `GET /openai/v1/models` with bearer (~30ms)                           |
| OpenRouter  | `GET /api/v1/auth/key` (returns quota info, ~100ms)                   |
| Ollama Cloud| `GET /api/tags` with bearer                                           |
| Cerebras    | `GET /v1/models` with bearer                                          |

On 401/403 → red "invalid key, paste again". On 429 → yellow "key works but
quota exhausted right now — saving anyway, routing will skip it until it
recovers". On timeout/network error → ask retry y/n.

We don't burn quota by doing a real chat completion. `GET /models`-style
endpoints are zero-cost for every provider in the matrix.

---

## Acceptance criteria

Given a fresh machine with no t4p config and a Gemini + OpenRouter key on
the clipboard, when the user runs `t4p init` and selects "coding agent" /
"quality" / Cline as the target, then:

- `~/.config/t4p/keys.env` exists, mode 600, with both keys.
- `~/.config/t4p/routing.yaml` exists with the documented chain.
- Cline's settings file has `openRouterModelId: deepseek/deepseek-v4-flash:free`
  and the OpenRouter key.
- The wizard exits 0 in ≤ 90 seconds (including key paste + 2 live validations).

Given an invalid Gemini key pasted, when validation runs, then the wizard
shows a red error and re-prompts the same field without losing progress on
other screens.

Given a re-run with existing configs, when the user accepts an overwrite,
then a `.t4p.bak` backup is written next to each touched file.

---

## What's intentionally not in v1

- No GUI. CLI only.
- No prompt routing by content (e.g., "if prompt has code → Groq"). Just
  static use-case-based chain. Smart-routing is v2.
- No cost tracking. Everything is free tier — if you exceed it, you're
  rate-limited, not billed.
- No telemetry. Zero phone-home.
- No support for non-free tiers initially. We ignore paid-only models.

---

## Open questions for review

1. **Use case taxonomy.** Are the 5 buckets in screen 1 right? Should
   "embeddings" be its own bucket or folded into RAG?
2. **Cerebras and Mistral.** Worth in the v1 matrix or v2?
3. **Cline routing.** Cline has no native fallback, so we either (a) pick one
   model from the chain, or (b) force the user to install LiteLLM proxy if
   they want the full chain. Which default?
4. **Key storage.** Plain `.env` chmod 600, or use the OS keychain
   (`keyring` lib in Go)? Keychain is safer but breaks the "copy this file
   to another machine" workflow.
5. **Telemetry.** Truly zero, or anonymous opt-in count of "wizard
   completed" so we know if anyone uses it?
