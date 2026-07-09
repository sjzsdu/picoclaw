package agent

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/session"
)

type workerPoolStatefulProvider struct {
	closeCount atomic.Int32
}

func (p *workerPoolStatefulProvider) Chat(
	_ context.Context,
	_ []providers.Message,
	_ []providers.ToolDefinition,
	_ string,
	_ map[string]any,
) (*providers.LLMResponse, error) {
	return &providers.LLMResponse{Content: "ok"}, nil
}

func (p *workerPoolStatefulProvider) GetDefaultModel() string {
	return "test"
}

func (p *workerPoolStatefulProvider) Close() {
	p.closeCount.Add(1)
}

func TestWorkerPool_Creation(t *testing.T) {
	cfg := &config.Config{}
	pool := NewWorkerPool(cfg, nil, nil, 4)

	if pool == nil {
		t.Fatal("NewWorkerPool returned nil")
	}

	if pool.WorkerCount() != 4 {
		t.Errorf("Expected 4 workers, got %d", pool.WorkerCount())
	}

	for i := 0; i < 4; i++ {
		worker := pool.GetWorkerByID(i)
		if worker == nil {
			t.Errorf("Worker %d is nil", i)
			continue
		}
		if worker.GetWorkerID() != i {
			t.Errorf("Worker %d has wrong ID: %d", i, worker.GetWorkerID())
		}
	}
}

func TestDispatcher_Route(t *testing.T) {
	d := &Dispatcher{workerCount: 4}

	routes := make(map[string]int)
	sessions := []string{"session-a", "session-b", "session-c", "session-d", "session-e"}

	for _, session := range sessions {
		routes[session] = d.Route(session)
	}

	// Verify consistency - same session should always route to same worker
	for _, session := range sessions {
		if d.Route(session) != routes[session] {
			t.Errorf("Route for %s changed from %d to %d", session, routes[session], d.Route(session))
		}
	}

	// Verify all sessions are routed to valid workers
	for _, worker := range routes {
		if worker < 0 || worker >= 4 {
			t.Errorf("Invalid worker index: %d", worker)
		}
	}
}

func TestDispatcher_RouteDistribution(t *testing.T) {
	d := &Dispatcher{workerCount: 4}

	// Generate many session keys and check distribution
	counts := make(map[int]int)
	for i := 0; i < 100; i++ {
		session := fmt.Sprintf("session-%d", i)
		worker := d.Route(session)
		counts[worker]++
	}

	// Check that distribution is reasonably balanced
	// With 100 sessions and 4 workers, expect ~25 per worker
	// Allow some variance (10-40 per worker)
	for worker, count := range counts {
		if count < 10 || count > 40 {
			t.Errorf("Worker %d has imbalanced load: %d sessions", worker, count)
		}
	}

	t.Logf("Distribution: %v", counts)
}

func TestWorkerPool_RouteMessage_SystemUsesPrimaryWorker(t *testing.T) {
	cfg := &config.Config{}
	pool := NewWorkerPool(cfg, nil, nil, 4)

	idx := pool.routeMessage(bus.InboundMessage{Channel: "system"})
	if idx != 0 {
		t.Fatalf("expected system messages to route to primary worker, got %d", idx)
	}
}

func TestWorkerPool_SetReloadFunc_PropagatesToAllWorkers(t *testing.T) {
	cfg := &config.Config{}
	pool := NewWorkerPool(cfg, nil, nil, 3)
	fn := func() error { return nil }

	pool.SetReloadFunc(fn)

	for i, worker := range pool.workers {
		if worker.reloadFunc == nil {
			t.Fatalf("worker %d missing reload func", i)
		}
	}
}

func TestWorkerPool_RouteMessage_UsesDeterministicSessionAffinity(t *testing.T) {
	cfg := &config.Config{}
	pool := NewWorkerPool(cfg, nil, &workerPoolStatefulProvider{}, 4)
	sessionKey := session.BuildOpaqueSessionKey("agent:default:main")

	msg := bus.InboundMessage{
		Context: bus.InboundContext{
			Channel:  "telegram",
			ChatID:   "chat-1",
			ChatType: "direct",
			SenderID: "user-1",
		},
		Channel:    "telegram",
		ChatID:     "chat-1",
		SenderID:   "user-1",
		SessionKey: sessionKey,
		Content:    "hello",
	}

	first := pool.routeMessage(msg)
	second := pool.routeMessage(msg)
	if first != second {
		t.Fatalf("expected stable route for same session key, got %d then %d", first, second)
	}
	if first < 0 || first >= pool.WorkerCount() {
		t.Fatalf("route out of range: %d", first)
	}
}

func TestWorkerPool_ReloadProviderAndConfig_ClosesSharedOldProviderOnce(t *testing.T) {
	cfg := &config.Config{}
	oldProvider := &workerPoolStatefulProvider{}
	pool := NewWorkerPool(cfg, nil, oldProvider, 3)
	newProvider := &workerPoolStatefulProvider{}

	if err := pool.ReloadProviderAndConfig(context.Background(), newProvider, cfg); err != nil {
		t.Fatalf("ReloadProviderAndConfig() error = %v", err)
	}

	if got := oldProvider.closeCount.Load(); got != 1 {
		t.Fatalf("expected shared old provider to close once, got %d", got)
	}
	if got := newProvider.closeCount.Load(); got != 0 {
		t.Fatalf("expected new provider to remain open, got %d closes", got)
	}
}
