package wizard_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/AleDeclerk/tokensforthepeople/internal/routing"
	"github.com/AleDeclerk/tokensforthepeople/internal/wizard"
)

// PrintChain is the only piece we can test without a TTY. The TUI itself
// is huh's responsibility.

func TestPrintChain_codingAgent_quality_noKeys(t *testing.T) {
	var buf bytes.Buffer
	err := wizard.PrintChain(&buf, wizard.Answers{
		UseCase:  routing.UseCaseCodingAgent,
		Priority: routing.PriorityQuality,
	})
	if err != nil {
		t.Fatalf("PrintChain: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"coding-agent",
		"quality",
		"gemini/gemini-2.5-flash",
		"mistral/codestral-latest",
		"github/openai/gpt-4.1",
		"nvidia_nim/qwen/qwen3-coder-480b-a35b-instruct",
		"openrouter/deepseek/deepseek-v4-flash:free",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q. Got:\n%s", want, out)
		}
	}
	// Every step lacks a key so all five should be flagged with "×".
	if got := strings.Count(out, "×"); got != 5 {
		t.Errorf("expected 5 missing-key markers, got %d. Output:\n%s", got, out)
	}
}

func TestPrintChain_withSomeKeys_showsValidatedSection(t *testing.T) {
	var buf bytes.Buffer
	err := wizard.PrintChain(&buf, wizard.Answers{
		UseCase:  routing.UseCaseCodingAgent,
		Priority: routing.PriorityQuality,
		Keys: map[string]string{
			"GEMINI_API_KEY": "AIza_x",
			"GROQ_API_KEY":   "gsk_y",
		},
	})
	if err != nil {
		t.Fatalf("PrintChain: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Validated keys: 2") {
		t.Errorf("expected validated-keys section. Got:\n%s", out)
	}
	if !strings.Contains(out, "✓ Gemini") || !strings.Contains(out, "✓ Groq") {
		t.Errorf("expected ✓ markers per provider. Got:\n%s", out)
	}
	// Only Gemini (first step) has a key in the coding/quality chain; the other
	// four steps (Mistral, GitHub, NVIDIA, OpenRouter) lack keys → four ×.
	if got := strings.Count(out, "×"); got != 4 {
		t.Errorf("expected 4 missing-key markers, got %d. Output:\n%s", got, out)
	}
}

func TestPrintChain_unknownUseCase_returnsError(t *testing.T) {
	var buf bytes.Buffer
	err := wizard.PrintChain(&buf, wizard.Answers{
		UseCase:  routing.UseCase("nope"),
		Priority: routing.PriorityQuality,
	})
	if err == nil {
		t.Fatal("expected error for unknown use case")
	}
}
