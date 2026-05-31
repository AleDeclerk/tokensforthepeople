package providers_test

import (
	"testing"

	"github.com/AleDeclerk/tokensforthepeople/internal/providers"
	"github.com/AleDeclerk/tokensforthepeople/internal/routing"
)

func TestByID_known(t *testing.T) {
	// Every entry in the canonical list must be retrievable by its own ID and
	// round-trip to the same record.
	for _, want := range providers.All {
		got, ok := providers.ByID(want.ID)
		if !ok {
			t.Errorf("ByID(%q) not found, but it is in All", want.ID)
			continue
		}
		if got.ID != want.ID || got.EnvVar != want.EnvVar {
			t.Errorf("ByID(%q) = {ID:%q EnvVar:%q}, want {ID:%q EnvVar:%q}",
				want.ID, got.ID, got.EnvVar, want.ID, want.EnvVar)
		}
	}
}

func TestByID_unknown(t *testing.T) {
	if _, ok := providers.ByID(routing.Provider("nope")); ok {
		t.Fatal("ByID(\"nope\") reported found, want not found")
	}
}

// Every provider the routing matrix can emit must have metadata here, otherwise
// the wizard would route to a provider it can't validate or write keys for.
func TestEveryRoutingProviderHasMetadata(t *testing.T) {
	routed := []routing.Provider{
		routing.ProviderGemini,
		routing.ProviderGroq,
		routing.ProviderOpenRouter,
		routing.ProviderOllamaCloud,
		routing.ProviderCerebras,
		routing.ProviderMistral,
		routing.ProviderNVIDIA,
		routing.ProviderGitHub,
		routing.ProviderZAI,
	}
	for _, id := range routed {
		if _, ok := providers.ByID(id); !ok {
			t.Errorf("routing provider %q has no providers.Provider metadata", id)
		}
	}
}

// Chat-probe validation needs a model id to POST; GET validation must not carry
// a stray probe model. Pin both invariants so a misconfigured entry is caught
// before it reaches the live validator.
func TestAll_validationModeInvariants(t *testing.T) {
	for _, p := range providers.All {
		switch p.Validate {
		case providers.ValidateChatProbe:
			if p.ProbeModel == "" {
				t.Errorf("provider %q: ValidateChatProbe but empty ProbeModel", p.ID)
			}
		case providers.ValidateGET:
			if p.ProbeModel != "" {
				t.Errorf("provider %q: ValidateGET but ProbeModel=%q (should be empty)", p.ID, p.ProbeModel)
			}
		default:
			t.Errorf("provider %q: unknown Validate mode %d", p.ID, p.Validate)
		}
	}
}

// The table is the single source of truth rendered into wizard hints and the
// matrix doc; missing fields would surface as blank prompts or broken configs.
func TestAll_requiredFieldsPresent(t *testing.T) {
	for _, p := range providers.All {
		if p.ID == "" {
			t.Errorf("provider %q: empty ID", p.Display)
		}
		if p.Display == "" {
			t.Errorf("provider %q: empty Display", p.ID)
		}
		if p.SignupURL == "" {
			t.Errorf("provider %q: empty SignupURL", p.ID)
		}
		if p.EnvVar == "" {
			t.Errorf("provider %q: empty EnvVar", p.ID)
		}
		if p.Endpoint == "" {
			t.Errorf("provider %q: empty Endpoint", p.ID)
		}
		// A query-param auth scheme is meaningless without the param name.
		if p.AuthScheme == providers.AuthQueryParam && p.QueryParamName == "" {
			t.Errorf("provider %q: AuthQueryParam but empty QueryParamName", p.ID)
		}
	}
}

// IDs and env vars are used as map keys downstream (keystore, emitters); a
// duplicate would silently clobber another provider's key.
func TestAll_idsAndEnvVarsUnique(t *testing.T) {
	seenID := map[routing.Provider]bool{}
	seenEnv := map[string]bool{}
	for _, p := range providers.All {
		if seenID[p.ID] {
			t.Errorf("duplicate provider ID %q", p.ID)
		}
		if seenEnv[p.EnvVar] {
			t.Errorf("duplicate env var %q", p.EnvVar)
		}
		seenID[p.ID] = true
		seenEnv[p.EnvVar] = true
	}
}
