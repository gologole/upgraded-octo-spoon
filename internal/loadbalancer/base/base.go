package base

import (
	"fmt"
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
	b.logger.Debug("Запуск базового балансировщика нагрузки")
	return nil
}

func (b *BaseLoadBalancer) AddBackend(backend backend.Backend) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.logger.Debug(fmt.Sprintf("Добавление нового бэкенда: id=%s, weight=%.2f",
		backend.ID(),
		backend.Weight()))

	b.backends[backend.ID()] = &BackendState{
		Backend: backend,
	}
	b.logger.Debug(fmt.Sprintf("Бэкенд %s успешно добавлен. Всего бэкендов: %d",
		backend.ID(),
		len(b.backends)))
}

func (b *BaseLoadBalancer) RemoveBackend(backend backend.Backend) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.logger.Debug(fmt.Sprintf("Удаление бэкенда: id=%s", backend.ID()))

	if _, exists := b.backends[backend.ID()]; exists {
		delete(b.backends, backend.ID())
		b.logger.Debug(fmt.Sprintf("Бэкенд %s успешно удален. Осталось бэкендов: %d",
			backend.ID(),
			len(b.backends)))
	} else {
		b.logger.Debug(fmt.Sprintf("Попытка удаления несуществующего бэкенда: %s", backend.ID()))
	}
}

func (b *BaseLoadBalancer) GetBackend(id string) *BackendState {
	b.mu.RLock()
	defer b.mu.RUnlock()

	state := b.backends[id]
	if state == nil {
		b.logger.Debug(fmt.Sprintf("Запрошен несуществующий бэкенд: %s", id))
		return nil
	}

	b.logger.Debug(fmt.Sprintf("Получен бэкенд %s: активных соединений=%d, всего запросов=%d, ошибок=%d, время ответа=%dms",
		id,
		state.Stats.ActiveConnections,
		state.Stats.TotalRequests,
		state.Stats.FailedRequests,
		state.Stats.ResponseTime))

	return state
}

func (b *BaseLoadBalancer) IncActiveConnections(id string) {
	if state := b.GetBackend(id); state != nil {
		newCount := atomic.AddInt64(&state.Stats.ActiveConnections, 1)
		b.logger.Debug(fmt.Sprintf("Увеличено количество активных соединений для бэкенда %s: %d", id, newCount))
	} else {
		b.logger.Debug(fmt.Sprintf("Попытка увеличить количество соединений для несуществующего бэкенда: %s", id))
	}
}

func (b *BaseLoadBalancer) DecActiveConnections(id string) {
	if state := b.GetBackend(id); state != nil {
		newCount := atomic.AddInt64(&state.Stats.ActiveConnections, -1)
		b.logger.Debug(fmt.Sprintf("Уменьшено количество активных соединений для бэкенда %s: %d", id, newCount))
	} else {
		b.logger.Debug(fmt.Sprintf("Попытка уменьшить количество соединений для несуществующего бэкенда: %s", id))
	}
}

func (b *BaseLoadBalancer) UpdateResponseTime(id string, responseTime int64) {
	if state := b.GetBackend(id); state != nil {
		oldTime := atomic.LoadInt64(&state.Stats.ResponseTime)
		atomic.StoreInt64(&state.Stats.ResponseTime, responseTime)
		b.logger.Debug(fmt.Sprintf("Обновлено время ответа для бэкенда %s: %dms -> %dms", id, oldTime, responseTime))
	} else {
		b.logger.Debug(fmt.Sprintf("Попытка обновить время ответа для несуществующего бэкенда: %s", id))
	}
}

func (b *BaseLoadBalancer) GetBackends() []*BackendState {
	b.mu.RLock()
	defer b.mu.RUnlock()

	backends := make([]*BackendState, 0, len(b.backends))
	for _, state := range b.backends {
		backends = append(backends, state)
	}

	b.logger.Debug(fmt.Sprintf("Получен список всех бэкендов (всего: %d)", len(backends)))
	for _, state := range backends {
		b.logger.Debug(fmt.Sprintf("Бэкенд %s: активных соединений=%d, всего запросов=%d, ошибок=%d, время ответа=%dms",
			state.Backend.ID(),
			state.Stats.ActiveConnections,
			state.Stats.TotalRequests,
			state.Stats.FailedRequests,
			state.Stats.ResponseTime))
	}

	return backends
}

// Logger возвращает логгер
func (b *BaseLoadBalancer) Logger() *logger.CustomZapLogger {
	return b.logger
}
