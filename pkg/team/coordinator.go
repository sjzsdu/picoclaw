package team

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/providers"
)

// TaskRouter routes tasks to appropriate roles
type TaskRouter struct {
	rules []TaskRoutingRule
}

// NewTaskRouter creates a new TaskRouter
func NewTaskRouter(rules []TaskRoutingRule) *TaskRouter {
	return &TaskRouter{
		rules: rules,
	}
}

// Route determines which roles should handle a task
func (tr *TaskRouter) Route(task *Task) []RoleType {
	var roles []RoleType

	// Find matching rules for the task type
	for _, rule := range tr.rules {
		if rule.TaskType == task.Type {
			roles = append(roles, rule.Role)
		}
	}

	// If no specific rules, use default role based on task type
	if len(roles) == 0 {
		roles = tr.getDefaultRoles(task.Type)
	}

	return roles
}

// getDefaultRoles returns default roles for a task type
func (tr *TaskRouter) getDefaultRoles(taskType TaskType) []RoleType {
	switch taskType {
	case TaskTypeExploration:
		return []RoleType{RoleCodeExplorer, RoleLibraryResearcher}
	case TaskTypePlanning:
		return []RoleType{RolePreAnalyzer, RoleStrategicPlanner, RolePlanReviewer}
	case TaskTypeExecution:
		return []RoleType{RoleCodeBuilder}
	case TaskTypeAdvisor:
		return []RoleType{RoleTechAdvisor}
	default:
		return []RoleType{RoleCodeBuilder}
	}
}

// TaskDecomposer decomposes complex tasks into subtasks
type TaskDecomposer struct {
}

// NewTaskDecomposer creates a new TaskDecomposer
func NewTaskDecomposer() *TaskDecomposer {
	return &TaskDecomposer{}
}

// Decompose splits a task into subtasks based on roles
func (td *TaskDecomposer) Decompose(task *Task, roles []RoleType) []*Task {
	if len(roles) == 0 {
		return []*Task{task}
	}

	// If only one role, create a copy with the role assigned
	if len(roles) == 1 {
		return []*Task{
			{
				ID:        task.ID,
				Type:      task.Type,
				Content:   task.Content,
				Context:   task.Context,
				DependsOn: task.DependsOn,
				Role:      roles[0],
				Priority:  task.Priority,
				CreatedAt: time.Now(),
			},
		}
	}

	// Create subtasks for each role
	var subtasks []*Task
	for i, role := range roles {
		subtask := &Task{
			ID:        fmt.Sprintf("%s-subtask-%d", task.ID, i),
			Type:      task.Type,
			Content:   task.Content,
			Context:   task.Context,
			DependsOn: task.DependsOn,
			Role:      role,
			Priority:  task.Priority - i,
			CreatedAt: time.Now(),
		}
		subtasks = append(subtasks, subtask)
	}

	return subtasks
}

// ResultAggregator aggregates results from multiple tasks
type ResultAggregator struct {
}

// NewResultAggregator creates a new ResultAggregator
func NewResultAggregator() *ResultAggregator {
	return &ResultAggregator{}
}

// Aggregate combines results from multiple subtasks
func (ra *ResultAggregator) Aggregate(results []*TaskResult) (*TaskResult, error) {
	if len(results) == 0 {
		return &TaskResult{
			Success: false,
			Error:   "no results to aggregate",
		}, nil
	}

	if len(results) == 1 {
		return results[0], nil
	}

	// Check if all results are successful
	allSuccess := true
	var errors []string
	var output strings.Builder
	var totalDuration time.Duration
	metadata := make(map[string]interface{})

	for _, result := range results {
		if !result.Success {
			allSuccess = false
			if result.Error != "" {
				errors = append(errors, result.Error)
			}
		}
		output.WriteString(result.Output)
		output.WriteString("\n")
		totalDuration += result.Duration

		// Merge metadata
		for k, v := range result.Metadata {
			metadata[k] = v
		}
	}

	return &TaskResult{
		TaskID:      "aggregated",
		Success:     allSuccess,
		Output:      output.String(),
		Error:       joinErrors(errors),
		Metadata:    metadata,
		Duration:    totalDuration,
		CompletedAt: time.Now(),
	}, nil
}

func joinErrors(errors []string) string {
	if len(errors) == 0 {
		return ""
	}
	result := ""
	for i, err := range errors {
		if i > 0 {
			result += "; "
		}
		result += err
	}
	return result
}

// ParallelExecutor executes tasks in parallel
type ParallelExecutor struct {
	maxParallel int
	semaphore   chan struct{}
}

// NewParallelExecutor creates a new ParallelExecutor
func NewParallelExecutor(maxParallel int) TaskExecutor {
	if maxParallel <= 0 {
		maxParallel = 5
	}
	executor := &ParallelExecutor{
		maxParallel: maxParallel,
		semaphore:   make(chan struct{}, maxParallel),
	}
	return executor
}

// ExecuteParallel executes tasks in parallel
func (pe *ParallelExecutor) ExecuteParallel(ctx context.Context, tasks []*Task, agents map[RoleType]*RoleAgent) ([]*TaskResult, error) {
	var wg sync.WaitGroup
	results := make(chan *TaskResult, len(tasks))

	for _, task := range tasks {
		wg.Add(1)
		go func(t *Task) {
			defer wg.Done()
			pe.semaphore <- struct{}{}
			defer func() { <-pe.semaphore }()

			result := pe.executeTask(ctx, t, agents)
			results <- result
		}(task)
	}

	wg.Wait()
	close(results)

	var resultList []*TaskResult
	for result := range results {
		resultList = append(resultList, result)
	}

	return resultList, nil
}

// ExecuteSequential executes tasks sequentially
func (pe *ParallelExecutor) ExecuteSequential(ctx context.Context, tasks []*Task, agents map[RoleType]*RoleAgent) ([]*TaskResult, error) {
	var results []*TaskResult

	for _, task := range tasks {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
			result := pe.executeTask(ctx, task, agents)
			results = append(results, result)
		}
	}

	return results, nil
}

// ExecutePipeline executes tasks in a pipeline (like CI/CD)
func (pe *ParallelExecutor) ExecutePipeline(ctx context.Context, tasks []*Task, agents map[RoleType]*RoleAgent) ([]*TaskResult, error) {
	// Pipeline mode: each stage waits for previous stage to complete
	// Results from previous task become context for next task
	var results []*TaskResult
	var prevResult *TaskResult

	for i, task := range tasks {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
			// Add previous result to task context
			if prevResult != nil {
				if task.Context == nil {
					task.Context = make(map[string]interface{})
				}
				task.Context["previous_result"] = prevResult.Output
			}

			result := pe.executeTask(ctx, task, agents)
			results = append(results, result)

			// For pipeline, continue even if one fails, but pass the failure context
			if i < len(tasks)-1 {
				nextTask := tasks[i+1]
				if nextTask.Context == nil {
					nextTask.Context = make(map[string]interface{})
				}
				nextTask.Context["previous_success"] = result.Success
				if !result.Success {
					nextTask.Context["previous_error"] = result.Error
				}
			}

			prevResult = result
		}
	}

	return results, nil
}

// executeTask executes a single task using the appropriate role agent
func (pe *ParallelExecutor) executeTask(ctx context.Context, task *Task, agents map[RoleType]*RoleAgent) *TaskResult {
	startTime := time.Now()

	roleAgent, ok := agents[task.Role]
	if !ok {
		return &TaskResult{
			TaskID:      task.ID,
			Success:     false,
			Error:       fmt.Sprintf("no agent available for role: %s", task.Role),
			Duration:    time.Since(startTime),
			CompletedAt: time.Now(),
		}
	}

	// Execute the task using the agent
	result, err := pe.executeWithAgent(ctx, roleAgent, task)
	if err != nil {
		return &TaskResult{
			TaskID:      task.ID,
			Success:     false,
			Error:       err.Error(),
			Duration:    time.Since(startTime),
			CompletedAt: time.Now(),
		}
	}

	return result
}

// executeWithAgent executes a task using a specific role agent
func (pe *ParallelExecutor) executeWithAgent(ctx context.Context, roleAgent *RoleAgent, task *Task) (*TaskResult, error) {
	startTime := time.Now()

	// Get the agent provider
	agentProvider := roleAgent.AgentProvider
	if agentProvider == nil {
		return nil, fmt.Errorf("agent provider is nil for role: %s", task.Role)
	}

	// Validate config
	if roleAgent.Config == nil {
		return nil, fmt.Errorf("role config is nil for role: %s", task.Role)
	}

	// Get model name from config (supports both string and object formats)
	modelName := GetModelName(roleAgent.Config.Model)

	// Build messages for the LLM call
	messages := []providers.Message{
		{Role: "user", Content: task.Content},
	}

	// Execute the LLM call
	response, err := agentProvider.Chat(ctx, messages, nil, modelName, nil)
	if err != nil {
		return nil, err
	}

	return &TaskResult{
		TaskID:      task.ID,
		Success:     true,
		Output:      response.Content,
		Duration:    time.Since(startTime),
		CompletedAt: time.Now(),
	}, nil
}

// TeamCoordinatorImpl is the concrete implementation of TeamCoordinator
type TeamCoordinatorImpl struct {
	config     *TeamConfig
	roleAgents map[RoleType]*RoleAgent
	router     *TaskRouter
	decomposer *TaskDecomposer
	aggregator *ResultAggregator
	executor   TaskExecutor
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.RWMutex
	started    bool
}

// AgentProvider provides the interface to call agent instances
type AgentProvider interface {
	Chat(ctx context.Context, messages []providers.Message, tools []providers.ToolDefinition, model string, opts map[string]any) (*providers.LLMResponse, error)
}

// NewTeam creates a new TeamCoordinator.
// Routing rules are derived from config roles if available, otherwise defaults are used.
func NewTeam(cfg *TeamConfig) (*TeamCoordinatorImpl, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Derive routing rules from configured roles, falling back to defaults
	rules := deriveRoutingRules(cfg)

	return &TeamCoordinatorImpl{
		config:     cfg,
		roleAgents: make(map[RoleType]*RoleAgent),
		router:     NewTaskRouter(rules),
		decomposer: NewTaskDecomposer(),
		aggregator: NewResultAggregator(),
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

// deriveRoutingRules builds routing rules from config roles.
// If config has roles, it generates rules based on which roles are present.
// Otherwise it falls back to default routing rules.
func deriveRoutingRules(cfg *TeamConfig) []TaskRoutingRule {
	if cfg == nil || len(cfg.Roles) == 0 {
		return defaultRoutingRules()
	}

	// Build a set of configured role names
	configuredRoles := make(map[RoleType]bool)
	for _, r := range cfg.Roles {
		configuredRoles[RoleType(r.Role)] = true
	}

	// Filter default routing rules to only include configured roles
	defaults := defaultRoutingRules()
	var rules []TaskRoutingRule
	for _, rule := range defaults {
		if configuredRoles[rule.Role] {
			rules = append(rules, rule)
		}
	}

	// If no rules matched (e.g. custom role names), fall back to defaults
	if len(rules) == 0 {
		return defaults
	}
	return rules
}

// RegisterRoleAgent registers a role agent with the coordinator
func (tc *TeamCoordinatorImpl) RegisterRoleAgent(role RoleType, agentProvider AgentProvider, roleCfg TeamRoleConfig) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.roleAgents[role] = &RoleAgent{
		Role:          role,
		AgentProvider: agentProvider,
		Config:        &roleCfg,
	}
}

// Start starts the team coordinator
func (tc *TeamCoordinatorImpl) Start() error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if tc.started {
		return fmt.Errorf("team coordinator already started")
	}

	// Initialize executor based on config
	mode := ExecutionMode(tc.config.DefaultExecutionMode)
	switch mode {
	case ExecutionModeParallel:
		tc.executor = NewParallelExecutor(tc.config.MaxParallelTasks)
	case ExecutionModeSequential:
		tc.executor = &SequentialExecutor{}
	case ExecutionModePipeline:
		tc.executor = &PipelineExecutor{}
	default:
		tc.executor = NewParallelExecutor(tc.config.MaxParallelTasks)
	}

	tc.started = true
	logger.InfoCF("team", "TeamCoordinator started", map[string]any{"mode": mode})

	return nil
}

// Stop stops the team coordinator
func (tc *TeamCoordinatorImpl) Stop() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if tc.cancel != nil {
		tc.cancel()
	}
	tc.started = false
	logger.InfoCF("team", "TeamCoordinator stopped", nil)
}

// Receive handles incoming tasks
func (tc *TeamCoordinatorImpl) Receive(task *Task) (*TaskResult, error) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	if !tc.started {
		return nil, fmt.Errorf("team coordinator not started")
	}

	// Route the task
	roles := tc.router.Route(task)
	logger.DebugCF("team", "Task routed", map[string]any{"task_id": task.ID, "roles": roles})

	// Decompose if needed
	subtasks := tc.decomposer.Decompose(task, roles)
	logger.DebugCF("team", "Task decomposed", map[string]any{"task_id": task.ID, "subtasks": len(subtasks)})

	// Execute based on execution mode
	var results []*TaskResult
	var err error

	mode := ExecutionMode(tc.config.DefaultExecutionMode)
	switch mode {
	case ExecutionModeParallel:
		results, err = tc.executor.ExecuteParallel(tc.ctx, subtasks, tc.roleAgents)
	case ExecutionModeSequential:
		results, err = tc.executor.ExecuteSequential(tc.ctx, subtasks, tc.roleAgents)
	case ExecutionModePipeline:
		results, err = tc.executor.ExecutePipeline(tc.ctx, subtasks, tc.roleAgents)
	}

	if err != nil {
		return nil, err
	}

	// Aggregate results
	return tc.aggregator.Aggregate(results)
}

// GetRoleByType returns the role agent for a given role type
func (tc *TeamCoordinatorImpl) GetRoleByType(role RoleType) *RoleAgent {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.roleAgents[role]
}

// GetAvailableRoles returns all registered role agents
func (tc *TeamCoordinatorImpl) GetAvailableRoles() map[RoleType]*RoleAgent {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	result := make(map[RoleType]*RoleAgent)
	for k, v := range tc.roleAgents {
		result[k] = v
	}
	return result
}

// GetConfig returns the team configuration
func (tc *TeamCoordinatorImpl) GetConfig() *TeamConfig {
	return tc.config
}

// IsStarted returns whether the coordinator is started
func (tc *TeamCoordinatorImpl) IsStarted() bool {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.started
}

// SequentialExecutor executes tasks sequentially
type SequentialExecutor struct{}

func (se *SequentialExecutor) ExecuteParallel(ctx context.Context, tasks []*Task, agents map[RoleType]*RoleAgent) ([]*TaskResult, error) {
	return se.ExecuteSequential(ctx, tasks, agents)
}

func (se *SequentialExecutor) ExecuteSequential(ctx context.Context, tasks []*Task, agents map[RoleType]*RoleAgent) ([]*TaskResult, error) {
	executor := NewParallelExecutor(0)
	return executor.ExecuteSequential(ctx, tasks, agents)
}

func (se *SequentialExecutor) ExecutePipeline(ctx context.Context, tasks []*Task, agents map[RoleType]*RoleAgent) ([]*TaskResult, error) {
	executor := NewParallelExecutor(0)
	return executor.ExecutePipeline(ctx, tasks, agents)
}

// PipelineExecutor executes tasks in pipeline mode
type PipelineExecutor struct{}

func (pe *PipelineExecutor) ExecuteParallel(ctx context.Context, tasks []*Task, agents map[RoleType]*RoleAgent) ([]*TaskResult, error) {
	executor := NewParallelExecutor(0)
	return executor.ExecutePipeline(ctx, tasks, agents)
}

func (pe *PipelineExecutor) ExecuteSequential(ctx context.Context, tasks []*Task, agents map[RoleType]*RoleAgent) ([]*TaskResult, error) {
	executor := NewParallelExecutor(0)
	return executor.ExecutePipeline(ctx, tasks, agents)
}

func (pe *PipelineExecutor) ExecutePipeline(ctx context.Context, tasks []*Task, agents map[RoleType]*RoleAgent) ([]*TaskResult, error) {
	executor := NewParallelExecutor(0)
	return executor.ExecutePipeline(ctx, tasks, agents)
}

// FromConfig returns the config.TeamToolsConfig directly
// TeamConfig is now an alias to config.TeamToolsConfig
func FromConfig(teamConfig *config.TeamToolsConfig) *config.TeamToolsConfig {
	if teamConfig == nil {
		return nil
	}
	return teamConfig
}

// parseTimeout parses timeout string to duration
func parseTimeout(timeoutStr string) time.Duration {
	if timeoutStr == "" {
		return 5 * time.Minute
	}
	if d, err := time.ParseDuration(timeoutStr); err == nil {
		return d
	}
	return 5 * time.Minute
}

// getDefaultRoleConfigs returns default role configurations
func getDefaultRoleConfigs() []TeamRoleConfig {
	return []TeamRoleConfig{
		{Role: "TaskLeader", Model: &config.AgentModelConfig{Primary: "claude-sonnet-4.6"}, MaxRetries: 3, Timeout: "10m"},
		{Role: "CodeBuilder", Model: &config.AgentModelConfig{Primary: "gpt4"}, MaxRetries: 3, Timeout: "15m"},
		{Role: "StrategicPlanner", Model: &config.AgentModelConfig{Primary: "claude-sonnet-4.6"}, MaxRetries: 2, Timeout: "5m"},
		{Role: "TechAdvisor", Model: &config.AgentModelConfig{Primary: "claude-sonnet-4.6"}, MaxRetries: 2, Timeout: "5m"},
		{Role: "LibraryResearcher", Model: &config.AgentModelConfig{Primary: "deepseek"}, MaxRetries: 2, Timeout: "3m"},
		{Role: "CodeExplorer", Model: &config.AgentModelConfig{Primary: "deepseek"}, MaxRetries: 2, Timeout: "3m"},
		{Role: "PreAnalyzer", Model: &config.AgentModelConfig{Primary: "claude-sonnet-4.6"}, MaxRetries: 2, Timeout: "5m"},
		{Role: "PlanReviewer", Model: &config.AgentModelConfig{Primary: "claude-sonnet-4.6"}, MaxRetries: 2, Timeout: "3m"},
		{Role: "VisionAnalyzer", Model: &config.AgentModelConfig{Primary: "gpt4"}, MaxRetries: 3, Timeout: "10m"},
	}
}

// defaultRoutingRules returns the default task routing rules
func defaultRoutingRules() []TaskRoutingRule {
	return []TaskRoutingRule{
		{TaskType: TaskTypeExploration, Role: RoleCodeExplorer, Priority: 1},
		{TaskType: TaskTypeExploration, Role: RoleLibraryResearcher, Priority: 2},
		{TaskType: TaskTypePlanning, Role: RolePreAnalyzer, Priority: 1},
		{TaskType: TaskTypePlanning, Role: RoleStrategicPlanner, Priority: 2},
		{TaskType: TaskTypePlanning, Role: RolePlanReviewer, Priority: 3},
		{TaskType: TaskTypeExecution, Role: RoleCodeBuilder, Priority: 1},
		{TaskType: TaskTypeExecution, Role: RoleVisionAnalyzer, Priority: 2},
		{TaskType: TaskTypeAdvisor, Role: RoleTechAdvisor, Priority: 1},
	}
}

// DefaultTeamConfig returns the default team configuration
func DefaultTeamConfig() *TeamConfig {
	return &TeamConfig{
		Enabled:              true,
		DefaultExecutionMode: string(ExecutionModeParallel),
		MaxParallelTasks:     5,
		EnableAdvisor:        true,
		Roles:                getDefaultRoleConfigs(),
	}
}

// GetModelName returns the model name from config, handling both string and object formats
func GetModelName(model *config.AgentModelConfig) string {
	if model == nil {
		return ""
	}
	return model.Primary
}
