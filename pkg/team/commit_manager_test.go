package team

import "testing"

func TestExtractGHRepo(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"git@github.com:owner/repo.git", "owner/repo"},
		{"https://github.com/owner/repo.git", "owner/repo"},
		{"git@github.com:org/project", "org/project"},
		{"https://gitlab.com/group/subgroup/project.git", "subgroup/project"},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := extractGHRepo(tt.url)
			if got != tt.want {
				t.Errorf("extractGHRepo(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestNewCommitManager(t *testing.T) {
	cm := NewCommitManager(true)
	if !cm.autoPR {
		t.Error("expected autoPR to be true")
	}

	cm2 := NewCommitManager(false)
	if cm2.autoPR {
		t.Error("expected autoPR to be false")
	}
}
