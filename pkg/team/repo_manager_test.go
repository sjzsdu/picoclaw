package team

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestResolveRepoName(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"git@github.com:owner/repo.git", "owner-repo"},
		{"https://github.com/owner/repo.git", "owner-repo"},
		{"git@gitlab.com:my-group/my-project.git", "my-group-my-project"},
		{"https://gitlab.com/group/subgroup/project.git", "group-subgroup-project"},
		{"git@github.com:org/repo", "org-repo"},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := resolveRepoName(tt.url)
			if got != tt.want {
				t.Errorf("resolveRepoName(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestResolveCloneURL_NoToken(t *testing.T) {
	rm := NewRepoManager(t.TempDir(), "", "team")
	url := "git@github.com:owner/repo.git"
	got := rm.resolveCloneURL(url)
	if got != url {
		t.Errorf("resolveCloneURL with no token should return original URL, got %q", got)
	}
}

func TestResolveCloneURL_WithToken(t *testing.T) {
	rm := NewRepoManager(t.TempDir(), "mytoken123", "team")

	tests := []struct {
		input string
		want  string
	}{
		{
			"git@github.com:owner/repo.git",
			"https://oauth2:mytoken123@github.com/owner/repo.git",
		},
		{
			"https://github.com/owner/repo.git",
			"https://oauth2:mytoken123@github.com/owner/repo.git",
		},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := rm.resolveCloneURL(tt.input)
			if got != tt.want {
				t.Errorf("resolveCloneURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewRepoManager_CreatesDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	rm := NewRepoManager(tmpDir, "", "team")

	if rm.baseDir != tmpDir {
		t.Errorf("baseDir = %q, want %q", rm.baseDir, tmpDir)
	}
	if rm.branchPrefix != "team" {
		t.Errorf("branchPrefix = %q, want %q", rm.branchPrefix, "team")
	}
}

func TestNewRepoManager_DefaultBranchPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	rm := NewRepoManager(tmpDir, "", "")

	if rm.branchPrefix != "team" {
		t.Errorf("branchPrefix = %q, want %q", rm.branchPrefix, "team")
	}
}

func TestRepoManager_InitLocalRepo(t *testing.T) {
	tmpDir := t.TempDir()
	rm := NewRepoManager(tmpDir, "", "team")

	repo, err := rm.InitLocalRepo(context.Background(), "test-task-123")
	if err != nil {
		t.Fatalf("InitLocalRepo failed: %v", err)
	}

	if repo.Name != "workspace" {
		t.Errorf("Name = %q, want %q", repo.Name, "workspace")
	}
	if repo.WorkBranch != "team/test-task-123" {
		t.Errorf("WorkBranch = %q, want %q", repo.WorkBranch, "team/test-task-123")
	}
	if repo.MainBranch != "main" {
		t.Errorf("MainBranch = %q, want %q", repo.MainBranch, "main")
	}

	// Verify it's a real git repo
	cmd := exec.CommandContext(context.Background(), "git", "-C", repo.LocalPath, "rev-parse", "--is-inside-work-tree")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("not a git repo: %v", err)
	}
	if strings.TrimSpace(string(output)) != "true" {
		t.Error("expected git repo")
	}

	// Verify we're on the right branch
	cmd = exec.CommandContext(context.Background(), "git", "-C", repo.LocalPath, "branch", "--show-current")
	output, err = cmd.Output()
	if err != nil {
		t.Fatalf("git branch failed: %v", err)
	}
	if strings.TrimSpace(string(output)) != "team/test-task-123" {
		t.Errorf("branch = %q, want %q", strings.TrimSpace(string(output)), "team/test-task-123")
	}
}
