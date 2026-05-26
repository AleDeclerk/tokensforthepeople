package routing_test

import (
	"testing"

	"github.com/AleDeclerk/tokensforthepeople/internal/routing"
)

// Each test pins one row of the decision matrix in docs/wizard.md so the
// matrix and the code can never silently drift.

func TestBuildChain_codingAgent_quality(t *testing.T) {
	chain, err := routing.BuildChain(routing.UseCaseCodingAgent, routing.PriorityQuality)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantModels := []string{
		"gemini/gemini-2.5-flash",
		"openrouter/deepseek/deepseek-v4-flash:free",
		"groq/llama-3.3-70b-versatile",
	}
	assertChain(t, chain, wantModels)
}

func TestBuildChain_codingAgent_latency(t *testing.T) {
	chain, err := routing.BuildChain(routing.UseCaseCodingAgent, routing.PriorityLatency)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Cerebras is conditional on having the key; without filtering it must
	// still appear in the canonical chain — the caller drops it later.
	wantModels := []string{
		"groq/llama-3.3-70b-versatile",
		"cerebras/llama-3.3-70b",
		"gemini/gemini-2.5-flash",
	}
	assertChain(t, chain, wantModels)
}

func TestBuildChain_codingAgent_privacy(t *testing.T) {
	chain, err := routing.BuildChain(routing.UseCaseCodingAgent, routing.PriorityPrivacy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantModels := []string{
		"openrouter/qwen/qwen-2.5-coder-32b-instruct:free",
		"openrouter/deepseek/deepseek-v4-flash:free",
		"ollama/qwen-397b-instruct",
	}
	assertChain(t, chain, wantModels)
}

func TestBuildChain_codingAgent_balanced(t *testing.T) {
	chain, err := routing.BuildChain(routing.UseCaseCodingAgent, routing.PriorityBalanced)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantModels := []string{
		"gemini/gemini-2.5-flash",
		"groq/llama-3.3-70b-versatile",
		"openrouter/deepseek/deepseek-v4-flash:free",
	}
	assertChain(t, chain, wantModels)
}

func TestBuildChain_generalChat_latency(t *testing.T) {
	chain, err := routing.BuildChain(routing.UseCaseGeneralChat, routing.PriorityLatency)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantModels := []string{
		"groq/llama-3.1-8b-instant",
		"groq/llama-3.3-70b-versatile",
		"gemini/gemini-2.5-flash",
	}
	assertChain(t, chain, wantModels)
}

func TestBuildChain_agentic_anyPriority(t *testing.T) {
	// Agentic ignores priority — tool calling is the only thing that matters,
	// and the chain is tuned for that.
	for _, p := range []routing.Priority{
		routing.PriorityQuality,
		routing.PriorityLatency,
		routing.PriorityBalanced,
		routing.PriorityPrivacy,
	} {
		chain, err := routing.BuildChain(routing.UseCaseAgentic, p)
		if err != nil {
			t.Fatalf("priority=%s: %v", p, err)
		}
		if got := chain[0].Model; got != "gemini/gemini-2.5-flash" {
			t.Errorf("priority=%s: agentic chain[0]=%q, want gemini-2.5-flash", p, got)
		}
	}
}

func TestBuildChain_rag_longContext(t *testing.T) {
	chain, err := routing.BuildChain(routing.UseCaseRAG, routing.PriorityQuality)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := chain[0].Model; got != "gemini/gemini-2.5-flash" {
		t.Errorf("RAG should lead with gemini (1M ctx), got %q", got)
	}
}

func TestBuildChain_unknownUseCase_returnsError(t *testing.T) {
	if _, err := routing.BuildChain(routing.UseCase("nope"), routing.PriorityQuality); err == nil {
		t.Fatal("expected error for unknown use case, got nil")
	}
}

func TestBuildChain_unknownPriority_returnsError(t *testing.T) {
	if _, err := routing.BuildChain(routing.UseCaseCodingAgent, routing.Priority("nope")); err == nil {
		t.Fatal("expected error for unknown priority, got nil")
	}
}

// ── helpers ───────────────────────────────────────────────────────────────

func assertChain(t *testing.T, got []routing.Step, wantModels []string) {
	t.Helper()
	if len(got) != len(wantModels) {
		t.Fatalf("chain length: got %d, want %d (chain=%+v)", len(got), len(wantModels), got)
	}
	for i, want := range wantModels {
		if got[i].Model != want {
			t.Errorf("chain[%d]: got %q, want %q", i, got[i].Model, want)
		}
	}
}
