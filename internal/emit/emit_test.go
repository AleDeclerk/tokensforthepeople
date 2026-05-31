package emit_test

import (
	"strings"
	"testing"

	"github.com/AleDeclerk/tokensforthepeople/internal/emit"
	"github.com/AleDeclerk/tokensforthepeople/internal/routing"
)

// One snapshot-style test per emitter. We assert key shape, not byte
// equality, because the comment header includes a timestamp.

func TestContinueEmitter_codingAgent_quality(t *testing.T) {
	chain := mustChain(t, routing.UseCaseCodingAgent, routing.PriorityQuality)
	keys := map[string]string{
		"GEMINI_API_KEY":     "AIza_x",
		"OPENROUTER_API_KEY": "sk-or-x",
	}
	out, err := emit.Continue(chain, keys)
	if err != nil {
		t.Fatalf("Continue: %v", err)
	}
	for _, want := range []string{
		`"models"`,
		`"provider": "gemini"`,
		`"model": "gemini-2.5-flash"`,
		`"${GEMINI_API_KEY}"`,
		`"provider": "openrouter"`,
		`"deepseek/deepseek-v4-flash:free"`,
		`"${OPENROUTER_API_KEY}"`,
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
	// Groq step has no validated key — emitter must drop the step, not
	// leak a ${GROQ_API_KEY} placeholder.
	if strings.Contains(string(out), "GROQ_API_KEY") {
		t.Errorf("emitter leaked Groq step despite no key. Output:\n%s", out)
	}
}

func TestContinueEmitter_emptyKeys_returnsError(t *testing.T) {
	chain := mustChain(t, routing.UseCaseCodingAgent, routing.PriorityQuality)
	if _, err := emit.Continue(chain, map[string]string{}); err == nil {
		t.Error("expected error when no chain step has a validated key")
	}
}

func TestAiderEmitter_picksTopOfChain_withKey(t *testing.T) {
	chain := mustChain(t, routing.UseCaseCodingAgent, routing.PriorityQuality)
	keys := map[string]string{"GEMINI_API_KEY": "AIza_x"}
	out, err := emit.Aider(chain, keys)
	if err != nil {
		t.Fatalf("Aider: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "model:") || !strings.Contains(got, "gemini/gemini-2.5-flash") {
		t.Errorf("expected gemini at top. Got:\n%s", got)
	}
}

func TestAiderEmitter_skipsTop_whenNoKey_falsBackToNext(t *testing.T) {
	chain := mustChain(t, routing.UseCaseCodingAgent, routing.PriorityQuality)
	// No Gemini key, but OpenRouter is available — aider gets the second step.
	keys := map[string]string{"OPENROUTER_API_KEY": "sk-or"}
	out, err := emit.Aider(chain, keys)
	if err != nil {
		t.Fatalf("Aider: %v", err)
	}
	got := string(out)
	if strings.Contains(got, "gemini/gemini-2.5-flash") {
		t.Errorf("aider should not pick gemini without key. Got:\n%s", got)
	}
	if !strings.Contains(got, "deepseek/deepseek-v4-flash:free") {
		t.Errorf("aider should fall back to deepseek. Got:\n%s", got)
	}
}

func TestLiteLLMEmitter_emitsFullChainAsSharedModelName(t *testing.T) {
	chain := mustChain(t, routing.UseCaseCodingAgent, routing.PriorityQuality)
	// Three providers that are all in the coding/quality chain, so the filtered
	// LiteLLM config carries exactly three shared-name entries.
	keys := map[string]string{
		"GEMINI_API_KEY":     "AIza_x",
		"MISTRAL_API_KEY":    "mst_x",
		"OPENROUTER_API_KEY": "sk-or-x",
	}
	out, err := emit.LiteLLM(chain, keys)
	if err != nil {
		t.Fatalf("LiteLLM: %v", err)
	}
	got := string(out)
	for _, want := range []string{
		"model_list:",
		"model_name: smart",
		"gemini/gemini-2.5-flash",
		"mistral/codestral-latest",
		"openrouter/deepseek/deepseek-v4-flash:free",
		"os.environ/GEMINI_API_KEY",
		"os.environ/MISTRAL_API_KEY",
		"os.environ/OPENROUTER_API_KEY",
		"router_settings:",
		"fallbacks:",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
	// Every step should appear under the same model_name so LiteLLM's
	// router does round-robin/fallback across them.
	if got, want := strings.Count(got, "model_name: smart"), 3; got != want {
		t.Errorf("got %d 'model_name: smart' lines, want %d", got, want)
	}
}

func TestClineEmitter_emitsSnippetWithInstructions(t *testing.T) {
	chain := mustChain(t, routing.UseCaseCodingAgent, routing.PriorityQuality)
	keys := map[string]string{"GEMINI_API_KEY": "AIza_x"}
	out, err := emit.Cline(chain, keys)
	if err != nil {
		t.Fatalf("Cline: %v", err)
	}
	got := string(out)
	for _, want := range []string{
		"VSCode", // instructions reference VSCode
		"settings.json",
		"cline.", // some cline.* setting key
		"gemini",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────

func mustChain(t *testing.T, uc routing.UseCase, pr routing.Priority) []routing.Step {
	t.Helper()
	c, err := routing.BuildChain(uc, pr)
	if err != nil {
		t.Fatalf("BuildChain: %v", err)
	}
	return c
}
