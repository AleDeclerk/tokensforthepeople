// Package validation runs zero-cost liveness checks against LLM providers.
//
// Every provider in providers.All exposes a GET endpoint that returns 2xx
// when the key is valid, 401/403 when invalid, 429 when over quota. We
// never call a chat-completion endpoint here — that would burn quota.
//
// Result statuses split the world into four buckets that the wizard maps
// to colored verdicts in the UI: OK, Invalid, QuotaExceeded, NetworkError.
package validation

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/AleDeclerk/tokensforthepeople/internal/providers"
)

// Status reflects what happened when we pinged the provider.
type Status int

const (
	// StatusOK — the key works and quota is not exhausted right now.
	StatusOK Status = iota
	// StatusInvalid — 401/403, or the caller passed an empty string.
	StatusInvalid
	// StatusQuotaExceeded — the key is valid but the provider returned 429.
	// We still save the key; routing will skip it until it recovers.
	StatusQuotaExceeded
	// StatusNetworkError — couldn't reach the provider at all.
	StatusNetworkError
)

// String makes statuses printable in test failures and logs.
func (s Status) String() string {
	switch s {
	case StatusOK:
		return "OK"
	case StatusInvalid:
		return "Invalid"
	case StatusQuotaExceeded:
		return "QuotaExceeded"
	case StatusNetworkError:
		return "NetworkError"
	}
	return fmt.Sprintf("Status(%d)", int(s))
}

// Result is what Ping returns. HTTPStatus is populated only when the call
// reached the server; Detail carries the network error string otherwise.
type Result struct {
	Status     Status
	HTTPStatus int
	Detail     string
}

// Ping issues a single zero-cost GET against the provider's metadata
// endpoint and returns a structured Result. The error return is reserved
// for programmer errors (e.g. malformed provider metadata); transport
// errors are folded into StatusNetworkError so callers can render them
// without `if err != nil` guards.
func Ping(p providers.Provider, key string, timeout time.Duration) (Result, error) {
	if key == "" {
		// Don't burn an HTTP round-trip just to be told the key is empty.
		return Result{Status: StatusInvalid, Detail: "empty key"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.Endpoint, nil)
	if err != nil {
		return Result{}, fmt.Errorf("build request: %w", err)
	}

	switch p.AuthScheme {
	case providers.AuthBearer:
		req.Header.Set("Authorization", "Bearer "+key)
	case providers.AuthQueryParam:
		q := req.URL.Query()
		q.Set(p.QueryParamName, key)
		req.URL.RawQuery = q.Encode()
	default:
		return Result{}, fmt.Errorf("provider %s: unknown auth scheme %d", p.ID, p.AuthScheme)
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return Result{Status: StatusNetworkError, Detail: err.Error()}, nil
	}
	defer resp.Body.Close()

	return classify(resp.StatusCode), nil
}

// classify is split out so the test suite can pin every branch without
// running a real server for each.
//
// Provider-specific quirks worth pinning here:
//   - Gemini returns 400 "API key not valid" instead of 401 for bad keys, so
//     every 4xx-not-429 has to land in Invalid, not NetworkError.
//   - 5xx and connection failures land in NetworkError so the wizard can
//     suggest "try again later".
func classify(code int) Result {
	switch {
	case code >= 200 && code < 300:
		return Result{Status: StatusOK, HTTPStatus: code}
	case code == http.StatusTooManyRequests:
		return Result{Status: StatusQuotaExceeded, HTTPStatus: code}
	case code >= 400 && code < 500:
		return Result{Status: StatusInvalid, HTTPStatus: code, Detail: http.StatusText(code)}
	default:
		return Result{
			Status:     StatusNetworkError,
			HTTPStatus: code,
			Detail:     http.StatusText(code),
		}
	}
}
