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
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

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

func TestApply_keysButNoTargets(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	rep, err := plan.Apply(plan.Input{
		UseCase:  routing.UseCaseCodingAgent,
		Priority: routing.PriorityQuality,
		Keys:     map[string]string{"GEMINI_API_KEY": "AIza-test"},
		// no Targets
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if rep.KeysPath == "" {
		t.Error("expected KeysPath to be set even with no targets")
	}
	if len(rep.Targets) != 0 {
		t.Errorf("expected no target results, got %d", len(rep.Targets))
	}
}
