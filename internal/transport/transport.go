package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

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
	p.server.Addr = port
	return p.server.ListenAndServe()
}

func (p *Proxy) Stop() error {
	return p.server.Shutdown(context.Background())
}

// handleRequest обрабатывает входящие HTTP запросы к бэкендам
func (p *Proxy) handleRequest(w http.ResponseWriter, r *http.Request) {
	// проверяем даст ли токен
	if !p.ratelimit.Allow(r.RemoteAddr) {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	customReq := request.NewRequest(r) //тут создаю свой кастомный интерфейс запроса)))

	backend := p.loadbalancer.Invoke(customReq)
	if backend == nil {
		http.Error(w, "No available backends", http.StatusServiceUnavailable)
		return
	}

	// Создаем запрос к бэкенду
	outReq := r.Clone(r.Context())
	outReq.RequestURI = ""
	outReq.URL.Host = backend.ID()
	outReq.URL.Scheme = "http"
	outReq.Host = backend.ID()

	// Отправляем запрос на бэкенд
	resp, err := backend.Handle(r.Context(), outReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Backend error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Копируем заголовки ответа
	for k, v := range resp.Header {
		w.Header()[k] = v
	}

	// Устанавливаем статус ответа
	w.WriteHeader(resp.StatusCode)

	// Копируем тело ответа
	if _, err := io.Copy(w, resp.Body); err != nil {
		// Логируем ошибку, но не отправляем клиенту, так как заголовки уже отправлены
		p.logger.Error(fmt.Sprintf("Error copying response body: %v\n", err))
	}
}

// handleRateLimit обрабатывает CRUD операции для rate limit пользователей
func (p *Proxy) handleRateLimit(w http.ResponseWriter, r *http.Request) {
	// Извлекаем userID из URL
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid URL format. Use /ratelimit/{userID}", http.StatusBadRequest)
		return
	}
	userID := parts[2]

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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getRateLimit возвращает текущие настройки rate limit для пользователя
func (p *Proxy) getRateLimit(w http.ResponseWriter, userID string) {
	limits := p.ratelimit.GetUserLimits(userID)
	if limits == nil {
		http.Error(w, "User limits not found", http.StatusNotFound)
		return
	}

	response := UserRateLimit{
		Rate:  limits.Rate,
		Burst: limits.Burst,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		p.logger.Error(fmt.Sprintf("Failed to encode response", "error", err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// createRateLimit создает новые настройки rate limit для пользователя
func (p *Proxy) createRateLimit(w http.ResponseWriter, r *http.Request, userID string) {
	var limits UserRateLimit
	if err := json.NewDecoder(r.Body).Decode(&limits); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Валидация
	if limits.Rate <= 0 || limits.Burst <= 0 {
		http.Error(w, "Rate and burst must be positive", http.StatusBadRequest)
		return
	}

	// Проверяем, существуют ли уже лимиты
	if existing := p.ratelimit.GetUserLimits(userID); existing != nil {
		http.Error(w, "Rate limits already exist for this user", http.StatusConflict)
		return
	}

	p.ratelimit.SetUserLimits(userID, limits.Rate, limits.Burst)

	w.WriteHeader(http.StatusCreated)
	p.logger.Info(fmt.Sprintf("Created rate limits for user", "userID", userID, "rate", limits.Rate, "burst", limits.Burst))
}

// updateRateLimit обновляет настройки rate limit для пользователя
func (p *Proxy) updateRateLimit(w http.ResponseWriter, r *http.Request, userID string) {
	var limits UserRateLimit
	if err := json.NewDecoder(r.Body).Decode(&limits); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Валидация
	if limits.Rate <= 0 || limits.Burst <= 0 {
		http.Error(w, "Rate and burst must be positive", http.StatusBadRequest)
		return
	}

	// Проверяем существование пользователя
	if existing := p.ratelimit.GetUserLimits(userID); existing == nil {
		http.Error(w, "User limits not found", http.StatusNotFound)
		return
	}

	p.ratelimit.UpdateUserLimits(userID, func(ul *ratelimit.UserLimits) {
		ul.Rate = limits.Rate
		ul.Burst = limits.Burst
	})

	w.WriteHeader(http.StatusOK)
	p.logger.Info(fmt.Sprintf("Updated rate limits for user", "userID", userID, "rate", limits.Rate, "burst", limits.Burst))
}

// deleteRateLimit удаляет настройки rate limit для пользователя
func (p *Proxy) deleteRateLimit(w http.ResponseWriter, userID string) {
	// Проверяем существование пользователя
	if existing := p.ratelimit.GetUserLimits(userID); existing == nil {
		http.Error(w, "User limits not found", http.StatusNotFound)
		return
	}

	p.ratelimit.DeleteUserLimits(userID)

	w.WriteHeader(http.StatusNoContent)
	p.logger.Info(fmt.Sprintf("Deleted rate limits for user", "userID", userID))
}
