package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// UserLimits содержит настройки лимитов для пользователя
type UserLimits struct {
	Rate  float64 // Количество запросов в секунду
	Burst int     // Максимальный размер корзины
}

// TokenBucket реализует алгоритм маркерного ведра с поддержкой пользовательских лимитов
type TokenBucket struct {
	// Дефолтные настройки
	defaultRate  float64
	defaultBurst int

	// Хранилище лимитеров для каждого пользователя
	limiters sync.Map // map[string]*rate.Limiter

	// Хранилище пользовательских настроек
	userLimits sync.Map // map[string]*UserLimits

	// Мьютекс для синхронизации операций с настройками
	mu sync.RWMutex
}

// NewTokenBucket создает новый TokenBucket с указанными параметрами по умолчанию
func NewTokenBucket(defaultRate float64, defaultBurst int) *TokenBucket {
	return &TokenBucket{
		defaultRate:  defaultRate,
		defaultBurst: defaultBurst,
	}
}

// Allow проверяет, можно ли пропустить запрос для указанного пользователя
func (tb *TokenBucket) Allow(userID string) bool {
	limiter := tb.getLimiter(userID)
	return limiter.Allow()
}

// SetUserLimits устанавливает лимиты для конкретного пользователя
func (tb *TokenBucket) SetUserLimits(userID string, myrate float64, burst int) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	// Сохраняем настройки
	tb.userLimits.Store(userID, &UserLimits{
		Rate:  myrate,
		Burst: burst,
	})

	// Создаем новый лимитер с указанными параметрами
	limiter := rate.NewLimiter(rate.Limit(myrate), burst)
	tb.limiters.Store(userID, limiter)
}

// GetUserLimits возвращает текущие лимиты пользователя
func (tb *TokenBucket) GetUserLimits(userID string) *UserLimits {
	if limits, ok := tb.userLimits.Load(userID); ok {
		return limits.(*UserLimits)
	}
	return &UserLimits{
		Rate:  tb.defaultRate,
		Burst: tb.defaultBurst,
	}
}

// DeleteUserLimits удаляет пользовательские лимиты
func (tb *TokenBucket) DeleteUserLimits(userID string) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.userLimits.Delete(userID)
	tb.limiters.Delete(userID)
}

// UpdateUserLimits обновляет лимиты пользователя
func (tb *TokenBucket) UpdateUserLimits(userID string, updateFn func(*UserLimits)) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	limits := tb.GetUserLimits(userID)
	updateFn(limits)

	// Сохраняем обновленные настройки
	tb.userLimits.Store(userID, limits)

	// Обновляем лимитер
	limiter := rate.NewLimiter(rate.Limit(limits.Rate), limits.Burst)
	tb.limiters.Store(userID, limiter)
}

// Wait ожидает, пока не появится доступный токен
func (tb *TokenBucket) Wait(userID string) time.Duration {
	limiter := tb.getLimiter(userID)
	now := time.Now()
	limiter.Wait(nil) // Используем nil контекст для простоты
	return time.Since(now)
}

// Reserve резервирует токен и возвращает время до его доступности
func (tb *TokenBucket) Reserve(userID string) time.Duration {
	limiter := tb.getLimiter(userID)
	return limiter.Reserve().Delay()
}

// getLimiter возвращает или создает лимитер для пользователя
func (tb *TokenBucket) getLimiter(userID string) *rate.Limiter {
	// Пытаемся получить существующий лимитер
	if limiter, ok := tb.limiters.Load(userID); ok {
		return limiter.(*rate.Limiter)
	}

	// Получаем настройки пользователя или используем дефолтные
	limits := tb.GetUserLimits(userID)

	// Создаем новый лимитер
	limiter := rate.NewLimiter(rate.Limit(limits.Rate), limits.Burst)
	tb.limiters.Store(userID, limiter)

	return limiter
}

// GetTokens возвращает текущее количество доступных токенов
func (tb *TokenBucket) GetTokens(userID string) float64 {
	limiter := tb.getLimiter(userID)
	return float64(limiter.Tokens())
}

// GetBurst возвращает максимальный размер корзины для пользователя
func (tb *TokenBucket) GetBurst(userID string) int {
	limits := tb.GetUserLimits(userID)
	return limits.Burst
}

// GetRate возвращает текущую скорость пополнения токенов для пользователя
func (tb *TokenBucket) GetRate(userID string) float64 {
	limits := tb.GetUserLimits(userID)
	return limits.Rate
}
