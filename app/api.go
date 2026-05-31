package main

import (
	"context"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/AleDeclerk/tokensforthepeople/internal/emit"
	"github.com/AleDeclerk/tokensforthepeople/internal/plan"
	"github.com/AleDeclerk/tokensforthepeople/internal/providers"
	"github.com/AleDeclerk/tokensforthepeople/internal/routing"
	"github.com/AleDeclerk/tokensforthepeople/internal/tools"
	"github.com/AleDeclerk/tokensforthepeople/internal/validation"
)

// API is bound to the frontend by Wails. Every exported method becomes a
// JS-callable function. Methods are thin adapters over internal/*; errors are
// returned as strings because the frontend only needs the message.
type API struct {
	ctx context.Context
}

// startup stores the Wails runtime context so OpenURL/OpenPath can call into
// the runtime. Wired as OnStartup in main.go.
func (a *API) startup(ctx context.Context) {
	a.ctx = ctx
}

// OpenURL opens a provider signup page in the system browser. Powers the
// "Get a key ↗" button.
func (a *API) OpenURL(url string) {
	if a.ctx != nil {
		runtime.BrowserOpenURL(a.ctx, url)
	}
}

// OpenPath opens a local folder in the system file manager. Powers the
// "Open folder" button on the Done screen. Wails has no portable
// reveal-in-finder, so we hand the directory to the default file:// handler.
func (a *API) OpenPath(path string) {
	if a.ctx != nil {
		runtime.BrowserOpenURL(a.ctx, "file://"+path)
	}
}

// ProviderInfo is the frontend-facing shape of one provider.
type ProviderInfo struct {
	ID          string `json:"id"`
	Display     string `json:"display"`
	SignupURL   string `json:"signupURL"`
	EnvVar      string `json:"envVar"`
	Easiest     bool   `json:"easiest"`
	Recommended bool   `json:"recommended"`
}

func toProviderInfo(p providers.Provider) ProviderInfo {
	return ProviderInfo{
		ID:        string(p.ID),
		Display:   p.Display,
		SignupURL: p.SignupURL,
		EnvVar:    p.EnvVar,
		Easiest:   p.ID == routing.ProviderGemini,
	}
}

// ListProviders returns provider metadata for the key-entry screen.
func (a *API) ListProviders() []ProviderInfo {
	out := make([]ProviderInfo, 0, len(providers.All))
	for _, p := range providers.All {
		out = append(out, toProviderInfo(p))
	}
	return out
}

// ProvidersForPlan returns every provider, ordered with the ones used by the
// (useCase, priority) chain first and flagged Recommended. An unknown pair
// yields all providers in canonical order with Recommended=false, so the keys
// screen still renders the full list.
func (a *API) ProvidersForPlan(useCase, priority string) []ProviderInfo {
	inChain := map[routing.Provider]bool{}
	if chain, err := routing.BuildChain(routing.UseCase(useCase), routing.Priority(priority)); err == nil {
		for _, s := range chain {
			inChain[s.Provider] = true
		}
	}

	recommended := make([]ProviderInfo, 0, len(providers.All))
	rest := make([]ProviderInfo, 0, len(providers.All))
	for _, p := range providers.All {
		info := toProviderInfo(p)
		if inChain[p.ID] {
			info.Recommended = true
			recommended = append(recommended, info)
		} else {
			rest = append(rest, info)
		}
	}
	return append(recommended, rest...)
}

// StepInfo is one (provider, model) entry in a plan.
type StepInfo struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// BuildPlan returns the routing chain for a (useCase, priority) pair. The
// second return is an error message ("" on success).
func (a *API) BuildPlan(useCase, priority string) ([]StepInfo, string) {
	chain, err := routing.BuildChain(routing.UseCase(useCase), routing.Priority(priority))
	if err != nil {
		return nil, err.Error()
	}
	out := make([]StepInfo, 0, len(chain))
	for _, s := range chain {
		out = append(out, StepInfo{Provider: string(s.Provider), Model: s.Model})
	}
	return out, ""
}

// KeyStatus is the frontend-facing validation outcome.
type KeyStatus struct {
	Status string `json:"status"` // "ok" | "quota" | "invalid" | "unreachable" | "error"
	Detail string `json:"detail"`
}

// ValidateKey pings the provider endpoint for a pasted key.
func (a *API) ValidateKey(providerID, key string) KeyStatus {
	p, ok := providers.ByID(routing.Provider(providerID))
	if !ok {
		return KeyStatus{Status: "error", Detail: "unknown provider"}
	}
	res, err := validation.Ping(p, key, 5*time.Second)
	if err != nil {
		return KeyStatus{Status: "error", Detail: err.Error()}
	}
	switch res.Status {
	case validation.StatusOK:
		return KeyStatus{Status: "ok"}
	case validation.StatusQuotaExceeded:
		return KeyStatus{Status: "quota"}
	case validation.StatusInvalid:
		return KeyStatus{Status: "invalid", Detail: res.Detail}
	default:
		return KeyStatus{Status: "unreachable", Detail: res.Detail}
	}
}

// TargetInfo reports a target and whether it was detected as installed.
type TargetInfo struct {
	ID       string `json:"id"`
	Detected bool   `json:"detected"`
}

// DetectTargets returns every known target with its detection flag.
func (a *API) DetectTargets() []TargetInfo {
	det := tools.DefaultDetector().Detect()
	out := make([]TargetInfo, 0, len(det))
	for _, d := range det {
		out = append(out, TargetInfo{ID: string(d.Target), Detected: d.Installed})
	}
	return out
}

// WriteRequest is the finished wizard state sent from the frontend.
type WriteRequest struct {
	UseCase  string            `json:"useCase"`
	Priority string            `json:"priority"`
	Keys     map[string]string `json:"keys"`    // envVar -> value
	Targets  []string          `json:"targets"` // target ids
}

// WriteResult mirrors plan.Report for the frontend.
type WriteResult struct {
	KeysPath string            `json:"keysPath"`
	Targets  []WriteTargetItem `json:"targets"`
	Error    string            `json:"error"`
}

type WriteTargetItem struct {
	Target  string `json:"target"`
	OK      bool   `json:"ok"`
	Path    string `json:"path"`
	Created bool   `json:"created"`
	Backup  string `json:"backup"`
	Err     string `json:"err"`
}

// WriteConfigs persists keys and renders the selected target configs.
func (a *API) WriteConfigs(req WriteRequest) WriteResult {
	targets := make([]emit.Target, 0, len(req.Targets))
	for _, t := range req.Targets {
		targets = append(targets, emit.Target(t))
	}
	rep, err := plan.Apply(plan.Input{
		UseCase:  routing.UseCase(req.UseCase),
		Priority: routing.Priority(req.Priority),
		Keys:     req.Keys,
		Targets:  targets,
	})
	if err != nil {
		return WriteResult{Error: err.Error()}
	}
	res := WriteResult{KeysPath: rep.KeysPath}
	for _, tr := range rep.Targets {
		res.Targets = append(res.Targets, WriteTargetItem{
			Target: string(tr.Target), OK: tr.OK, Path: tr.Path,
			Created: tr.Created, Backup: tr.Backup, Err: tr.Err,
		})
	}
	return res
}
