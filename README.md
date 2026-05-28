# tokensforthepeople

> Free LLM tokens for the rest of us.

[![ci](https://github.com/AleDeclerk/tokensforthepeople/actions/workflows/ci.yml/badge.svg)](https://github.com/AleDeclerk/tokensforthepeople/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/AleDeclerk/tokensforthepeople?include_prereleases&label=release)](https://github.com/AleDeclerk/tokensforthepeople/releases/latest)
[![license](https://img.shields.io/github/license/AleDeclerk/tokensforthepeople)](./LICENSE)
[![go report](https://goreportcard.com/badge/github.com/AleDeclerk/tokensforthepeople)](https://goreportcard.com/report/github.com/AleDeclerk/tokensforthepeople)

A small Go CLI that interviews you for ~60 seconds and leaves you with a
working multi-provider LLM setup wired into your editor (Cline, Continue.dev,
Aider) and/or a local LiteLLM proxy, all using free tiers of Gemini,
OpenRouter, Groq, Ollama Cloud, and Cerebras.

```
$ t4p init
? What's your main use case?           > Code editor / coding agent
? What matters more?                   > Quality
? Which keys do you already have?      [x] Gemini  [x] OpenRouter  [ ] Groq
? Paste your Gemini API key            ********** ✓ valid
? Paste your OpenRouter API key        ********** ✓ valid
? Where should I write the config?     [x] Continue.dev  [x] Aider

✓ wrote ~/.config/t4p/keys.env (chmod 600)
✓ created ~/.continue/config.json
✓ created ~/.aider.conf.yml

Done. Reopen your editor and you're on free LLMs.
```

## Install

### Homebrew (macOS / Linux)

```bash
brew install AleDeclerk/tap/t4p
```

### `go install`

```bash
go install github.com/AleDeclerk/tokensforthepeople/cmd/t4p@latest
```

### Direct binary download

Grab the tarball for your platform from the
[latest release](https://github.com/AleDeclerk/tokensforthepeople/releases/latest):

```bash
# macOS Apple Silicon — adjust os/arch for your machine
VER=$(curl -s https://api.github.com/repos/AleDeclerk/tokensforthepeople/releases/latest \
        | grep '"tag_name"' | cut -d'"' -f4 | sed 's/^v//')
curl -L "https://github.com/AleDeclerk/tokensforthepeople/releases/download/v${VER}/t4p_${VER}_darwin_arm64.tar.gz" \
  | tar xz
./t4p init
```

### From source

```bash
git clone https://github.com/AleDeclerk/tokensforthepeople.git
cd tokensforthepeople
go build -o t4p ./cmd/t4p
./t4p init
```

Requires Go 1.25 or newer. No CGO, no external runtime dependencies.

## What it does

1. Asks you a handful of questions (use case, latency vs quality, privacy).
2. Validates the API keys you paste against each provider's `GET /models`
   endpoint — zero quota burn, ~50ms per provider.
3. Maps your answers to an ordered fallback chain (the
   [decision matrix](./docs/wizard.md#routing-decision-matrix) is the
   opinionated part).
4. Emits ready-to-use configs:
   - `litellm_config.yaml` (for the LiteLLM proxy)
   - `~/.continue/config.json` (Continue.dev)
   - `~/.aider.conf.yml` (Aider CLI)
   - `cline-snippet.json` (paste into VSCode settings for Cline)

Keys live in `~/.config/t4p/keys.env` (chmod 600). Configs reference them as
`${VAR}` placeholders so you can rotate keys without touching configs.

## What it is not

- **Not a free-endpoint scraper.** We don't do `g4f`-style ToS-violation
  proxying of paid services. Real provider keys only.
- **Not a model serving layer.** Real inference runs at the providers.
- **Not a SaaS.** Everything runs on your machine. Zero telemetry, zero
  phone-home.

## Non-interactive mode

For CI, dotfile bootstraps, or scripts:

```bash
t4p init \
  --use-case=coding-agent \
  --priority=quality \
  --key=gemini=$GEMINI_API_KEY \
  --key=openrouter=$OPENROUTER_API_KEY \
  --targets=continue,aider,litellm \
  --write
```

Use cases: `coding-agent | general-chat | agentic | rag | other`.
Priorities: `quality | latency | balanced | privacy`.
Targets: `cline | continue | aider | litellm`.

## Free-tier matrix

The wizard's routing decisions are pinned to a curated table of free-tier
quotas and capabilities for each provider. See
[`docs/wizard.md`](./docs/wizard.md#routing-decision-matrix).

A scheduled health-checker that updates the matrix automatically is on the
roadmap.

## Project layout

```
cmd/t4p/                Main package — CLI dispatcher
internal/routing/       Decision matrix (use case + priority -> chain)
internal/providers/     Static metadata per provider (endpoint, env var, ...)
internal/validation/    GET /models live key validation
internal/keystore/      Atomic dotenv write (chmod 600 + backups)
internal/emit/          Per-target config emitters (pure functions)
internal/tools/         Detect installed tools (Cline/Continue/Aider/LiteLLM)
internal/wizard/        TUI screens 1..4 + the orchestrator
docs/wizard.md          Design spec — the source of truth for behavior
```

## Contributing

Bug reports and PRs welcome. Before submitting:

```bash
go test ./...
go vet ./...
```

CI runs the same on Linux and macOS for every PR.

## License

MIT. See [`LICENSE`](./LICENSE).
