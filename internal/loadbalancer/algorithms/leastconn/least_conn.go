package leastconn

import (
	"math"

	"cloud.ru_test/internal/loadbalancer/base"
	"cloud.ru_test/pkg/backend"
	"cloud.ru_test/pkg/logger"
	"cloud.ru_test/pkg/request"
)

// LeastConn реализует алгоритм выбора бэкенда с наименьшим количеством соединений
type LeastConn struct {
	*base.BaseLoadBalancer
}

// NewLeastConn создает новый балансировщик по наименьшему количеству соединений
func NewLeastConn(logger *logger.CustomZapLogger) *LeastConn {
	return &LeastConn{
		BaseLoadBalancer: base.NewBaseLoadBalancer(logger),
	}
}

// Invoke выбирает бэкенд с наименьшим количеством активных соединений
func (l *LeastConn) Invoke(request request.Request) backend.Backend {
	backends := l.GetBackends()
	if len(backends) == 0 {
		l.Logger().Error("нет доступных бэкендов")
		return nil
	}

	var selected *backend.Backend
	minConn := int64(math.MaxInt64)

	// Находим бэкенд с минимальным количеством соединений
	for _, b := range backends {
		connections := b.Stats.ActiveConnections
		if connections < minConn {
			minConn = connections
			backend := b.Backend
			selected = &backend
		}
	}

	if selected == nil {
		l.Logger().Error("не удалось выбрать бэкенд")
		return nil
	}

	return *selected
}
