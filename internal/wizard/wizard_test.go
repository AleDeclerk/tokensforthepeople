package wizard_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/AleDeclerk/tokensforthepeople/internal/routing"
	"github.com/AleDeclerk/tokensforthepeople/internal/wizard"
)

// We don't test the TUI itself — huh has its own coverage. We test the
// non-interactive side (PrintChain) so a future change to the matrix
// surfaces in a deterministic diff.

func TestPrintChain_codingAgent_quality(t *testing.T) {
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
		"openrouter/deepseek/deepseek-v4-flash:free",
		"groq/llama-3.3-70b-versatile",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q. Got:\n%s", want, out)
		}
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
