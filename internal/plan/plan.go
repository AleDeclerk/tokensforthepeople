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
