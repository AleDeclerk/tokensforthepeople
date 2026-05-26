package emit

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/AleDeclerk/tokensforthepeople/internal/routing"
)

// WrittenFile is the report returned for each file the writer touched.
// The wizard's screen 5 turns this into a green checklist.
type WrittenFile struct {
	Path    string
	Bytes   int
	Backup  string // empty if no prior file existed
	Created bool   // true == new file; false == overwrite
}

// WriteAtomic writes content to path with 0o644, creating parent dirs and
// dropping a timestamped .t4p.bak next to any pre-existing file.
//
// Unlike keystore.Write we use 0o644 here — these configs are not secrets;
// the secrets stay in keys.env. The atomic temp+rename pattern still
// applies so a crash mid-write can't corrupt the target.
func WriteAtomic(path string, content []byte) (WrittenFile, error) {
	report := WrittenFile{Path: path, Bytes: len(content)}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return report, fmt.Errorf("emit: mkdir parent: %w", err)
	}

	if _, err := os.Stat(path); err == nil {
		src, rerr := os.ReadFile(path)
		if rerr != nil {
			return report, fmt.Errorf("emit: read for backup: %w", rerr)
		}
		stamp := time.Now().UTC().Format("20060102-150405")
		bak := path + "." + stamp + ".t4p.bak"
		if werr := os.WriteFile(bak, src, 0o600); werr != nil {
			return report, fmt.Errorf("emit: write backup: %w", werr)
		}
		report.Backup = bak
	} else if !os.IsNotExist(err) {
		return report, fmt.Errorf("emit: stat: %w", err)
	} else {
		report.Created = true
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".emit-*.tmp")
	if err != nil {
		return report, fmt.Errorf("emit: temp file: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return report, fmt.Errorf("emit: write temp: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return report, fmt.Errorf("emit: chmod temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return report, fmt.Errorf("emit: close temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return report, fmt.Errorf("emit: rename: %w", err)
	}
	return report, nil
}

// Render dispatches to the right emitter for the given target. main.go and
// the wizard hit this instead of duplicating the switch.
func Render(target Target, chain []routing.Step, keys map[string]string) ([]byte, error) {
	switch target {
	case TargetContinue:
		return Continue(chain, keys)
	case TargetAider:
		return Aider(chain, keys)
	case TargetLiteLLM:
		return LiteLLM(chain, keys)
	case TargetCline:
		return Cline(chain, keys)
	}
	return nil, fmt.Errorf("emit: unknown target %q", target)
}
