package emit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultPath returns where each target's config lives by convention.
// XDG_CONFIG_HOME is honored for t4p-owned snippets; per-tool dotfiles
// stay in $HOME so they match what each tool actually reads.
func DefaultPath(target Target) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if xdg == "" {
		xdg = filepath.Join(home, ".config")
	}

	switch target {
	case TargetContinue:
		return filepath.Join(home, ".continue", "config.json"), nil
	case TargetAider:
		return filepath.Join(home, ".aider.conf.yml"), nil
	case TargetLiteLLM:
		return filepath.Join(xdg, "t4p", "litellm_config.yaml"), nil
	case TargetCline:
		// We never write into VSCode settings directly; the snippet lives
		// under t4p's config dir and the user pastes it manually.
		return filepath.Join(xdg, "t4p", "cline-snippet.json"), nil
	}
	return "", fmt.Errorf("emit: unknown target %q", target)
}
