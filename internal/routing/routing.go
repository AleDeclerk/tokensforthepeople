// Package routing turns a (use case, priority) pair into an ordered chain
// of (provider, model) steps. It is the single source of truth for the
// decision matrix documented in docs/wizard.md.
//
// The chain returned here is unconditional — it does not know which API
// keys the user has. Callers (wizard, config emitters) filter by available
// providers downstream.
package routing

import "fmt"

// UseCase is the high-level intent picked on screen 1 of the wizard.
type UseCase string

const (
	UseCaseCodingAgent UseCase = "coding-agent"
	UseCaseGeneralChat UseCase = "general-chat"
	UseCaseAgentic     UseCase = "agentic"
	UseCaseRAG         UseCase = "rag"
	UseCaseOther       UseCase = "other"
)

// Priority is the tradeoff picked on screen 2.
type Priority string

const (
	PriorityQuality  Priority = "quality"
	PriorityLatency  Priority = "latency"
	PriorityBalanced Priority = "balanced"
	PriorityPrivacy  Priority = "privacy"
)

// Provider is the LLM service we route to.
type Provider string

const (
	ProviderGemini      Provider = "gemini"
	ProviderGroq        Provider = "groq"
	ProviderOpenRouter  Provider = "openrouter"
	ProviderOllamaCloud Provider = "ollama"
	ProviderCerebras    Provider = "cerebras"
)

// Step is one entry in the fallback chain. Model uses LiteLLM's canonical
// "provider/model-id" naming so configs render cleanly.
type Step struct {
	Provider Provider
	Model    string
}

// BuildChain returns the ordered fallback chain for the given pair.
// The mapping mirrors the table in docs/wizard.md; every change here must
// update the doc and vice versa (the tests pin both).
func BuildChain(uc UseCase, pr Priority) ([]Step, error) {
	if !isKnownUseCase(uc) {
		return nil, fmt.Errorf("unknown use case %q", uc)
	}
	if !isKnownPriority(pr) {
		return nil, fmt.Errorf("unknown priority %q", pr)
	}

	// Agentic ignores priority — tool calling reliability dictates the chain.
	if uc == UseCaseAgentic {
		return []Step{
			{ProviderGemini, "gemini/gemini-2.5-flash"},
			{ProviderGroq, "groq/llama-3.3-70b-versatile"},
			{ProviderOpenRouter, "openrouter/deepseek/deepseek-v4-flash:free"},
		}, nil
	}

	// RAG prefers long context; priority only nudges the fallback order.
	if uc == UseCaseRAG {
		return []Step{
			{ProviderGemini, "gemini/gemini-2.5-flash"},
			{ProviderOpenRouter, "openrouter/deepseek/deepseek-v4-flash:free"},
			{ProviderGroq, "groq/llama-3.3-70b-versatile"},
		}, nil
	}

	key := matrixKey{useCase: uc, priority: pr}
	chain, ok := decisionMatrix[key]
	if !ok {
		// Fall back to balanced if a (uc, priority) pair is undefined. Keeps
		// the wizard from crashing on edge combos; tests pin the documented
		// rows so this branch only fires for "other" + niche priorities.
		fallback, ok := decisionMatrix[matrixKey{useCase: uc, priority: PriorityBalanced}]
		if !ok {
			return nil, fmt.Errorf("no chain defined for (%s, %s)", uc, pr)
		}
		return fallback, nil
	}
	return chain, nil
}

// ── internals ────────────────────────────────────────────────────────────

type matrixKey struct {
	useCase  UseCase
	priority Priority
}

// decisionMatrix lives here as data, not branches, so the diff between
// `docs/wizard.md` and reality is one PR away from being a single table.
var decisionMatrix = map[matrixKey][]Step{
	{UseCaseCodingAgent, PriorityQuality}: {
		{ProviderGemini, "gemini/gemini-2.5-flash"},
		{ProviderOpenRouter, "openrouter/deepseek/deepseek-v4-flash:free"},
		{ProviderGroq, "groq/llama-3.3-70b-versatile"},
	},
	{UseCaseCodingAgent, PriorityLatency}: {
		{ProviderGroq, "groq/llama-3.3-70b-versatile"},
		{ProviderCerebras, "cerebras/llama-3.3-70b"},
		{ProviderGemini, "gemini/gemini-2.5-flash"},
	},
	{UseCaseCodingAgent, PriorityPrivacy}: {
		{ProviderOpenRouter, "openrouter/qwen/qwen-2.5-coder-32b-instruct:free"},
		{ProviderOpenRouter, "openrouter/deepseek/deepseek-v4-flash:free"},
		{ProviderOllamaCloud, "ollama/qwen-397b-instruct"},
	},
	{UseCaseCodingAgent, PriorityBalanced}: {
		{ProviderGemini, "gemini/gemini-2.5-flash"},
		{ProviderGroq, "groq/llama-3.3-70b-versatile"},
		{ProviderOpenRouter, "openrouter/deepseek/deepseek-v4-flash:free"},
	},
	{UseCaseGeneralChat, PriorityQuality}: {
		{ProviderGemini, "gemini/gemini-2.5-flash"},
		{ProviderOpenRouter, "openrouter/meta-llama/llama-3.3-70b-instruct:free"},
		{ProviderGroq, "groq/llama-3.3-70b-versatile"},
	},
	{UseCaseGeneralChat, PriorityLatency}: {
		{ProviderGroq, "groq/llama-3.1-8b-instant"},
		{ProviderGroq, "groq/llama-3.3-70b-versatile"},
		{ProviderGemini, "gemini/gemini-2.5-flash"},
	},
	{UseCaseGeneralChat, PriorityBalanced}: {
		{ProviderGemini, "gemini/gemini-2.5-flash"},
		{ProviderGroq, "groq/llama-3.3-70b-versatile"},
		{ProviderOpenRouter, "openrouter/deepseek/deepseek-v4-flash:free"},
	},
	{UseCaseGeneralChat, PriorityPrivacy}: {
		{ProviderOpenRouter, "openrouter/qwen/qwen-2.5-72b-instruct:free"},
		{ProviderOpenRouter, "openrouter/deepseek/deepseek-v4-flash:free"},
		{ProviderOllamaCloud, "ollama/qwen-397b-instruct"},
	},
}

func isKnownUseCase(uc UseCase) bool {
	switch uc {
	case UseCaseCodingAgent, UseCaseGeneralChat, UseCaseAgentic, UseCaseRAG, UseCaseOther:
		return true
	}
	return false
}

func isKnownPriority(pr Priority) bool {
	switch pr {
	case PriorityQuality, PriorityLatency, PriorityBalanced, PriorityPrivacy:
		return true
	}
	return false
}
