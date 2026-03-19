package team

import (
	"os"
	"path/filepath"
	"testing"
)

func TestToolFactory_BuildForRepo(t *testing.T) {
	tmpDir := t.TempDir()
	repo := &Repo{
		Name:      "test-repo",
		LocalPath: tmpDir,
	}

	// Create a test file in the worktree
	testFile := filepath.Join(tmpDir, "hello.txt")
	os.WriteFile(testFile, []byte("hello world"), 0o644)

	factory := NewToolFactory()
	registry, err := factory.BuildForRepo(repo)
	if err != nil {
		t.Fatalf("BuildForRepo failed: %v", err)
	}

	// Verify tools are registered
	expectedTools := []string{"read_file", "write_file", "list_dir", "edit_file", "append_file", "exec"}
	for _, name := range expectedTools {
		if _, ok := registry.Get(name); !ok {
			t.Errorf("expected tool %q to be registered", name)
		}
	}
}

func TestToolFactory_BuildForRepo_ToolCount(t *testing.T) {
	tmpDir := t.TempDir()
	repo := &Repo{
		Name:      "test-repo",
		LocalPath: tmpDir,
	}

	factory := NewToolFactory()
	registry, err := factory.BuildForRepo(repo)
	if err != nil {
		t.Fatalf("BuildForRepo failed: %v", err)
	}

	// Should have exactly 6 tools
	defs := registry.GetDefinitions()
	if len(defs) != 6 {
		t.Errorf("expected 6 tools, got %d", len(defs))
	}
}
