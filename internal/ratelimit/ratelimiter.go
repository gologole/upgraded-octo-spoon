package ratelimit

import "time"

// RateLimiter определяет интерфейс для ограничения запросов
type RateLimiter interface {
	// Allow проверяет, можно ли пропустить запрос
	Allow(userID string) bool

	// Wait ожидает, пока не появится доступный токен
	Wait(userID string) time.Duration

	// Reserve резервирует токен и возвращает время до его доступности
	Reserve(userID string) time.Duration

	// GetTokens возвращает текущее количество доступных токенов
	GetTokens(userID string) float64

	// GetBurst возвращает максимальный размер корзины
	GetBurst(userID string) int

	// GetRate возвращает текущую скорость пополнения токенов
	GetRate(userID string) float64

	// SetUserLimits устанавливает лимиты для пользователя
	SetUserLimits(userID string, rate float64, burst int)

	// GetUserLimits возвращает текущие лимиты пользователя
	GetUserLimits(userID string) *UserLimits

	// DeleteUserLimits удаляет пользовательские лимиты
	DeleteUserLimits(userID string)

	// UpdateUserLimits обновляет лимиты пользователя
	UpdateUserLimits(userID string, updateFn func(*UserLimits))
}
