package api

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/routing"
)

const maxWorkspaceFileSize = 1 << 20
const maxWorkspaceFiles = 200

type workspaceFileInfo struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Exists     bool   `json:"exists"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modified_at"`
	CanEdit    bool   `json:"can_edit"`
}

type workspaceFileContent struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Content string `json:"content"`
	Exists  bool   `json:"exists"`
}

func (h *Handler) handleListAgentWorkspaceFiles(w http.ResponseWriter, r *http.Request) {
	agentID, workspace, err := agentWorkspaceFromRequest(h.configPath, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	files, err := listWorkspaceFiles(workspace)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list workspace files: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"agent_id":  agentID,
		"workspace": workspace,
		"files":     files,
	})
}

func (h *Handler) handleGetAgentWorkspaceFile(w http.ResponseWriter, r *http.Request) {
	_, workspace, err := agentWorkspaceFromRequest(h.configPath, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	name := r.PathValue("filename")
	content, err := readWorkspaceFile(workspace, name)
	if err != nil {
		status := http.StatusInternalServerError
		if errorsIs(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(content)
}

func (h *Handler) handleUpdateAgentWorkspaceFile(w http.ResponseWriter, r *http.Request) {
	_, workspace, err := agentWorkspaceFromRequest(h.configPath, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxWorkspaceFileSize+1))
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	if len(body) > maxWorkspaceFileSize {
		http.Error(w, "content exceeds 1MB limit", http.StatusBadRequest)
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	name := r.PathValue("filename")
	content, err := writeWorkspaceFile(workspace, name, req.Content)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "file": content})
}

func agentWorkspaceFromRequest(configPath string, r *http.Request) (string, string, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return "", "", fmt.Errorf("Failed to load config: %v", err)
	}
	agentID := routing.NormalizeAgentID(strings.TrimSpace(r.URL.Query().Get("agent_id")))
	if agentID == "" {
		agentID = routing.DefaultAgentID
	}
	if agentID != routing.DefaultAgentID {
		if _, found := findAgentConfig(cfg, agentID); !found {
			return "", "", fmt.Errorf("agent not found")
		}
	}
	var agentCfg *config.AgentConfig
	if index, found := findAgentConfig(cfg, agentID); found {
		agentCfg = &cfg.Agents.List[index]
	}
	return agentID, resolveAgentWorkspace(agentCfg, &cfg.Agents.Defaults), nil
}

func listWorkspaceFiles(workspace string) ([]workspaceFileInfo, error) {
	root, err := filepath.Abs(workspace)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return []workspaceFileInfo{}, nil
		}
		return nil, err
	}
	files := make([]workspaceFileInfo, 0)
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if len(files) >= maxWorkspaceFiles {
			return filepath.SkipAll
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		files = append(files, workspaceFileInfo{
			Name:       filepath.Base(rel),
			Path:       rel,
			Exists:     true,
			Size:       info.Size(),
			ModifiedAt: info.ModTime().UTC().Format(time.RFC3339),
			CanEdit:    info.Size() <= maxWorkspaceFileSize,
		})
		return nil
	})
	if err != nil && err != filepath.SkipAll {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files, nil
}

func readWorkspaceFile(workspace, name string) (*workspaceFileContent, error) {
	target, rel, err := sanitizeWorkspaceFilename(workspace, name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("Failed to read workspace file: %v", err)
	}
	if len(data) > maxWorkspaceFileSize {
		return nil, fmt.Errorf("workspace file exceeds 1MB limit")
	}
	return &workspaceFileContent{
		Name:    filepath.Base(rel),
		Path:    rel,
		Content: string(data),
		Exists:  true,
	}, nil
}

func writeWorkspaceFile(workspace, name, content string) (*workspaceFileContent, error) {
	target, rel, err := sanitizeWorkspaceFilename(workspace, name)
	if err != nil {
		return nil, err
	}
	if len(content) > maxWorkspaceFileSize {
		return nil, fmt.Errorf("content exceeds 1MB limit")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return nil, fmt.Errorf("Failed to create workspace directory: %v", err)
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("Failed to write workspace file: %v", err)
	}
	return &workspaceFileContent{
		Name:    filepath.Base(rel),
		Path:    rel,
		Content: content,
		Exists:  true,
	}, nil
}

func sanitizeWorkspaceFilename(workspace, name string) (string, string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", "", fmt.Errorf("filename is required")
	}
	root, err := filepath.Abs(workspace)
	if err != nil {
		return "", "", fmt.Errorf("invalid workspace path: %v", err)
	}
	cleanRel := filepath.Clean(filepath.FromSlash(trimmed))
	if cleanRel == "." || cleanRel == "" {
		return "", "", fmt.Errorf("filename is required")
	}
	if filepath.IsAbs(cleanRel) || strings.HasPrefix(cleanRel, "..") {
		return "", "", fmt.Errorf("invalid workspace file path")
	}
	target := filepath.Join(root, cleanRel)
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", "", fmt.Errorf("invalid workspace file path: %v", err)
	}
	if targetAbs != root && !strings.HasPrefix(targetAbs, root+string(os.PathSeparator)) {
		return "", "", fmt.Errorf("invalid workspace file path")
	}
	return targetAbs, filepath.ToSlash(cleanRel), nil
}

func errorsIs(err, target error) bool {
	return err != nil && target != nil && os.IsNotExist(err) && target == os.ErrNotExist
}
