package team

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/sipeed/picoclaw/pkg/logger"
)

// CommitResult holds the result of a commit + push + PR operation
type CommitResult struct {
	RepoName  string
	CommitSHA string
	PRURL     string
	Error     error
}

// CommitManager handles git commit, push, and PR creation
type CommitManager struct {
	autoPR bool
}

// NewCommitManager creates a new CommitManager
func NewCommitManager(autoPR bool) *CommitManager {
	return &CommitManager{autoPR: autoPR}
}

// HasChanges checks if there are uncommitted changes in the worktree
func (cm *CommitManager) HasChanges(ctx context.Context, repo *Repo) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repo.LocalPath, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status failed: %w", err)
	}
	return strings.TrimSpace(string(output)) != "", nil
}

// CommitAndPush stages all changes, commits, and pushes to remote
func (cm *CommitManager) CommitAndPush(ctx context.Context, repo *Repo, message string) (string, error) {
	// Check for changes first
	hasChanges, err := cm.HasChanges(ctx, repo)
	if err != nil {
		return "", err
	}
	if !hasChanges {
		logger.InfoCF("team", "No changes to commit", map[string]any{"repo": repo.Name})
		return "", nil
	}

	// git add -A
	cmd := exec.CommandContext(ctx, "git", "-C", repo.LocalPath, "add", "-A")
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git add failed: %w\n%s", err, string(output))
	}

	// git commit
	cmd = exec.CommandContext(ctx, "git", "-C", repo.LocalPath, "commit", "-m", message)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git commit failed: %w\n%s", err, string(output))
	}

	// Get commit SHA
	cmd = exec.CommandContext(ctx, "git", "-C", repo.LocalPath, "rev-parse", "HEAD")
	shaOutput, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD failed: %w", err)
	}
	sha := strings.TrimSpace(string(shaOutput))

	// git push -u origin {branch}
	cmd = exec.CommandContext(ctx, "git", "-C", repo.LocalPath, "push", "-u", "origin", repo.WorkBranch)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git push failed: %w\n%s", err, string(output))
	}

	logger.InfoCF("team", "Committed and pushed", map[string]any{
		"repo":   repo.Name,
		"sha":    sha[:8],
		"branch": repo.WorkBranch,
	})

	return sha, nil
}

// CreatePR creates a pull request using the gh CLI
func (cm *CommitManager) CreatePR(ctx context.Context, repo *Repo, title, body string) (string, error) {
	if !cm.autoPR {
		return "", nil
	}

	cmd := exec.CommandContext(ctx, "gh", "pr", "create",
		"--repo", extractGHRepo(repo.URL),
		"--head", repo.WorkBranch,
		"--base", repo.MainBranch,
		"--title", title,
		"--body", body,
	)
	cmd.Dir = repo.LocalPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh pr create failed: %w\n%s", err, string(output))
	}

	prURL := strings.TrimSpace(string(output))
	logger.InfoCF("team", "PR created", map[string]any{"repo": repo.Name, "url": prURL})
	return prURL, nil
}

// extractGHRepo extracts "owner/repo" from a git URL for gh CLI
// "git@github.com:owner/repo.git" → "owner/repo"
// "https://github.com/owner/repo.git" → "owner/repo"
func extractGHRepo(rawURL string) string {
	if strings.HasPrefix(rawURL, "git@") {
		parts := strings.SplitN(rawURL, ":", 2)
		if len(parts) == 2 {
			return strings.TrimSuffix(parts[1], ".git")
		}
	}
	// HTTPS
	rawURL = strings.TrimSuffix(rawURL, ".git")
	parts := strings.Split(rawURL, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	return rawURL
}

// CommitAll commits and pushes all repos in a task, then creates PRs
func (cm *CommitManager) CommitAll(ctx context.Context, repos []*Repo, taskDescription string) ([]CommitResult, error) {
	var results []CommitResult

	for _, repo := range repos {
		result := CommitResult{RepoName: repo.Name}

		sha, err := cm.CommitAndPush(ctx, repo, taskDescription)
		if err != nil {
			result.Error = err
			results = append(results, result)
			continue
		}
		result.CommitSHA = sha

		if sha != "" && cm.autoPR {
			prURL, err := cm.CreatePR(ctx, repo, taskDescription, fmt.Sprintf("Automated PR from team task.\n\n%s", taskDescription))
			if err != nil {
				result.Error = fmt.Errorf("commit succeeded but PR creation failed: %w", err)
			}
			result.PRURL = prURL
		}

		results = append(results, result)
	}

	return results, nil
}
