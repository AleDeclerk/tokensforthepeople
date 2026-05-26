// Package tools detects which downstream tools (Cline, Continue.dev, Aider,
// LiteLLM) are installed on the user's machine.
//
// Detection is best-effort and read-only: we check well-known directories
// and the PATH. Anything we find pre-checks itself in the wizard's target
// selector; anything we don't find is still selectable, just unchecked.
package tools

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/AleDeclerk/tokensforthepeople/internal/emit"
)

// DetectedTarget is one row in the detection report — the target id, its
// install status, and the canonical config path we'd write to.
type DetectedTarget struct {
	Target     emit.Target
	Display    string
	Installed  bool
	ConfigPath string // empty if we'd write to a t4p-owned snippet instead
}

// Detector encapsulates the side effects so tests can inject fakes. The
// zero value works in production via the package-level DefaultDetector.
type Detector struct {
	// Home overrides os.UserHomeDir for tests.
	Home string
	// LookPath overrides exec.LookPath for tests.
	LookPath func(name string) (string, error)
}

// DefaultDetector reads $HOME and the real PATH.
func DefaultDetector() Detector {
	home, _ := os.UserHomeDir()
	return Detector{Home: home, LookPath: exec.LookPath}
}

// Detect returns the canonical list of targets with their installed status.
// The order matches emit.All so the wizard renders consistently.
func (d Detector) Detect() []DetectedTarget {
	out := make([]DetectedTarget, 0, len(emit.All))
	for _, target := range emit.All {
		out = append(out, d.one(target))
	}
	return out
}

func (d Detector) one(target emit.Target) DetectedTarget {
	switch target {
	case emit.TargetCline:
		return d.cline()
	case emit.TargetContinue:
		return d.continueDev()
	case emit.TargetAider:
		return d.aider()
	case emit.TargetLiteLLM:
		return d.litellm()
	}
	return DetectedTarget{Target: target}
}

func (d Detector) cline() DetectedTarget {
	// Cline ships as a VSCode extension; presence of its globalStorage dir
	// is the most reliable signal we can check without spawning VSCode.
	candidates := []string{
		// macOS
		filepath.Join(d.Home, "Library", "Application Support", "Code", "User",
			"globalStorage", "saoudrizwan.claude-dev"),
		// Linux
		filepath.Join(d.Home, ".config", "Code", "User",
			"globalStorage", "saoudrizwan.claude-dev"),
		// Windows is intentionally omitted — we'll add it when v1 ships.
	}
	if runtime.GOOS == "windows" {
		candidates = append(candidates,
			filepath.Join(d.Home, "AppData", "Roaming", "Code", "User",
				"globalStorage", "saoudrizwan.claude-dev"))
	}
	installed := false
	for _, c := range candidates {
		if dirExists(c) {
			installed = true
			break
		}
	}
	return DetectedTarget{
		Target:     emit.TargetCline,
		Display:    "Cline (VSCode)",
		Installed:  installed,
		ConfigPath: "", // we emit a paste-this snippet, not the real settings.json
	}
}

func (d Detector) continueDev() DetectedTarget {
	dir := filepath.Join(d.Home, ".continue")
	return DetectedTarget{
		Target:     emit.TargetContinue,
		Display:    "Continue.dev",
		Installed:  dirExists(dir),
		ConfigPath: filepath.Join(dir, "config.json"),
	}
}

func (d Detector) aider() DetectedTarget {
	_, err := d.LookPath("aider")
	return DetectedTarget{
		Target:     emit.TargetAider,
		Display:    "Aider",
		Installed:  err == nil,
		ConfigPath: filepath.Join(d.Home, ".aider.conf.yml"),
	}
}

func (d Detector) litellm() DetectedTarget {
	_, err := d.LookPath("litellm")
	return DetectedTarget{
		Target:     emit.TargetLiteLLM,
		Display:    "LiteLLM proxy",
		Installed:  err == nil,
		ConfigPath: filepath.Join(d.Home, ".config", "t4p", "litellm_config.yaml"),
	}
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
