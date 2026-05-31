package main

import (
	"testing"

	"github.com/AleDeclerk/tokensforthepeople/internal/emit"
	"github.com/AleDeclerk/tokensforthepeople/internal/routing"
)

func TestListProviders_nonEmptyWithHints(t *testing.T) {
	a := &API{}
	got := a.ListProviders()
	if len(got) == 0 {
		t.Fatal("ListProviders returned nothing")
	}
	var gemini *ProviderInfo
	for i := range got {
		if got[i].ID == string(routing.ProviderGemini) {
			gemini = &got[i]
		}
	}
	if gemini == nil {
		t.Fatal("Gemini not in provider list")
	}
	if gemini.SignupURL == "" {
		t.Error("Gemini SignupURL empty")
	}
	if !gemini.Easiest {
		t.Error("Gemini should be marked Easiest")
	}
}

func TestBuildPlan_returnsChain(t *testing.T) {
	a := &API{}
	steps, err := a.BuildPlan("coding-agent", "quality")
	if err != "" {
		t.Fatalf("BuildPlan error: %s", err)
	}
	if len(steps) == 0 {
		t.Fatal("expected a non-empty chain")
	}
}

func TestBuildPlan_unknown_returnsError(t *testing.T) {
	a := &API{}
	if _, err := a.BuildPlan("nope", "quality"); err == "" {
		t.Fatal("expected error string for unknown use case")
	}
}

func TestDetectTargets_returnsAllKnown(t *testing.T) {
	a := &API{}
	got := a.DetectTargets()
	if len(got) != len(emit.All) {
		t.Fatalf("DetectTargets returned %d, want %d", len(got), len(emit.All))
	}
}

func TestWriteConfigs_noKeys_returnsError(t *testing.T) {
	a := &API{}
	res := a.WriteConfigs(WriteRequest{UseCase: "coding-agent", Priority: "quality"})
	if res.Error == "" {
		t.Fatal("expected error when no keys")
	}
}

func TestWriteConfigs_writesContinue(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	a := &API{}
	res := a.WriteConfigs(WriteRequest{
		UseCase:  "coding-agent",
		Priority: "quality",
		Keys:     map[string]string{"GEMINI_API_KEY": "AIza-test"},
		Targets:  []string{"continue"},
	})
	if res.Error != "" {
		t.Fatalf("unexpected error: %s", res.Error)
	}
	if len(res.Targets) != 1 || !res.Targets[0].OK {
		t.Fatalf("expected 1 ok target, got %+v", res.Targets)
	}
}

func TestValidateKey_unknownProvider(t *testing.T) {
	a := &API{}
	if got := a.ValidateKey("nope", "x"); got.Status != "error" {
		t.Fatalf("status = %q, want error", got.Status)
	}
}
