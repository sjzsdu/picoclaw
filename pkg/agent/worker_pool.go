package agent

import (
	"context"
	"hash/fnv"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/sipeed/picoclaw/pkg/audio/asr"
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/media"
	"github.com/sipeed/picoclaw/pkg/providers"
)

type WorkerPool struct {
	workers    []*AgentLoop
	mailboxes  []chan bus.InboundMessage
	dispatcher *Dispatcher
	cfg        *config.Config
	msgBus     *bus.MessageBus
	provider   providers.LLMProvider
	ctx        context.Context
	cancel     context.CancelFunc
	running    atomic.Bool
	mu         sync.RWMutex
	dispatchWg sync.WaitGroup
}

type Dispatcher struct {
	workerCount int
}

func NewWorkerPool(
	cfg *config.Config,
	msgBus *bus.MessageBus,
	provider providers.LLMProvider,
	workerCount int,
) *WorkerPool {
	if workerCount <= 0 {
		workerCount = runtime.NumCPU()
	}

	ctx, cancel := context.WithCancel(context.Background())

	pool := &WorkerPool{
		workers:    make([]*AgentLoop, workerCount),
		mailboxes:  make([]chan bus.InboundMessage, workerCount),
		dispatcher: &Dispatcher{workerCount: workerCount},
		cfg:        cfg,
		msgBus:     msgBus,
		provider:   provider,
		ctx:        ctx,
		cancel:     cancel,
	}

	mailboxBuffer := 64
	if cfg != nil {
		mailboxBuffer = cfg.Gateway.GetInboundBuffer()
	}
	if mailboxBuffer <= 0 {
		mailboxBuffer = 64
	}

	for i := 0; i < workerCount; i++ {
		pool.mailboxes[i] = make(chan bus.InboundMessage, mailboxBuffer)
		pool.workers[i] = NewAgentLoop(cfg, msgBus, provider)
		pool.workers[i].SetWorkerID(i)
	}

	return pool
}

// Start starts all workers in the pool.
func (p *WorkerPool) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running.Load() {
		return
	}

	p.running.Store(true)
	p.dispatchWg.Add(1)
	go p.dispatchLoop()

	for i, worker := range p.workers {
		go func(id int, w *AgentLoop) {
			logger.InfoCF("worker_pool", "Starting worker", map[string]any{
				"worker_id": id,
			})
			if err := w.RunInbox(p.ctx, p.mailboxes[id]); err != nil {
				logger.ErrorCF("worker_pool", "Worker exited with error", map[string]any{
					"worker_id": id,
					"error":     err.Error(),
				})
			}
		}(i, worker)
	}

	logger.InfoCF("worker_pool", "Worker pool started", map[string]any{
		"worker_count": len(p.workers),
	})
}

// Stop stops all workers in the pool.
func (p *WorkerPool) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running.Load() {
		return
	}

	p.cancel()
	p.running.Store(false)
	p.dispatchWg.Wait()

	for i, worker := range p.workers {
		worker.Stop()
		logger.InfoCF("worker_pool", "Worker stopped", map[string]any{
			"worker_id": i,
		})
	}

	logger.InfoCF("worker_pool", "Worker pool stopped", nil)
}

func (p *WorkerPool) dispatchLoop() {
	defer p.dispatchWg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		case msg, ok := <-p.msgBus.InboundChan():
			if !ok {
				return
			}

			workerIdx := p.routeMessage(msg)
			select {
			case p.mailboxes[workerIdx] <- msg:
			case <-p.ctx.Done():
				return
			}
		}
	}
}

func (p *WorkerPool) routeMessage(msg bus.InboundMessage) int {
	if len(p.workers) == 0 {
		return 0
	}
	control := p.workers[0]
	if control == nil {
		return 0
	}
	if sessionKey, _, ok := control.resolveSteeringTarget(msg); ok {
		return p.dispatcher.Route(sessionKey)
	}
	return 0
}

// Close releases resources held by all workers.
func (p *WorkerPool) Close() {
	for _, worker := range p.workers {
		worker.Close()
	}
}

func (p *WorkerPool) SetChannelManager(cm *channels.Manager) {
	for _, worker := range p.workers {
		worker.SetChannelManager(cm)
	}
}

func (p *WorkerPool) SetMediaStore(store media.MediaStore) {
	for _, worker := range p.workers {
		worker.SetMediaStore(store)
	}
}

func (p *WorkerPool) SetTranscriber(t asr.Transcriber) {
	for _, worker := range p.workers {
		worker.SetTranscriber(t)
	}
}

func (p *WorkerPool) SetReloadFunc(fn func() error) {
	for _, worker := range p.workers {
		worker.SetReloadFunc(fn)
	}
}

func (p *WorkerPool) ReloadProviderAndConfig(
	ctx context.Context,
	provider providers.LLMProvider,
	cfg *config.Config,
) error {
	var oldProvider providers.LLMProvider
	if len(p.workers) > 0 {
		if existing, ok := extractProvider(p.workers[0].GetRegistry()); ok {
			oldProvider = existing
		}
	}

	p.mu.Lock()
	p.cfg = cfg
	p.provider = provider
	p.mu.Unlock()

	for _, worker := range p.workers {
		if err := worker.reloadProviderAndConfig(ctx, provider, cfg, false); err != nil {
			return err
		}
	}

	if oldProvider != nil && oldProvider != provider {
		if stateful, ok := oldProvider.(providers.StatefulProvider); ok {
			stateful.Close()
		}
	}

	return nil
}

// GetWorker returns the worker for the given session key.
// This is used for operations that need direct worker access (e.g., steering).
func (p *WorkerPool) GetWorker(sessionKey string) *AgentLoop {
	idx := p.dispatcher.Route(sessionKey)
	return p.workers[idx]
}

// GetWorkerByID returns the worker by its index.
func (p *WorkerPool) GetWorkerByID(id int) *AgentLoop {
	if id < 0 || id >= len(p.workers) {
		return nil
	}
	return p.workers[id]
}

// WorkerCount returns the number of workers in the pool.
func (p *WorkerPool) WorkerCount() int {
	return len(p.workers)
}

// Route returns the worker index for a given session key.
func (d *Dispatcher) Route(sessionKey string) int {
	h := fnv.New32a()
	h.Write([]byte(sessionKey))
	return int(h.Sum32()) % d.workerCount
}

// ClearRoute removes a session key from the route table.
func (d *Dispatcher) ClearRoute(sessionKey string) {
	_ = sessionKey
}

// RouteCount returns the number of active routes.
func (d *Dispatcher) RouteCount() int {
	return 0
}
