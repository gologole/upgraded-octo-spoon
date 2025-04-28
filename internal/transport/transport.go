package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cloud.ru_test/pkg/logger"
	"cloud.ru_test/pkg/request"

	"cloud.ru_test/internal/loadbalancer"
	"cloud.ru_test/internal/ratelimit"
)

// UserRateLimit представляет настройки rate limit для пользователя
type UserRateLimit struct {
	Rate  float64 `json:"rate"`  // Запросов в секунду
	Burst int     `json:"burst"` // Максимальный размер корзины
}

type Server interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

type Proxy struct {
	loadbalancer loadbalancer.LoadBalancer
	ratelimit    ratelimit.RateLimiter
	server       *http.Server
	logger       *logger.CustomZapLogger
}

func NewProxy(lb loadbalancer.LoadBalancer, limiter ratelimit.RateLimiter, appLogger *logger.CustomZapLogger) *Proxy {
	p := &Proxy{
		loadbalancer: lb,
		ratelimit:    limiter,
		logger:       appLogger,
	}

	// Создаем HTTP сервер
	mux := http.NewServeMux()

	// Основной прокси хендлер
	mux.HandleFunc("/", p.handleRequest)

	mux.HandleFunc("/ratelimit/", p.handleRateLimit)

	p.server = &http.Server{
		Handler: mux,
	}

	return p
}

func (p *Proxy) Start(port string) error {
	p.logger.Debug(fmt.Sprintf("Запуск прокси-сервера на порту %s", port))

	// Добавляем настройки для быстрого освобождения порта
	p.server.Addr = port
	p.server.SetKeepAlivesEnabled(false) // Отключаем keep-alive для быстрого закрытия соединений

	// Запускаем сервер в отдельной горутине
	go func() {
		if err := p.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			p.logger.Error(fmt.Sprintf("Ошибка запуска сервера: %v", err))
		}
	}()

	// Даем серверу время на запуск
	time.Sleep(100 * time.Millisecond)

	return nil
}

func (p *Proxy) Stop() error {
	p.logger.Debug("Начало graceful shutdown прокси-сервера")

	// Создаем контекст с таймаутом для graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Перестаем принимать новые соединения и ждем завершения текущих
	if err := p.server.Shutdown(ctx); err != nil {
		p.logger.Error(fmt.Sprintf("Ошибка при graceful shutdown: %v", err))
		// Если не удалось graceful shutdown, закрываем принудительно
		if err := p.server.Close(); err != nil {
			p.logger.Error(fmt.Sprintf("Ошибка при принудительном закрытии: %v", err))
		}
		return err
	}

	p.logger.Debug("Прокси-сервер успешно остановлен")
	return nil
}

// handleRequest обрабатывает входящие HTTP запросы к бэкендам
func (p *Proxy) handleRequest(w http.ResponseWriter, r *http.Request) {
	p.logger.Debug(fmt.Sprintf("Получен новый запрос: %s %s от %s", r.Method, r.URL.Path, r.RemoteAddr))

	// проверяем даст ли токен
	if !p.ratelimit.Allow(r.RemoteAddr) {
		p.logger.Debug(fmt.Sprintf("Превышен rate limit для %s", r.RemoteAddr))
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}
	p.logger.Debug(fmt.Sprintf("Rate limit проверка пройдена для %s", r.RemoteAddr))

	customReq := request.NewRequest(r)
	p.logger.Debug(fmt.Sprintf("Создан кастомный запрос для пользователя %s", customReq.GetUserID()))

	backend := p.loadbalancer.Invoke(customReq)
	if backend == nil {
		p.logger.Debug("Не найдено доступных бэкендов")
		http.Error(w, "No available backends", http.StatusServiceUnavailable)
		return
	}
	p.logger.Debug(fmt.Sprintf("Выбран бэкенд %s для запроса", backend.ID()))

	// Создаем URL для запроса к бэкенду
	backendURL := backend.URL() + r.URL.Path
	if r.URL.RawQuery != "" {
		backendURL += "?" + r.URL.RawQuery
	}
	p.logger.Debug(fmt.Sprintf("Проксирование запроса к %s", backendURL))

	outReq, err := http.NewRequestWithContext(r.Context(), r.Method, backendURL, r.Body)
	if err != nil {
		p.logger.Error(fmt.Sprintf("Ошибка создания запроса к бэкенду: %v", err))
		http.Error(w, "Ошибка создания запроса к бэкенду", http.StatusInternalServerError)
		return
	}

	// Копируем заголовки из оригинального запроса
	outReq.Header = r.Header.Clone()
	p.logger.Debug("Заголовки запроса скопированы")

	// Добавляем заголовки прокси
	outReq.Header.Set("X-Forwarded-For", r.RemoteAddr)
	outReq.Header.Set("X-Proxy-ID", "cloud-ru-proxy")
	outReq.Header.Set("X-Real-IP", r.RemoteAddr)
	p.logger.Debug("Добавлены прокси-заголовки")

	// Отправляем запрос на бэкенд
	start := time.Now()
	resp, err := backend.Handle(r.Context(), outReq)
	duration := time.Since(start)

	if err != nil {
		p.logger.Debug(fmt.Sprintf("Ошибка при запросе к бэкенду %s: %v, URL: %s", backend.ID(), err, backendURL))
		http.Error(w, fmt.Sprintf("Backend error: %v", err), http.StatusBadGateway)
		return
	}
	p.logger.Debug(fmt.Sprintf("Получен ответ от бэкенда %s за %v, статус: %d", backend.ID(), duration, resp.StatusCode))
	defer resp.Body.Close()

	// Копируем заголовки ответа
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	p.logger.Debug("Заголовки ответа скопированы")

	// Устанавливаем статус ответа
	w.WriteHeader(resp.StatusCode)

	// Копируем тело ответа
	written, err := io.Copy(w, resp.Body)
	if err != nil {
		p.logger.Error(fmt.Sprintf("Error copying response body: %v\n", err))
	} else {
		p.logger.Debug(fmt.Sprintf("Тело ответа успешно отправлено клиенту, размер: %d байт", written))
	}
}

// handleRateLimit обрабатывает CRUD операции для rate limit пользователей
func (p *Proxy) handleRateLimit(w http.ResponseWriter, r *http.Request) {
	p.logger.Debug(fmt.Sprintf("Получен запрос к API rate limit: %s %s", r.Method, r.URL.Path))

	// Извлекаем userID из URL
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		p.logger.Debug("Некорректный формат URL для rate limit API")
		http.Error(w, "Invalid URL format. Use /ratelimit/{userID}", http.StatusBadRequest)
		return
	}
	userID := parts[2]
	p.logger.Debug(fmt.Sprintf("Обработка rate limit для пользователя: %s", userID))

	switch r.Method {
	case http.MethodGet:
		p.getRateLimit(w, userID)
	case http.MethodPost:
		p.createRateLimit(w, r, userID)
	case http.MethodPut:
		p.updateRateLimit(w, r, userID)
	case http.MethodDelete:
		p.deleteRateLimit(w, userID)
	default:
		p.logger.Debug(fmt.Sprintf("Неподдерживаемый метод %s для rate limit API", r.Method))
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getRateLimit возвращает текущие настройки rate limit для пользователя
func (p *Proxy) getRateLimit(w http.ResponseWriter, userID string) {
	p.logger.Debug(fmt.Sprintf("Получение настроек rate limit для пользователя %s", userID))

	limits := p.ratelimit.GetUserLimits(userID)
	if limits == nil {
		p.logger.Debug(fmt.Sprintf("Настройки rate limit не найдены для пользователя %s", userID))
		http.Error(w, "User limits not found", http.StatusNotFound)
		return
	}
	p.logger.Debug(fmt.Sprintf("Найдены настройки rate limit для %s: rate=%.2f, burst=%d", userID, limits.Rate, limits.Burst))

	response := UserRateLimit{
		Rate:  limits.Rate,
		Burst: limits.Burst,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		p.logger.Error(fmt.Sprintf("Failed to encode response", "error", err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	} else {
		p.logger.Debug(fmt.Sprintf("Успешно отправлены настройки rate limit для %s", userID))
	}
}

// createRateLimit создает новые настройки rate limit для пользователя
func (p *Proxy) createRateLimit(w http.ResponseWriter, r *http.Request, userID string) {
	p.logger.Debug(fmt.Sprintf("Создание новых настроек rate limit для пользователя %s", userID))

	var limits UserRateLimit
	if err := json.NewDecoder(r.Body).Decode(&limits); err != nil {
		p.logger.Debug(fmt.Sprintf("Ошибка декодирования тела запроса: %v", err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Валидация
	if limits.Rate <= 0 || limits.Burst <= 0 {
		p.logger.Debug(fmt.Sprintf("Некорректные значения rate/burst: rate=%.2f, burst=%d", limits.Rate, limits.Burst))
		http.Error(w, "Rate and burst must be positive", http.StatusBadRequest)
		return
	}

	// Проверяем, существуют ли уже лимиты
	if existing := p.ratelimit.GetUserLimits(userID); existing != nil {
		p.logger.Debug(fmt.Sprintf("Rate limit уже существует для пользователя %s", userID))
		http.Error(w, "Rate limits already exist for this user", http.StatusConflict)
		return
	}

	p.ratelimit.SetUserLimits(userID, limits.Rate, limits.Burst)
	p.logger.Debug(fmt.Sprintf("Успешно созданы настройки rate limit для %s: rate=%.2f, burst=%d", userID, limits.Rate, limits.Burst))

	w.WriteHeader(http.StatusCreated)
}

// updateRateLimit обновляет настройки rate limit для пользователя
func (p *Proxy) updateRateLimit(w http.ResponseWriter, r *http.Request, userID string) {
	p.logger.Debug(fmt.Sprintf("Обновление настроек rate limit для пользователя %s", userID))

	var limits UserRateLimit
	if err := json.NewDecoder(r.Body).Decode(&limits); err != nil {
		p.logger.Debug(fmt.Sprintf("Ошибка декодирования тела запроса: %v", err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Валидация
	if limits.Rate <= 0 || limits.Burst <= 0 {
		p.logger.Debug(fmt.Sprintf("Некорректные значения rate/burst: rate=%.2f, burst=%d", limits.Rate, limits.Burst))
		http.Error(w, "Rate and burst must be positive", http.StatusBadRequest)
		return
	}

	// Проверяем существование пользователя
	if existing := p.ratelimit.GetUserLimits(userID); existing == nil {
		p.logger.Debug(fmt.Sprintf("Настройки rate limit не найдены для пользователя %s", userID))
		http.Error(w, "User limits not found", http.StatusNotFound)
		return
	}

	p.ratelimit.UpdateUserLimits(userID, func(ul *ratelimit.UserLimits) {
		ul.Rate = limits.Rate
		ul.Burst = limits.Burst
	})
	p.logger.Debug(fmt.Sprintf("Успешно обновлены настройки rate limit для %s: rate=%.2f, burst=%d", userID, limits.Rate, limits.Burst))

	w.WriteHeader(http.StatusOK)
}

// deleteRateLimit удаляет настройки rate limit для пользователя
func (p *Proxy) deleteRateLimit(w http.ResponseWriter, userID string) {
	p.logger.Debug(fmt.Sprintf("Удаление настроек rate limit для пользователя %s", userID))

	// Проверяем существование пользователя
	if existing := p.ratelimit.GetUserLimits(userID); existing == nil {
		p.logger.Debug(fmt.Sprintf("Настройки rate limit не найдены для пользователя %s", userID))
		http.Error(w, "User limits not found", http.StatusNotFound)
		return
	}

	p.ratelimit.DeleteUserLimits(userID)
	p.logger.Debug(fmt.Sprintf("Успешно удалены настройки rate limit для пользователя %s", userID))

	w.WriteHeader(http.StatusNoContent)
}
