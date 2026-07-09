package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/routing"
)

func (h *Handler) registerAgentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/agents", h.handleListAgents)
	mux.HandleFunc("GET /api/agents/{id}/detail", h.handleGetAgentDetail)
	mux.HandleFunc("POST /api/agent-configs", h.handleCreateAgentConfig)
	mux.HandleFunc("GET /api/agent-configs", h.handleListAgentConfigs)
	mux.HandleFunc("GET /api/agent-configs/{id}", h.handleGetAgentConfig)
	mux.HandleFunc("PUT /api/agent-configs/{id}", h.handleUpdateAgentConfig)
	mux.HandleFunc("DELETE /api/agent-configs/{id}", h.handleDeleteAgentConfig)
	mux.HandleFunc("GET /api/agent-workspace-files", h.handleListAgentWorkspaceFiles)
	mux.HandleFunc("GET /api/agent-workspace-files/{filename...}", h.handleGetAgentWorkspaceFile)
	mux.HandleFunc("PUT /api/agent-workspace-files/{filename...}", h.handleUpdateAgentWorkspaceFile)
}

type agentResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	IsDefault   bool   `json:"is_default"`
	ModelName   string `json:"model_name"`
	SkillsCount int    `json:"skills_count"`
}

func (h *Handler) handleListAgents(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}

	defaultAgentID := configuredDefaultAgentID(cfg)
	agents := buildAgentResponses(cfg, defaultAgentID)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"agents":        agents,
		"default_agent": defaultAgentID,
	}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func buildAgentResponses(cfg *config.Config, defaultAgentID string) []agentResponse {
	others := make([]agentResponse, 0, len(cfg.Agents.List))
	mainAgent := agentResponse{
		ID:          routing.DefaultAgentID,
		Name:        "Main",
		IsDefault:   true,
		ModelName:   cfg.Agents.Defaults.GetModelName(),
		SkillsCount: 0,
	}

	for _, agent := range cfg.Agents.List {
		id := routing.NormalizeAgentID(agent.ID)
		if id == routing.DefaultAgentID {
			continue
		}
		name := agent.Name
		if name == "" {
			name = id
		}

		response := agentResponse{
			ID:          id,
			Name:        name,
			IsDefault:   false,
			ModelName:   effectiveAgentModelName(agent, cfg.Agents.Defaults),
			SkillsCount: len(agent.Skills),
		}
		others = append(others, response)
	}

	agents := make([]agentResponse, 0, len(others)+1)
	agents = append(agents, mainAgent)
	agents = append(agents, others...)

	return agents
}

func effectiveAgentModelName(agent config.AgentConfig, defaults config.AgentDefaults) string {
	if agent.Model != nil && agent.Model.Primary != "" {
		return agent.Model.Primary
	}
	return defaults.GetModelName()
}
