package validation_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AleDeclerk/tokensforthepeople/internal/providers"
	"github.com/AleDeclerk/tokensforthepeople/internal/routing"
	"github.com/AleDeclerk/tokensforthepeople/internal/validation"
)

// Tests run against an httptest.Server, not the real providers. That keeps
// CI offline-friendly and lets us pin behavior for every status code we
// care about. The validator accepts an override URL so we can point a real
// provider's metadata at our test server.

func TestPing_bearer_validKey(t *testing.T) {
	gotAuth := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := mustFind(t, routing.ProviderGroq)
	p.Endpoint = srv.URL

	result, err := validation.Ping(p, "gsk_real_key", 2*time.Second)
	if err != nil {
		t.Fatalf("Ping returned error: %v", err)
	}
	if result.Status != validation.StatusOK {
		t.Errorf("status: got %v, want %v", result.Status, validation.StatusOK)
	}
	if gotAuth != "Bearer gsk_real_key" {
		t.Errorf("auth header: got %q, want Bearer gsk_real_key", gotAuth)
	}
}

func TestPing_queryParam_validKey(t *testing.T) {
	gotKey := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.URL.Query().Get("key")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := mustFind(t, routing.ProviderGemini)
	p.Endpoint = srv.URL

	result, err := validation.Ping(p, "AIza_test", 2*time.Second)
	if err != nil {
		t.Fatalf("Ping returned error: %v", err)
	}
	if result.Status != validation.StatusOK {
		t.Errorf("status: got %v, want OK", result.Status)
	}
	if gotKey != "AIza_test" {
		t.Errorf("query param: got %q, want AIza_test", gotKey)
	}
	// The validator must not also send a bearer header when in query-param mode.
}

func TestPing_invalidKey_returnsInvalid(t *testing.T) {
	// 400 is Gemini's actual response for "API key not valid"; 401/403 cover
	// every other provider. The wizard must treat all three identically.
	for _, code := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(code)
			}))
			defer srv.Close()
			p := mustFind(t, routing.ProviderGroq)
			p.Endpoint = srv.URL
			result, err := validation.Ping(p, "bad", 2*time.Second)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Status != validation.StatusInvalid {
				t.Errorf("status: got %v, want Invalid", result.Status)
			}
		})
	}
}

func TestPing_quotaExceeded_returnsQuota(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	p := mustFind(t, routing.ProviderGroq)
	p.Endpoint = srv.URL

	result, err := validation.Ping(p, "ok-but-throttled", 2*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != validation.StatusQuotaExceeded {
		t.Errorf("status: got %v, want QuotaExceeded", result.Status)
	}
}

func TestPing_networkError_returnsNetworkError(t *testing.T) {
	p := mustFind(t, routing.ProviderGroq)
	// Reserved test address that should always refuse / hang.
	p.Endpoint = "http://127.0.0.1:1"

	result, err := validation.Ping(p, "x", 500*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != validation.StatusNetworkError {
		t.Errorf("status: got %v, want NetworkError", result.Status)
	}
}

func TestPing_emptyKey_returnsInvalidWithoutCalling(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	p := mustFind(t, routing.ProviderGroq)
	p.Endpoint = srv.URL

	result, _ := validation.Ping(p, "", 2*time.Second)
	if called {
		t.Error("Ping must short-circuit on empty key without making a request")
	}
	if result.Status != validation.StatusInvalid {
		t.Errorf("status: got %v, want Invalid", result.Status)
	}
}

func TestPing_chatProbe_validKey(t *testing.T) {
	var gotMethod, gotAuth, gotCT string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := mustFind(t, routing.ProviderGitHub) // ValidateChatProbe, ProbeModel set
	p.Endpoint = srv.URL

	result, err := validation.Ping(p, "ghp_real", 2*time.Second)
	if err != nil {
		t.Fatalf("Ping returned error: %v", err)
	}
	if result.Status != validation.StatusOK {
		t.Errorf("status: got %v, want OK", result.Status)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method: got %q, want POST", gotMethod)
	}
	if gotAuth != "Bearer ghp_real" {
		t.Errorf("auth: got %q, want Bearer ghp_real", gotAuth)
	}
	if gotCT != "application/json" {
		t.Errorf("content-type: got %q, want application/json", gotCT)
	}
	if body["model"] != p.ProbeModel {
		t.Errorf("body model: got %v, want %q", body["model"], p.ProbeModel)
	}
	if mt, ok := body["max_tokens"].(float64); !ok || mt != 1 {
		t.Errorf("body max_tokens: got %v, want 1", body["max_tokens"])
	}
}

func TestPing_chatProbe_invalidKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	p := mustFind(t, routing.ProviderZAI) // ValidateChatProbe
	p.Endpoint = srv.URL

	result, err := validation.Ping(p, "bad", 2*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != validation.StatusInvalid {
		t.Errorf("status: got %v, want Invalid", result.Status)
	}
}

func mustFind(t *testing.T, id routing.Provider) providers.Provider {
	t.Helper()
	p, ok := providers.ByID(id)
	if !ok {
		t.Fatalf("provider %q not in providers.All", id)
	}
	return p
}
