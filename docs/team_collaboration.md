# Multi-Agent Team Collaboration System

## Overview

The Team Collaboration System is a new module in picoclaw that enables multiple AI agents to work together on tasks. Inspired by systems like oh-my-opencode, this implementation provides a semantic role-based architecture that differs from traditional single-agent approaches.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    TeamCoordinator                               │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐             │
│  │ TaskRouter  │  │TaskDecompose│  │ResultAggre- │             │
│  │             │  │             │  │   gator     │             │
│  └─────────────┘  └─────────────┘  └─────────────┘             │
└─────────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        ▼                     ▼                     ▼
┌───────────────┐     ┌───────────────┐     ┌───────────────┐
│  Exploration  │     │   Planning    │     │  Execution   │
│     Team      │     │     Team      │     │     Team     │
├───────────────┤     ├───────────────┤     ├───────────────┤
│ • CodeExplorer│     │ • PreAnalyzer │     │ • CodeBuilder │
│ •LibraryRe-   │     │ •Strategic-   │     │ •Vision-     │
│   searcher    │     │   Planner     │     │   Analyzer   │
│               │     │ •PlanReviewer │     │               │
└───────────────┘     └───────────────┘     └───────────────┘
        │                     │                     │
        └─────────────────────┼─────────────────────┘
                              ▼
                    ┌─────────────────┐
                    │  Advisor Team   │
                    ├─────────────────┤
                    │ • TechAdvisor   │
                    └─────────────────┘
```

## Core Components

### 1. Types (pkg/team/types.go)

- **TaskType**: Four categories - exploration, planning, execution, advisor
- **RoleType**: Nine semantic roles (see below)
- **ExecutionMode**: parallel, sequential, pipeline
- **Task**: Work unit with content, context, dependencies
- **TaskResult**: Execution result with output and metadata

### 2. Coordinator (pkg/team/coordinator.go)

- **TaskRouter**: Routes tasks to appropriate roles based on task type
- **TaskDecomposer**: Splits complex tasks into subtasks
- **ResultAggregator**: Merges results from multiple agents
- **ParallelExecutor**: Concurrent task execution with semaphore control
- **SequentialExecutor**: Sequential task execution
- **PipelineExecutor**: Pipeline mode (like CI/CD)

### 3. Configuration (pkg/team/config.go)

Default configuration for all 9 semantic roles with model, tools, and system prompts.

## Role Definitions

| Role | Responsibility | Default Model |
|------|----------------|---------------|
| **TaskLeader** | Main orchestrator, task routing and coordination | claude-sonnet-4-20250514 |
| **CodeBuilder** | Code implementation and modification | claude-sonnet-4-20250514 |
| **StrategicPlanner** | Interview-style deep planning | claude-sonnet-4-20250514 |
| **TechAdvisor** | Technical consultation, architecture guidance | claude-sonnet-4-20250514 |
| **LibraryResearcher** | Third-party library research | claude-sonnet-4-20250514 |
| **CodeExplorer** | Codebase structure exploration | claude-sonnet-4-20250514 |
| **PreAnalyzer** | Pre-implementation analysis | claude-sonnet-4-20250514 |
| **PlanReviewer** | Plan validation and review | claude-sonnet-4-20250514 |
| **VisionAnalyzer** | Multi-modal vision, UI/UX analysis | claude-sonnet-4-20250514 |

## Task Classification

Tasks are automatically classified based on their content:

| Task Type | Keywords | Roles Assigned |
|-----------|----------|----------------|
| **exploration** | explore, find, search, understand | CodeExplorer, LibraryResearcher |
| **planning** | plan, design, strategy, approach | PreAnalyzer, StrategicPlanner, PlanReviewer |
| **execution** | implement, code, build, create, fix | CodeBuilder, VisionAnalyzer |
| **advisor** | review, advise, consult, help | TechAdvisor |

## Workflow Examples

### Example 1: Parallel Execution

**User Request**: "Find all files related to authentication and check if there are any good third-party auth libraries"

```go
task := &Task{
    ID:      "task-001",
    Type:    TaskTypeExploration,
    Content: "Find all files related to authentication and check if there are any good third-party auth libraries",
}

result, err := coordinator.Receive(task)
```

**Workflow**:
```
TaskRouter.Route() → [CodeExplorer, LibraryResearcher]
                     ↓
TaskDecomposer.Decompose() → [subtask-1, subtask-2]
                              ↓
ParallelExecutor.ExecuteParallel() 
  → CodeExplorer: reads files, finds auth-related code
  → LibraryResearcher: searches for auth libraries
                     ↓
ResultAggregator.Aggregate() → Combined result
```

### Example 2: Sequential Execution (Dependent Tasks)

**User Request**: "First analyze the requirements, then create a detailed plan"

```go
task := &Task{
    ID:        "task-002",
    Type:      TaskTypePlanning,
    Content:   "Create a user authentication system with OAuth2 support",
    DependsOn: []string{}, // This task depends on analysis first
}

result, err := coordinator.Receive(task)
```

**Workflow**:
```
TaskRouter.Route() → [PreAnalyzer, StrategicPlanner, PlanReviewer]
                     ↓
TaskDecomposer.Decompose() → [subtask-1, subtask-2, subtask-3]
                              ↓
SequentialExecutor.ExecuteSequential()
  → PreAnalyzer: analyzes requirements → output feeds into next
  → StrategicPlanner: creates plan based on analysis → feeds to next
  → PlanReviewer: validates the plan
                     ↓
ResultAggregator.Aggregate() → Final plan with validation
```

### Example 3: Pipeline Execution

**User Request**: "Refactor the entire authentication module"

```go
task := &Task{
    ID:      "task-003",
    Type:    TaskTypeExecution,
    Content: "Refactor the entire authentication module to use a better architecture",
}

// Set execution mode to pipeline
config := DefaultTeamConfig()
config.DefaultExecutionMode = ExecutionModePipeline

result, err := coordinator.Receive(task)
```

**Workflow**:
```
TaskRouter.Route() → [CodeBuilder]
                     ↓
TaskDecomposer.Decompose() → [subtask-1, subtask-2, ...]
                              ↓
PipelineExecutor.ExecutePipeline()
  Stage 1: Explore codebase → results pass to next stage
  Stage 2: Analyze current implementation → results pass to next
  Stage 3: Create refactoring plan → results pass to next
  Stage 4: Execute refactoring → final result
                     ↓
ResultAggregator.Aggregate() → Complete refactored code
```

### Example 4: Advisor Mode

**User Request**: "I'm having trouble with the database connection, can you help?"

```go
task := &Task{
    ID:      "task-004",
    Type:    TaskTypeAdvisor,
    Content: "I'm having trouble with the database connection pool, it keeps timing out",
}

result, err := coordinator.Receive(task)
```

**Workflow**:
```
TaskRouter.Route() → [TechAdvisor]
                     ↓
TechAdvisor analyzes the problem and provides:
  - Root cause analysis
  - Potential solutions
  - Best practices for connection pooling
  - Code examples if applicable
```

### Example 5: Vision Analysis

**User Request**: "Analyze this UI design screenshot and suggest improvements"

```go
task := &Task{
    ID:      "task-005",
    Type:    TaskTypeExecution,
    Content: "Analyze the UI design in the attached image and suggest accessibility improvements",
}

result, err := coordinator.Receive(task)
```

**Workflow**:
```
TaskRouter.Route() → [VisionAnalyzer]
                     ↓
VisionAnalyzer:
  - Describes UI elements and layout
  - Identifies accessibility issues
  - Suggests improvements
  - Reviews color schemes and contrast
```

## Integration with picoclaw

To integrate the Team System into your existing picoclaw setup:

```go
// 1. Create team configuration
config := team.DefaultTeamConfig()
config.DefaultExecutionMode = team.ExecutionModeParallel

// 2. Create coordinator
coordinator, err := team.NewTeam(config)
if err != nil {
    log.Fatal(err)
}

// 3. Register role agents
coordinator.RegisterRoleAgent(
    team.RoleCodeBuilder,
    yourAgentProvider,
    *config.GetRoleConfig(team.RoleCodeBuilder),
)

// 4. Start coordinator
if err := coordinator.Start(); err != nil {
    log.Fatal(err)
}

// 5. Process tasks
task := &team.Task{
    ID:      "my-task",
    Type:    team.TaskTypeExecution,
    Content: "Create a new API endpoint for user management",
}

result, err := coordinator.Receive(task)
```

## Configuration Customization

You can customize roles, routing rules, and execution parameters:

```go
config := &team.TeamConfig{
    EnableTeamMode:       true,
    DefaultExecutionMode: team.ExecutionModeParallel,
    MaxParallelTasks:     3,  // Limit concurrent tasks
    EnableAdvisor:        true,
    
    // Custom roles (override defaults)
    Roles: []team.TeamRoleConfig{
        {
            Role:        team.RoleCodeBuilder,
            Model:       "gpt-4o",
            SystemPrompt: "You are a Go expert...",
            Tools:       []string{"read_file", "write_file"},
        },
    },
    
    // Custom routing rules
    TaskRoutingRules: []team.TaskRoutingRule{
        {
            TaskType: team.TaskTypeExecution,
            Role:     team.RoleCodeBuilder,
            Priority: 1,
        },
    },
}
```

## Dependencies and Results Passing

The system supports task dependencies - later tasks can access results from earlier tasks:

```go
task := &team.Task{
    ID:      "pipeline-task",
    Type:    team.TaskTypeExecution,
    Content: "Implement the feature",
    Context: map[string]interface{}{
        // Previous task results are automatically passed
        "previous_result": "...",
        "previous_success": true,
    },
}
```

## Error Handling

- Automatic retry with configurable backoff
- Fallback to alternative models
- Graceful degradation when agents fail
- Detailed error aggregation in results
