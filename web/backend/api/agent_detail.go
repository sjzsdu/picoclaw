package api

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	cronpkg "github.com/sipeed/picoclaw/pkg/cron"
	"github.com/sipeed/picoclaw/pkg/routing"
)

const (
	agentDetailMaxFileBytes      = 256 * 1024
	agentDetailMaxRecentSessions = 8
	agentDetailMaxRecentMemory   = 8
	agentDetailMaxStateFiles     = 20
)

var agentDetailPromptFiles = []string{
	"AGENT.md",
	"AGENTS.md",
	"SOUL.md",
	"USER.md",
	"IDENTITY.md",
	"HEARTBEAT.md",
	"README.md",
}

type agentDetailResponse struct {
	Agent       agentConfigResponse       `json:"agent"`
	Workspace   string                    `json:"workspace"`
	PromptFiles []agentDetailFile         `json:"prompt_files"`
	Memory      agentDetailMemory         `json:"memory"`
	Skills      []agentDetailSkill        `json:"skills"`
	Sessions    []sessionListItem         `json:"sessions"`
	Cron        agentDetailCron           `json:"cron"`
	StateFiles  []agentDetailFile         `json:"state_files"`
	Directories agentDetailDirectoryState `json:"directories"`
}

type agentDetailFile struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Exists     bool   `json:"exists"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modified_at,omitempty"`
	Content    string `json:"content,omitempty"`
	Truncated  bool   `json:"truncated,omitempty"`
	CanRead    bool   `json:"can_read"`
}

type agentDetailMemory struct {
	LongTerm    agentDetailFile   `json:"long_term"`
	RecentDaily []agentDetailFile `json:"recent_daily"`
}

type agentDetailSkill struct {
	Name        string `json:"name"`
	Configured  bool   `json:"configured"`
	Resolved    bool   `json:"resolved"`
	Source      string `json:"source,omitempty"`
	Path        string `json:"path,omitempty"`
	Description string `json:"description,omitempty"`
}

type agentDetailCron struct {
	File  agentDetailFile   `json:"file"`
	Jobs  []cronpkg.CronJob `json:"jobs,omitempty"`
	Count int               `json:"count"`
}

type agentDetailDirectoryState struct {
	MemoryExists   bool `json:"memory_exists"`
	SkillsExists   bool `json:"skills_exists"`
	SessionsExists bool `json:"sessions_exists"`
	CronExists     bool `json:"cron_exists"`
	StateExists    bool `json:"state_exists"`
}

func (h *Handler) handleGetAgentDetail(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}

	agentID := routing.NormalizeAgentID(r.PathValue("id"))
	agent, found := getAgentConfigResponse(cfg, agentID)
	if !found {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}

	workspace := resolveAgentWorkspaceForID(cfg, agentID)
	detail := agentDetailResponse{
		Agent:       agent,
		Workspace:   workspace,
		PromptFiles: buildAgentPromptFiles(workspace),
		Memory:      buildAgentMemoryDetail(workspace),
		Skills:      buildAgentSkillDetails(cfg, agent),
		Sessions:    h.buildAgentSessionSummaries(workspace),
		Cron:        buildAgentCronDetail(workspace),
		StateFiles:  buildAgentStateFiles(workspace),
		Directories: buildAgentDirectoryState(workspace),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(detail); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func resolveAgentWorkspaceForID(cfg *config.Config, agentID string) string {
	if agentID != routing.DefaultAgentID {
		if index, found := findAgentConfig(cfg, agentID); found {
			return resolveAgentWorkspace(&cfg.Agents.List[index], &cfg.Agents.Defaults)
		}
	}
	return resolveAgentWorkspace(nil, &cfg.Agents.Defaults)
}

func buildAgentPromptFiles(workspace string) []agentDetailFile {
	files := make([]agentDetailFile, 0, len(agentDetailPromptFiles))
	for _, name := range agentDetailPromptFiles {
		files = append(files, readAgentDetailFile(workspace, name, true))
	}
	return files
}

func buildAgentMemoryDetail(workspace string) agentDetailMemory {
	return agentDetailMemory{
		LongTerm:    readAgentDetailFile(workspace, filepath.Join("memory", "MEMORY.md"), true),
		RecentDaily: listRecentMemoryFiles(workspace),
	}
}

func listRecentMemoryFiles(workspace string) []agentDetailFile {
	root := filepath.Join(workspace, "memory")
	entries := make([]agentDetailFile, 0)
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if filepath.Base(path) == "MEMORY.md" || strings.ToLower(filepath.Ext(path)) != ".md" {
			return nil
		}
		rel, relErr := filepath.Rel(workspace, path)
		if relErr != nil {
			return nil
		}
		entries = append(entries, readAgentDetailFile(workspace, filepath.ToSlash(rel), false))
		return nil
	})
	sort.Slice(entries, func(i, j int) bool { return entries[i].ModifiedAt > entries[j].ModifiedAt })
	if len(entries) > agentDetailMaxRecentMemory {
		entries = entries[:agentDetailMaxRecentMemory]
	}
	return entries
}

func buildAgentSkillDetails(cfg *config.Config, agent agentConfigResponse) []agentDetailSkill {
	items, err := buildSkillSupportItems(cfg)
	byName := map[string]skillSupportItem{}
	if err == nil {
		for _, item := range items {
			byName[item.Name] = item
		}
	}
	seen := map[string]struct{}{}
	result := make([]agentDetailSkill, 0, len(agent.Skills))
	for _, name := range agent.Skills {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		seen[name] = struct{}{}
		detail := agentDetailSkill{Name: name, Configured: true}
		if item, ok := byName[name]; ok {
			detail.Resolved = true
			detail.Source = item.Source
			detail.Path = item.Path
			detail.Description = item.Description
		}
		result = append(result, detail)
	}
	for _, item := range byName {
		if _, ok := seen[item.Name]; ok {
			continue
		}
		if item.Source != "workspace" {
			continue
		}
		result = append(result, agentDetailSkill{
			Name:        item.Name,
			Resolved:    true,
			Source:      item.Source,
			Path:        item.Path,
			Description: item.Description,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Configured != result[j].Configured {
			return result[i].Configured
		}
		return result[i].Name < result[j].Name
	})
	return result
}

func (h *Handler) buildAgentSessionSummaries(workspace string) []sessionListItem {
	_, toolFeedbackMaxArgsLength, err := h.sessionRuntimeSettings()
	if err != nil {
		return nil
	}
	dir := resolveSessionsDir(workspace)
	items := make([]sessionListItem, 0)
	seen := make(map[string]struct{})
	if refs, err := h.findPicoJSONLSessions(dir); err == nil {
		for _, ref := range refs {
			if _, exists := seen[ref.ID]; exists {
				continue
			}
			sess, loadErr := h.readJSONLSession(dir, ref.Key)
			if loadErr != nil || isEmptySession(sess) {
				continue
			}
			seen[ref.ID] = struct{}{}
			items = append(items, buildSessionListItem(ref.ID, sess, toolFeedbackMaxArgsLength))
		}
	}
	if refs, err := h.findLegacyPicoSessions(dir); err == nil {
		for _, ref := range refs {
			if _, exists := seen[ref.ID]; exists {
				continue
			}
			sess, loadErr := h.readLegacySession(ref.Path)
			if loadErr != nil || isEmptySession(sess) {
				continue
			}
			seen[ref.ID] = struct{}{}
			items = append(items, buildSessionListItem(ref.ID, sess, toolFeedbackMaxArgsLength))
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Updated > items[j].Updated })
	if len(items) > agentDetailMaxRecentSessions {
		items = items[:agentDetailMaxRecentSessions]
	}
	return items
}

func buildAgentCronDetail(workspace string) agentDetailCron {
	file := readAgentDetailFile(workspace, filepath.Join("cron", "jobs.json"), true)
	cron := agentDetailCron{File: file}
	if !file.Exists || file.Content == "" {
		return cron
	}
	var store cronpkg.CronStore
	if err := json.Unmarshal([]byte(file.Content), &store); err != nil {
		return cron
	}
	cron.Jobs = store.Jobs
	cron.Count = len(store.Jobs)
	return cron
}

func buildAgentStateFiles(workspace string) []agentDetailFile {
	root := filepath.Join(workspace, "state")
	files := make([]agentDetailFile, 0)
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if len(files) >= agentDetailMaxStateFiles {
			return filepath.SkipAll
		}
		rel, relErr := filepath.Rel(workspace, path)
		if relErr != nil {
			return nil
		}
		files = append(files, readAgentDetailFile(workspace, filepath.ToSlash(rel), false))
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files
}

func buildAgentDirectoryState(workspace string) agentDetailDirectoryState {
	exists := func(name string) bool {
		info, err := os.Stat(filepath.Join(workspace, name))
		return err == nil && info.IsDir()
	}
	return agentDetailDirectoryState{
		MemoryExists:   exists("memory"),
		SkillsExists:   exists("skills"),
		SessionsExists: exists("sessions"),
		CronExists:     exists("cron"),
		StateExists:    exists("state"),
	}
}

func readAgentDetailFile(workspace, rel string, includeContent bool) agentDetailFile {
	rel = filepath.ToSlash(filepath.Clean(rel))
	name := filepath.Base(rel)
	file := agentDetailFile{Name: name, Path: rel, CanRead: includeContent}
	if strings.HasPrefix(rel, "../") || filepath.IsAbs(rel) || isSensitiveAgentDetailPath(rel) {
		file.CanRead = false
		return file
	}
	target := filepath.Join(workspace, filepath.FromSlash(rel))
	info, err := os.Stat(target)
	if err != nil || info.IsDir() {
		return file
	}
	file.Exists = true
	file.Size = info.Size()
	file.ModifiedAt = info.ModTime().UTC().Format(time.RFC3339)
	if !includeContent || info.Size() > agentDetailMaxFileBytes {
		file.CanRead = includeContent && info.Size() <= agentDetailMaxFileBytes
		file.Truncated = includeContent && info.Size() > agentDetailMaxFileBytes
		return file
	}
	data, err := os.ReadFile(target)
	if err != nil {
		file.CanRead = false
		return file
	}
	file.Content = string(data)
	return file
}

func isSensitiveAgentDetailPath(rel string) bool {
	lower := strings.ToLower(filepath.ToSlash(rel))
	base := strings.ToLower(filepath.Base(lower))
	if strings.HasPrefix(base, ".") {
		return true
	}
	if base == ".env" || strings.Contains(base, "secret") || strings.Contains(base, "token") || strings.Contains(base, "credential") {
		return true
	}
	return strings.Contains(lower, "/.git/") || strings.Contains(lower, "/.ssh/")
}
