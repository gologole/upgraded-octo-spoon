package loadbalancer

import (
	"cloud.ru_test/config"
	"cloud.ru_test/internal/loadbalancer/algorithms/leastconn"
	roundrobin "cloud.ru_test/internal/loadbalancer/algorithms/round_robin"
	"cloud.ru_test/internal/loadbalancer/algorithms/weighted"
	"cloud.ru_test/internal/loadbalancer/base"
	"cloud.ru_test/pkg/backend"
	"cloud.ru_test/pkg/logger"
	"cloud.ru_test/pkg/request"
	"fmt"
)

// LoadBalancer определяет интерфейс балансировщика нагрузки
type LoadBalancer interface {
	// Start запускает балансировщик
	Start() error
	// AddBackend добавляет новый бэкенд
	AddBackend(backend backend.Backend)
	// RemoveBackend удаляет бэкенд
	RemoveBackend(backend backend.Backend)
	// Invoke выбирает следующий бэкенд для запроса
	Invoke(request request.Request) backend.Backend
	// GetBackend возвращает состояние бэкенда по ID
	GetBackend(id string) *base.BackendState
	// GetBackends возвращает список всех бэкендов
	GetBackends() []*base.BackendState
	// IncActiveConnections увеличивает счетчик активных соединений
	IncActiveConnections(id string)
	// DecActiveConnections уменьшает счетчик активных соединений
	DecActiveConnections(id string)
	// UpdateResponseTime обновляет время ответа бэкенда
	UpdateResponseTime(id string, responseTime int64)
}

// New создает новый балансировщик на основе конфигурации
func New(cfg config.LoadBalancerConfig, appLogger *logger.CustomZapLogger) (LoadBalancer, error) {
	switch cfg.Method {
	case "RoundRobin":
		return roundrobin.New(appLogger), nil
	case "WeightedRoundRobin":
		return weighted.New(appLogger), nil
	case "LeastConnections":
		return leastconn.NewLeastConn(appLogger), nil
	default:
		err := fmt.Errorf("неподдерживаемый метод балансировки: %s", cfg.Method)
		appLogger.Error(err.Error())
		return nil, err
	}
}
