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

func TestProvidersForPlan_recommendsChainProviders(t *testing.T) {
	a := &API{}
	got := a.ProvidersForPlan("coding-agent", "quality")
	if len(got) != len(a.ListProviders()) {
		t.Fatalf("ProvidersForPlan returned %d, want all %d providers", len(got), len(a.ListProviders()))
	}

	rec := map[string]bool{}
	for _, p := range got {
		rec[p.ID] = p.Recommended
	}
	// coding/quality chain = gemini, mistral, github, nvidia, openrouter.
	for _, id := range []string{"gemini", "mistral", "github", "nvidia", "openrouter"} {
		if !rec[id] {
			t.Errorf("provider %q should be Recommended for coding/quality", id)
		}
	}
	// groq is not in that chain.
	if rec["groq"] {
		t.Error("groq should not be Recommended for coding/quality")
	}

	// Recommended providers must sort before the rest.
	seenNonRec := false
	for _, p := range got {
		if !p.Recommended {
			seenNonRec = true
		} else if seenNonRec {
			t.Errorf("recommended provider %q appeared after a non-recommended one", p.ID)
		}
	}
}

func TestProvidersForPlan_unknownPair_noneRecommended(t *testing.T) {
	a := &API{}
	for _, p := range a.ProvidersForPlan("nope", "nope") {
		if p.Recommended {
			t.Errorf("provider %q recommended for unknown pair", p.ID)
		}
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
