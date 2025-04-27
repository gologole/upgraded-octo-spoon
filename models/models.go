package models


// UserRateLimit представляет настройки rate limit для пользователя
type UserRateLimit struct {
	Rate  float64 `json:"rate"`  // Запросов в секунду
	Burst int     `json:"burst"` // Максимальный размер корзины
}