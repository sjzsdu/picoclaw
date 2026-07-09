package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/routing"
)

func TestHandleGetAgentDetail(t *testing.T) {
	configPath, cleanup := setupOAuthTestEnv(t)
	defer cleanup()

	workspace := filepath.Join(t.TempDir(), "workspace")
	mustWrite := func(rel, content string) {
		t.Helper()
		path := filepath.Join(workspace, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
	}
	mustWrite("AGENT.md", "# Agent\nBe useful.")
	mustWrite("memory/MEMORY.md", "# Memory\nUser likes concise answers.")
	mustWrite("memory/202606/20260624.md", "daily note")
	mustWrite("cron/jobs.json", `{"version":1,"jobs":[{"id":"job-1","name":"Daily","enabled":true,"schedule":{"kind":"cron","expr":"0 9 * * *"},"payload":{"kind":"message","message":"hello"},"state":{},"createdAtMs":1,"updatedAtMs":1}]}`)
	mustWrite("state/runtime.json", `{"ok":true}`)
	mustWrite("skills/workspace-skill/SKILL.md", "---\nname: workspace-skill\ndescription: Workspace skill\n---\n\n# Skill")

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	cfg.Agents.Defaults.Workspace = workspace
	cfg.Agents.List = []config.AgentConfig{{
		ID:        "support",
		Name:      "Support",
		Workspace: workspace,
		Skills:    []string{"workspace-skill", "missing-skill"},
	}}
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	h := NewHandler(configPath)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agents/support/detail", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp agentDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if resp.Agent.ID != "support" || resp.Workspace != workspace {
		t.Fatalf("agent/workspace = %#v %q", resp.Agent, resp.Workspace)
	}
	if len(resp.PromptFiles) == 0 || !resp.PromptFiles[0].Exists || resp.PromptFiles[0].Content == "" {
		t.Fatalf("expected AGENT.md content, got %#v", resp.PromptFiles)
	}
	if !resp.Memory.LongTerm.Exists || resp.Memory.LongTerm.Content == "" || len(resp.Memory.RecentDaily) != 1 {
		t.Fatalf("memory detail = %#v", resp.Memory)
	}
	if resp.Cron.Count != 1 || len(resp.Cron.Jobs) != 1 {
		t.Fatalf("cron detail = %#v", resp.Cron)
	}
	if len(resp.StateFiles) != 1 || !resp.Directories.StateExists {
		t.Fatalf("state detail = %#v dirs=%#v", resp.StateFiles, resp.Directories)
	}

	byName := map[string]agentDetailSkill{}
	for _, skill := range resp.Skills {
		byName[skill.Name] = skill
	}
	if !byName["workspace-skill"].Resolved || !byName["missing-skill"].Configured || byName["missing-skill"].Resolved {
		t.Fatalf("skills = %#v", resp.Skills)
	}
}

func TestHandleGetAgentDetailIncludesImplicitMain(t *testing.T) {
	configPath, cleanup := setupOAuthTestEnv(t)
	defer cleanup()

	h := NewHandler(configPath)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+routing.DefaultAgentID+"/detail", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
}
