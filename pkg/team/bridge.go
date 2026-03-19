package team

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/tools"
)

// TeamCoordinatorBridge bridges TeamCoordinator with picoclaw's AgentLoop
type TeamCoordinatorBridge struct {
	coordinator *TeamCoordinatorImpl
	agentLoop   interface {
		ProcessDirect(ctx context.Context, content, sessionKey string) (string, error)
		ProcessDirectWithChannel(ctx context.Context, content, sessionKey, channel, chatID string) (string, error)
	}
	provider      providers.LLMProvider
	defaultModel  string
	repoManager   *RepoManager
	toolFactory   *ToolFactory
	commitManager *CommitManager
	mu            sync.RWMutex
	running       bool
}

// NewTeamCoordinatorBridge creates a new bridge between TeamCoordinator and AgentLoop
func NewTeamCoordinatorBridge(
	provider providers.LLMProvider,
	agentLoop interface {
		ProcessDirect(ctx context.Context, content, sessionKey string) (string, error)
		ProcessDirectWithChannel(ctx context.Context, content, sessionKey, channel, chatID string) (string, error)
	},
	config *TeamConfig,
) (*TeamCoordinatorBridge, error) {
	// Use default config if not provided
	if config == nil {
		config = DefaultTeamConfig()
	}

	// Create agent provider that uses the actual AgentLoop
	agentProvider := &AgentLoopProvider{
		agentLoop: agentLoop,
		provider:  provider,
	}

	// Create coordinator with config
	coordinator, err := NewTeam(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create team coordinator: %w", err)
	}

	// Register role agents from config
	for _, roleCfg := range config.Roles {
		coordinator.RegisterRoleAgent(RoleType(roleCfg.Role), agentProvider, roleCfg)
	}

	return &TeamCoordinatorBridge{
		coordinator: coordinator,
		agentLoop:   agentLoop,
		provider:    provider,
		running:     false,
	}, nil
}

// AgentLoopProvider implements AgentProvider interface using picoclaw's AgentLoop
type AgentLoopProvider struct {
	agentLoop interface {
		ProcessDirect(ctx context.Context, content, sessionKey string) (string, error)
		ProcessDirectWithChannel(ctx context.Context, content, sessionKey, channel, chatID string) (string, error)
	}
	provider providers.LLMProvider
}

// Chat calls the AgentLoop to process the task
// Add nil check for safety
func (p *AgentLoopProvider) Chat(
	ctx context.Context,
	messages []providers.Message,
	tools []providers.ToolDefinition,
	model string,
	options map[string]any,
) (*providers.LLMResponse, error) {
	// Convert team messages to a single content string
	var content string
	if len(messages) > 0 {
		lastMsg := messages[len(messages)-1]
		content = lastMsg.Content
	}

	// Use unique session key per task to avoid conflicts
	sessionKey := fmt.Sprintf("team-session-%d", time.Now().UnixNano())
	response, err := p.agentLoop.ProcessDirect(ctx, content, sessionKey)
	if err != nil {
		return nil, err
	}

	return &providers.LLMResponse{
		Content:      response,
		ToolCalls:    []providers.ToolCall{},
		FinishReason: "stop",
	}, nil
}

// Start starts the team coordinator.
func (b *TeamCoordinatorBridge) Start() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.running {
		logger.WarnCF("team", "TeamCoordinatorBridge already running", nil)
		return
	}

	// Start coordinator first to avoid race condition
	if err := b.coordinator.Start(); err != nil {
		logger.ErrorCF("team", "Failed to start team coordinator", map[string]any{"error": err.Error()})
		return
	}

	b.running = true
	logger.InfoCF("team", "TeamCoordinatorBridge started", nil)
}

// Stop stops the team coordinator
func (b *TeamCoordinatorBridge) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running {
		return
	}

	b.running = false
	b.coordinator.Stop()

	logger.InfoCF("team", "TeamCoordinatorBridge stopped", nil)
}

// ReceiveTask submits a task to the team for processing
func (b *TeamCoordinatorBridge) ReceiveTask(ctx context.Context, task Task) (Task, <-chan TaskResult, error) {
	// Convert team task to coordinator task format
	coordinatorTask := &Task{
		ID:          task.ID,
		Type:        task.Type,
		Content:     task.Content,
		Context:     task.Context,
		DependsOn:   task.DependsOn,
		Role:        task.Role,
		Priority:    task.Priority,
		CreatedAt:   task.CreatedAt,
		RetryPolicy: task.RetryPolicy,
	}

	resultChan := make(chan TaskResult, 1)

	go func() {
		result, err := b.coordinator.Receive(coordinatorTask)
		if err != nil {
			resultChan <- TaskResult{
				TaskID:      task.ID,
				Success:     false,
				Error:       err.Error(),
				CompletedAt: time.Now(),
			}
			return
		}
		resultChan <- TaskResult{
			TaskID:      result.TaskID,
			Success:     result.Success,
			Output:      result.Output,
			Error:       result.Error,
			Metadata:    result.Metadata,
			Duration:    result.Duration,
			CompletedAt: result.CompletedAt,
		}
	}()

	return task, resultChan, nil
}

// ReceiveTaskSync submits a task and waits for the result
func (b *TeamCoordinatorBridge) ReceiveTaskSync(ctx context.Context, task Task) (TaskResult, error) {
	_, resultChan, err := b.ReceiveTask(ctx, task)
	if err != nil {
		return TaskResult{}, err
	}

	// Wait for the result
	select {
	case <-ctx.Done():
		return TaskResult{}, ctx.Err()
	case result := <-resultChan:
		return result, nil
	}
}

// GetCoordinator returns the team coordinator instance
func (b *TeamCoordinatorBridge) GetCoordinator() *TeamCoordinatorImpl {
	return b.coordinator
}

// IsRunning returns whether the bridge is running
func (b *TeamCoordinatorBridge) IsRunning() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.running
}

// SetRepoManager sets the repo manager for multi-repo workflows
func (b *TeamCoordinatorBridge) SetRepoManager(rm *RepoManager) { b.repoManager = rm }

// SetToolFactory sets the tool factory for per-repo tool registries
func (b *TeamCoordinatorBridge) SetToolFactory(tf *ToolFactory) { b.toolFactory = tf }

// SetCommitManager sets the commit manager for git operations
func (b *TeamCoordinatorBridge) SetCommitManager(cm *CommitManager) { b.commitManager = cm }

// SetDefaultModel sets the fallback model name for when roles don't specify one.
func (b *TeamCoordinatorBridge) SetDefaultModel(model string) { b.defaultModel = model }

// resolveModel returns the model name to use for LLM calls.
// TODO: This is a temporary workaround. The team execution layer needs redesign
// to properly resolve model_name → ModelConfig → provider per role.
func (b *TeamCoordinatorBridge) resolveModel() string {
	if b.defaultModel != "" {
		return b.defaultModel
	}
	if b.provider != nil {
		return b.provider.GetDefaultModel()
	}
	return ""
}

// TeamLoopProvider wraps AgentProvider for team coordinator
type TeamLoopProvider struct {
	coordinator *TeamCoordinatorImpl
}

// NewTeamLoopProvider creates a new team loop provider
func NewTeamLoopProvider(coordinator *TeamCoordinatorImpl) *TeamLoopProvider {
	return &TeamLoopProvider{coordinator: coordinator}
}

// ExecuteTask executes a task through the coordinator
func (p *TeamLoopProvider) ExecuteTask(ctx context.Context, taskType TaskType, content string) (string, error) {
	task := &Task{
		ID:        fmt.Sprintf("team-task-%d", time.Now().UnixNano()),
		Type:      taskType,
		Content:   content,
		CreatedAt: time.Now(),
	}

	result, err := p.coordinator.Receive(task)
	if err != nil {
		return "", err
	}

	if !result.Success {
		return "", fmt.Errorf("task failed: %s", result.Error)
	}

	return result.Output, nil
}

// RegisterTeamTools registers team collaboration tools to an agent's tool registry
func RegisterTeamTools(
	registry *tools.ToolRegistry,
	bridge *TeamCoordinatorBridge,
) {
	// Team collaboration tool - allows agents to invoke team tasks
	teamTool := NewTeamTool(bridge)
	registry.Register(teamTool)

	logger.InfoCF("team", "Registered team collaboration tools", nil)
}

// TeamTool implements the team collaboration tool for agents
type TeamTool struct {
	bridge *TeamCoordinatorBridge
}

// NewTeamTool creates a new team tool
func NewTeamTool(bridge *TeamCoordinatorBridge) *TeamTool {
	return &TeamTool{bridge: bridge}
}

// Name returns the tool name
func (t *TeamTool) Name() string {
	return "team"
}

// Description returns the tool description
func (t *TeamTool) Description() string {
	return `Delegate coding tasks to a professional development team. This is the primary tool ` +
		`for ALL code-related work including: writing code, reading/analyzing codebases, debugging, ` +
		`refactoring, architecture design, code review, writing technical documentation, ` +
		`and any task a software engineer would do. ` +
		`The team has specialized roles (CodeBuilder, CodeExplorer, StrategicPlanner, TechAdvisor) ` +
		`and can work across multiple git repositories. ` +
		`If no repos are specified, a fresh local workspace is created automatically.`
}

// Parameters returns the tool parameters
func (t *TeamTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"task": map[string]any{
				"type":        "string",
				"description": "The task description for the team to execute",
			},
			"task_type": map[string]any{
				"type": "string",
				"enum": []string{"exploration", "planning", "execution", "advisor"},
				"description": `Task classification:
- exploration: Codebase exploration, library research, information retrieval
- planning: Task planning, analysis, architecture design
- execution: Code implementation, refactoring, bug fixes
- advisor: Technical consultation, code review, architecture advice`,
			},
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
			"execution_mode": map[string]any{
				"type":        "string",
				"enum":        []string{"parallel", "sequential", "pipeline"},
				"description": "How to execute the task: parallel (concurrent), sequential (ordered), pipeline (staged)",
				"default":     "parallel",
			},
		},
		"required": []string{"task", "task_type"},
	}
}

// Execute executes the team tool
func (t *TeamTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	// Add nil check for safety
	if t == nil || t.bridge == nil {
		return tools.ErrorResult("team bridge not initialized")
	}

	taskContent, ok := args["task"].(string)
	if !ok || taskContent == "" {
		return tools.ErrorResult("task is required")
	}

	taskTypeStr, ok := args["task_type"].(string)
	if !ok || taskTypeStr == "" {
		return tools.ErrorResult("task_type is required (exploration, planning, execution, advisor)")
	}

	// Parse repos
	var repos []RepoSpec
	if reposRaw, ok := args["repos"].([]any); ok && len(reposRaw) > 0 {
		for _, r := range reposRaw {
			rmap, ok := r.(map[string]any)
			if !ok {
				return tools.ErrorResult("each repo must be an object with url field")
			}
			repoURL, ok := rmap["url"].(string)
			if !ok || repoURL == "" {
				return tools.ErrorResult("repo.url is required")
			}
			repos = append(repos, RepoSpec{URL: repoURL})
		}
	}

	// Create the task
	teamTask := Task{
		ID:        generateTaskID(),
		Type:      TaskType(taskTypeStr),
		Content:   taskContent,
		Repos:     repos,
		Context:   map[string]any{},
		CreatedAt: time.Now(),
	}

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
}

// ExecuteTaskWithRepos orchestrates the full multi-repo workflow:
// 1. Prepare worktrees for all repos
// 2. Build tool registries per repo
// 3. Execute roles using RunToolLoop with per-repo tools
// 4. Commit, push, create PRs
// 5. Cleanup worktrees on success
func (b *TeamCoordinatorBridge) ExecuteTaskWithRepos(ctx context.Context, task Task) (string, error) {
	if b.repoManager == nil {
		return "", fmt.Errorf("repo manager not configured")
	}
	if b.toolFactory == nil {
		return "", fmt.Errorf("tool factory not configured")
	}

	logger.InfoCF("team", "Starting multi-repo task", map[string]any{
		"task_id": task.ID,
		"repos":   len(task.Repos),
	})

	// 1. Prepare worktrees for all repos
	var repos []*Repo
	for _, spec := range task.Repos {
		repo, err := b.repoManager.CreateWorktree(ctx, task.ID, spec)
		if err != nil {
			// Cleanup already-created worktrees on failure
			for _, r := range repos {
				b.repoManager.RemoveWorktree(ctx, r)
			}
			return "", fmt.Errorf("failed to prepare repo %s: %w", spec.URL, err)
		}
		repos = append(repos, repo)
	}

	// 2. Build tool registries per repo
	repoTools := make(map[string]*tools.ToolRegistry)
	for _, repo := range repos {
		registry, err := b.toolFactory.BuildForRepo(repo)
		if err != nil {
			return "", fmt.Errorf("failed to build tools for repo %s: %w", repo.Name, err)
		}
		repoTools[repo.Name] = registry
	}

	// 3. Build context prompt describing available repos
	var repoContext strings.Builder
	repoContext.WriteString("You are a member of a professional coding team. ")
	repoContext.WriteString("You write clean, tested, production-quality code.\n\n")
	repoContext.WriteString("You are working on the following repositories:\n\n")
	for _, repo := range repos {
		repoContext.WriteString(fmt.Sprintf("- **%s** (branch: %s, based on: %s)\n", repo.Name, repo.WorkBranch, repo.MainBranch))
		repoContext.WriteString(fmt.Sprintf("  Working directory: %s\n", repo.LocalPath))
	}
	repoContext.WriteString("\nUse the file tools to read and modify code. Use exec to run commands.\n")
	repoContext.WriteString("Task: " + task.Content + "\n")

	// Use the first repo's tools for the main execution.
	// For multi-repo, the exec tool can access other repo paths directly.
	var primaryTools *tools.ToolRegistry
	if len(repos) > 0 {
		primaryTools = repoTools[repos[0].Name]
	}

	messages := []providers.Message{
		{Role: "user", Content: repoContext.String()},
	}

	// 4. Execute via RunToolLoop
	loopResult, err := tools.RunToolLoop(ctx, tools.ToolLoopConfig{
		Provider:      b.provider,
		Model:         b.resolveModel(),
		Tools:         primaryTools,
		MaxIterations: 30,
	}, messages, "", "")
	if err != nil {
		return "", fmt.Errorf("task execution failed: %w", err)
	}

	// 5. Commit, push, create PR
	var summary strings.Builder
	summary.WriteString("## Task completed\n\n")
	if loopResult != nil {
		summary.WriteString(loopResult.Content)
	}

	if b.commitManager != nil {
		commitResults, err := b.commitManager.CommitAll(ctx, repos, task.Content)
		if err != nil {
			return "", fmt.Errorf("commit failed: %w", err)
		}

		summary.WriteString("\n\n## Git Results\n\n")
		for _, cr := range commitResults {
			if cr.Error != nil {
				summary.WriteString(fmt.Sprintf("- %s: ERROR: %v\n", cr.RepoName, cr.Error))
			} else if cr.CommitSHA != "" {
				summary.WriteString(fmt.Sprintf("- %s: committed %s", cr.RepoName, cr.CommitSHA[:8]))
				if cr.PRURL != "" {
					summary.WriteString(fmt.Sprintf(" → PR: %s", cr.PRURL))
				}
				summary.WriteString("\n")
			} else {
				summary.WriteString(fmt.Sprintf("- %s: no changes\n", cr.RepoName))
			}
		}
	}

	// 6. Cleanup worktrees on success
	if b.coordinator.config.CleanupOnSuccess {
		for _, repo := range repos {
			b.repoManager.RemoveWorktree(ctx, repo)
		}
	}

	logger.InfoCF("team", "Multi-repo task completed", map[string]any{
		"task_id": task.ID,
		"repos":   len(repos),
	})

	return summary.String(), nil
}

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

	// 4. Execute via RunToolLoop
	loopResult, err := tools.RunToolLoop(ctx, tools.ToolLoopConfig{
		Provider:      b.provider,
		Model:         b.resolveModel(),
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

// generateTaskID generates a unique task ID
func generateTaskID() string {
	return "task-" + timeString()
}

func timeString() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
