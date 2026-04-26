package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/routing"
)

type agentConfigResponse struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	IsMain             bool     `json:"is_main"`
	IsImplicit         bool     `json:"is_implicit"`
	IsDefault          bool     `json:"is_default"`
	ModelName          string   `json:"model_name"`
	ModelFallbacks     []string `json:"model_fallbacks,omitempty"`
	EffectiveModel     string   `json:"effective_model_name"`
	Workspace          string   `json:"workspace"`
	EffectiveWorkspace string   `json:"effective_workspace"`
	Skills             []string `json:"skills,omitempty"`
	SkillsCount        int      `json:"skills_count"`
	CanDelete          bool     `json:"can_delete"`
	DeleteBlockReason  string   `json:"delete_block_reason,omitempty"`
	Index              int      `json:"index"`
}

type agentConfigRequest struct {
	ID             string   `json:"id,omitempty"`
	Name           string   `json:"name"`
	Workspace      string   `json:"workspace"`
	ModelName      string   `json:"model_name"`
	ModelFallbacks []string `json:"model_fallbacks,omitempty"`
	Skills         []string `json:"skills,omitempty"`
	IsDefault      bool     `json:"is_default"`
}

func (h *Handler) handleListAgentConfigs(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}

	defaultAgentID := configuredDefaultAgentID(cfg)
	agents := buildAgentConfigResponses(cfg, defaultAgentID)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"agents":        agents,
		"default_agent": defaultAgentID,
		"total":         len(agents),
	}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) handleGetAgentConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}

	agentID := routing.NormalizeAgentID(r.PathValue("id"))
	response, found := getAgentConfigResponse(cfg, agentID)
	if !found {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) handleCreateAgentConfig(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeAgentConfigRequest(w, r)
	if !ok {
		return
	}
	if strings.TrimSpace(req.ID) == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}

	agentID := routing.NormalizeAgentID(req.ID)
	if _, exists := findAgentConfig(cfg, agentID); exists {
		http.Error(w, fmt.Sprintf("agent %q already exists", agentID), http.StatusConflict)
		return
	}

	agentCfg, err := buildManagedAgentConfig(cfg, req, agentID, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cfg.Agents.List = append(cfg.Agents.List, *agentCfg)
	applyDefaultAgentSelection(cfg)
	if err := validateAndSaveAgentConfig(h.configPath, cfg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response, _ := getAgentConfigResponse(cfg, agentID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "agent": response})
}

func (h *Handler) handleUpdateAgentConfig(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeAgentConfigRequest(w, r)
	if !ok {
		return
	}

	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}

	agentID := routing.NormalizeAgentID(r.PathValue("id"))
	index, existing := findAgentConfig(cfg, agentID)
	if !existing && agentID != routing.DefaultAgentID {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}

	var previous *config.AgentConfig
	if existing {
		copy := cfg.Agents.List[index]
		previous = &copy
	}

	agentCfg, err := buildManagedAgentConfig(cfg, req, agentID, previous)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if existing {
		cfg.Agents.List[index] = *agentCfg
	} else {
		cfg.Agents.List = append([]config.AgentConfig{*agentCfg}, cfg.Agents.List...)
	}
	applyDefaultAgentSelection(cfg)
	if err := validateAndSaveAgentConfig(h.configPath, cfg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response, _ := getAgentConfigResponse(cfg, agentID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "agent": response})
}

func (h *Handler) handleDeleteAgentConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}

	agentID := routing.NormalizeAgentID(r.PathValue("id"))
	index, exists := findAgentConfig(cfg, agentID)
	if !exists {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}
	if reason := deleteBlockReason(cfg, agentID); reason != "" {
		http.Error(w, reason, http.StatusBadRequest)
		return
	}

	cfg.Agents.List = append(cfg.Agents.List[:index], cfg.Agents.List[index+1:]...)
	if err := validateAndSaveAgentConfig(h.configPath, cfg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func decodeAgentConfigRequest(w http.ResponseWriter, r *http.Request) (agentConfigRequest, bool) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return agentConfigRequest{}, false
	}
	defer r.Body.Close()

	var req agentConfigRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return agentConfigRequest{}, false
	}
	return req, true
}

func buildAgentConfigResponses(cfg *config.Config, defaultAgentID string) []agentConfigResponse {
	responses := make([]agentConfigResponse, 0, len(cfg.Agents.List)+1)
	for index := range cfg.Agents.List {
		id := routing.NormalizeAgentID(cfg.Agents.List[index].ID)
		if id == routing.DefaultAgentID {
			continue
		}
		responses = append(responses, buildAgentConfigResponse(cfg, &cfg.Agents.List[index], index, false, defaultAgentID))
	}
	implicitMain := config.AgentConfig{ID: routing.DefaultAgentID, Name: "Main"}
	responses = append([]agentConfigResponse{buildAgentConfigResponse(cfg, &implicitMain, -1, true, defaultAgentID)}, responses...)
	return responses
}

func getAgentConfigResponse(cfg *config.Config, agentID string) (agentConfigResponse, bool) {
	defaultAgentID := configuredDefaultAgentID(cfg)
	norm := routing.NormalizeAgentID(agentID)
	if norm == routing.DefaultAgentID {
		implicitMain := config.AgentConfig{ID: routing.DefaultAgentID, Name: "Main"}
		return buildAgentConfigResponse(cfg, &implicitMain, -1, true, defaultAgentID), true
	}
	if index, found := findAgentConfig(cfg, norm); found {
		return buildAgentConfigResponse(cfg, &cfg.Agents.List[index], index, false, defaultAgentID), true
	}
	return agentConfigResponse{}, false
}

func buildAgentConfigResponse(cfg *config.Config, agentCfg *config.AgentConfig, index int, isImplicit bool, defaultAgentID string) agentConfigResponse {
	name := strings.TrimSpace(agentCfg.Name)
	if name == "" {
		if routing.NormalizeAgentID(agentCfg.ID) == routing.DefaultAgentID {
			name = "Main"
		} else {
			name = routing.NormalizeAgentID(agentCfg.ID)
		}
	}
	modelName := ""
	modelFallbacks := []string(nil)
	if agentCfg.Model != nil {
		modelName = strings.TrimSpace(agentCfg.Model.Primary)
		modelFallbacks = append([]string(nil), agentCfg.Model.Fallbacks...)
	}
	id := routing.NormalizeAgentID(agentCfg.ID)
	return agentConfigResponse{
		ID:                 id,
		Name:               name,
		IsMain:             id == routing.DefaultAgentID,
		IsImplicit:         isImplicit,
		IsDefault:          id == routing.DefaultAgentID,
		ModelName:          modelName,
		ModelFallbacks:     modelFallbacks,
		EffectiveModel:     effectiveAgentModelName(*agentCfg, cfg.Agents.Defaults),
		Workspace:          strings.TrimSpace(agentCfg.Workspace),
		EffectiveWorkspace: resolveAgentWorkspace(agentCfg, &cfg.Agents.Defaults),
		Skills:             append([]string(nil), agentCfg.Skills...),
		SkillsCount:        len(agentCfg.Skills),
		CanDelete:          deleteBlockReason(cfg, id) == "",
		DeleteBlockReason:  deleteBlockReason(cfg, id),
		Index:              index,
	}
}

func findAgentConfig(cfg *config.Config, agentID string) (int, bool) {
	norm := routing.NormalizeAgentID(agentID)
	for index := range cfg.Agents.List {
		if routing.NormalizeAgentID(cfg.Agents.List[index].ID) == norm {
			return index, true
		}
	}
	return -1, false
}

func configuredDefaultAgentID(cfg *config.Config) string {
	return routing.DefaultAgentID
}

func buildManagedAgentConfig(cfg *config.Config, req agentConfigRequest, agentID string, previous *config.AgentConfig) (*config.AgentConfig, error) {
	if agentID == routing.DefaultAgentID {
		return nil, fmt.Errorf("main agent is derived from agents.defaults and cannot be managed in agents.list")
	}
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	if len(req.ModelFallbacks) > 0 && strings.TrimSpace(req.ModelName) == "" {
		return nil, fmt.Errorf("model_name is required when model_fallbacks are provided")
	}
	if modelName := strings.TrimSpace(req.ModelName); modelName != "" && !modelExists(cfg, modelName) {
		return nil, fmt.Errorf("model %q not found in model_list", modelName)
	}

	agentCfg := config.AgentConfig{ID: agentID}
	if previous != nil {
		agentCfg = *previous
		agentCfg.ID = agentID
	}

	agentCfg.Name = strings.TrimSpace(req.Name)
	agentCfg.Workspace = strings.TrimSpace(req.Workspace)
	agentCfg.Skills = normalizeStringList(req.Skills)
	agentCfg.Default = false
	if strings.TrimSpace(req.ModelName) == "" {
		agentCfg.Model = nil
	} else {
		agentCfg.Model = &config.AgentModelConfig{
			Primary:   strings.TrimSpace(req.ModelName),
			Fallbacks: normalizeStringList(req.ModelFallbacks),
		}
	}
	return &agentCfg, nil
}

func applyDefaultAgentSelection(cfg *config.Config) {
	for index := range cfg.Agents.List {
		cfg.Agents.List[index].Default = false
	}
}

func validateAndSaveAgentConfig(configPath string, cfg *config.Config) error {
	if errs := validateManagedAgentList(cfg); len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	if errs := validateConfig(cfg); len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	if err := config.SaveConfig(configPath, cfg); err != nil {
		return fmt.Errorf("Failed to save config: %v", err)
	}
	return nil
}

func validateManagedAgentList(cfg *config.Config) []string {
	var errs []string
	seen := make(map[string]struct{})
	for _, agent := range cfg.Agents.List {
		id := routing.NormalizeAgentID(agent.ID)
		if id == routing.DefaultAgentID {
			continue
		}
		if _, exists := seen[id]; exists {
			errs = append(errs, fmt.Sprintf("duplicate agent id %q", id))
			continue
		}
		seen[id] = struct{}{}
		if strings.TrimSpace(agent.Name) == "" {
			errs = append(errs, fmt.Sprintf("agent %q must have a name", id))
		}
		if agent.Model != nil {
			if strings.TrimSpace(agent.Model.Primary) == "" && len(agent.Model.Fallbacks) > 0 {
				errs = append(errs, fmt.Sprintf("agent %q has model_fallbacks but no model_name", id))
			}
			if primary := strings.TrimSpace(agent.Model.Primary); primary != "" && !modelExists(cfg, primary) {
				errs = append(errs, fmt.Sprintf("agent %q references unknown model %q", id, primary))
			}
			for _, fallback := range agent.Model.Fallbacks {
				if fallback = strings.TrimSpace(fallback); fallback != "" && !modelExists(cfg, fallback) {
					errs = append(errs, fmt.Sprintf("agent %q references unknown fallback model %q", id, fallback))
				}
			}
		}
	}
	return errs
}

func modelExists(cfg *config.Config, modelName string) bool {
	for _, model := range cfg.ModelList {
		if model != nil && model.ModelName == modelName {
			return true
		}
	}
	return false
}

func deleteBlockReason(cfg *config.Config, agentID string) string {
	if agentID == routing.DefaultAgentID {
		return "The main agent cannot be deleted."
	}
	if cfg.Agents.Dispatch != nil {
		for _, rule := range cfg.Agents.Dispatch.Rules {
			if routing.NormalizeAgentID(rule.Agent) == agentID {
				return "This agent is referenced by channel bindings."
			}
		}
	}
	for _, agent := range cfg.Agents.List {
		if agent.Subagents == nil {
			continue
		}
		for _, allowedAgentID := range agent.Subagents.AllowAgents {
			if routing.NormalizeAgentID(allowedAgentID) == agentID {
				return "This agent is referenced by another agent's subagent allowlist."
			}
		}
	}
	return ""
}

func normalizeStringList(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}
