package team

import (
	"context"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
)

// TeamConfig is an alias to config.TeamToolsConfig
type TeamConfig = config.TeamToolsConfig

// TeamRoleConfig is an alias to config.TeamRoleConfig
type TeamRoleConfig = config.TeamRoleConfig

// TaskType represents the category of a task
type TaskType string

const (
	TaskTypeExploration TaskType = "exploration" // 代码探索、信息检索
	TaskTypePlanning    TaskType = "planning"    // 任务规划、分析
	TaskTypeExecution   TaskType = "execution"   // 代码实现、修改
	TaskTypeAdvisor     TaskType = "advisor"     // 技术咨询、审查
)

// RoleType defines the semantic role names for the team
type RoleType string

const (
	RoleTaskLeader        RoleType = "TaskLeader"        // 主编排器
	RoleCodeBuilder       RoleType = "CodeBuilder"       // 代码实现
	RoleStrategicPlanner  RoleType = "StrategicPlanner"  // 战略规划
	RoleTechAdvisor       RoleType = "TechAdvisor"       // 技术顾问
	RoleLibraryResearcher RoleType = "LibraryResearcher" // 开源代码库研究
	RoleCodeExplorer      RoleType = "CodeExplorer"      // 代码探索
	RolePreAnalyzer       RoleType = "PreAnalyzer"       // 前置分析
	RolePlanReviewer      RoleType = "PlanReviewer"      // 计划审查
	RoleVisionAnalyzer    RoleType = "VisionAnalyzer"    // 多模态视觉
)

// ExecutionMode defines how tasks should be executed
type ExecutionMode string

const (
	ExecutionModeParallel   ExecutionMode = "parallel"   // 并行执行
	ExecutionModeSequential ExecutionMode = "sequential" // 串行执行
	ExecutionModePipeline   ExecutionMode = "pipeline"   // 流水线执行
)

// Task represents a unit of work to be executed by the team
type Task struct {
	ID          string                 `json:"id"`
	Type        TaskType               `json:"type"`
	Content     string                 `json:"content"`
	Context     map[string]interface{} `json:"context"`
	DependsOn   []string               `json:"depends_on,omitempty"`
	Role        RoleType               `json:"role"`
	Priority    int                    `json:"priority"`
	CreatedAt   time.Time              `json:"created_at"`
	RetryPolicy *RetryPolicy           `json:"retry_policy,omitempty"`
	Repos       []RepoSpec             `json:"repos,omitempty"` // Repositories to work on
}

// RetryPolicy defines retry behavior for failed tasks
type RetryPolicy struct {
	MaxAttempts int           `json:"max_attempts"`
	Backoff     time.Duration `json:"backoff"`
	Multiplier  float64       `json:"multiplier"`
}

// TaskResult holds the result of a task execution
type TaskResult struct {
	TaskID      string                 `json:"task_id"`
	Success     bool                   `json:"success"`
	Output      string                 `json:"output"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata"`
	Duration    time.Duration          `json:"duration"`
	CompletedAt time.Time              `json:"completed_at"`
}

// TaskRoutingRule defines how tasks are routed to roles
type TaskRoutingRule struct {
	TaskType  TaskType `json:"task_type"`
	Role      RoleType `json:"role"`
	Priority  int      `json:"priority"`
	Condition string   `json:"condition,omitempty"`
}

// TeamMessageType represents the type of team message
type TeamMessageType string

const (
	TeamMessageTypeTask    TeamMessageType = "task"
	TeamMessageTypeResult  TeamMessageType = "result"
	TeamMessageTypeControl TeamMessageType = "control"
)

// TeamMessage represents a message in the team communication
type TeamMessage struct {
	Type     TeamMessageType `json:"type"`
	Task     *Task           `json:"task,omitempty"`
	Result   *TaskResult     `json:"result,omitempty"`
	Sender   RoleType        `json:"sender"`
	Receiver RoleType        `json:"receiver"`
	ThreadID string          `json:"thread_id"`
}

// TaskHandler is the interface for handling tasks
type TaskHandler interface {
	HandleTask(ctx context.Context, task *Task) (*TaskResult, error)
}

// RoleAgent wraps an AgentProvider with role-specific configuration
type RoleAgent struct {
	Role          RoleType
	AgentProvider AgentProvider
	Config        *TeamRoleConfig
	executing     bool
	lastResult    *TaskResult
}

// TaskExecutor defines the interface for task execution engines
type TaskExecutor interface {
	ExecuteParallel(ctx context.Context, tasks []*Task, agents map[RoleType]*RoleAgent) ([]*TaskResult, error)
	ExecuteSequential(ctx context.Context, tasks []*Task, agents map[RoleType]*RoleAgent) ([]*TaskResult, error)
	ExecutePipeline(ctx context.Context, tasks []*Task, agents map[RoleType]*RoleAgent) ([]*TaskResult, error)
}
