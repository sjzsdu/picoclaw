package agent

import (
	"os"
	"path/filepath"
	"testing"

	pkgroot "github.com/sipeed/picoclaw/pkg"
)

func TestMemoryStoreUsesEnvMemoryDir(t *testing.T) {
	workspace := t.TempDir()
	memoryDir := filepath.Join(t.TempDir(), "pc-memory")
	t.Setenv(pkgroot.MemoryDirEnv, memoryDir)

	store := NewMemoryStore(workspace)
	if got := filepath.Clean(store.memoryDir); got != filepath.Clean(memoryDir) {
		t.Fatalf("memoryDir = %q, want %q", store.memoryDir, memoryDir)
	}
	if _, err := os.Stat(memoryDir); err != nil {
		t.Fatalf("memory dir missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "memory")); !os.IsNotExist(err) {
		t.Fatalf("workspace memory dir should not be created, err=%v", err)
	}
}
