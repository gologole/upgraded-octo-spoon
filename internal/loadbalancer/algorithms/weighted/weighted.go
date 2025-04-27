package weighted

import (
	"sync"
	"sync/atomic"

	"cloud.ru_test/internal/loadbalancer/base"
	"cloud.ru_test/pkg/backend"
	"cloud.ru_test/pkg/logger"
	"cloud.ru_test/pkg/request"
)

// WeightedRoundRobin реализует алгоритм взвешенного Round Robin
type WeightedRoundRobin struct {
	*base.BaseLoadBalancer
	current     uint64
	weightMutex sync.RWMutex
}

// New создает новый взвешенный балансировщик
func New(logger *logger.CustomZapLogger) *WeightedRoundRobin {
	return &WeightedRoundRobin{
		BaseLoadBalancer: base.NewBaseLoadBalancer(logger),
		current:          0,
	}
}

// AddBackend переопределяет метод базового балансировщика для установки веса
func (w *WeightedRoundRobin) AddBackend(b backend.Backend) {
	w.weightMutex.Lock()
	defer w.weightMutex.Unlock()

	// Вызываем базовую реализацию
	w.BaseLoadBalancer.AddBackend(b)

	// Устанавливаем вес из конфигурации бэкенда
	if state := w.GetBackend(b.ID()); state != nil {
		weight := b.Weight()
		if weight <= 0 {
			weight = 1.0 // Дефолтный вес
		}
		state.Weight = weight
	}
}

// Invoke выбирает следующий бэкенд для запроса с учетом весов
func (w *WeightedRoundRobin) Invoke(request request.Request) backend.Backend {
	w.weightMutex.RLock()
	defer w.weightMutex.RUnlock()

	backends := w.GetBackends()
	if len(backends) == 0 {
		w.Logger().Error("нет доступных бэкендов")
		return nil
	}

	// Вычисляем общий вес
	var totalWeight float64
	for _, b := range backends {
		totalWeight += b.Weight
	}

	// Атомарно увеличиваем счетчик
	next := atomic.AddUint64(&w.current, 1)

	// Выбираем бэкенд на основе весов
	var accumWeight float64
	target := float64(next%uint64(1000)) / 1000.0 * totalWeight

	for _, b := range backends {
		accumWeight += b.Weight
		if accumWeight >= target {
			return b.Backend
		}
	}

	// На случай ошибок округления возвращаем последний бэкенд
	return backends[len(backends)-1].Backend
}
