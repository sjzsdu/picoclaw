package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/routing"
)

func TestHandleListAgentConfigs_IncludesImplicitMain(t *testing.T) {
	configPath, cleanup := setupOAuthTestEnv(t)
	defer cleanup()

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	cfg.Agents.List = []config.AgentConfig{{ID: "support", Name: "Support", Default: true}}
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	h := NewHandler(configPath)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent-configs", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Agents       []agentConfigResponse `json:"agents"`
		DefaultAgent string                `json:"default_agent"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if resp.DefaultAgent != "support" {
		t.Fatalf("default_agent = %q, want %q", resp.DefaultAgent, "support")
	}
	if len(resp.Agents) != 2 {
		t.Fatalf("len(agents) = %d, want 2", len(resp.Agents))
	}
	if resp.Agents[0].ID != routing.DefaultAgentID || !resp.Agents[0].IsImplicit {
		t.Fatalf("main agent = %#v, want implicit main", resp.Agents[0])
	}
	if !resp.Agents[1].IsDefault || resp.Agents[1].ID != "support" {
		t.Fatalf("support agent = %#v, want default support", resp.Agents[1])
	}
}

func TestHandleUpdateAgentConfig_ImplicitMainCreatesExplicitConfig(t *testing.T) {
	configPath, cleanup := setupOAuthTestEnv(t)
	defer cleanup()

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	cfg.Agents.List = []config.AgentConfig{{ID: "support", Name: "Support", Default: true}}
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	h := NewHandler(configPath)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := []byte(`{"name":"Main Agent","workspace":"","model_name":"","skills":["gh-cli"],"is_default":true}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/agent-configs/main", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	cfg, err = config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	index, found := findAgentConfig(cfg, routing.DefaultAgentID)
	if !found {
		t.Fatalf("expected explicit main config to be created")
	}
	if !cfg.Agents.List[index].Default {
		t.Fatalf("main agent should be default")
	}
	if cfg.Agents.List[index].Name != "Main Agent" {
		t.Fatalf("main name = %q", cfg.Agents.List[index].Name)
	}
}

func TestHandleCreateAndDeleteAgentConfig(t *testing.T) {
	configPath, cleanup := setupOAuthTestEnv(t)
	defer cleanup()

	h := NewHandler(configPath)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	createBody := []byte(`{"id":"writer","name":"Writer","workspace":"","model_name":"","skills":["gh-cli"],"is_default":false}`)
	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/api/agent-configs", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("create status = %d, want 200, body=%s", createRec.Code, createRec.Body.String())
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if _, found := findAgentConfig(cfg, "writer"); !found {
		t.Fatalf("expected created agent to exist")
	}

	deleteRec := httptest.NewRecorder()
	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/agent-configs/writer", nil)
	mux.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204, body=%s", deleteRec.Code, deleteRec.Body.String())
	}

	cfg, err = config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if _, found := findAgentConfig(cfg, "writer"); found {
		t.Fatalf("agent should be deleted")
	}
}

func TestHandleWorkspaceFilesReadAndWrite(t *testing.T) {
	configPath, cleanup := setupOAuthTestEnv(t)
	defer cleanup()

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	workspace := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "notes.md"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cfg.Agents.Defaults.Workspace = workspace
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	h := NewHandler(configPath)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/api/agent-workspace-files?agent_id=main", nil)
	mux.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200, body=%s", listRec.Code, listRec.Body.String())
	}

	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/api/agent-workspace-files/notes.md?agent_id=main", nil)
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200, body=%s", getRec.Code, getRec.Body.String())
	}

	putBody := []byte(`{"content":"updated"}`)
	putRec := httptest.NewRecorder()
	putReq := httptest.NewRequest(http.MethodPut, "/api/agent-workspace-files/notes.md?agent_id=main", bytes.NewReader(putBody))
	putReq.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("put status = %d, want 200, body=%s", putRec.Code, putRec.Body.String())
	}

	data, err := os.ReadFile(filepath.Join(workspace, "notes.md"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "updated" {
		t.Fatalf("workspace file content = %q, want %q", string(data), "updated")
	}
}
