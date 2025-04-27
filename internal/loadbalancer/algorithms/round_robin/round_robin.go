package roundrobin

import (
	"sync/atomic"

	"cloud.ru_test/internal/loadbalancer/base"
	"cloud.ru_test/pkg/backend"
	"cloud.ru_test/pkg/logger"
	"cloud.ru_test/pkg/request"
)

// RoundRobin реализует алгоритм балансировки Round Robin
type RoundRobin struct {
	*base.BaseLoadBalancer
	current uint64
}

// New создает новый балансировщик Round Robin
func New(logger *logger.CustomZapLogger) *RoundRobin {
	return &RoundRobin{
		BaseLoadBalancer: base.NewBaseLoadBalancer(logger),
		current:          0,
	}
}

// Invoke выбирает следующий бэкенд для запроса
func (r *RoundRobin) Invoke(request request.Request) backend.Backend {
	backends := r.GetBackends()
	if len(backends) == 0 {
		r.Logger().Error("нет доступных бэкендов")
		return nil
	}

	// Атомарно увеличиваем счетчик и берем остаток от деления
	next := atomic.AddUint64(&r.current, 1) % uint64(len(backends))
	return backends[next].Backend
}
