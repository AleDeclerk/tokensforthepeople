package tools_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/AleDeclerk/tokensforthepeople/internal/emit"
	"github.com/AleDeclerk/tokensforthepeople/internal/tools"
)

func TestDetect_continue_installedWhenDirExists(t *testing.T) {
	home := t.TempDir()
	mkdir(t, filepath.Join(home, ".continue"))

	d := tools.Detector{Home: home, LookPath: noBins}
	got := d.Detect()
	if !targetInstalled(got, emit.TargetContinue) {
		t.Errorf("expected Continue detected. Got: %+v", got)
	}
}

func TestDetect_continue_notInstalledWhenDirMissing(t *testing.T) {
	d := tools.Detector{Home: t.TempDir(), LookPath: noBins}
	got := d.Detect()
	if targetInstalled(got, emit.TargetContinue) {
		t.Errorf("expected Continue NOT detected (no ~/.continue). Got: %+v", got)
	}
}

func TestDetect_aider_installedWhenBinaryOnPath(t *testing.T) {
	d := tools.Detector{
		Home: t.TempDir(),
		LookPath: func(name string) (string, error) {
			if name == "aider" {
				return "/usr/local/bin/aider", nil
			}
			return "", errors.New("not found")
		},
	}
	got := d.Detect()
	if !targetInstalled(got, emit.TargetAider) {
		t.Errorf("expected Aider detected. Got: %+v", got)
	}
}

func TestDetect_litellm_installedWhenBinaryOnPath(t *testing.T) {
	d := tools.Detector{
		Home: t.TempDir(),
		LookPath: func(name string) (string, error) {
			if name == "litellm" {
				return "/usr/local/bin/litellm", nil
			}
			return "", errors.New("not found")
		},
	}
	got := d.Detect()
	if !targetInstalled(got, emit.TargetLiteLLM) {
		t.Errorf("expected LiteLLM detected. Got: %+v", got)
	}
}

func TestDetect_cline_installedWhenVSCodeExtensionDirExists(t *testing.T) {
	home := t.TempDir()
	// Mimic the macOS layout. The Linux layout is asserted in its own test.
	mkdir(t, filepath.Join(home, "Library", "Application Support", "Code", "User",
		"globalStorage", "saoudrizwan.claude-dev"))

	d := tools.Detector{Home: home, LookPath: noBins}
	got := d.Detect()
	if !targetInstalled(got, emit.TargetCline) {
		t.Errorf("expected Cline detected on macOS layout. Got: %+v", got)
	}
}

func TestDetect_allMissingProducesNoInstalledTargets(t *testing.T) {
	d := tools.Detector{Home: t.TempDir(), LookPath: noBins}
	got := d.Detect()
	for _, target := range got {
		if target.Installed {
			t.Errorf("expected nothing installed; %s came back true", target.Target)
		}
	}
}

func TestDetect_alwaysReturnsAllKnownTargets(t *testing.T) {
	d := tools.Detector{Home: t.TempDir(), LookPath: noBins}
	got := d.Detect()
	if len(got) != len(emit.All) {
		t.Fatalf("expected %d targets, got %d", len(emit.All), len(got))
	}
}

// ── helpers ───────────────────────────────────────────────────────────────

func noBins(name string) (string, error) {
	return "", errors.New("not found")
}

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func targetInstalled(targets []tools.DetectedTarget, id emit.Target) bool {
	for _, t := range targets {
		if t.Target == id {
			return t.Installed
		}
	}
	return false
}
