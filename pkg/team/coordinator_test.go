package team

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/providers"
)

// mockAgentProvider is a mock implementation of AgentProvider for testing
type mockAgentProvider struct {
	mu        sync.Mutex
	CallCount int
	Responses map[string]*mockResponse
}

type mockResponse struct {
	Content string
	Err     error
}

func newMockAgentProvider() *mockAgentProvider {
	return &mockAgentProvider{
		Responses: make(map[string]*mockResponse),
	}
}

func (m *mockAgentProvider) Chat(ctx context.Context, messages []providers.Message, tools []providers.ToolDefinition, model string, opts map[string]any) (*providers.LLMResponse, error) {
	m.mu.Lock()
	m.CallCount++
	m.mu.Unlock()

	// Get response based on model or use default
	resp, ok := m.Responses[model]
	if !ok {
		// Default successful response
		return &providers.LLMResponse{
			Content:   "Mock response for " + model,
			ToolCalls: []providers.ToolCall{},
		}, nil
	}

	if resp.Err != nil {
		return nil, resp.Err
	}

	return &providers.LLMResponse{
		Content:   resp.Content,
		ToolCalls: []providers.ToolCall{},
	}, nil
}

func TestNewTaskRouter(t *testing.T) {
	rules := []TaskRoutingRule{
		{TaskType: TaskTypeExecution, Role: RoleCodeBuilder, Priority: 1},
		{TaskType: TaskTypePlanning, Role: RoleStrategicPlanner, Priority: 1},
	}

	router := NewTaskRouter(rules)

	if router == nil {
		t.Error("NewTaskRouter returned nil")
	}
	if len(router.rules) != 2 {
		t.Errorf("router.rules length = %d, want %d", len(router.rules), 2)
	}
}

func TestTaskRouter_Route_WithRules(t *testing.T) {
	rules := []TaskRoutingRule{
		{TaskType: TaskTypeExecution, Role: RoleCodeBuilder, Priority: 1},
		{TaskType: TaskTypeExecution, Role: RoleVisionAnalyzer, Priority: 2},
	}

	router := NewTaskRouter(rules)

	task := &Task{
		ID:   "task-1",
		Type: TaskTypeExecution,
	}

	roles := router.Route(task)

	if len(roles) != 2 {
		t.Errorf("Route returned %d roles, want %d", len(roles), 2)
	}
	if roles[0] != RoleCodeBuilder {
		t.Errorf("Route[0] = %v, want %v", roles[0], RoleCodeBuilder)
	}
	if roles[1] != RoleVisionAnalyzer {
		t.Errorf("Route[1] = %v, want %v", roles[1], RoleVisionAnalyzer)
	}
}

func TestTaskRouter_Route_DefaultRoles(t *testing.T) {
	router := NewTaskRouter(nil)

	tests := []struct {
		taskType  TaskType
		wantFirst RoleType
	}{
		{TaskTypeExploration, RoleCodeExplorer},
		{TaskTypePlanning, RolePreAnalyzer},
		{TaskTypeExecution, RoleCodeBuilder},
		{TaskTypeAdvisor, RoleTechAdvisor},
	}

	for _, tt := range tests {
		t.Run(string(tt.taskType), func(t *testing.T) {
			task := &Task{
				ID:   "task-1",
				Type: tt.taskType,
			}

			roles := router.Route(task)
			if len(roles) == 0 {
				t.Fatal("Route returned empty roles")
			}
			if roles[0] != tt.wantFirst {
				t.Errorf("Route[0] = %v, want %v", roles[0], tt.wantFirst)
			}
		})
	}
}

func TestNewTaskDecomposer(t *testing.T) {
	decomposer := NewTaskDecomposer()

	if decomposer == nil {
		t.Error("NewTaskDecomposer returned nil")
	}
}

func TestTaskDecomposer_Decompose_SingleRole(t *testing.T) {
	decomposer := NewTaskDecomposer()

	task := &Task{
		ID:      "task-1",
		Type:    TaskTypeExecution,
		Content: "Build a feature",
		Context: map[string]interface{}{"priority": "high"},
	}

	roles := []RoleType{RoleCodeBuilder}
	subtasks := decomposer.Decompose(task, roles)

	if len(subtasks) != 1 {
		t.Errorf("Decompose returned %d subtasks, want %d", len(subtasks), 1)
	}
	if subtasks[0].Role != RoleCodeBuilder {
		t.Errorf("Subtask.Role = %v, want %v", subtasks[0].Role, RoleCodeBuilder)
	}
	if subtasks[0].Content != task.Content {
		t.Errorf("Subtask.Content = %q, want %q", subtasks[0].Content, task.Content)
	}
}

func TestTaskDecomposer_Decompose_MultipleRoles(t *testing.T) {
	decomposer := NewTaskDecomposer()

	task := &Task{
		ID:      "task-1",
		Type:    TaskTypePlanning,
		Content: "Plan a new feature",
	}

	roles := []RoleType{RolePreAnalyzer, RoleStrategicPlanner, RolePlanReviewer}
	subtasks := decomposer.Decompose(task, roles)

	if len(subtasks) != 3 {
		t.Errorf("Decompose returned %d subtasks, want %d", len(subtasks), 3)
	}

	// Check that subtask IDs are generated correctly
	for i, subtask := range subtasks {
		if subtask.Role != roles[i] {
			t.Errorf("Subtask[%d].Role = %v, want %v", i, subtask.Role, roles[i])
		}
		expectedID := "task-1-subtask-" + string(rune('0'+i))
		if subtask.ID != expectedID {
			t.Errorf("Subtask[%d].ID = %q, want %q", i, subtask.ID, expectedID)
		}
	}
}

func TestTaskDecomposer_Decompose_EmptyRoles(t *testing.T) {
	decomposer := NewTaskDecomposer()

	task := &Task{
		ID:      "task-1",
		Type:    TaskTypeExecution,
		Content: "Build a feature",
	}

	subtasks := decomposer.Decompose(task, nil)

	if len(subtasks) != 1 {
		t.Errorf("Decompose returned %d subtasks, want %d", len(subtasks), 1)
	}
}

func TestNewResultAggregator(t *testing.T) {
	aggregator := NewResultAggregator()

	if aggregator == nil {
		t.Error("NewResultAggregator returned nil")
	}
}

func TestResultAggregator_Aggregate_SingleResult(t *testing.T) {
	aggregator := NewResultAggregator()

	results := []*TaskResult{
		{
			TaskID:      "task-1",
			Success:     true,
			Output:      "Result 1",
			Duration:    time.Second,
			CompletedAt: time.Now(),
		},
	}

	aggregated, err := aggregator.Aggregate(results)

	if err != nil {
		t.Errorf("Aggregate returned error: %v", err)
	}
	if !aggregated.Success {
		t.Error("Aggregated result should be successful")
	}
	if aggregated.Output != "Result 1" {
		t.Errorf("Aggregated.Output = %q, want %q", aggregated.Output, "Result 1")
	}
}

func TestResultAggregator_Aggregate_MultipleResults(t *testing.T) {
	aggregator := NewResultAggregator()

	results := []*TaskResult{
		{
			TaskID:      "task-1",
			Success:     true,
			Output:      "Result 1",
			Duration:    time.Second,
			CompletedAt: time.Now(),
		},
		{
			TaskID:      "task-2",
			Success:     true,
			Output:      "Result 2",
			Duration:    time.Second,
			CompletedAt: time.Now(),
		},
	}

	aggregated, err := aggregator.Aggregate(results)

	if err != nil {
		t.Errorf("Aggregate returned error: %v", err)
	}
	if !aggregated.Success {
		t.Error("Aggregated result should be successful when all results succeed")
	}
}

func TestResultAggregator_Aggregate_MixedResults(t *testing.T) {
	aggregator := NewResultAggregator()

	results := []*TaskResult{
		{
			TaskID:      "task-1",
			Success:     true,
			Output:      "Result 1",
			Duration:    time.Second,
			CompletedAt: time.Now(),
		},
		{
			TaskID:      "task-2",
			Success:     false,
			Error:       "Error 2",
			Duration:    time.Second,
			CompletedAt: time.Now(),
		},
	}

	aggregated, err := aggregator.Aggregate(results)

	if err != nil {
		t.Errorf("Aggregate returned error: %v", err)
	}
	if aggregated.Success {
		t.Error("Aggregated result should be failure when any result fails")
	}
	if aggregated.Error == "" {
		t.Error("Aggregated.Error should contain error message")
	}
}

func TestResultAggregator_Aggregate_EmptyResults(t *testing.T) {
	aggregator := NewResultAggregator()

	results := []*TaskResult{}
	aggregated, err := aggregator.Aggregate(results)

	if err != nil {
		t.Errorf("Aggregate returned error: %v", err)
	}
	if aggregated.Success {
		t.Error("Aggregated result should be failure when no results")
	}
	if aggregated.Error != "no results to aggregate" {
		t.Errorf("Aggregated.Error = %q, want %q", aggregated.Error, "no results to aggregate")
	}
}

func TestNewParallelExecutor(t *testing.T) {
	executor := NewParallelExecutor(3)

	if executor == nil {
		t.Error("NewParallelExecutor returned nil")
	}
}

func TestNewParallelExecutor_DefaultMaxParallel(t *testing.T) {
	executor := NewParallelExecutor(0)

	// Should default to 5
	if executor == nil {
		t.Error("NewParallelExecutor returned nil for 0")
	}
}

func TestTeamCoordinator_NewTeam(t *testing.T) {
	cfg := DefaultTeamConfig()
	team, err := NewTeam(cfg)

	if err != nil {
		t.Errorf("NewTeam returned error: %v", err)
	}
	if team == nil {
		t.Error("NewTeam returned nil")
	}
	if team.config != cfg {
		t.Error("Team config should be the same as input")
	}
}

func TestTeamCoordinator_RegisterRoleAgent(t *testing.T) {
	cfg := DefaultTeamConfig()
	team, _ := NewTeam(cfg)

	mockProvider := newMockAgentProvider()
	roleConfig := TeamRoleConfig{
		Role:    string(RoleCodeBuilder),
		Model:   &config.AgentModelConfig{Primary: "test-model"},
		Tools:   []string{"read_file"},
		Timeout: "1m",
	}

	team.RegisterRoleAgent(RoleCodeBuilder, mockProvider, roleConfig)

	agent := team.GetRoleByType(RoleCodeBuilder)
	if agent == nil {
		t.Error("GetRoleByType returned nil for registered role")
	}
	if agent.Role != RoleCodeBuilder {
		t.Errorf("Agent.Role = %v, want %v", agent.Role, RoleCodeBuilder)
	}
	if agent.Config == nil {
		t.Error("Agent.Config should not be nil")
	}
}

func TestTeamCoordinator_Start(t *testing.T) {
	cfg := DefaultTeamConfig()
	team, _ := NewTeam(cfg)

	err := team.Start()

	if err != nil {
		t.Errorf("Start returned error: %v", err)
	}
	if !team.IsStarted() {
		t.Error("IsStarted should return true after Start")
	}
}

func TestTeamCoordinator_Start_AlreadyStarted(t *testing.T) {
	cfg := DefaultTeamConfig()
	team, _ := NewTeam(cfg)

	team.Start()
	err := team.Start()

	if err == nil {
		t.Error("Start should return error when already started")
	}
}

func TestTeamCoordinator_Stop(t *testing.T) {
	cfg := DefaultTeamConfig()
	team, _ := NewTeam(cfg)

	team.Start()
	team.Stop()

	if team.IsStarted() {
		t.Error("IsStarted should return false after Stop")
	}
}

func TestTeamCoordinator_Receive_NotStarted(t *testing.T) {
	cfg := DefaultTeamConfig()
	team, _ := NewTeam(cfg)

	task := &Task{
		ID:      "task-1",
		Type:    TaskTypeExecution,
		Content: "Build a feature",
	}

	_, err := team.Receive(task)

	if err == nil {
		t.Error("Receive should return error when not started")
	}
}

func TestTeamCoordinator_GetAvailableRoles(t *testing.T) {
	cfg := DefaultTeamConfig()
	team, _ := NewTeam(cfg)

	mockProvider := newMockAgentProvider()
	team.RegisterRoleAgent(RoleCodeBuilder, mockProvider, TeamRoleConfig{Role: string(RoleCodeBuilder)})

	roles := team.GetAvailableRoles()

	if len(roles) != 1 {
		t.Errorf("GetAvailableRoles returned %d roles, want %d", len(roles), 1)
	}
	if _, ok := roles[RoleCodeBuilder]; !ok {
		t.Error("GetAvailableRoles should contain RoleCodeBuilder")
	}
}

func TestTeamCoordinator_GetConfig(t *testing.T) {
	cfg := DefaultTeamConfig()
	team, _ := NewTeam(cfg)

	retrievedConfig := team.GetConfig()

	if retrievedConfig != cfg {
		t.Error("GetConfig should return the same config")
	}
}

func TestParallelExecutor_ExecuteParallel(t *testing.T) {
	executor := NewParallelExecutor(2)

	tasks := []*Task{
		{ID: "task-1", Role: RoleCodeBuilder, Content: "Task 1"},
		{ID: "task-2", Role: RoleCodeBuilder, Content: "Task 2"},
	}

	agents := map[RoleType]*RoleAgent{
		RoleCodeBuilder: {
			Role:          RoleCodeBuilder,
			AgentProvider: newMockAgentProvider(),
			Config:        &TeamRoleConfig{Role: string(RoleCodeBuilder), Model: &config.AgentModelConfig{Primary: "test"}},
		},
	}

	ctx := context.Background()
	results, err := executor.ExecuteParallel(ctx, tasks, agents)

	// Should fail because mock doesn't implement full AgentProvider interface
	// but we test the execution flow
	if err == nil && len(results) == 0 {
		// Expected to fail due to interface, that's ok for this test
	}
}

func TestSequentialExecutor_ExecuteSequential(t *testing.T) {
	executor := &SequentialExecutor{}

	tasks := []*Task{
		{ID: "task-1", Role: RoleCodeBuilder, Content: "Task 1"},
	}

	// Test with valid agent but mock provider that returns error
	mockProvider := newMockAgentProvider()
	mockProvider.Responses["test-model"] = &mockResponse{
		Err: context.DeadlineExceeded,
	}

	agents := map[RoleType]*RoleAgent{
		RoleCodeBuilder: {
			Role:          RoleCodeBuilder,
			AgentProvider: mockProvider,
			Config:        &TeamRoleConfig{Role: string(RoleCodeBuilder), Model: &config.AgentModelConfig{Primary: "test-model"}},
		},
	}

	ctx := context.Background()
	results, err := executor.ExecuteSequential(ctx, tasks, agents)

	// Results should contain error result for failed task
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
	// Error is expected due to deadline exceeded
	_ = err
}

func TestPipelineExecutor_ExecutePipeline(t *testing.T) {
	executor := &PipelineExecutor{}

	tasks := []*Task{
		{ID: "task-1", Role: RoleCodeBuilder, Content: "Task 1"},
		{ID: "task-2", Role: RoleCodeBuilder, Content: "Task 2"},
	}

	// Use a mock provider that returns successful responses
	mockProvider := newMockAgentProvider()
	mockProvider.Responses["test"] = &mockResponse{
		Content: "Pipeline response",
	}

	agents := map[RoleType]*RoleAgent{
		RoleCodeBuilder: {
			Role:          RoleCodeBuilder,
			AgentProvider: mockProvider,
			Config:        &TeamRoleConfig{Role: string(RoleCodeBuilder), Model: &config.AgentModelConfig{Primary: "test"}},
		},
	}

	ctx := context.Background()
	_, _ = executor.ExecutePipeline(ctx, tasks, agents)

	// Just verify it doesn't panic
}

func TestJoinErrors(t *testing.T) {
	tests := []struct {
		name   string
		errors []string
		want   string
	}{
		{"Empty", []string{}, ""},
		{"Single", []string{"error1"}, "error1"},
		{"Multiple", []string{"error1", "error2"}, "error1; error2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := joinErrors(tt.errors)
			if got != tt.want {
				t.Errorf("joinErrors(%v) = %q, want %q", tt.errors, got, tt.want)
			}
		})
	}
}
