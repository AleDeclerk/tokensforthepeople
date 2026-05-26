# tokensforthepeople

> Free LLM tokens for the rest of us.

A small Go CLI that interviews you for 60 seconds and leaves you with a working
multi-provider LLM setup using the free tiers of Gemini, OpenRouter, Groq, and
Ollama Cloud — wired into your editor (Cline, Continue.dev, Aider) and/or a
local proxy.

**Status:** pre-alpha, in design. Wizard spec in [`docs/wizard.md`](docs/wizard.md).

## What it does

1. Asks you a handful of questions (use case, latency vs quality, privacy, target tool).
2. Validates the API keys you paste (live ping).
3. Emits ready-to-use configs:
   - `litellm_config.yaml` (for the LiteLLM proxy)
   - `cline_settings.json` (VSCode Cline extension)
   - `continue_config.json` (Continue.dev)
   - `aider.conf.yml` (Aider CLI)
4. Optionally runs `t4p serve` — a tiny OpenAI-compatible proxy in pure Go with
   fallback routing across the free tiers.

## What it is not

- Not a scraper of free ChatGPT/Bing endpoints (we don't do `g4f`-style ToS-violation).
- Not a model serving layer. Real inference is the providers'.
- Not a hosted SaaS. Everything runs on your machine.

## Free-tier matrix

Live, kept up-to-date by a GitHub Action that pings each provider every 6h.
See [`MATRIX.md`](MATRIX.md).

## Install

```bash
# Coming soon
brew install AleDeclerk/tap/t4p
```

## License

MIT. See [`LICENSE`](LICENSE).
