package team

import (
	"fmt"

	"github.com/sipeed/picoclaw/pkg/tools"
)

// ToolFactory builds per-repo tool registries
type ToolFactory struct{}

// NewToolFactory creates a new ToolFactory
func NewToolFactory() *ToolFactory {
	return &ToolFactory{}
}

// BuildForRepo creates a ToolRegistry with file and exec tools bound to a repo's worktree.
// The tools are unrestricted (restrict=false) since the worktree is already an isolated directory.
func (tf *ToolFactory) BuildForRepo(repo *Repo) (*tools.ToolRegistry, error) {
	registry := tools.NewToolRegistry()

	// File tools — workspace = repo.LocalPath, restrict = false (worktree is the sandbox)
	registry.Register(tools.NewReadFileTool(repo.LocalPath, false, 0))
	registry.Register(tools.NewWriteFileTool(repo.LocalPath, false))
	registry.Register(tools.NewListDirTool(repo.LocalPath, false))
	registry.Register(tools.NewEditFileTool(repo.LocalPath, false))
	registry.Register(tools.NewAppendFileTool(repo.LocalPath, false))

	// Exec tool — working directory = repo.LocalPath
	execTool, err := tools.NewExecToolWithConfig(repo.LocalPath, false, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec tool for repo %s: %w", repo.Name, err)
	}
	registry.Register(execTool)

	return registry, nil
}
