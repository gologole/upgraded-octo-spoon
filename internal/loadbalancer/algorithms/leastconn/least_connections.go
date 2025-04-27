package leastconn

import (
	"cloud.ru_test/internal/loadbalancer/base"
	"cloud.ru_test/pkg/backend"
	"cloud.ru_test/pkg/logger"
	"cloud.ru_test/pkg/request"
	"fmt"
	"math"
)

// LeastConnections реализует алгоритм Least Connections
type LeastConnections struct {
	*base.BaseLoadBalancer
}

// New создает новый Least Connections балансировщик
func New(logger *logger.CustomZapLogger) *LeastConnections {
	return &LeastConnections{
		BaseLoadBalancer: base.NewBaseLoadBalancer(logger),
	}
}

func (lc *LeastConnections) Start() error {
	return nil
}

// Invoke выбирает бэкенд с наименьшим количеством активных соединений
func (lc *LeastConnections) Invoke(req request.Request) backend.Backend {
	backends := lc.GetBackends()
	if len(backends) == 0 {
		lc.Logger().Warn("нет доступных бэкендов")
		return nil
	}

	var selected *base.BackendState
	minConn := int64(math.MaxInt64)

	// Находим бэкенд с минимальным количеством соединений
	for _, state := range backends {
		activeConn := state.Stats.ActiveConnections
		if activeConn < minConn {
			minConn = activeConn
			selected = state
		}
	}

	if selected == nil {
		// Если что-то пошло не так, возвращаем первый бэкенд
		lc.Logger().Warn("ошибка выбора бэкенда по количеству соединений, используем первый доступный")
		selected = backends[0]
	}

	lc.IncActiveConnections(selected.Backend.ID())
	lc.Logger().Debug(fmt.Sprintf("выбран бэкенд",
		"id", selected.Backend.ID(),
		"activeConnections", selected.Stats.ActiveConnections))

	return selected.Backend
}
