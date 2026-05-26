// Package wizard runs the t4p init interactive flow.
//
// Screen 1 picks the use case; screen 2 picks the latency/quality/privacy
// tradeoff; screen 3 collects + live-validates the user's provider keys.
// The selections feed routing.BuildChain to produce the ordered fallback
// list shown back to the user.
package wizard

import (
	"fmt"
	"io"
	"time"

	"github.com/charmbracelet/huh"

	"github.com/AleDeclerk/tokensforthepeople/internal/providers"
	"github.com/AleDeclerk/tokensforthepeople/internal/routing"
	"github.com/AleDeclerk/tokensforthepeople/internal/validation"
)

// validationTimeout is short enough that the wizard never feels stuck and
// long enough that a healthy provider responds. Five seconds is a 50x
// margin over the typical ~50–100ms response.
const validationTimeout = 5 * time.Second

// Answers is the strongly-typed result of one wizard run. Keeping it small
// and serializable lets us snapshot it for tests and for replay/debugging.
type Answers struct {
	UseCase  routing.UseCase
	Priority routing.Priority

	// Keys is the validated set: ENV_VAR_NAME -> key string. Only keys
	// that pinged successfully (OK or QuotaExceeded) make it here.
	Keys map[string]string
}

// Run drives the interactive prompts and returns the user's selections.
// On EOF or ctrl-C the underlying form returns huh.ErrUserAborted, which
// we surface verbatim so callers can exit cleanly with a non-zero code.
func Run() (Answers, error) {
	a := Answers{Keys: map[string]string{}}

	// Screens 1+2 — use case and priority.
	intro := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[routing.UseCase]().
				Title("What's your main use case?").
				Description("Picks the chain. You can rerun the wizard anytime.").
				Options(
					huh.NewOption("Code editor / coding agent  (Cline, Continue, Aider)", routing.UseCaseCodingAgent),
					huh.NewOption("General chat                (ChatGPT-like UI)", routing.UseCaseGeneralChat),
					huh.NewOption("Agentic workflows           (LangChain / CrewAI / custom)", routing.UseCaseAgentic),
					huh.NewOption("RAG / Q&A over docs", routing.UseCaseRAG),
					huh.NewOption("Other / advanced", routing.UseCaseOther),
				).
				Value(&a.UseCase),
		),
		huh.NewGroup(
			huh.NewSelect[routing.Priority]().
				Title("What matters more?").
				Description("Sets the primary provider. Fallbacks fire on quota or 5xx.").
				Options(
					huh.NewOption("Quality          (slower, smarter — Gemini first)", routing.PriorityQuality),
					huh.NewOption("Latency          (snappy autocomplete — Groq first)", routing.PriorityLatency),
					huh.NewOption("Balanced", routing.PriorityBalanced),
					huh.NewOption("Privacy first    (avoid US providers — DeepSeek/Qwen first)", routing.PriorityPrivacy),
				).
				Value(&a.Priority),
		),
	)
	if err := intro.Run(); err != nil {
		return a, err
	}

	// Screen 3a — which providers do you have keys for?
	var selected []routing.Provider
	pickKeys := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[routing.Provider]().
				Title("Which keys do you already have?").
				Description("Tick the providers you have keys for. We'll validate each one live.\nLeave blank to skip — you can rerun later.").
				Options(providerOptions()...).
				Value(&selected),
		),
	)
	if err := pickKeys.Run(); err != nil {
		return a, err
	}

	// Screen 3b — paste + live-validate each selected key.
	for _, id := range selected {
		p, ok := providers.ByID(id)
		if !ok {
			continue
		}
		key, err := promptAndValidate(p)
		if err != nil {
			return a, err
		}
		if key != "" {
			a.Keys[p.EnvVar] = key
		}
	}

	return a, nil
}

// providerOptions renders providers.All as huh options. Lives here so the
// wizard's order matches the canonical providers.All order.
func providerOptions() []huh.Option[routing.Provider] {
	opts := make([]huh.Option[routing.Provider], 0, len(providers.All))
	for _, p := range providers.All {
		label := fmt.Sprintf("%s  (signup: %s)", p.Display, p.SignupURL)
		opts = append(opts, huh.NewOption(label, p.ID))
	}
	return opts
}

// promptAndValidate runs one huh.Input that re-prompts on invalid keys
// using huh's built-in Validate hook. Returns the accepted key, or empty
// string if the user submitted blank.
func promptAndValidate(p providers.Provider) (string, error) {
	var key string
	input := huh.NewInput().
		Title(fmt.Sprintf("%s API key", p.Display)).
		Description(fmt.Sprintf("Paste your key. Get one at %s.\nLeave blank to skip.", p.SignupURL)).
		EchoMode(huh.EchoModePassword).
		Validate(func(s string) error {
			if s == "" {
				return nil // blank == skip
			}
			res, err := validation.Ping(p, s, validationTimeout)
			if err != nil {
				return fmt.Errorf("%s validation error: %w", p.Display, err)
			}
			switch res.Status {
			case validation.StatusOK:
				return nil
			case validation.StatusQuotaExceeded:
				// Accept the key but warn — routing will skip until recovers.
				return nil
			case validation.StatusInvalid:
				return fmt.Errorf("invalid %s key (HTTP %d) — re-paste or leave blank to skip", p.Display, res.HTTPStatus)
			case validation.StatusNetworkError:
				return fmt.Errorf("could not reach %s (%s) — check your internet, then re-paste or leave blank", p.Display, res.Detail)
			}
			return nil
		}).
		Value(&key)

	if err := input.Run(); err != nil {
		return "", err
	}
	return key, nil
}

// PrintChain writes a human-readable preview of the chain matching `a` to w.
// Used by `t4p init` to show the user what they just chose. The output is
// stable so we can snapshot-test it.
func PrintChain(w io.Writer, a Answers) error {
	chain, err := routing.BuildChain(a.UseCase, a.Priority)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "\nRouting for %q + %q:\n\n", a.UseCase, a.Priority)
	for i, step := range chain {
		marker := "  "
		// Mark providers we don't have keys for so the user sees the gap.
		if !hasKeyFor(a, step.Provider) {
			marker = "× "
		}
		fmt.Fprintf(w, "  %d. %s%s\n", i+1, marker, step.Model)
	}
	if len(a.Keys) > 0 {
		fmt.Fprintf(w, "\nValidated keys: %d\n", len(a.Keys))
		for _, p := range providers.All {
			if _, ok := a.Keys[p.EnvVar]; ok {
				fmt.Fprintf(w, "  ✓ %s\n", p.Display)
			}
		}
	}
	fmt.Fprintln(w, "\nNext: t4p init --write (writes configs — not implemented yet)")
	return nil
}

func hasKeyFor(a Answers, prov routing.Provider) bool {
	p, ok := providers.ByID(prov)
	if !ok {
		return false
	}
	_, has := a.Keys[p.EnvVar]
	return has
}
