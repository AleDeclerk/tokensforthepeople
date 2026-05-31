# Desktop app — design

> Status: approved design, pending spec review
> Author: brainstorming session 2026-05-31
> Milestone: this is a parallel track to the v0.2 CLI roadmap, not a replacement.

## Goal

Make t4p usable by people who do not live in a terminal. Today the only way in
is `t4p init`, a terminal TUI. We add a double-click desktop app with a native
window whose UI is web-based, so a non-developer can set up free LLM access
without typing a command. The existing CLI and TUI stay as the developer and CI
path.

## Decisions (locked during brainstorming)

- **Entry point:** a desktop app built with **Wails** (Go backend, web frontend
  in a native window). It is the primary face for non-developers.
- **CLI/TUI:** the code is unchanged and `t4p init` plus the non-interactive
  flags remain the developer and CI path. What changes is promotion: the app
  becomes the default front door a new user is steered to (README leads with it,
  see Distribution), and the terminal TUI is no longer the promoted entry point,
  only the developer one. The app and the CLI are two front ends over the same
  backend.
- **UI language:** English, matching the rest of the product.
- **Signing:** phased. Phase 1 ships unsigned (macOS users open via right-click
  → Open the first time). Phase 2 adds Apple notarization and Windows signing
  when it is worth the USD 99/year.

## Architecture

The whole point is that the app adds a front end, not a second implementation.
Every piece of logic the wizard needs already exists in `internal/` and is
tested. The app calls it through a thin binding layer.

```
tokensforthepeople (repo)
├── cmd/t4p/            CLI + TUI. Unchanged.
├── internal/           validation · routing · providers · emit · keystore · tools
│                        ↑ same packages, consumed by both front ends
└── app/                NEW — Wails project
    ├── main.go         window bootstrap (Wails app, embeds frontend/dist)
    ├── api.go          binding methods, thin wrappers over internal/*
    ├── api_test.go     tests for the binding layer
    └── frontend/       web UI (vanilla TS + Vite; no heavy framework)
        ├── index.html
        ├── src/        one module per screen + a small state store
        └── dist/       built assets, embedded into the Go binary
```

### Binding layer (`app/api.go`)

The frontend never touches disk, network, or env directly. It calls these
methods, each of which is a thin adapter over an existing `internal/` function.
Each does one thing and is independently testable.

| Method | Wraps | Returns |
|--------|-------|---------|
| `ListProviders()` | `providers.All` | id, display, signup URL, env var, "easiest" hint |
| `ValidateKey(id, key)` | `validation.Ping` | status: ok / quota / invalid / unreachable |
| `BuildPlan(useCase, priority)` | `routing.BuildChain` | ordered chain of (provider, model) |
| `DetectTargets()` | `internal/tools` | which editors are installed (for defaults) |
| `WriteConfigs(answers)` | `keystore.Write` + `emit.Render`/`WriteAtomic` | per-target written/updated/failed report |
| `OpenURL(url)` | Wails runtime `BrowserOpenURL` | — (powers "Get a key ↗") |
| `OpenPath(path)` | OS file manager | — (powers "Open folder") |

`WriteConfigs` reuses the exact write-and-report loop that `cmd/t4p`'s
`writeOutputs` runs. To avoid duplicating it, that loop is extracted from
`cmd/t4p/main.go` into an exported helper in `internal/emit` (or a small
`internal/plan` package) that both the CLI and the app call. This is the
"make the change easy, then make the easy change" refactor: extract first as
its own commit, then have both callers use it.

## Screens and flow

Four steps, mirroring the TUI but visual. Back/Next throughout; state held in
the frontend until the final write.

1. **Use case + priority.** Radio groups. Maps to `BuildPlan`.
2. **Paste keys.** One row per provider (`ListProviders`), each with a paste
   field, a live status chip (debounced `ValidateKey`), and a **"Get a key ↗"**
   button that calls `OpenURL` to the provider's signup page. Copy says "you
   only need one to start." This button is the real unlock for non-developers:
   the hardest step today is knowing where to get a key.
3. **Targets.** Checkboxes pre-checked from `DetectTargets`. Maps to the target
   list in `WriteConfigs`.
4. **Done.** Calls `WriteConfigs`, shows a per-target success/fail summary, an
   "Open folder" button (`OpenPath`), and the existing "reopen your editor"
   message.

### Wireframe

```
┌─ tokensforthepeople ───────────────┐  ┌─ Paste your keys ──────────────────────┐
│   Free LLM tokens for the rest of  │  │  You only need ONE to start.           │
│   us                               │  │  Gemini   [AIza········] ✓ valid       │
│   Set up free AI in your editor    │  │           easiest · free  [Get a key ↗]│
│   What will you use it for?        │  │  Groq     [···········]   [Get a key ↗]│
│   ( ) Coding assistant             │  │  OpenRouter[··········]   [Get a key ↗]│
│   ( ) General chat                 │  │  Cerebras [···········]   [Get a key ↗]│
│   ( ) Agents / tools               │  │  Ollama   [···········]   [Get a key ↗]│
│   ( ) Documents / RAG              │  │  ● validating  ✓ valid  ✗ invalid      │
│                        [ Next → ]  │  │                  [← Back] [Next →]      │
└────────────────────────────────────┘  └────────────────────────────────────────┘
┌─ Where should it go? ──────────────┐  ┌─ Done ─────────────────────────────────┐
│  Detected what's installed:        │  │  ✓ Keys saved securely (chmod 600)     │
│   [x] Continue.dev   (found)       │  │  ✓ Continue.dev configured             │
│   [x] Aider          (found)       │  │  ✓ Aider configured                    │
│   [ ] Cline          (not found)   │  │  Reopen your editor and you're on      │
│   [ ] LiteLLM proxy                │  │  free LLMs.                            │
│                  [← Back] [Write →]│  │              [Open folder] [Done]      │
└────────────────────────────────────┘  └────────────────────────────────────────┘
```

## Data flow

```
frontend screen  ──calls──▶  app/api.go method  ──delegates──▶  internal/* (tested)
       ▲                                                            │
       └────────────── typed result (status, plan, report) ◀────────┘
```

No secret is logged. Keys live only in the frontend state and the
`ValidateKey`/`WriteConfigs` arguments until written to `keys.env` (chmod 600,
the existing keystore behavior). The audit-safe behavior of the CLI carries over
because the same keystore code runs.

## Error handling

- **Invalid / unreachable key:** the row's status chip shows the state inline;
  it never blocks moving on, because one valid key is enough. Mirrors the CLI,
  where `validation` classifies ok / quota / invalid / network error.
- **No valid key at all by step 3:** the "Write" button is disabled with a hint,
  matching the CLI's "no keys collected; nothing to write" guard.
- **A target write fails:** the Done screen lists it as failed (red) while
  others show success, reusing the per-target report `WriteConfigs` returns.
- **Window closed mid-flow:** nothing is written until step 4, so closing early
  leaves the system untouched.

## Testing

- **Binding layer (`app/api_test.go`):** each method tested against the same
  fakes the `internal/` tests already use (e.g. `validation` against a mock
  server, `WriteConfigs` against a temp HOME). The binding is thin, so these are
  small.
- **Extracted write loop:** the existing `cmd/t4p` write tests move/extend to
  cover the shared helper, so both front ends are covered by one set of tests.
- **Frontend:** kept logic-light on purpose. Any non-trivial state (e.g. "is at
  least one key valid") lives in a small testable store module with unit tests;
  screens stay declarative.
- **No new test framework for Go.** Frontend uses Vitest (ships with Vite).

## Distribution (Phase 1, unsigned)

- Wails build produces `t4p.app` (macOS), and later `.exe` (Windows) and a Linux
  binary. Built in CI on tag, attached to the GitHub release alongside the
  existing CLI tarballs.
- macOS: shipped in a `.dmg` with a short "right-click → Open the first time"
  note, because the app is not yet notarized.
- The CLI release pipeline is untouched; the app is an additional artifact.
- README gets a "Download the app" section above the existing `go install`
  section, so non-developers hit the easy path first and developers still find
  the CLI.

## Phase 2 (deferred, recorded so Phase 1 does not block it)

- Apple Developer Program + notarization in the release workflow.
- Windows code signing.
- Auto-update check (the app can call the same GitHub releases API the CLI uses).

## Out of scope

- No change to the routing matrix, provider set, or key rules.
- No bundling of a LiteLLM runtime; "LiteLLM proxy" remains a config target, as
  in the CLI.
- The v0.2 CLI features (`apply`, `doctor`, `routing.yaml`) are a separate track
  and not required for the app.

## Build sequence

1. Refactor: extract the write-and-report loop from `cmd/t4p/main.go` into a
   shared exported helper; CLI uses it; tests follow. (own commit)
2. Scaffold the Wails project under `app/` with an empty window that loads a
   placeholder frontend.
3. Implement `app/api.go` binding methods + `api_test.go`.
4. Build the four frontend screens against the bindings, with the state store
   and its tests.
5. Wire "Get a key ↗" and "Open folder" via the Wails runtime.
6. CI: add a build job that produces the app artifacts on tag; attach to the
   release.
7. Docs: README "Download the app" section; update ROADMAP.

## Acceptance criteria

- Given the app is open, when the user picks a use case and priority and clicks
  Next, then the key screen shows one row per provider with a "Get a key" link.
- Given a valid key pasted into a provider row, when validation completes, then
  the row shows a success state within a few seconds without burning quota.
- Given at least one valid key and a detected target, when the user clicks
  Write, then the same config files the CLI would write are written (keys.env at
  chmod 600 plus each selected target), and the Done screen reports each.
- Given no valid key, when the user reaches the targets screen, then Write is
  disabled with an explanation.
- Given the user never reaches the final screen, when they close the window,
  then no files are written.
