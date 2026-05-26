// Package emit renders per-tool config files from a routing chain and the
// set of validated API keys.
//
// Each emitter is a pure function: (chain, keys) -> []byte. No filesystem
// I/O lives here — the caller writes the bytes. This keeps the emitters
// trivially testable and lets the wizard show a "preview before write".
//
// Design rules:
//   - Drop chain steps for providers without a validated key. Don't leak
//     placeholder env vars the user can't fulfill.
//   - Use env var placeholders, not raw key values, so config files are
//     safe to read on a 644 filesystem (real keys live in keys.env, 600).
//   - Be deterministic: same inputs produce same bytes (modulo the
//     timestamp in the header comment, which we keep stable per-call).
package emit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/AleDeclerk/tokensforthepeople/internal/providers"
	"github.com/AleDeclerk/tokensforthepeople/internal/routing"
)

// Target is the user-facing identifier for one emitter.
type Target string

const (
	TargetContinue Target = "continue"
	TargetAider    Target = "aider"
	TargetLiteLLM  Target = "litellm"
	TargetCline    Target = "cline"
)

// All lists every supported target in the order they appear in the wizard.
var All = []Target{TargetCline, TargetContinue, TargetAider, TargetLiteLLM}

// timeFunc is overridable in tests if we ever need byte-equal snapshots.
// Production calls use time.Now.
var timeFunc = time.Now

// ── Continue.dev ─────────────────────────────────────────────────────────

// Continue emits a Continue.dev config.json with one entry per chain step
// that has a validated key. Env var placeholders ${VAR} reference the
// keys.env file (Continue supports env interpolation in JSON values).
func Continue(chain []routing.Step, keys map[string]string) ([]byte, error) {
	available := filterAvailable(chain, keys)
	if len(available) == 0 {
		return nil, fmt.Errorf("continue: no chain step has a validated key")
	}

	type model struct {
		Title    string `json:"title"`
		Provider string `json:"provider"`
		Model    string `json:"model"`
		APIKey   string `json:"apiKey"`
	}
	type config struct {
		Models []model `json:"models"`
	}

	cfg := config{Models: make([]model, 0, len(available))}
	for _, step := range available {
		p, _ := providers.ByID(step.Provider)
		cfg.Models = append(cfg.Models, model{
			Title:    fmt.Sprintf("%s (%s)", step.Model, p.Display),
			Provider: string(step.Provider),
			Model:    stripProviderPrefix(step.Model, step.Provider),
			APIKey:   fmt.Sprintf("${%s}", p.EnvVar),
		})
	}

	var buf bytes.Buffer
	buf.WriteString(headerJSON())
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cfg); err != nil {
		return nil, fmt.Errorf("continue: encode: %w", err)
	}
	return buf.Bytes(), nil
}

// ── Aider ────────────────────────────────────────────────────────────────

// Aider only supports one model per session, so we pick the highest-ranked
// chain step the user has a key for. Aider reads env vars from the shell;
// the user is expected to source keys.env via direnv or `set -a; . keys.env`.
func Aider(chain []routing.Step, keys map[string]string) ([]byte, error) {
	available := filterAvailable(chain, keys)
	if len(available) == 0 {
		return nil, fmt.Errorf("aider: no chain step has a validated key")
	}
	top := available[0]

	var buf bytes.Buffer
	buf.WriteString(headerYAML())
	fmt.Fprintf(&buf, "model: %s\n", top.Model)
	buf.WriteString("# To load the matching key, run:\n")
	buf.WriteString("#   set -a && . ~/.config/t4p/keys.env && set +a\n")
	buf.WriteString("# or use direnv with an `.envrc` that sources the same file.\n")
	return buf.Bytes(), nil
}

// ── LiteLLM proxy ────────────────────────────────────────────────────────

// LiteLLM emits a config.yaml that the LiteLLM proxy consumes. Every chain
// step lives under the same model_name ("smart"), so LiteLLM's router does
// fallback across them in order.
func LiteLLM(chain []routing.Step, keys map[string]string) ([]byte, error) {
	available := filterAvailable(chain, keys)
	if len(available) == 0 {
		return nil, fmt.Errorf("litellm: no chain step has a validated key")
	}

	var buf bytes.Buffer
	buf.WriteString(headerYAML())
	buf.WriteString("model_list:\n")
	for _, step := range available {
		p, _ := providers.ByID(step.Provider)
		fmt.Fprintf(&buf, "  - model_name: smart\n")
		fmt.Fprintf(&buf, "    litellm_params:\n")
		fmt.Fprintf(&buf, "      model: %s\n", step.Model)
		fmt.Fprintf(&buf, "      api_key: os.environ/%s\n", p.EnvVar)
	}
	buf.WriteString("\nrouter_settings:\n")
	buf.WriteString("  routing_strategy: simple-shuffle\n")
	buf.WriteString("  num_retries: 1\n")
	buf.WriteString("  fallbacks:\n")
	buf.WriteString("    - smart: [smart]\n")
	return buf.Bytes(), nil
}

// ── Cline (snippet, not auto-merge) ──────────────────────────────────────

// Cline configures via VSCode user settings. We don't auto-merge into
// settings.json because schema details have moved several times — instead
// we emit a snippet the user pastes manually, with copy-paste instructions.
func Cline(chain []routing.Step, keys map[string]string) ([]byte, error) {
	available := filterAvailable(chain, keys)
	if len(available) == 0 {
		return nil, fmt.Errorf("cline: no chain step has a validated key")
	}
	top := available[0]
	p, _ := providers.ByID(top.Provider)

	var buf bytes.Buffer
	buf.WriteString("// Cline configuration snippet — paste into your VSCode settings.json.\n")
	buf.WriteString("// Open with: Cmd/Ctrl+Shift+P -> 'Preferences: Open User Settings (JSON)'.\n")
	buf.WriteString("//\n")
	buf.WriteString("// Cline accepts a single provider at a time. To use the full fallback chain,\n")
	buf.WriteString("// install LiteLLM and point Cline at the t4p proxy (t4p serve).\n\n")
	settings := map[string]string{
		"cline.provider": string(top.Provider),
		"cline.modelId":  stripProviderPrefix(top.Model, top.Provider),
		"cline.apiKey":   fmt.Sprintf("${%s}", p.EnvVar),
	}
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(settings); err != nil {
		return nil, fmt.Errorf("cline: encode: %w", err)
	}
	return buf.Bytes(), nil
}

// ── helpers ──────────────────────────────────────────────────────────────

// filterAvailable returns only the chain steps whose provider has a
// validated key in `keys`. Preserves chain order.
func filterAvailable(chain []routing.Step, keys map[string]string) []routing.Step {
	out := make([]routing.Step, 0, len(chain))
	for _, step := range chain {
		p, ok := providers.ByID(step.Provider)
		if !ok {
			continue
		}
		if _, has := keys[p.EnvVar]; !has {
			continue
		}
		out = append(out, step)
	}
	return out
}

// stripProviderPrefix turns LiteLLM-style "gemini/gemini-2.5-flash" into
// the bare model id "gemini-2.5-flash" that Cline and Continue expect.
func stripProviderPrefix(model string, prov routing.Provider) string {
	pfx := string(prov) + "/"
	if strings.HasPrefix(model, pfx) {
		return strings.TrimPrefix(model, pfx)
	}
	// LiteLLM has a few two-segment prefixes (e.g. "openrouter/deepseek/...").
	// In those cases we strip only the first segment.
	if i := strings.Index(model, "/"); i >= 0 {
		return model[i+1:]
	}
	return model
}

func headerJSON() string {
	return fmt.Sprintf("// Written by t4p init on %s. Edit at your own risk.\n",
		timeFunc().UTC().Format("2006-01-02T15:04:05Z"))
}

func headerYAML() string {
	return fmt.Sprintf("# Written by t4p init on %s. Edit at your own risk.\n",
		timeFunc().UTC().Format("2006-01-02T15:04:05Z"))
}
