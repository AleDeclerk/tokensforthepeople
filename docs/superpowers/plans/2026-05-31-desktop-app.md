# Desktop App Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a double-click desktop app (Wails) that walks a non-developer through setting up free LLM access, reusing the existing `internal/*` backend.

**Architecture:** A new `app/` Wails project (Go backend + web frontend in a native window) calls a thin binding layer (`app/api.go`) that delegates to the already-tested `internal/*` packages. The CLI/TUI is untouched except for one refactor that extracts the shared write-and-report loop so both front ends use the same code.

**Tech Stack:** Go 1.26, Wails v2, vanilla TypeScript + Vite (frontend), Vitest (frontend tests), Go's `testing` (backend tests).

**Spec:** `docs/superpowers/specs/2026-05-31-desktop-app-design.md`

---

## Confirmed existing APIs (do not change these signatures)

These were read from the codebase. The binding layer calls them as-is.

```go
// internal/emit
type Target string
const ( TargetContinue Target = "continue"; TargetAider Target = "aider"
        TargetLiteLLM Target = "litellm"; TargetCline Target = "cline" )
var All = []Target{TargetCline, TargetContinue, TargetAider, TargetLiteLLM}
func DefaultPath(target Target) (string, error)
func Render(target Target, chain []routing.Step, keys map[string]string) ([]byte, error)
func WriteAtomic(path string, content []byte) (WrittenFile, error)
type WrittenFile struct { Path string; Created bool; Backup string; Bytes int }

// internal/keystore
func DefaultPath() (string, error)
func Write(path string, keys map[string]string) error   // errors on empty map

// internal/tools  (NOTE: no package-level Detect; use the Detector)
func DefaultDetector() Detector
func (d Detector) Detect() []DetectedTarget
type DetectedTarget struct { Target emit.Target; Display string; Installed bool; ConfigPath string }

// internal/routing
func BuildChain(uc UseCase, pr Priority) ([]Step, error)
type Step struct { Provider Provider; Model string }

// internal/providers
var All []Provider
func ByID(id routing.Provider) (Provider, bool)
type Provider struct { ID routing.Provider; Display, SignupURL, EnvVar, Endpoint string; /* ... */ }

// internal/validation
type Status int
const ( StatusOK Status = iota; StatusQuotaExceeded; StatusInvalid; StatusNetworkError )
type Result struct { Status Status; HTTPStatus int; Detail string }
func Ping(p providers.Provider, key string, timeout time.Duration) (Result, error)

// internal/wizard
type Answers struct {
    UseCase  routing.UseCase
    Priority routing.Priority
    Keys     map[string]string
    Targets  []emit.Target
    // detected is private
}
```

## File structure

```
internal/plan/plan.go        NEW — shared write-and-report loop (extracted from cmd/t4p)
internal/plan/plan_test.go   NEW — tests for the loop
cmd/t4p/main.go              MODIFY — writeOutputs delegates to internal/plan
app/main.go                  NEW — Wails bootstrap
app/api.go                   NEW — binding layer
app/api_test.go              NEW — binding tests
app/wails.json               NEW — Wails project config
app/frontend/index.html      NEW
app/frontend/package.json    NEW
app/frontend/vite.config.ts  NEW
app/frontend/src/main.ts     NEW — screen router
app/frontend/src/store.ts    NEW — wizard state + derived flags
app/frontend/src/store.test.ts NEW
app/frontend/src/screens/*.ts NEW — one module per screen
.github/workflows/app.yml    NEW — build app artifacts on tag
README.md                    MODIFY — "Download the app" section
ROADMAP.md                   MODIFY — note the app track
```

---

## Task 0: Install the Wails CLI

**Files:** none (local toolchain).

- [ ] **Step 1: Confirm Go and Node are present**

Run: `go version && node --version && npm --version`
Expected: Go 1.26.x, Node present (this machine has v25.8.2), npm present (v11.x).
Wails needs Node 18+; any current Node is fine.

- [ ] **Step 2: Install Wails v2 CLI**

Run: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
Expected: exit 0, binary at `$(go env GOPATH)/bin/wails`.

- [ ] **Step 3: Verify and run doctor**

Run: `wails version && wails doctor`
Expected: prints a version; `doctor` reports the system is ready to build (it checks for the OS webview, which ships with macOS).

---

## Task 1: Extract the shared write-and-report loop

The current `cmd/t4p/main.go` `writeOutputs` contains the keys-then-targets write loop. Extract the reusable core into `internal/plan` so the app and the CLI share one tested implementation. Behavior must not change.

**Files:**
- Create: `internal/plan/plan.go`
- Create: `internal/plan/plan_test.go`
- Modify: `cmd/t4p/main.go` (have `writeOutputs` call the new package)

- [ ] **Step 1: Write the failing test**

```go
// internal/plan/plan_test.go
package plan_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AleDeclerk/tokensforthepeople/internal/emit"
	"github.com/AleDeclerk/tokensforthepeople/internal/plan"
	"github.com/AleDeclerk/tokensforthepeople/internal/routing"
)

func TestApply_writesKeysAndTargets(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)       // keystore.DefaultPath honors this
	t.Setenv("HOME", dir)                  // emit.DefaultPath uses HOME for ~/.continue

	in := plan.Input{
		UseCase:  routing.UseCaseCodingAgent,
		Priority: routing.PriorityQuality,
		Keys:     map[string]string{"GEMINI_API_KEY": "AIza-test"},
		Targets:  []emit.Target{emit.TargetContinue},
	}
	rep, err := plan.Apply(in)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if rep.KeysPath == "" {
		t.Error("expected KeysPath to be set")
	}
	if len(rep.Targets) != 1 || !rep.Targets[0].OK {
		t.Fatalf("expected 1 successful target, got %+v", rep.Targets)
	}
	if _, err := os.Stat(filepath.Join(dir, "t4p", "keys.env")); err != nil {
		t.Errorf("keys.env not written: %v", err)
	}
}

func TestApply_noKeys_errors(t *testing.T) {
	_, err := plan.Apply(plan.Input{})
	if err == nil {
		t.Fatal("expected error when no keys provided")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/plan/ -run TestApply -v`
Expected: FAIL — package `internal/plan` does not compile (`plan.Input`, `plan.Apply` undefined).

- [ ] **Step 3: Write the implementation**

```go
// internal/plan/plan.go

// Package plan applies a finished set of wizard answers to disk: it writes
// the keystore and renders every selected target config. Both the CLI and
// the desktop app call Apply so the write behavior lives in one place.
package plan

import (
	"fmt"

	"github.com/AleDeclerk/tokensforthepeople/internal/emit"
	"github.com/AleDeclerk/tokensforthepeople/internal/keystore"
	"github.com/AleDeclerk/tokensforthepeople/internal/routing"
)

// Input is the finished wizard state needed to write configs.
type Input struct {
	UseCase  routing.UseCase
	Priority routing.Priority
	Keys     map[string]string
	Targets  []emit.Target
}

// TargetResult reports the outcome of writing one target config.
type TargetResult struct {
	Target  emit.Target
	OK      bool
	Path    string
	Created bool
	Backup  string
	Err     string
}

// Report is the full outcome of Apply.
type Report struct {
	KeysPath string
	Targets  []TargetResult
}

// Apply writes the keystore, then renders and writes every target. It mirrors
// the order the CLI used: keys first (every config references env vars sourced
// from keys.env), then targets. A per-target failure is recorded, not fatal.
func Apply(in Input) (Report, error) {
	var rep Report
	if len(in.Keys) == 0 {
		return rep, fmt.Errorf("no keys to write")
	}

	keyPath, err := keystore.DefaultPath()
	if err != nil {
		return rep, fmt.Errorf("resolve config dir: %w", err)
	}
	if err := keystore.Write(keyPath, in.Keys); err != nil {
		return rep, fmt.Errorf("write keys: %w", err)
	}
	rep.KeysPath = keyPath

	if len(in.Targets) == 0 {
		return rep, nil
	}

	chain, err := routing.BuildChain(in.UseCase, in.Priority)
	if err != nil {
		return rep, fmt.Errorf("build chain: %w", err)
	}

	for _, target := range in.Targets {
		tr := TargetResult{Target: target}
		path, err := emit.DefaultPath(target)
		if err != nil {
			tr.Err = err.Error()
			rep.Targets = append(rep.Targets, tr)
			continue
		}
		content, err := emit.Render(target, chain, in.Keys)
		if err != nil {
			tr.Err = err.Error()
			rep.Targets = append(rep.Targets, tr)
			continue
		}
		written, err := emit.WriteAtomic(path, content)
		if err != nil {
			tr.Err = err.Error()
			rep.Targets = append(rep.Targets, tr)
			continue
		}
		tr.OK = true
		tr.Path = written.Path
		tr.Created = written.Created
		tr.Backup = written.Backup
		rep.Targets = append(rep.Targets, tr)
	}
	return rep, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/plan/ -run TestApply -v`
Expected: PASS (both cases).

- [ ] **Step 5: Rewire `cmd/t4p` writeOutputs to use the package**

In `cmd/t4p/main.go`, replace the body of `writeOutputs` so it builds a
`plan.Input` from the `wizard.Answers`, calls `plan.Apply`, and prints the same
human output from the returned `Report`. Keep the exit-code contract: return 1
if `Apply` errors or any target failed, else 0. Add `"github.com/AleDeclerk/tokensforthepeople/internal/plan"` to imports. Example body:

```go
func writeOutputs(ans wizard.Answers) int {
	rep, err := plan.Apply(plan.Input{
		UseCase:  ans.UseCase,
		Priority: ans.Priority,
		Keys:     ans.Keys,
		Targets:  ans.Targets,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "\n✓ wrote %s (chmod 600)\n", rep.KeysPath)
	if len(rep.Targets) == 0 {
		fmt.Fprintln(os.Stdout, "  no targets selected — keys.env is enough for direnv / manual use")
		return 0
	}
	failed := 0
	for _, tr := range rep.Targets {
		if !tr.OK {
			fmt.Fprintf(os.Stderr, "  ✗ %s: %s\n", tr.Target, tr.Err)
			failed++
			continue
		}
		verb := "updated"
		if tr.Created {
			verb = "created"
		}
		fmt.Fprintf(os.Stdout, "  ✓ %s %s\n", verb, tr.Path)
		if tr.Backup != "" {
			fmt.Fprintf(os.Stdout, "    backed up to %s\n", tr.Backup)
		}
	}
	if failed > 0 {
		fmt.Fprintf(os.Stderr, "\n%d of %d targets failed.\n", failed, len(rep.Targets))
		return 1
	}
	return 0
}
```

- [ ] **Step 6: Run the full suite (CLI tests must still pass)**

Run: `go test -race -count=1 ./...`
Expected: all `ok`, including the existing `cmd/t4p` write tests (`TestWriteOutputs_noKeys_returnsOne` still returns 1, since `plan.Apply` errors on empty keys).

- [ ] **Step 7: Commit**

```bash
git add internal/plan/ cmd/t4p/main.go
git commit -m "refactor(plan): extract shared write-and-report loop

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 2: Scaffold the Wails project

**Files:**
- Create: `app/` (via `wails init`), then trim to the structure above.

- [ ] **Step 1: Generate the project**

Run from repo root:
```bash
wails init -n app -t vanilla-ts -d app
```
Expected: creates `app/` with `main.go`, `wails.json`, `frontend/` (Vite + vanilla TS), and a `go.mod`. (`-t vanilla-ts` selects the vanilla TypeScript template; no React/Svelte.)

- [ ] **Step 2: Make the app use the repo module, not a nested one**

Wails creates `app/go.mod` with its own module. Delete it so `app/` is part of the root module and can import `internal/*`:
```bash
rm app/go.mod app/go.sum 2>/dev/null
```
Then ensure root `go.mod` has the Wails dependency:
```bash
cd /Users/alejandrodeclerk/repos/tokensforthepeople && go get github.com/wailsapp/wails/v2@latest && go mod tidy
```
Expected: root `go.mod` now requires `github.com/wailsapp/wails/v2`.

- [ ] **Step 3: Verify it builds and runs a blank window**

Run: `cd app && wails build`
Expected: exit 0, produces `app/build/bin/app.app` (macOS). (Optional manual check: `open app/build/bin/app.app` shows an empty window. This needs a GUI session; in headless CI only the build matters.)

- [ ] **Step 4: Commit the scaffold**

```bash
git add app/ go.mod go.sum
git commit -m "feat(app): scaffold Wails project (vanilla-ts)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 3: Binding layer — providers, plan, validation

Implement `app/api.go` with the methods the frontend calls. They are thin
adapters over `internal/*`. Wails binds the methods of a struct passed to it.

**Files:**
- Create/replace: `app/api.go`
- Create: `app/api_test.go`

- [ ] **Step 1: Write the failing test**

```go
// app/api_test.go
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
	// Gemini is the promoted "easiest" provider per the spec.
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./app/ -run 'TestListProviders|TestBuildPlan|TestDetectTargets' -v`
Expected: FAIL — `API`, `ProviderInfo` undefined.

- [ ] **Step 3: Write the implementation**

```go
// app/api.go
package main

import (
	"time"

	"github.com/AleDeclerk/tokensforthepeople/internal/emit"
	"github.com/AleDeclerk/tokensforthepeople/internal/plan"
	"github.com/AleDeclerk/tokensforthepeople/internal/providers"
	"github.com/AleDeclerk/tokensforthepeople/internal/routing"
	"github.com/AleDeclerk/tokensforthepeople/internal/tools"
	"github.com/AleDeclerk/tokensforthepeople/internal/validation"
)

// API is bound to the frontend by Wails. Every exported method becomes a
// JS-callable function. Methods are thin adapters over internal/*; errors are
// returned as strings because Wails marshals Go errors awkwardly and the
// frontend only needs the message.
type API struct{}

// ProviderInfo is the frontend-facing shape of one provider.
type ProviderInfo struct {
	ID        string `json:"id"`
	Display   string `json:"display"`
	SignupURL string `json:"signupURL"`
	EnvVar    string `json:"envVar"`
	Easiest   bool   `json:"easiest"`
}

// ListProviders returns provider metadata for the key-entry screen.
func (a *API) ListProviders() []ProviderInfo {
	out := make([]ProviderInfo, 0, len(providers.All))
	for _, p := range providers.All {
		out = append(out, ProviderInfo{
			ID:        string(p.ID),
			Display:   p.Display,
			SignupURL: p.SignupURL,
			EnvVar:    p.EnvVar,
			Easiest:   p.ID == routing.ProviderGemini,
		})
	}
	return out
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
// tools.DefaultDetector().Detect() returns []tools.DetectedTarget in emit.All
// order, each with .Target and .Installed.
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
	Keys     map[string]string `json:"keys"`     // envVar -> value
	Targets  []string          `json:"targets"`  // target ids
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./app/ -run 'TestListProviders|TestBuildPlan|TestDetectTargets' -v`
Expected: PASS.

- [ ] **Step 5: Add WriteConfigs + ValidateKey tests**

```go
// append to app/api_test.go
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
```

- [ ] **Step 6: Run the new tests**

Run: `go test ./app/ -run 'TestWriteConfigs|TestValidateKey' -v`
Expected: PASS. (`TestWriteConfigs_writesContinue` makes no network call — keys are written and configs rendered without validating.)

- [ ] **Step 7: Commit**

```bash
git add app/api.go app/api_test.go
git commit -m "feat(app): binding layer over internal packages

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 4: Wire the binding into the Wails app + OpenURL/OpenPath

**Files:**
- Modify: `app/main.go`

- [ ] **Step 1: Replace main.go to bind API and add runtime helpers**

```go
// app/main.go
package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"context"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	api := &API{}
	err := wails.Run(&options.App{
		Title:  "tokensforthepeople",
		Width:  720,
		Height: 560,
		AssetServer: &assetserver.Options{Assets: assets},
		OnStartup: func(ctx context.Context) { api.ctx = ctx },
		Bind:      []interface{}{api},
	})
	if err != nil {
		panic(err)
	}
}
```

- [ ] **Step 2: Add ctx + OpenURL/OpenPath to API**

Append to `app/api.go`:
```go
import "context"   // add to the import block

// add field to API:
//   type API struct{ ctx context.Context }

// OpenURL opens a provider signup page in the system browser.
func (a *API) OpenURL(url string) {
	if a.ctx != nil {
		runtime.BrowserOpenURL(a.ctx, url)
	}
}
```
(Place `runtime` import: `github.com/wailsapp/wails/v2/pkg/runtime`. Change the
`API` struct definition to `type API struct{ ctx context.Context }`.)

> Note on OpenPath: Wails has no portable "reveal in file manager". For Phase 1,
> `OpenURL("file://" + dir)` opens the folder in the default handler on macOS and
> Linux. Implement `OpenPath` as a thin wrapper:
```go
// OpenPath opens a local folder in the system file manager.
func (a *API) OpenPath(path string) {
	if a.ctx != nil {
		runtime.BrowserOpenURL(a.ctx, "file://"+path)
	}
}
```

- [ ] **Step 3: Verify it still compiles and tests pass**

Run: `go build ./app/ && go test ./app/ -v`
Expected: build OK; tests PASS (the `API{}` zero value has a nil ctx, and the
existing tests don't call OpenURL/OpenPath, so nil-guard holds).

- [ ] **Step 4: Commit**

```bash
git add app/main.go app/api.go
git commit -m "feat(app): bind API to window, add OpenURL/OpenPath

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 5: Frontend state store

**Files:**
- Create: `app/frontend/src/store.ts`
- Create: `app/frontend/src/store.test.ts`
- Modify: `app/frontend/package.json` (add vitest)

- [ ] **Step 1: Add vitest to package.json**

Run: `cd app/frontend && npm i -D vitest`
Then ensure a test script exists in `package.json`:
```json
"scripts": { "dev": "vite", "build": "vite build", "test": "vitest run" }
```

- [ ] **Step 2: Write the failing test**

```ts
// app/frontend/src/store.test.ts
import { describe, it, expect } from "vitest";
import { createStore } from "./store";

describe("store", () => {
  it("starts on step 0 with no valid keys", () => {
    const s = createStore();
    expect(s.state.step).toBe(0);
    expect(s.hasValidKey()).toBe(false);
  });

  it("records a valid key and reports it", () => {
    const s = createStore();
    s.setKeyStatus("gemini", "GEMINI_API_KEY", "AIza-x", "ok");
    expect(s.hasValidKey()).toBe(true);
    expect(s.validKeys()).toEqual({ GEMINI_API_KEY: "AIza-x" });
  });

  it("excludes invalid keys from validKeys", () => {
    const s = createStore();
    s.setKeyStatus("gemini", "GEMINI_API_KEY", "bad", "invalid");
    expect(s.hasValidKey()).toBe(false);
    expect(s.validKeys()).toEqual({});
  });

  it("treats quota as usable (valid but exhausted)", () => {
    const s = createStore();
    s.setKeyStatus("groq", "GROQ_API_KEY", "gsk-x", "quota");
    expect(s.hasValidKey()).toBe(true);
  });
});
```

- [ ] **Step 3: Run to verify failure**

Run: `cd app/frontend && npm test`
Expected: FAIL — `./store` has no `createStore`.

- [ ] **Step 4: Implement the store**

```ts
// app/frontend/src/store.ts

export type KeyState = {
  providerID: string;
  envVar: string;
  value: string;
  status: "" | "validating" | "ok" | "quota" | "invalid" | "unreachable" | "error";
};

export type WizardState = {
  step: number;            // 0..3
  useCase: string;
  priority: string;
  keys: Record<string, KeyState>;   // providerID -> KeyState
  targets: Record<string, boolean>; // target id -> selected
};

// "ok" and "quota" both mean the key works (quota is valid-but-exhausted).
const USABLE = new Set(["ok", "quota"]);

export function createStore() {
  const state: WizardState = {
    step: 0,
    useCase: "",
    priority: "",
    keys: {},
    targets: {},
  };

  return {
    state,
    setKeyStatus(
      providerID: string,
      envVar: string,
      value: string,
      status: KeyState["status"],
    ) {
      state.keys[providerID] = { providerID, envVar, value, status };
    },
    hasValidKey(): boolean {
      return Object.values(state.keys).some((k) => USABLE.has(k.status));
    },
    validKeys(): Record<string, string> {
      const out: Record<string, string> = {};
      for (const k of Object.values(state.keys)) {
        if (USABLE.has(k.status)) out[k.envVar] = k.value;
      }
      return out;
    },
    selectedTargets(): string[] {
      return Object.entries(state.targets)
        .filter(([, on]) => on)
        .map(([id]) => id);
    },
  };
}
```

- [ ] **Step 5: Run to verify pass**

Run: `cd app/frontend && npm test`
Expected: PASS (4 tests).

- [ ] **Step 6: Commit**

```bash
git add app/frontend/src/store.ts app/frontend/src/store.test.ts app/frontend/package.json app/frontend/package-lock.json
git commit -m "feat(app): frontend wizard state store with tests

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 6: Frontend screens

Build the four screens as declarative modules driven by the store. Keep logic in
the store (already tested); screens render and dispatch. Wails generates JS
bindings for the Go API under `app/frontend/wailsjs/go/main/API.js` during build.

**Files:**
- Modify: `app/frontend/index.html` (root container + load main.ts)
- Create: `app/frontend/src/main.ts` (screen router)
- Create: `app/frontend/src/screens/usecase.ts`
- Create: `app/frontend/src/screens/keys.ts`
- Create: `app/frontend/src/screens/targets.ts`
- Create: `app/frontend/src/screens/done.ts`
- Create: `app/frontend/src/style.css`

- [ ] **Step 1: Index + router**

```html
<!-- app/frontend/index.html -->
<!doctype html>
<html lang="en">
  <head><meta charset="utf-8" /><title>tokensforthepeople</title>
    <link rel="stylesheet" href="/src/style.css" /></head>
  <body><div id="app"></div><script type="module" src="/src/main.ts"></script></body>
</html>
```

```ts
// app/frontend/src/main.ts
import { createStore } from "./store";
import { renderUseCase } from "./screens/usecase";
import { renderKeys } from "./screens/keys";
import { renderTargets } from "./screens/targets";
import { renderDone } from "./screens/done";

const store = createStore();
const root = document.getElementById("app")!;

export function render() {
  root.innerHTML = "";
  const screens = [renderUseCase, renderKeys, renderTargets, renderDone];
  root.appendChild(screens[store.state.step](store, render));
}
render();
```

- [ ] **Step 2: Use-case screen**

```ts
// app/frontend/src/screens/usecase.ts
type Store = ReturnType<typeof import("../store").createStore>;

const USE_CASES = [
  ["coding-agent", "Coding assistant"],
  ["general-chat", "General chat"],
  ["agentic", "Agents / tools"],
  ["rag", "Documents / RAG"],
];
const PRIORITIES = [
  ["quality", "Best quality"],
  ["latency", "Fastest"],
  ["balanced", "Balanced"],
  ["privacy", "Most private"],
];

export function renderUseCase(store: Store, render: () => void): HTMLElement {
  const el = document.createElement("section");
  el.innerHTML = `<h1>Free LLM tokens for the rest of us</h1>
    <p>Set up free AI in your editor in about a minute.</p>
    <h2>What will you use it for?</h2>
    <div id="uc"></div><h2>What matters most?</h2><div id="pr"></div>
    <button id="next" disabled>Next →</button>`;
  const uc = el.querySelector("#uc")!, pr = el.querySelector("#pr")!;
  const next = el.querySelector("#next") as HTMLButtonElement;
  const refresh = () => { next.disabled = !(store.state.useCase && store.state.priority); };
  for (const [id, label] of USE_CASES) {
    const b = document.createElement("button");
    b.textContent = label; b.className = store.state.useCase === id ? "sel" : "";
    b.onclick = () => { store.state.useCase = id; render(); };
    uc.appendChild(b);
  }
  for (const [id, label] of PRIORITIES) {
    const b = document.createElement("button");
    b.textContent = label; b.className = store.state.priority === id ? "sel" : "";
    b.onclick = () => { store.state.priority = id; render(); };
    pr.appendChild(b);
  }
  next.onclick = () => { store.state.step = 1; render(); };
  refresh();
  return el;
}
```

- [ ] **Step 3: Keys screen (calls ListProviders, ValidateKey, OpenURL)**

```ts
// app/frontend/src/screens/keys.ts
import { ListProviders, ValidateKey, OpenURL } from "../../wailsjs/go/main/API";
type Store = ReturnType<typeof import("../store").createStore>;

export function renderKeys(store: Store, render: () => void): HTMLElement {
  const el = document.createElement("section");
  el.innerHTML = `<h1>Paste your keys</h1><p>You only need ONE to start.</p>
    <div id="rows">Loading…</div>
    <button id="back">← Back</button><button id="next">Next →</button>`;
  const rows = el.querySelector("#rows")!;
  (el.querySelector("#back") as HTMLButtonElement).onclick = () => { store.state.step = 0; render(); };
  (el.querySelector("#next") as HTMLButtonElement).onclick = () => { store.state.step = 2; render(); };

  ListProviders().then((providers) => {
    rows.innerHTML = "";
    for (const p of providers) {
      const row = document.createElement("div"); row.className = "keyrow";
      const known = store.state.keys[p.id];
      // Build via DOM, not innerHTML: the key value and provider display must
      // never be interpolated into an HTML string (XSS / breakage on quotes).
      const label = document.createElement("label");
      label.textContent = p.display + (p.easiest ? " · easiest" : "");
      const input = document.createElement("input");
      input.type = "password"; input.placeholder = "paste key";
      input.value = known?.value ?? "";
      const chip = document.createElement("span");
      chip.className = "chip"; chip.textContent = known?.status ?? "";
      const link = document.createElement("button");
      link.className = "link"; link.textContent = "Get a key ↗";
      link.onclick = () => OpenURL(p.signupURL);
      row.append(label, input, chip, link);
      let timer: number | undefined;
      input.oninput = () => {
        const val = input.value.trim();
        clearTimeout(timer);
        if (!val) { store.setKeyStatus(p.id, p.envVar, "", ""); chip.textContent = ""; return; }
        chip.textContent = "validating…";
        timer = window.setTimeout(async () => {
          const res = await ValidateKey(p.id, val);
          store.setKeyStatus(p.id, p.envVar, val, res.status as any);
          chip.textContent = res.status;
        }, 600);
      };
      rows.appendChild(row);
    }
  });
  return el;
}
```

- [ ] **Step 4: Targets screen (DetectTargets)**

```ts
// app/frontend/src/screens/targets.ts
import { DetectTargets } from "../../wailsjs/go/main/API";
type Store = ReturnType<typeof import("../store").createStore>;

const LABEL: Record<string, string> = {
  continue: "Continue.dev", aider: "Aider", cline: "Cline", litellm: "LiteLLM proxy",
};

export function renderTargets(store: Store, render: () => void): HTMLElement {
  const el = document.createElement("section");
  el.innerHTML = `<h1>Where should it go?</h1><div id="t">Loading…</div>
    <button id="back">← Back</button>
    <button id="write" disabled>Write →</button>`;
  const t = el.querySelector("#t")!;
  const write = el.querySelector("#write") as HTMLButtonElement;
  write.disabled = !store.hasValidKey();
  if (!store.hasValidKey()) write.title = "Add at least one valid key first";
  (el.querySelector("#back") as HTMLButtonElement).onclick = () => { store.state.step = 1; render(); };
  write.onclick = () => { store.state.step = 3; render(); };

  DetectTargets().then((targets) => {
    t.innerHTML = "";
    for (const tt of targets) {
      if (!(tt.id in store.state.targets)) store.state.targets[tt.id] = tt.detected;
      const row = document.createElement("label"); row.className = "trow";
      row.innerHTML = `<input type="checkbox" ${store.state.targets[tt.id] ? "checked" : ""}/>
        ${LABEL[tt.id] ?? tt.id} ${tt.detected ? "(found)" : ""}`;
      (row.querySelector("input") as HTMLInputElement).onchange = (e) => {
        store.state.targets[tt.id] = (e.target as HTMLInputElement).checked;
      };
      t.appendChild(row);
    }
  });
  return el;
}
```

- [ ] **Step 5: Done screen (WriteConfigs, OpenPath)**

```ts
// app/frontend/src/screens/done.ts
import { WriteConfigs, OpenPath } from "../../wailsjs/go/main/API";
type Store = ReturnType<typeof import("../store").createStore>;

export function renderDone(store: Store, _render: () => void): HTMLElement {
  const el = document.createElement("section");
  el.innerHTML = `<h1>Setting up…</h1><div id="out"></div>`;
  const out = el.querySelector("#out")!;
  WriteConfigs({
    useCase: store.state.useCase,
    priority: store.state.priority,
    keys: store.validKeys(),
    targets: store.selectedTargets(),
  }).then((res) => {
    if (res.error) { out.innerHTML = `<p class="err">${res.error}</p>`; return; }
    const lines = [`<p>✓ Keys saved securely (chmod 600)</p>`];
    for (const tr of res.targets) {
      lines.push(tr.ok
        ? `<p>✓ ${tr.target} configured</p>`
        : `<p class="err">✗ ${tr.target}: ${tr.err}</p>`);
    }
    lines.push(`<p>Reopen your editor and you're on free LLMs.</p>`);
    out.innerHTML = lines.join("");
    const folder = document.createElement("button");
    folder.textContent = "Open folder";
    folder.onclick = () => OpenPath(res.keysPath.replace(/\/keys\.env$/, ""));
    out.appendChild(folder);
  });
  return el;
}
```

- [ ] **Step 6: Minimal stylesheet**

```css
/* app/frontend/src/style.css */
body { font: 15px/1.5 system-ui, sans-serif; margin: 0; padding: 2rem; }
h1 { font-size: 1.4rem; } h2 { font-size: 1rem; margin-top: 1.2rem; }
button { margin: .25rem; padding: .5rem .9rem; border: 1px solid #ccc;
  border-radius: 8px; background: #fff; cursor: pointer; }
button.sel { background: #111; color: #fff; }
button:disabled { opacity: .4; cursor: not-allowed; }
.keyrow, .trow { display: flex; gap: .5rem; align-items: center; margin: .4rem 0; }
.keyrow input[type=password] { flex: 1; padding: .4rem; }
.chip { min-width: 5rem; font-size: .85rem; color: #555; }
.link { font-size: .85rem; } .err { color: #b00; }
```

- [ ] **Step 7: Build the app (compiles frontend + generates wailsjs bindings)**

Run: `cd app && wails build`
Expected: exit 0. The build generates `frontend/wailsjs/go/main/API.{js,d.ts}`
from the bound Go methods, then bundles the frontend and embeds it. If the
import paths in steps 3-5 error, confirm the generated path is
`../../wailsjs/go/main/API` relative to `src/screens/`.

- [ ] **Step 8: Frontend tests still pass**

Run: `cd app/frontend && npm test`
Expected: PASS (store tests; screens are not unit-tested, logic lives in store).

- [ ] **Step 9: Commit**

```bash
git add app/frontend/index.html app/frontend/src/
git commit -m "feat(app): four wizard screens wired to the binding

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 7: CI build job for the app

**Files:**
- Create: `.github/workflows/app.yml`

- [ ] **Step 1: Write the workflow**

```yaml
# .github/workflows/app.yml
name: app
on:
  push:
    tags: ["v*"]
  pull_request:
    paths: ["app/**", "internal/**", ".github/workflows/app.yml"]

jobs:
  build:
    strategy:
      matrix:
        os: [macos-latest]   # Phase 1: macOS only; add windows-latest in Phase 2
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.26" }
      - uses: actions/setup-node@v4
        with: { node-version: "22" }
      - name: Install Wails
        run: go install github.com/wailsapp/wails/v2/cmd/wails@latest
      - name: Build app
        run: cd app && wails build
      - name: Test
        run: go test ./... && (cd app/frontend && npm ci && npm test)
      - name: Upload artifact
        uses: actions/upload-artifact@v4
        with:
          name: t4p-app-${{ matrix.os }}
          path: app/build/bin/*
```

- [ ] **Step 2: Validate YAML locally**

Run: `python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/app.yml'))" && echo OK`
Expected: `OK`.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/app.yml
git commit -m "ci(app): build desktop app artifacts on tag and PR

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 8: Docs

**Files:**
- Modify: `README.md` (new "Download the app" section above `go install`)
- Modify: `ROADMAP.md` (note the app track under Planned)

- [ ] **Step 1: Add README section**

Insert directly under the intro code block, before `## Install`:

```markdown
## Download the app (easiest, no terminal)

Not a developer? Download the desktop app, open it, and follow four screens:
pick what you'll use AI for, paste a free key (the app links you to where to get
one), choose your editor, and click Write.

Grab `t4p.app` from the [latest release](https://github.com/AleDeclerk/tokensforthepeople/releases/latest).

> macOS note: the app is not yet notarized (Phase 2). The first time you open it,
> right-click the app and choose Open, then confirm. After that it opens normally.

Prefer the command line? Keep reading.
```

- [ ] **Step 2: Add ROADMAP note**

Under `## Planned`, add:

```markdown
### Desktop app (parallel track)

A double-click Wails app that runs the same setup as `t4p init` in a native
window, aimed at non-developers. Design:
[docs/superpowers/specs/2026-05-31-desktop-app-design.md](./docs/superpowers/specs/2026-05-31-desktop-app-design.md).
Phase 1 ships unsigned; Phase 2 adds notarization and Windows signing.
```

- [ ] **Step 3: Commit**

```bash
git add README.md ROADMAP.md
git commit -m "docs: document the desktop app for non-developers

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Final verification

- [ ] **Run the whole Go suite with race**

Run: `go test -race -count=1 ./...`
Expected: all `ok`, including `internal/plan` and `app`.

- [ ] **Run the frontend suite**

Run: `cd app/frontend && npm test`
Expected: PASS.

- [ ] **Build the app end-to-end**

Run: `cd app && wails build`
Expected: exit 0; `app/build/bin/app.app` exists.

- [ ] **gofmt clean**

Run: `gofmt -l . | grep -v '^app/frontend' ; echo "done"`
Expected: only `done` printed (no unformatted Go files).

- [ ] **Manual smoke (needs a GUI session, do once before merge)**

Open `app/build/bin/app.app`, walk the four screens with a real free key into a
temp HOME, confirm configs are written and "Get a key ↗" opens the browser.

---

## Self-review notes

- **Spec coverage:** entry point (Task 2,4), binding layer all 7 methods
  (Task 3,4), four screens (Task 6), "Get a key" via OpenURL (Task 4,6),
  detection-driven targets (Task 3,6), reuse via extracted loop (Task 1),
  distribution/CI (Task 7), README+ROADMAP (Task 8), phased unsigned (Task 7
  matrix macOS-only + README note). All covered.
- **Type consistency:** `plan.Input`/`plan.Report`/`TargetResult` used
  consistently across Task 1 and Task 3; `API` method names match the JS imports
  in Task 6 (`ListProviders`, `ValidateKey`, `BuildPlan`, `DetectTargets`,
  `WriteConfigs`, `OpenURL`, `OpenPath`); `KeyState.status` strings match the Go
  `KeyStatus.Status` values ("ok"/"quota"/"invalid"/"unreachable"/"error").
- **Known risk:** the generated wailsjs binding path is assumed to be
  `../../wailsjs/go/main/API`. Task 6 Step 7 calls this out to verify against the
  actual generated location.
```
