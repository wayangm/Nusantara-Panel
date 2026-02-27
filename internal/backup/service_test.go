package backup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRunAndRestore(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, "state.json")
	backupDir := filepath.Join(root, "backups")

	if err := os.WriteFile(state, []byte(`{"v":"one"}`), 0o640); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	svc := NewService(true, state, backupDir, nil)
	result, err := svc.Run(context.Background())
	if err != nil {
		t.Fatalf("run backup: %v", err)
	}
	if _, err := os.Stat(result.File); err != nil {
		t.Fatalf("backup file not found: %v", err)
	}

	if err := os.WriteFile(state, []byte(`{"v":"two"}`), 0o640); err != nil {
		t.Fatalf("mutate state: %v", err)
	}

	if err := svc.Restore(context.Background(), result.File); err != nil {
		t.Fatalf("restore: %v", err)
	}

	raw, err := os.ReadFile(state)
	if err != nil {
		t.Fatalf("read restored state: %v", err)
	}
	if string(raw) != `{"v":"one"}` {
		t.Fatalf("unexpected restored state: %s", string(raw))
	}
}

