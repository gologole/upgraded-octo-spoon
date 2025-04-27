package request

import (
	"net"
	"net/http"
	"time"
)

// Request представляет интерфейс для работы с запросом
type Request interface {
	// GetUserID возвращает идентификатор пользователя (IPv4)
	GetUserID() string

	// GetResponseTime возвращает время ответа
	GetResponseTime() time.Duration

	// GetOriginalRequest возвращает оригинальный http.Request
	GetOriginalRequest() *http.Request

	// SetResponseTime устанавливает время ответа
	SetResponseTime(duration time.Duration)
}

// Wrapper интерфейс для обертки HTTP-обработчиков
type Wrapper interface {
	// Wrap оборачивает http.Handler для работы с Request
	Wrap(handler http.Handler) http.Handler
}

// ResponseWriter расширяет стандартный http.ResponseWriter
type ResponseWriter interface {
	http.ResponseWriter
	// GetStatus возвращает статус ответа
	GetStatus() int
	// GetRequest возвращает связанный Request
	GetRequest() Request
}

// BaseRequest базовая реализация запроса
type BaseRequest struct {
	originalRequest *http.Request
	responseTime    time.Duration
	userID          string
}

// NewRequest создает новый запрос
func NewRequest(req *http.Request) *BaseRequest {
	return &BaseRequest{
		originalRequest: req,
		userID:          extractUserID(req),
	}
}

func (r *BaseRequest) GetUserID() string {
	return r.userID
}

func (r *BaseRequest) GetResponseTime() time.Duration {
	return r.responseTime
}

func (r *BaseRequest) GetOriginalRequest() *http.Request {
	return r.originalRequest
}

func (r *BaseRequest) SetResponseTime(duration time.Duration) {
	r.responseTime = duration
}

// extractUserID извлекает IPv4 адрес из запроса
func extractUserID(req *http.Request) string {
	// Сначала проверяем X-Forwarded-For
	forwardedFor := req.Header.Get("X-Forwarded-For")
	if forwardedFor != "" {
		// Берем первый IP из списка
		ips := net.ParseIP(forwardedFor)
		if ips != nil && ips.To4() != nil {
			return ips.String()
		}
	}

	// Затем проверяем X-Real-IP
	realIP := req.Header.Get("X-Real-IP")
	if realIP != "" {
		ip := net.ParseIP(realIP)
		if ip != nil && ip.To4() != nil {
			return ip.String()
		}
	}

	// В последнюю очередь берем RemoteAddr
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err == nil {
		ip := net.ParseIP(host)
		if ip != nil && ip.To4() != nil {
			return ip.String()
		}
	}

	return req.RemoteAddr
}

// RequestWrapper обертка для http.Handler, которая создает Request
type RequestWrapper struct {
	handler http.Handler
}

// NewRequestWrapper создает новый wrapper для http.Handler
func NewRequestWrapper(handler http.Handler) *RequestWrapper {
	return &RequestWrapper{
		handler: handler,
	}
}

func (w *RequestWrapper) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	request := NewRequest(req)
	start := time.Now()

	// Оборачиваем ResponseWriter для перехвата статуса ответа
	wrapper := &responseWriterWrapper{
		ResponseWriter: resp,
		request:        request,
	}

	// Вызываем оригинальный handler
	w.handler.ServeHTTP(wrapper, req)

	// Устанавливаем время ответа
	request.SetResponseTime(time.Since(start))
}

// responseWriterWrapper для перехвата ответа
type responseWriterWrapper struct {
	http.ResponseWriter
	request *BaseRequest
	status  int
}

func (w *responseWriterWrapper) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
