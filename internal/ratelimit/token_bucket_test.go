package ratelimit

import (
	"testing"
	"time"
)

func TestTokenBucket_Allow(t *testing.T) {
	tb := NewTokenBucket(10, 1) // 10 rps, burst 1

	// Первый запрос должен пройти
	if !tb.Allow("user1") {
		t.Error("первый запрос должен быть разрешен")
	}

	// Второй запрос должен быть отклонен (burst = 1)
	if tb.Allow("user1") {
		t.Error("второй запрос должен быть отклонен")
	}

	// Ждем пополнения токенов
	time.Sleep(time.Second / 10) // Ждем 100ms для одного токена

	// Теперь запрос должен пройти
	if !tb.Allow("user1") {
		t.Error("запрос должен быть разрешен после ожидания")
	}
}

func TestTokenBucket_UserLimits(t *testing.T) {
	tb := NewTokenBucket(10, 1) // Дефолтные настройки: 10 rps, burst 1

	// Устанавливаем пользовательские лимиты
	tb.SetUserLimits("user2", 2, 2) // 2 rps, burst 2

	// Проверяем получение лимитов
	limits := tb.GetUserLimits("user2")
	if limits.Rate != 2 || limits.Burst != 2 {
		t.Errorf("неверные лимиты: got rate=%v burst=%v, want rate=2 burst=2", limits.Rate, limits.Burst)
	}

	// Проверяем работу с пользовательскими лимитами
	if !tb.Allow("user2") {
		t.Error("первый запрос должен быть разрешен")
	}
	if !tb.Allow("user2") {
		t.Error("второй запрос должен быть разрешен (burst=2)")
	}
	if tb.Allow("user2") {
		t.Error("третий запрос должен быть отклонен")
	}

	// Удаляем пользовательские лимиты
	tb.DeleteUserLimits("user2")

	// Проверяем возврат к дефолтным настройкам
	limits = tb.GetUserLimits("user2")
	if limits.Rate != 10 || limits.Burst != 1 {
		t.Errorf("неверные дефолтные лимиты: got rate=%v burst=%v, want rate=10 burst=1", limits.Rate, limits.Burst)
	}
}

func TestTokenBucket_UpdateLimits(t *testing.T) {
	tb := NewTokenBucket(10, 1)

	// Устанавливаем начальные лимиты
	tb.SetUserLimits("user3", 5, 1)

	// Обновляем только rate
	tb.UpdateUserLimits("user3", func(limits *UserLimits) {
		limits.Rate = 15
	})

	limits := tb.GetUserLimits("user3")
	if limits.Rate != 15 || limits.Burst != 1 {
		t.Errorf("неверные лимиты после обновления: got rate=%v burst=%v, want rate=15 burst=1", limits.Rate, limits.Burst)
	}
}

func TestTokenBucket_Concurrent(t *testing.T) {
	tb := NewTokenBucket(100, 10)
	done := make(chan bool)

	// Запускаем несколько горутин для конкурентного доступа
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				tb.Allow("user4")
				tb.GetUserLimits("user4")
				tb.SetUserLimits("user4", float64(j%10+1), (j%5)+1)
			}
			done <- true
		}()
	}

	// Ждем завершения всех горутин
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestTokenBucket_Reserve(t *testing.T) {
	tb := NewTokenBucket(2, 1) // 2 rps, burst 1

	// Первый запрос должен быть доступен немедленно
	delay := tb.Reserve("user5")
	if delay > time.Millisecond {
		t.Errorf("первый запрос должен быть доступен немедленно, got delay=%v", delay)
	}

	// Второй запрос должен ждать примерно 500ms
	delay = tb.Reserve("user5")
	expectedDelay := time.Second / 2 // 500ms для rate=2
	if delay < time.Duration(float64(expectedDelay)*0.9) || delay > time.Duration(float64(expectedDelay)*1.1) {
		t.Errorf("неверная задержка для второго запроса: got=%v, want=%v±10%%", delay, expectedDelay)
	}
}
