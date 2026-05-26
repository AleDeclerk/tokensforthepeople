package keystore_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AleDeclerk/tokensforthepeople/internal/keystore"
)

func TestWrite_createsFileWith600Perms(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keys.env")

	err := keystore.Write(path, map[string]string{
		"GEMINI_API_KEY": "AIza_x",
		"GROQ_API_KEY":   "gsk_y",
	})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("perms: got %o, want 600", mode)
	}
}

func TestWrite_serializesKeysAlphabeticallyForDeterministicDiffs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keys.env")
	if err := keystore.Write(path, map[string]string{
		"OPENROUTER_API_KEY": "sk-or-1",
		"GEMINI_API_KEY":     "AIza_1",
		"GROQ_API_KEY":       "gsk_1",
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// Header line + 3 KEY=value lines. Keys sorted alphabetically.
	wantOrder := []string{"GEMINI_API_KEY=", "GROQ_API_KEY=", "OPENROUTER_API_KEY="}
	idxs := make([]int, len(wantOrder))
	for i, k := range wantOrder {
		idxs[i] = strings.Index(string(got), k)
		if idxs[i] == -1 {
			t.Fatalf("missing line %q in output:\n%s", k, got)
		}
	}
	for i := 1; i < len(idxs); i++ {
		if idxs[i] < idxs[i-1] {
			t.Errorf("keys not sorted: %v appears before %v", wantOrder[i], wantOrder[i-1])
		}
	}
}

func TestWrite_createsParentDirectoryIfMissing(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "c", "keys.env")
	if err := keystore.Write(nested, map[string]string{"X": "y"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := os.Stat(nested); err != nil {
		t.Errorf("expected nested file to exist: %v", err)
	}
}

func TestWrite_overwrites_existingFile_andBacksUp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keys.env")
	if err := os.WriteFile(path, []byte("OLD_KEY=old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := keystore.Write(path, map[string]string{"NEW_KEY": "new"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	// Backup must be next to the file. Loose contract: same dir, .t4p.bak suffix.
	backups, _ := filepath.Glob(filepath.Join(dir, "*.t4p.bak"))
	if len(backups) == 0 {
		t.Fatal("expected a *.t4p.bak backup to be written")
	}
	bak, _ := os.ReadFile(backups[0])
	if !strings.Contains(string(bak), "OLD_KEY=old") {
		t.Errorf("backup didn't preserve old content. Got: %q", bak)
	}
}

func TestWrite_emptyMap_returnsError(t *testing.T) {
	if err := keystore.Write(filepath.Join(t.TempDir(), "keys.env"), nil); err == nil {
		t.Error("expected error when writing empty key set")
	}
}
