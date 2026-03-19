# Team Tool as Primary Coding Team — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the `team` tool the default handler for all coding-related tasks, with the main agent delegating automatically via system prompt guidance.

**Architecture:** Three changes work together: (1) system prompt tells the main agent to delegate coding tasks, (2) team tool's description makes it clearly the "coding team" so function calling naturally selects it, (3) repos becomes optional — when absent, team auto-creates a local git workspace.

**Tech Stack:** Go, picoclaw tools framework, git

---

### Task 1: Add `InitLocalRepo` to RepoManager

**Files:**
- Modify: `pkg/team/repo_manager.go`
- Modify: `pkg/team/repo_manager_test.go`

**Step 1: Write test for InitLocalRepo**

In `pkg/team/repo_manager_test.go`, add:

```go
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
```

Add `"context"`, `"os/exec"`, `"strings"` to test imports.

**Step 2: Run test to verify it fails**

```bash
go test ./pkg/team/... -run TestRepoManager_InitLocalRepo -v
```

Expected: FAIL — `InitLocalRepo` not defined

**Step 3: Implement InitLocalRepo**

In `pkg/team/repo_manager.go`, add after `RemoveWorktree`:

```go
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
```

**Step 4: Run test to verify it passes**

```bash
go test ./pkg/team/... -run TestRepoManager_InitLocalRepo -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add pkg/team/repo_manager.go pkg/team/repo_manager_test.go
git commit -m "feat(team): add InitLocalRepo for tasks without remote repos"
```

---

### Task 2: Rewrite team tool Description and make repos optional

**Files:**
- Modify: `pkg/team/bridge.go`

**Step 1: Replace Description()**

Find in `pkg/team/bridge.go`:

```go
func (t *TeamTool) Description() string {
	return `Invoke a team of specialized agents to collaborate on complex tasks. ` +
		`Specify the task type (exploration, planning, execution, advisor), ` +
		`and the team will automatically select the appropriate roles.`
}
```

Replace with:

```go
func (t *TeamTool) Description() string {
	return `Delegate coding tasks to a professional development team. This is the primary tool ` +
		`for ALL code-related work including: writing code, reading/analyzing codebases, debugging, ` +
		`refactoring, architecture design, code review, writing technical documentation, ` +
		`and any task a software engineer would do. ` +
		`The team has specialized roles (CodeBuilder, CodeExplorer, StrategicPlanner, TechAdvisor) ` +
		`and can work across multiple git repositories. ` +
		`If no repos are specified, a fresh local workspace is created automatically.`
}
```

**Step 2: Update Parameters() — repos becomes optional**

In `Parameters()`, change the `"required"` line from:

```go
"required": []string{"task", "task_type", "repos"},
```

to:

```go
"required": []string{"task", "task_type"},
```

And update the repos description:

```go
"repos": map[string]any{
	"type": "array",
	"items": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "Git repository URL (SSH or HTTPS)",
			},
		},
		"required": []string{"url"},
	},
	"description": "Optional. Repositories to work on. Each repo will be cloned/cached and a worktree created. If omitted, a fresh local git workspace is created automatically.",
},
```

**Step 3: Verify it compiles**

```bash
go build ./pkg/team/...
```

Expected: no errors

**Step 4: Commit**

```bash
git add pkg/team/bridge.go
git commit -m "feat(team): rewrite description as coding team, make repos optional"
```

---

### Task 3: Add `ExecuteTaskLocal` and update Execute routing

**Files:**
- Modify: `pkg/team/bridge.go`

**Step 1: Implement ExecuteTaskLocal**

Add after `ExecuteTaskWithRepos` in `pkg/team/bridge.go`:

```go
// ExecuteTaskLocal handles tasks without remote repos by creating a local git workspace.
// Flow: git init → build tools → RunToolLoop → commit locally (no push, no PR).
func (b *TeamCoordinatorBridge) ExecuteTaskLocal(ctx context.Context, task Task) (string, error) {
	if b.repoManager == nil {
		return "", fmt.Errorf("repo manager not configured")
	}
	if b.toolFactory == nil {
		return "", fmt.Errorf("tool factory not configured")
	}

	logger.InfoCF("team", "Starting local task", map[string]any{
		"task_id": task.ID,
	})

	// 1. Create local repo
	repo, err := b.repoManager.InitLocalRepo(ctx, task.ID)
	if err != nil {
		return "", fmt.Errorf("failed to create local repo: %w", err)
	}

	// 2. Build tool registry
	registry, err := b.toolFactory.BuildForRepo(repo)
	if err != nil {
		return "", fmt.Errorf("failed to build tools: %w", err)
	}

	// 3. Build context prompt
	var taskContext strings.Builder
	taskContext.WriteString("You are a member of a professional coding team. ")
	taskContext.WriteString("You write clean, tested, production-quality code.\n\n")
	taskContext.WriteString(fmt.Sprintf("Working directory: %s\n", repo.LocalPath))
	taskContext.WriteString(fmt.Sprintf("Branch: %s\n\n", repo.WorkBranch))
	taskContext.WriteString("Use the file tools to read and modify code. Use exec to run commands.\n\n")
	taskContext.WriteString("Task: " + task.Content + "\n")

	messages := []providers.Message{
		{Role: "user", Content: taskContext.String()},
	}

	// Get model
	modelName := ""
	if len(b.coordinator.config.Roles) > 0 {
		modelName = GetModelName(b.coordinator.config.Roles[0].Model)
	}
	if modelName == "" && b.provider != nil {
		modelName = b.provider.GetDefaultModel()
	}

	// 4. Execute via RunToolLoop
	loopResult, err := tools.RunToolLoop(ctx, tools.ToolLoopConfig{
		Provider:      b.provider,
		Model:         modelName,
		Tools:         registry,
		MaxIterations: 30,
	}, messages, "", "")
	if err != nil {
		return "", fmt.Errorf("task execution failed: %w", err)
	}

	// 5. Commit locally (no push, no PR)
	var summary strings.Builder
	summary.WriteString("## Task completed\n\n")
	if loopResult != nil {
		summary.WriteString(loopResult.Content)
	}

	if b.commitManager != nil {
		hasChanges, _ := b.commitManager.HasChanges(ctx, repo)
		if hasChanges {
			// Only commit locally — don't push (no remote)
			cmd := exec.CommandContext(ctx, "git", "-C", repo.LocalPath, "add", "-A")
			if output, err := cmd.CombinedOutput(); err != nil {
				logger.ErrorCF("team", "git add failed", map[string]any{"error": string(output)})
			} else {
				cmd = exec.CommandContext(ctx, "git", "-C", repo.LocalPath, "commit", "-m", task.Content)
				if output, err := cmd.CombinedOutput(); err != nil {
					logger.ErrorCF("team", "git commit failed", map[string]any{"error": string(output)})
				}
			}
			summary.WriteString(fmt.Sprintf("\n\nCode saved at: %s\n", repo.LocalPath))
		} else {
			summary.WriteString("\n\nNo code changes produced.\n")
		}
	}

	logger.InfoCF("team", "Local task completed", map[string]any{
		"task_id": task.ID,
		"path":    repo.LocalPath,
	})

	return summary.String(), nil
}
```

Add `"os/exec"` to bridge.go imports.

**Step 2: Update Execute() routing**

Replace the no-repos branch in `Execute()`. Currently:

```go
	// If repos are specified, use the multi-repo workflow
	if len(repos) > 0 {
		result, err := t.bridge.ExecuteTaskWithRepos(ctx, teamTask)
		if err != nil {
			return tools.ErrorResult(err.Error())
		}
		return tools.NewToolResult(result)
	}

	// Otherwise, use the existing coordinator workflow
	result, err := t.bridge.ReceiveTaskSync(ctx, teamTask)
```

Replace with:

```go
	// If repos are specified, use the multi-repo workflow
	if len(repos) > 0 {
		result, err := t.bridge.ExecuteTaskWithRepos(ctx, teamTask)
		if err != nil {
			return tools.ErrorResult(err.Error())
		}
		return tools.NewToolResult(result)
	}

	// No repos specified — use local workspace
	result, err := t.bridge.ExecuteTaskLocal(ctx, teamTask)
	if err != nil {
		return tools.ErrorResult(err.Error())
	}
	return tools.NewToolResult(result)
```

This removes the old coordinator fallback for the no-repos case. The coordinator path via `ReceiveTaskSync` is still available internally but the tool now always uses the coding team workflow.

**Step 3: Verify**

```bash
go build ./pkg/team/...
go test ./pkg/team/... -v
```

Expected: PASS, build OK

**Step 4: Commit**

```bash
git add pkg/team/bridge.go
git commit -m "feat(team): add ExecuteTaskLocal, route no-repos tasks to local workspace"
```

---

### Task 4: Add coding team context to ExecuteTaskWithRepos

**Files:**
- Modify: `pkg/team/bridge.go`

**Step 1: Add team identity to the repo context prompt**

In `ExecuteTaskWithRepos`, find:

```go
	// 3. Build context prompt describing available repos
	var repoContext strings.Builder
	repoContext.WriteString("You are working on the following repositories:\n\n")
```

Replace with:

```go
	// 3. Build context prompt describing available repos
	var repoContext strings.Builder
	repoContext.WriteString("You are a member of a professional coding team. ")
	repoContext.WriteString("You write clean, tested, production-quality code.\n\n")
	repoContext.WriteString("You are working on the following repositories:\n\n")
```

**Step 2: Verify**

```bash
go build ./pkg/team/...
```

Expected: no errors

**Step 3: Commit**

```bash
git add pkg/team/bridge.go
git commit -m "feat(team): add professional coding team identity to context prompts"
```

---

### Task 5: Add coding delegation rule to main agent system prompt

**Files:**
- Modify: `pkg/agent/context.go`

**Step 1: Add rule 5 to getIdentity()**

In `pkg/agent/context.go`, find in `getIdentity()`:

```go
4. **Context summaries** - Conversation summaries provided as context are approximate references only. They may be incomplete or outdated. Always defer to explicit user instructions over summary content.`,
```

Replace with:

```go
4. **Context summaries** - Conversation summaries provided as context are approximate references only. They may be incomplete or outdated. Always defer to explicit user instructions over summary content.

5. **Coding Team** - For ALL coding-related tasks, delegate to the ` + "`team`" + ` tool. This includes: writing code, reading/analyzing codebases, debugging, refactoring, code review, architecture design, codebase exploration, writing technical documentation, and any work a software engineer would do. Do NOT use read_file/write_file/edit_file/exec directly for coding work — the team has specialized roles and isolated workspaces for this. Only use file tools directly for non-code tasks (memory files, config, etc.) or when the user explicitly asks you to handle it yourself.`,
```

**Step 2: Verify**

```bash
go build ./pkg/agent/...
```

Expected: no errors

**Step 3: Commit**

```bash
git add pkg/agent/context.go
git commit -m "feat(agent): add coding team delegation rule to system prompt"
```

---

### Task 6: Run all tests and verify

**Step 1: Run all team tests**

```bash
go test ./pkg/team/... -v -count=1
```

Expected: all PASS

**Step 2: Run config and agent tests**

```bash
go test ./pkg/config/... -count=1
go test ./pkg/agent/... -count=1
```

Expected: PASS

**Step 3: Build entire project**

```bash
go generate ./cmd/picoclaw/internal/onboard/...
go build ./...
```

Expected: no errors

---

## Summary of Commits

| # | Message |
|---|---------|
| 1 | `feat(team): add InitLocalRepo for tasks without remote repos` |
| 2 | `feat(team): rewrite description as coding team, make repos optional` |
| 3 | `feat(team): add ExecuteTaskLocal, route no-repos tasks to local workspace` |
| 4 | `feat(team): add professional coding team identity to context prompts` |
| 5 | `feat(agent): add coding team delegation rule to system prompt` |
| 6 | (verification only) |
