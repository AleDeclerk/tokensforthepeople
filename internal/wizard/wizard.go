// Package wizard runs the t4p init interactive flow.
//
// Screen 1 picks the use case; screen 2 picks the latency/quality/privacy
// tradeoff. The selections feed routing.BuildChain to produce the ordered
// fallback list shown back to the user. Subsequent slices (keys, target
// tools, write) build on top of these answers.
package wizard

import (
	"fmt"
	"io"

	"github.com/charmbracelet/huh"

	"github.com/AleDeclerk/tokensforthepeople/internal/routing"
)

// Answers is the strongly-typed result of one wizard run. Keeping it small
// and serializable lets us snapshot it for tests and for replay/debugging.
type Answers struct {
	UseCase  routing.UseCase
	Priority routing.Priority
}

// Run drives the interactive prompts and returns the user's selections.
// On EOF or ctrl-C the underlying form returns huh.ErrUserAborted, which
// we surface verbatim so callers can exit cleanly with a non-zero code.
func Run() (Answers, error) {
	var a Answers

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

	if err := form.Run(); err != nil {
		return a, err
	}
	return a, nil
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
		fmt.Fprintf(w, "  %d. %s\n", i+1, step.Model)
	}
	fmt.Fprintln(w, "\nNext: t4p init --keys (not implemented yet)")
	return nil
}
