# Roadmap

This document tracks where t4p is going. It is updated per milestone, not per
commit. The shipped surface lives in the [README](./README.md); the original
product spec lives in [docs/wizard.md](./docs/wizard.md).

## Shipped

### v0.1.0 (2026-05-28)

The first-run wizard. `t4p init` interviews the user and writes validated keys
plus target configs for Cline, Continue.dev, Aider, and a LiteLLM proxy, using
free tiers of Gemini, OpenRouter, Groq, Ollama Cloud, and Cerebras. Three
install paths: `go install`, direct tarball, and Homebrew tap.

### v0.1.1 (pending release)

A fix: binaries built with `go install` reported their version as `dev` because
the version string is injected by ldflags, which only the goreleaser and
Homebrew builds set. `go install` now falls back to the module version recorded
in the binary's build info, so it reports the real tag. The fix is covered by
unit tests on `resolveVersion`.

## Planned

### v0.2 — Day-2 trust

v0.1 gets a user set up once. v0.2 is about the second day: re-checking that the
setup still works and re-rendering configs without sitting through the wizard
again. It completes the architecture the spec already describes (a single
`routing.yaml` as the source of truth that every target config is rendered
from) and turns the `doctor` stub into a working health check. Everything in
this milestone is local and depends only on the standard library plus the
packages already in use.

#### 1. `routing.yaml` as the source of truth

Today `t4p init --write` renders each target config directly and keeps nothing
in between. The spec calls for a `~/.config/t4p/routing.yaml` that holds the
resolved chain, and for every target config to be rendered from it. v0.2 writes
that file during `init --write`.

The schema extends the one in the spec with a `targets` list, because `t4p
apply` (below) needs to know which configs to regenerate:

```yaml
version: 1
use_case: coding-agent
priority: quality
targets: [continue, aider]
chain:
  - provider: gemini
    model: gemini/gemini-2.5-flash
  - provider: openrouter
    model: openrouter/deepseek/deepseek-v4-flash:free
  - provider: groq
    model: groq/llama-3.3-70b-versatile
```

YAML emission already exists in `internal/emit` for the LiteLLM config, so this
needs no new dependency.

#### 2. `t4p apply`

Reads `routing.yaml` and `keys.env`, then re-renders every target listed in the
file. This is the same render-and-write loop that `init --write` runs after key
collection, so the loop is extracted into a shared function and both commands
call it. Use cases: the user edits the chain by hand, adds a key later, or wants
to re-point a tool after reinstalling it.

#### 3. `t4p doctor`

Reads `keys.env`, pings each configured provider with the same zero-cost
validation call the wizard uses, and reports per provider: valid, valid but
quota-exhausted, invalid, or unreachable. Exit code is non-zero when any key is
invalid, so it works in a pre-commit hook or CI step. This requires a
`keystore.Load` to read `keys.env` back, which does not exist yet.

#### Build sequence

1. `keystore.Load` to read `keys.env` into a map (needed by `apply` and
   `doctor`).
2. `routing.yaml` read and write in `internal/emit` (or a small `internal/state`
   package), reusing the existing YAML style.
3. Extract the per-target render-and-write loop out of `writeOutputs` so `init`
   and `apply` share it, then have `init --write` also write `routing.yaml`.
4. Implement `t4p apply` on top of the shared loop.
5. Implement `t4p doctor` on top of `keystore.Load` and `validation.Ping`.
6. Tests: `keystore.Load` round-trip, `routing.yaml` round-trip, `apply` against
   a temp `HOME`, `doctor` against a mock server (the pattern `validation` tests
   already use).
7. Docs: update README, `docs/wizard.md`, and the `t4p` usage text for `apply`
   and `doctor`.

#### Acceptance criteria

- Given a completed `init --write`, when the user inspects the config dir, then
  `~/.config/t4p/routing.yaml` exists and lists the chosen chain and targets.
- Given an existing `routing.yaml` and a valid `keys.env`, when the user runs
  `t4p apply`, then every target in the file is re-rendered and a `.t4p.bak`
  backup is written next to each file that already existed.
- Given a `keys.env` with one valid and one revoked key, when the user runs
  `t4p doctor`, then the valid key reports OK, the revoked key reports invalid,
  and the process exits non-zero.

## Later

These are out of scope for v0.2. They are recorded so the v0.2 design does not
paint them into a corner.

- **`t4p serve` and `--with-proxy`.** Manage a local LiteLLM proxy so single-
  model tools like Cline get the full fallback chain. Deferred because it adds a
  LiteLLM runtime dependency on the user's machine and process management across
  platforms.
- **`t4p update-matrix`.** Refresh the routing matrix from a published source as
  free tiers change. Needs a hosted, versioned matrix and a merge strategy.
- **`docs/routing-rationale.md`.** A per-row justification for every entry in the
  decision matrix, anchored to a real provider quirk.
- **`--use-keychain`.** Store keys in the OS keychain instead of a `.env` file.
- **Content-based routing and cost tracking.** Both are explicitly v2 in the
  spec.
