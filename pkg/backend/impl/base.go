package impl

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"cloud.ru_test/pkg/backend"
)

// BaseBackend базовая реализация бэкенда
type BaseBackend struct {
	id       string
	url      string
	weight   float64
	isAlive  bool
	stats    backend.LoadStats
	client   *http.Client
	statsMux sync.RWMutex

	// Окно для подсчета статистики (1 минута)
	requestTimes    []time.Duration // Времена ответов
	requestTimesIdx int             // Индекс для циклического буфера
	timesMux        sync.RWMutex

	// Счетчики для подсчета RPS
	requestCount    atomic.Int64
	lastCountReset  time.Time
	successCount    atomic.Int64
	lastSuccessTime time.Time
}

// NewBackend создает новый бэкенд
func NewBackend(id, url string, weight float64) *BaseBackend {
	b := &BaseBackend{
		id:      id,
		url:     url,
		weight:  weight,
		isAlive: true,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		requestTimes:   make([]time.Duration, 60), // Храним историю за минуту
		lastCountReset: time.Now(),
	}

	// Запускаем обновление статистики
	go b.updateStats()

	return b
}

func (b *BaseBackend) ID() string {
	return b.id
}

func (b *BaseBackend) Weight() float64 {
	return b.weight
}

func (b *BaseBackend) SetWeight(weight float64) {
	b.weight = weight
}

func (b *BaseBackend) IsAlive() bool {
	return b.isAlive
}

func (b *BaseBackend) GetLoadStats() backend.LoadStats {
	b.statsMux.RLock()
	defer b.statsMux.RUnlock()
	return b.stats
}

func (b *BaseBackend) Handle(ctx context.Context, req *http.Request) (*http.Response, error) {
	start := time.Now()

	// Увеличиваем счетчик активных соединений
	atomic.AddInt64(&b.stats.ActiveConnections, 1)
	defer atomic.AddInt64(&b.stats.ActiveConnections, -1)

	// Клонируем запрос и обновляем URL
	outReq := req.Clone(ctx)
	outReq.URL.Host = b.url
	outReq.Host = b.url

	// Отправляем запрос
	resp, err := b.client.Do(outReq)

	// Обновляем статистику
	duration := time.Since(start)
	b.updateRequestStats(duration, err == nil)

	return resp, err
}

func (b *BaseBackend) updateRequestStats(duration time.Duration, success bool) {
	// Обновляем времена ответов
	b.timesMux.Lock()
	b.requestTimes[b.requestTimesIdx] = duration
	b.requestTimesIdx = (b.requestTimesIdx + 1) % len(b.requestTimes)
	b.timesMux.Unlock()

	// Увеличиваем счетчики
	b.requestCount.Add(1)
	if success {
		b.successCount.Add(1)
	}
}

func (b *BaseBackend) updateStats() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		b.statsMux.Lock()

		// Обновляем RPS
		now := time.Now()
		elapsed := now.Sub(b.lastCountReset).Seconds()
		if elapsed > 0 {
			count := b.requestCount.Load()
			b.stats.RequestsPerSecond = float64(count) / elapsed
			b.requestCount.Store(0)
			b.lastCountReset = now
		}

		// Обновляем Success Rate
		totalRequests := b.requestCount.Load()
		if totalRequests > 0 {
			successRequests := b.successCount.Load()
			b.stats.SuccessRate = float64(successRequests) / float64(totalRequests)
		}

		// Обновляем среднее время ответа
		b.timesMux.RLock()
		var total time.Duration
		count := 0
		for _, t := range b.requestTimes {
			if t > 0 {
				total += t
				count++
			}
		}
		b.timesMux.RUnlock()

		if count > 0 {
			b.stats.AvgResponseTime = total / time.Duration(count)
		}

		b.statsMux.Unlock()
	}
}
