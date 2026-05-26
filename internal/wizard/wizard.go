// Package wizard runs the t4p init interactive flow.
//
// Screens 1+2 pick use case and priority; screen 3 collects + live-validates
// provider keys; screen 4 multi-selects target tools with detection-driven
// defaults; screen 5 previews and confirms the writes. Each screen feeds
// later ones via the shared Answers struct.
package wizard

import (
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/charmbracelet/huh"

	"github.com/AleDeclerk/tokensforthepeople/internal/emit"
	"github.com/AleDeclerk/tokensforthepeople/internal/providers"
	"github.com/AleDeclerk/tokensforthepeople/internal/routing"
	"github.com/AleDeclerk/tokensforthepeople/internal/tools"
	"github.com/AleDeclerk/tokensforthepeople/internal/validation"
)

// validationTimeout is short enough that the wizard never feels stuck and
// long enough that a healthy provider responds. Five seconds is a 50x
// margin over the typical ~50–100ms response.
const validationTimeout = 5 * time.Second

// Answers is the strongly-typed result of one wizard run.
type Answers struct {
	UseCase  routing.UseCase
	Priority routing.Priority

	// Keys is the validated set: ENV_VAR_NAME -> key string. Only keys
	// that pinged successfully (OK or QuotaExceeded) make it here.
	Keys map[string]string

	// Targets are the downstream tools the user wants configs written for.
	Targets []emit.Target
}

// Run drives the interactive prompts and returns the user's selections.
func Run() (Answers, error) {
	a := Answers{Keys: map[string]string{}}

	if err := runIntro(&a); err != nil {
		return a, err
	}
	if err := runKeys(&a); err != nil {
		return a, err
	}
	if err := runTargets(&a); err != nil {
		return a, err
	}
	return a, nil
}

func runIntro(a *Answers) error {
	form := huh.NewForm(
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
	return form.Run()
}

func runKeys(a *Answers) error {
	var selected []routing.Provider
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[routing.Provider]().
				Title("Which keys do you already have?").
				Description("Tick the providers you have keys for. We'll validate each one live.\nLeave blank to skip — you can rerun later.").
				Options(providerOptions()...).
				Value(&selected),
		),
	)
	if err := form.Run(); err != nil {
		return err
	}
	for _, id := range selected {
		p, ok := providers.ByID(id)
		if !ok {
			continue
		}
		key, err := promptAndValidate(p)
		if err != nil {
			return err
		}
		if key != "" {
			a.Keys[p.EnvVar] = key
		}
	}
	return nil
}

func runTargets(a *Answers) error {
	detector := tools.DefaultDetector()
	detected := detector.Detect()

	// Pre-check detected tools so the wizard meets the user where they are.
	preChecked := make([]emit.Target, 0, len(detected))
	for _, t := range detected {
		if t.Installed {
			preChecked = append(preChecked, t.Target)
		}
	}
	a.Targets = preChecked

	opts := make([]huh.Option[emit.Target], 0, len(detected))
	for _, t := range detected {
		suffix := "    not detected"
		if t.Installed {
			suffix = "    ✓ detected"
		}
		opts = append(opts, huh.NewOption(fmt.Sprintf("%-18s%s", t.Display, suffix), t.Target))
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[emit.Target]().
				Title("Where should I write the config?").
				Description("Detected tools are pre-checked. Files get a .t4p.bak before any overwrite.").
				Options(opts...).
				Value(&a.Targets),
		),
	)
	return form.Run()
}

// providerOptions renders providers.All as huh options.
func providerOptions() []huh.Option[routing.Provider] {
	opts := make([]huh.Option[routing.Provider], 0, len(providers.All))
	for _, p := range providers.All {
		label := fmt.Sprintf("%s  (signup: %s)", p.Display, p.SignupURL)
		opts = append(opts, huh.NewOption(label, p.ID))
	}
	return opts
}

func promptAndValidate(p providers.Provider) (string, error) {
	var key string
	input := huh.NewInput().
		Title(fmt.Sprintf("%s API key", p.Display)).
		Description(fmt.Sprintf("Paste your key. Get one at %s.\nLeave blank to skip.", p.SignupURL)).
		EchoMode(huh.EchoModePassword).
		Validate(func(s string) error {
			if s == "" {
				return nil
			}
			res, err := validation.Ping(p, s, validationTimeout)
			if err != nil {
				return fmt.Errorf("%s validation error: %w", p.Display, err)
			}
			switch res.Status {
			case validation.StatusOK, validation.StatusQuotaExceeded:
				return nil
			case validation.StatusInvalid:
				return fmt.Errorf("invalid %s key (HTTP %d) — re-paste or leave blank", p.Display, res.HTTPStatus)
			case validation.StatusNetworkError:
				return fmt.Errorf("could not reach %s (%s) — check your internet", p.Display, res.Detail)
			}
			return nil
		}).
		Value(&key)
	if err := input.Run(); err != nil {
		return "", err
	}
	return key, nil
}

// PrintChain writes a human-readable preview to w.
func PrintChain(w io.Writer, a Answers) error {
	chain, err := routing.BuildChain(a.UseCase, a.Priority)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "\nRouting for %q + %q:\n\n", a.UseCase, a.Priority)
	for i, step := range chain {
		marker := "  "
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
	if len(a.Targets) > 0 {
		// Sort for stable output regardless of multiselect order.
		targets := make([]string, len(a.Targets))
		for i, t := range a.Targets {
			targets[i] = string(t)
		}
		sort.Strings(targets)
		fmt.Fprintf(w, "\nTargets selected: %d\n", len(targets))
		for _, t := range targets {
			path, _ := emit.DefaultPath(emit.Target(t))
			fmt.Fprintf(w, "  → %s  (%s)\n", t, path)
		}
	}
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
