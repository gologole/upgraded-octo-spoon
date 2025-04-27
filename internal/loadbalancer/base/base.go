package base

import (
	"sync"
	"sync/atomic"

	"cloud.ru_test/pkg/backend"
	"cloud.ru_test/pkg/logger"
)

// Stats хранит статистику бэкенда
type Stats struct {
	ActiveConnections int64
	TotalRequests     uint64
	FailedRequests    uint64
	ResponseTime      int64 // в миллисекундах
}

// BackendState хранит состояние бэкенда
type BackendState struct {
	Backend backend.Backend
	Stats   Stats
	Weight  float64
}

// BaseLoadBalancer содержит общую функциональность для всех алгоритмов
type BaseLoadBalancer struct {
	backends map[string]*BackendState
	mu       sync.RWMutex
	logger   *logger.CustomZapLogger
}

// NewBaseLoadBalancer создает новый базовый балансировщик
func NewBaseLoadBalancer(logger *logger.CustomZapLogger) *BaseLoadBalancer {
	return &BaseLoadBalancer{
		backends: make(map[string]*BackendState),
		logger:   logger,
	}
}

func (b *BaseLoadBalancer) Start() error {
	return nil
}

func (b *BaseLoadBalancer) AddBackend(backend backend.Backend) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.backends[backend.ID()] = &BackendState{
		Backend: backend,
	}
}

func (b *BaseLoadBalancer) RemoveBackend(backend backend.Backend) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.backends, backend.ID())
}

func (b *BaseLoadBalancer) GetBackend(id string) *BackendState {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.backends[id]
}

func (b *BaseLoadBalancer) IncActiveConnections(id string) {
	if state := b.GetBackend(id); state != nil {
		atomic.AddInt64(&state.Stats.ActiveConnections, 1)
	}
}

func (b *BaseLoadBalancer) DecActiveConnections(id string) {
	if state := b.GetBackend(id); state != nil {
		atomic.AddInt64(&state.Stats.ActiveConnections, -1)
	}
}

func (b *BaseLoadBalancer) UpdateResponseTime(id string, responseTime int64) {
	if state := b.GetBackend(id); state != nil {
		atomic.StoreInt64(&state.Stats.ResponseTime, responseTime)
	}
}

func (b *BaseLoadBalancer) GetBackends() []*BackendState {
	b.mu.RLock()
	defer b.mu.RUnlock()

	backends := make([]*BackendState, 0, len(b.backends))
	for _, state := range b.backends {
		backends = append(backends, state)
	}
	return backends
}

// Logger возвращает логгер
func (b *BaseLoadBalancer) Logger() *logger.CustomZapLogger {
	return b.logger
}
