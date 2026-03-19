package team

import (
	"testing"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
)

func TestTaskType_Constants(t *testing.T) {
	tests := []struct {
		name     string
		want     TaskType
		expected string
	}{
		{"Exploration", TaskTypeExploration, "exploration"},
		{"Planning", TaskTypePlanning, "planning"},
		{"Execution", TaskTypeExecution, "execution"},
		{"Advisor", TaskTypeAdvisor, "advisor"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.want) != tt.expected {
				t.Errorf("TaskType %s = %q, want %q", tt.name, tt.want, tt.expected)
			}
		})
	}
}

func TestRoleType_Constants(t *testing.T) {
	tests := []struct {
		name     string
		want     RoleType
		expected string
	}{
		{"TaskLeader", RoleTaskLeader, "TaskLeader"},
		{"CodeBuilder", RoleCodeBuilder, "CodeBuilder"},
		{"StrategicPlanner", RoleStrategicPlanner, "StrategicPlanner"},
		{"TechAdvisor", RoleTechAdvisor, "TechAdvisor"},
		{"LibraryResearcher", RoleLibraryResearcher, "LibraryResearcher"},
		{"CodeExplorer", RoleCodeExplorer, "CodeExplorer"},
		{"PreAnalyzer", RolePreAnalyzer, "PreAnalyzer"},
		{"PlanReviewer", RolePlanReviewer, "PlanReviewer"},
		{"VisionAnalyzer", RoleVisionAnalyzer, "VisionAnalyzer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.want) != tt.expected {
				t.Errorf("RoleType %s = %q, want %q", tt.name, tt.want, tt.expected)
			}
		})
	}
}

func TestExecutionMode_Constants(t *testing.T) {
	tests := []struct {
		name     string
		want     ExecutionMode
		expected string
	}{
		{"Parallel", ExecutionModeParallel, "parallel"},
		{"Sequential", ExecutionModeSequential, "sequential"},
		{"Pipeline", ExecutionModePipeline, "pipeline"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.want) != tt.expected {
				t.Errorf("ExecutionMode %s = %q, want %q", tt.name, tt.want, tt.expected)
			}
		})
	}
}

func TestTask_creation(t *testing.T) {
	now := time.Now()
	retryPolicy := &RetryPolicy{
		MaxAttempts: 3,
		Backoff:     time.Second,
		Multiplier:  2.0,
	}

	task := &Task{
		ID:          "task-1",
		Type:        TaskTypeExecution,
		Content:     "Write a unit test",
		Context:     map[string]interface{}{"priority": "high"},
		DependsOn:   []string{"task-0"},
		Role:        RoleCodeBuilder,
		Priority:    1,
		CreatedAt:   now,
		RetryPolicy: retryPolicy,
	}

	if task.ID != "task-1" {
		t.Errorf("Task.ID = %q, want %q", task.ID, "task-1")
	}
	if task.Type != TaskTypeExecution {
		t.Errorf("Task.Type = %v, want %v", task.Type, TaskTypeExecution)
	}
	if task.Role != RoleCodeBuilder {
		t.Errorf("Task.Role = %v, want %v", task.Role, RoleCodeBuilder)
	}
	if len(task.DependsOn) != 1 || task.DependsOn[0] != "task-0" {
		t.Errorf("Task.DependsOn = %v, want [%v]", task.DependsOn, []string{"task-0"})
	}
	if task.RetryPolicy.MaxAttempts != 3 {
		t.Errorf("Task.RetryPolicy.MaxAttempts = %d, want %d", task.RetryPolicy.MaxAttempts, 3)
	}
}

func TestTaskResult_creation(t *testing.T) {
	now := time.Now()
	result := &TaskResult{
		TaskID:      "task-1",
		Success:     true,
		Output:      "Task completed successfully",
		Error:       "",
		Metadata:    map[string]interface{}{"iterations": 5},
		Duration:    time.Second,
		CompletedAt: now,
	}

	if !result.Success {
		t.Error("TaskResult.Success = false, want true")
	}
	if result.Output != "Task completed successfully" {
		t.Errorf("TaskResult.Output = %q, want %q", result.Output, "Task completed successfully")
	}
	if result.Error != "" {
		t.Errorf("TaskResult.Error = %q, want empty", result.Error)
	}
}

func TestTaskResult_failure(t *testing.T) {
	result := &TaskResult{
		TaskID:      "task-1",
		Success:     false,
		Output:      "",
		Error:       "something went wrong",
		Metadata:    nil,
		Duration:    time.Second,
		CompletedAt: time.Now(),
	}

	if result.Success {
		t.Error("TaskResult.Success = true, want false")
	}
	if result.Error != "something went wrong" {
		t.Errorf("TaskResult.Error = %q, want %q", result.Error, "something went wrong")
	}
}

func TestTeamRoleConfig_creation(t *testing.T) {
	roleCfg := &TeamRoleConfig{
		Role:         string(RoleCodeBuilder),
		Model:        &config.AgentModelConfig{Primary: "claude-sonnet-4-20250514", Fallbacks: []string{"gpt-4o"}},
		MaxRetries:   3,
		Timeout:      "10m",
		Tools:        []string{"read_file", "write_file"},
		SystemPrompt: "You are a code builder",
	}

	if roleCfg.Role != string(RoleCodeBuilder) {
		t.Errorf("TeamRoleConfig.Role = %v, want %v", roleCfg.Role, string(RoleCodeBuilder))
	}
	if roleCfg.Model == nil || roleCfg.Model.Primary != "claude-sonnet-4-20250514" {
		t.Errorf("TeamRoleConfig.Model.Primary = %v, want %v", roleCfg.Model, "claude-sonnet-4-20250514")
	}
	if len(roleCfg.Model.Fallbacks) != 1 {
		t.Errorf("TeamRoleConfig.Model.Fallbacks length = %d, want %d", len(roleCfg.Model.Fallbacks), 1)
	}
	if len(roleCfg.Tools) != 2 {
		t.Errorf("TeamRoleConfig.Tools length = %d, want %d", len(roleCfg.Tools), 2)
	}
}

func TestTeamConfig_creation(t *testing.T) {
	teamCfg := &TeamConfig{
		Enabled:              true,
		DefaultExecutionMode: string(ExecutionModeParallel),
		MaxParallelTasks:     5,
		EnableAdvisor:        true,
		Roles:                []TeamRoleConfig{},
	}

	if !teamCfg.Enabled {
		t.Error("TeamConfig.Enabled = false, want true")
	}
	if teamCfg.DefaultExecutionMode != string(ExecutionModeParallel) {
		t.Errorf("TeamConfig.DefaultExecutionMode = %v, want %v", teamCfg.DefaultExecutionMode, string(ExecutionModeParallel))
	}
	if teamCfg.MaxParallelTasks != 5 {
		t.Errorf("TeamConfig.MaxParallelTasks = %d, want %d", teamCfg.MaxParallelTasks, 5)
	}
}

func TestTaskRoutingRule_creation(t *testing.T) {
	rule := &TaskRoutingRule{
		TaskType:  TaskTypeExploration,
		Role:      RoleCodeExplorer,
		Priority:  1,
		Condition: "complexity > 5",
	}

	if rule.TaskType != TaskTypeExploration {
		t.Errorf("TaskRoutingRule.TaskType = %v, want %v", rule.TaskType, TaskTypeExploration)
	}
	if rule.Role != RoleCodeExplorer {
		t.Errorf("TaskRoutingRule.Role = %v, want %v", rule.Role, RoleCodeExplorer)
	}
	if rule.Priority != 1 {
		t.Errorf("TaskRoutingRule.Priority = %d, want %d", rule.Priority, 1)
	}
}

func TestTeamMessage_creation(t *testing.T) {
	task := &Task{
		ID:      "task-1",
		Type:    TaskTypeExecution,
		Content: "Build a feature",
	}

	msg := &TeamMessage{
		Type:     TeamMessageTypeTask,
		Task:     task,
		Sender:   RoleTaskLeader,
		Receiver: RoleCodeBuilder,
		ThreadID: "thread-1",
	}

	if msg.Type != TeamMessageTypeTask {
		t.Errorf("TeamMessage.Type = %v, want %v", msg.Type, TeamMessageTypeTask)
	}
	if msg.Task.ID != "task-1" {
		t.Errorf("TeamMessage.Task.ID = %q, want %q", msg.Task.ID, "task-1")
	}
	if msg.Sender != RoleTaskLeader {
		t.Errorf("TeamMessage.Sender = %v, want %v", msg.Sender, RoleTaskLeader)
	}
	if msg.Receiver != RoleCodeBuilder {
		t.Errorf("TeamMessage.Receiver = %v, want %v", msg.Receiver, RoleCodeBuilder)
	}
}

func TestTeamMessageType_Constants(t *testing.T) {
	tests := []struct {
		name     string
		want     TeamMessageType
		expected string
	}{
		{"Task", TeamMessageTypeTask, "task"},
		{"Result", TeamMessageTypeResult, "result"},
		{"Control", TeamMessageTypeControl, "control"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.want) != tt.expected {
				t.Errorf("TeamMessageType %s = %q, want %q", tt.name, tt.want, tt.expected)
			}
		})
	}
}
