package main

import (
	"os"
	"testing"

	"github.com/AleDeclerk/tokensforthepeople/internal/emit"
	"github.com/AleDeclerk/tokensforthepeople/internal/wizard"
)

// silenceStdio redirects stdout/stderr to /dev/null for the duration of a test
// so exit-code assertions don't spam the test log with wizard previews.
func silenceStdio(t *testing.T) {
	t.Helper()
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = devnull, devnull
	t.Cleanup(func() {
		os.Stdout, os.Stderr = origOut, origErr
		devnull.Close()
	})
}

// resolveVersion is the whole reason `go install` users see a real tag instead
// of "dev": ldflags wins when set, build info fills the gap, and a local
// "(devel)" build stays "dev".
func TestResolveVersion(t *testing.T) {
	cases := []struct {
		name          string
		ldflags       string
		buildInfo     string
		haveBuildInfo bool
		want          string
	}{
		{"ldflags set wins over build info", "0.1.0", "v0.2.0", true, "0.1.0"},
		{"ldflags set, no build info", "0.1.0", "", false, "0.1.0"},
		{"go install: dev falls back to module version", "dev", "v0.1.0", true, "v0.1.0"},
		{"local build: devel stays dev", "dev", "(devel)", true, "dev"},
		{"empty build info stays dev", "dev", "", true, "dev"},
		{"no build info at all stays dev", "dev", "", false, "dev"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveVersion(tc.ldflags, tc.buildInfo, tc.haveBuildInfo)
			if got != tc.want {
				t.Errorf("resolveVersion(%q, %q, %v) = %q, want %q",
					tc.ldflags, tc.buildInfo, tc.haveBuildInfo, got, tc.want)
			}
		})
	}
}

func TestIsKnownTarget(t *testing.T) {
	for _, known := range emit.All {
		if !isKnownTarget(known) {
			t.Errorf("isKnownTarget(%q) = false, want true (it is in emit.All)", known)
		}
	}
	if isKnownTarget(emit.Target("bogus")) {
		t.Error("isKnownTarget(\"bogus\") = true, want false")
	}
}

func TestBuildNonInteractiveAnswers_targetsParse(t *testing.T) {
	// No keys → no network calls; this exercises only the target parsing path.
	ans, err := buildNonInteractiveAnswers("coding-agent", "quality", nil, " Continue , AIDER ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []emit.Target{"continue", "aider"}
	if len(ans.Targets) != len(want) {
		t.Fatalf("got %d targets %v, want %d %v", len(ans.Targets), ans.Targets, len(want), want)
	}
	for i, w := range want {
		if ans.Targets[i] != w {
			t.Errorf("target[%d] = %q, want %q (should be trimmed + lowercased)", i, ans.Targets[i], w)
		}
	}
}

func TestBuildNonInteractiveAnswers_unknownTarget(t *testing.T) {
	if _, err := buildNonInteractiveAnswers("coding-agent", "quality", nil, "continue,bogus"); err == nil {
		t.Fatal("expected error for unknown target, got nil")
	}
}

// Both error paths below fire before any key validation, so they need no
// network: malformed "provider=value" is caught by the split, and an unknown
// provider is caught by the metadata lookup.
func TestBuildNonInteractiveAnswers_badKeyFormat(t *testing.T) {
	if _, err := buildNonInteractiveAnswers("coding-agent", "quality", []string{"noequalssign"}, ""); err == nil {
		t.Fatal("expected error for key missing '=', got nil")
	}
}

func TestBuildNonInteractiveAnswers_unknownProvider(t *testing.T) {
	if _, err := buildNonInteractiveAnswers("coding-agent", "quality", []string{"bogus=secret"}, ""); err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
}

// The exit codes below are part of t4p's scripting contract: 0 = ok, 2 = bad
// invocation. These paths take no keys, so they make no network calls.
func TestRunInit_nonInteractiveNoWrite_returnsZero(t *testing.T) {
	silenceStdio(t)
	code := runInit([]string{"--use-case", "coding-agent", "--priority", "quality", "--targets", "continue,aider"})
	if code != 0 {
		t.Errorf("runInit exit = %d, want 0", code)
	}
}

func TestRunInit_priorityWithoutUseCase_returnsTwo(t *testing.T) {
	silenceStdio(t)
	if code := runInit([]string{"--priority", "quality"}); code != 2 {
		t.Errorf("runInit exit = %d, want 2 (use-case and priority must be set together)", code)
	}
}

func TestRunInit_unknownTarget_returnsTwo(t *testing.T) {
	silenceStdio(t)
	if code := runInit([]string{"--use-case", "coding-agent", "--priority", "quality", "--targets", "bogus"}); code != 2 {
		t.Errorf("runInit exit = %d, want 2", code)
	}
}

func TestWriteOutputs_noKeys_returnsOne(t *testing.T) {
	silenceStdio(t)
	if code := writeOutputs(wizard.Answers{}); code != 1 {
		t.Errorf("writeOutputs with no keys exit = %d, want 1", code)
	}
}
