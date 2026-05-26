// Package keystore writes validated API keys to a dotenv file with strict
// permissions and a deterministic on-disk shape.
//
// File format is the conventional KEY=value-per-line dotenv that LiteLLM,
// Cline, Continue, Aider, direnv, and every other tool in the chain can
// consume. We never quote, never escape — keys from these providers are
// always alnum + dash/underscore.
//
// Determinism matters: the wizard reruns must produce identical bytes for
// identical inputs so diffs are meaningful and so anyone code-reviewing
// a PR can read the file.
package keystore

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Write persists keys to path with mode 0600, creating parent dirs as
// needed. If path already exists, a timestamped <basename>.t4p.bak file
// is dropped next to it before overwriting.
//
// An empty keys map is an error — the wizard should not produce one and
// silently writing an empty file would mask the bug.
func Write(path string, keys map[string]string) error {
	if len(keys) == 0 {
		return fmt.Errorf("keystore: refusing to write empty key set")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("keystore: mkdir parent: %w", err)
	}

	if err := backupIfExists(path); err != nil {
		return err
	}

	content := render(keys)

	// Write via a temp file then rename so a crash mid-write can't leave a
	// half-written keys file in place.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".keys-*.tmp")
	if err != nil {
		return fmt.Errorf("keystore: temp file: %w", err)
	}
	tmpPath := tmp.Name()
	// Defense in depth: the temp file inherits whatever umask gave us; we
	// chmod explicitly before any rename so the destination never has a
	// world-readable window.
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("keystore: chmod temp: %w", err)
	}
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("keystore: write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("keystore: close temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("keystore: rename: %w", err)
	}
	return nil
}

func render(keys map[string]string) string {
	names := make([]string, 0, len(keys))
	for k := range keys {
		names = append(names, k)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("# Written by t4p init on ")
	b.WriteString(time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	b.WriteString(". Do not commit.\n")
	for _, name := range names {
		b.WriteString(name)
		b.WriteByte('=')
		b.WriteString(keys[name])
		b.WriteByte('\n')
	}
	return b.String()
}

func backupIfExists(path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("keystore: stat: %w", err)
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("keystore: read for backup: %w", err)
	}
	stamp := time.Now().UTC().Format("20060102-150405")
	bak := path + "." + stamp + ".t4p.bak"
	if err := os.WriteFile(bak, src, 0o600); err != nil {
		return fmt.Errorf("keystore: write backup: %w", err)
	}
	return nil
}

// DefaultPath returns the canonical keys.env location, honoring
// XDG_CONFIG_HOME first and falling back to ~/.config/t4p/ on every OS.
// We deliberately do not use os.UserConfigDir() because on macOS it
// returns ~/Library/Application Support, which collides with how most CLI
// tools (gh, aider, direnv) lay out their dotfiles.
func DefaultPath() (string, error) {
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, "t4p", "keys.env"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "t4p", "keys.env"), nil
}
