package state

import (
	"os"
	"path/filepath"
	"testing"

	pkgroot "github.com/sipeed/picoclaw/pkg"
)

func TestManagerUsesEnvStateDir(t *testing.T) {
	workspace := t.TempDir()
	stateDir := filepath.Join(t.TempDir(), "pc-state")
	t.Setenv(pkgroot.StateDirEnv, stateDir)

	manager := NewManager(workspace)
	if got := filepath.Clean(manager.stateFile); got != filepath.Clean(filepath.Join(stateDir, "state.json")) {
		t.Fatalf("stateFile = %q, want under %q", manager.stateFile, stateDir)
	}
	if _, err := os.Stat(stateDir); err != nil {
		t.Fatalf("state dir missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "state")); !os.IsNotExist(err) {
		t.Fatalf("workspace state dir should not be created, err=%v", err)
	}
}
