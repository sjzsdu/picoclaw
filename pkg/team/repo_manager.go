package team

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sipeed/picoclaw/pkg/logger"
)

func expandHome(path string) string {
	if path == "" {
		return path
	}
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		if len(path) > 1 && path[1] == '/' {
			return home + path[1:]
		}
		return home
	}
	return path
}

// RepoSpec describes a repository to work on
type RepoSpec struct {
	URL string `json:"url"`
}

// Repo represents a repository checked out as a worktree for a specific task
type Repo struct {
	Name       string // derived from URL, e.g. "owner-repo"
	URL        string // original URL
	CloneURL   string // URL used for clone (may include token)
	BarePath   string // path to bare repo, e.g. ~/.picoclaw/team-workspace/.bare-repos/owner-repo.git
	LocalPath  string // worktree path, e.g. ~/.picoclaw/team-workspace/tasks/{task-id}/owner-repo
	WorkBranch string // e.g. "team/{task-id}"
	MainBranch string // e.g. "main" or "master"
}

// RepoManager manages bare repo cache and worktree lifecycle
type RepoManager struct {
	baseDir      string // ~/.picoclaw/team-workspace
	bareDir      string // ~/.picoclaw/team-workspace/.bare-repos
	tasksDir     string // ~/.picoclaw/team-workspace/tasks
	gitToken     string
	branchPrefix string
}

// NewRepoManager creates a new RepoManager
func NewRepoManager(baseDir, gitToken, branchPrefix string) *RepoManager {
	if branchPrefix == "" {
		branchPrefix = "team"
	}
	baseDir = expandHome(baseDir)
	rm := &RepoManager{
		baseDir:      baseDir,
		bareDir:      filepath.Join(baseDir, ".bare-repos"),
		tasksDir:     filepath.Join(baseDir, "tasks"),
		gitToken:     gitToken,
		branchPrefix: branchPrefix,
	}
	os.MkdirAll(rm.bareDir, 0o755)
	os.MkdirAll(rm.tasksDir, 0o755)
	return rm
}

// resolveRepoName extracts a filesystem-safe name from a git URL
// "git@github.com:owner/repo.git" → "owner-repo"
// "https://github.com/owner/repo.git" → "owner-repo"
func resolveRepoName(rawURL string) string {
	// Handle SSH URLs: git@host:owner/repo.git
	if strings.Contains(rawURL, ":") && strings.HasPrefix(rawURL, "git@") {
		// git@github.com:owner/repo.git → owner/repo.git
		parts := strings.SplitN(rawURL, ":", 2)
		if len(parts) == 2 {
			path := strings.TrimSuffix(parts[1], ".git")
			return strings.ReplaceAll(path, "/", "-")
		}
	}

	// Handle HTTPS URLs
	parsed, err := url.Parse(rawURL)
	if err == nil && parsed.Path != "" {
		path := strings.TrimPrefix(parsed.Path, "/")
		path = strings.TrimSuffix(path, ".git")
		return strings.ReplaceAll(path, "/", "-")
	}

	// Fallback: use the whole string, sanitized
	name := strings.TrimSuffix(rawURL, ".git")
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, ":", "-")
	return name
}

// resolveCloneURL rewrites the URL to use token auth if configured
func (rm *RepoManager) resolveCloneURL(rawURL string) string {
	if rm.gitToken == "" {
		return rawURL
	}

	// Rewrite SSH URLs to HTTPS with token
	// git@github.com:owner/repo.git → https://oauth2:TOKEN@github.com/owner/repo.git
	if strings.HasPrefix(rawURL, "git@") {
		parts := strings.SplitN(rawURL, ":", 2)
		if len(parts) == 2 {
			host := strings.TrimPrefix(parts[0], "git@")
			path := parts[1]
			return fmt.Sprintf("https://oauth2:%s@%s/%s", rm.gitToken, host, path)
		}
	}

	// For HTTPS URLs, inject token
	parsed, err := url.Parse(rawURL)
	if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		parsed.User = url.UserPassword("oauth2", rm.gitToken)
		return parsed.String()
	}

	return rawURL
}

// EnsureBareRepo clones a bare repo if it doesn't exist, or fetches if it does
func (rm *RepoManager) EnsureBareRepo(ctx context.Context, spec RepoSpec) (string, error) {
	name := resolveRepoName(spec.URL)
	barePath := filepath.Join(rm.bareDir, name+".git")
	cloneURL := rm.resolveCloneURL(spec.URL)

	if _, err := os.Stat(barePath); os.IsNotExist(err) {
		// Clone bare
		logger.InfoCF("team", "Cloning bare repo", map[string]any{"url": spec.URL, "path": barePath})
		cmd := exec.CommandContext(ctx, "git", "clone", "--bare", cloneURL, barePath)
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		if output, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("git clone --bare failed: %w\n%s", err, string(output))
		}
	} else {
		// Fetch all
		logger.InfoCF("team", "Fetching bare repo", map[string]any{"url": spec.URL, "path": barePath})
		cmd := exec.CommandContext(ctx, "git", "-C", barePath, "fetch", "--all", "--prune")
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		if output, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("git fetch --all failed: %w\n%s", err, string(output))
		}
	}

	return barePath, nil
}

// DetectMainBranch detects the default branch (main/master) from the remote HEAD
func (rm *RepoManager) DetectMainBranch(ctx context.Context, barePath string) (string, error) {
	// Try symbolic-ref first
	cmd := exec.CommandContext(ctx, "git", "-C", barePath, "symbolic-ref", "refs/remotes/origin/HEAD")
	output, err := cmd.Output()
	if err == nil {
		// refs/remotes/origin/main → main
		ref := strings.TrimSpace(string(output))
		parts := strings.Split(ref, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1], nil
		}
	}

	// Fallback: check if origin/main or origin/master exists
	for _, branch := range []string{"main", "master"} {
		cmd := exec.CommandContext(ctx, "git", "-C", barePath, "rev-parse", "--verify", "origin/"+branch)
		if err := cmd.Run(); err == nil {
			return branch, nil
		}
	}

	return "", fmt.Errorf("cannot detect main branch in %s", barePath)
}

// CreateWorktree creates a worktree for a task from the remote main branch
func (rm *RepoManager) CreateWorktree(ctx context.Context, taskID string, spec RepoSpec) (*Repo, error) {
	name := resolveRepoName(spec.URL)

	// Ensure bare repo
	barePath, err := rm.EnsureBareRepo(ctx, spec)
	if err != nil {
		return nil, err
	}

	// Detect main branch
	mainBranch, err := rm.DetectMainBranch(ctx, barePath)
	if err != nil {
		return nil, err
	}

	// Create worktree directory
	worktreePath := filepath.Join(rm.tasksDir, taskID, name)
	workBranch := fmt.Sprintf("%s/%s", rm.branchPrefix, taskID)

	logger.InfoCF("team", "Creating worktree", map[string]any{
		"bare":     barePath,
		"worktree": worktreePath,
		"branch":   workBranch,
		"base":     "origin/" + mainBranch,
	})

	cmd := exec.CommandContext(ctx, "git", "-C", barePath, "worktree", "add",
		worktreePath, "-b", workBranch, "origin/"+mainBranch)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git worktree add failed: %w\n%s", err, string(output))
	}

	return &Repo{
		Name:       name,
		URL:        spec.URL,
		CloneURL:   rm.resolveCloneURL(spec.URL),
		BarePath:   barePath,
		LocalPath:  worktreePath,
		WorkBranch: workBranch,
		MainBranch: mainBranch,
	}, nil
}

// RemoveWorktree removes a worktree (bare repo stays)
func (rm *RepoManager) RemoveWorktree(ctx context.Context, repo *Repo) error {
	logger.InfoCF("team", "Removing worktree", map[string]any{"path": repo.LocalPath})

	cmd := exec.CommandContext(ctx, "git", "-C", repo.BarePath, "worktree", "remove", "--force", repo.LocalPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		// If worktree doesn't exist, that's fine
		if !strings.Contains(string(output), "is not a working tree") {
			return fmt.Errorf("git worktree remove failed: %w\n%s", err, string(output))
		}
	}

	// Also remove the task directory if empty
	taskDir := filepath.Dir(repo.LocalPath)
	entries, _ := os.ReadDir(taskDir)
	if len(entries) == 0 {
		os.Remove(taskDir)
	}

	return nil
}

// InitLocalRepo creates a fresh git repo for tasks that don't specify a remote repository.
// The repo is created at {tasksDir}/{taskID}/workspace with an initial commit.
func (rm *RepoManager) InitLocalRepo(ctx context.Context, taskID string) (*Repo, error) {
	repoPath := filepath.Join(rm.tasksDir, taskID, "workspace")
	workBranch := fmt.Sprintf("%s/%s", rm.branchPrefix, taskID)

	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create local repo dir: %w", err)
	}

	// git init
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "init", "-b", "main")
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git init failed: %w\n%s", err, string(output))
	}

	// Create initial commit so we can branch
	cmd = exec.CommandContext(ctx, "git", "-C", repoPath, "commit", "--allow-empty", "-m", "initial commit")
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git commit failed: %w\n%s", err, string(output))
	}

	// Create and checkout work branch
	cmd = exec.CommandContext(ctx, "git", "-C", repoPath, "checkout", "-b", workBranch)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git checkout -b failed: %w\n%s", err, string(output))
	}

	logger.InfoCF("team", "Created local repo", map[string]any{
		"path":   repoPath,
		"branch": workBranch,
	})

	return &Repo{
		Name:       "workspace",
		URL:        "",
		CloneURL:   "",
		BarePath:   "",
		LocalPath:  repoPath,
		WorkBranch: workBranch,
		MainBranch: "main",
	}, nil
}
