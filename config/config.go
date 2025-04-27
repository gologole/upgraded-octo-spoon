package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config основная конфигурация приложения
type Config struct {
	// Стратегия балансировки нагрузки
	LoadBalancer LoadBalancerConfig `yaml:"loadBalancer"`

	// Список бэкендов
	Backends []BackendConfig `yaml:"backends"`

	// Настройки rate limiter
	RateLimiter *RateLimiterConfig `yaml:"rateLimiter,omitempty"`

	// Настройки логгера
	Logger *LoggerConfig `yaml:"logger"`
}

// LoadBalancerConfig конфигурация балансировщика
type LoadBalancerConfig struct {
	// Метод балансировки: RoundRobin, WeightedRoundRobin, LeastConnections
	Method string `yaml:"method"`

	// Дополнительные параметры метода балансировки
	Params map[string]interface{} `yaml:"params,omitempty"`
}

// BackendConfig конфигурация бэкенда
type BackendConfig struct {
	// ID бэкенда
	ID string `yaml:"id"`

	// URL бэкенда
	URL string `yaml:"url"`

	// Вес бэкенда (для weighted методов)
	Weight *float64 `yaml:"weight,omitempty"`

	// Таймаут подключения
	ConnectTimeout time.Duration `yaml:"connectTimeout"`

	// Таймаут чтения
	ReadTimeout time.Duration `yaml:"readTimeout"`

	// Максимальное количество соединений
	MaxConnections int `yaml:"maxConnections"`
}

// RateLimiterConfig конфигурация rate limiter
type RateLimiterConfig struct {
	// Включен ли rate limiter
	Enabled bool `yaml:"enabled"`

	// Тип rate limiter (пока поддерживается только TokenBucket)
	Type string `yaml:"type"`

	// Настройки для token bucket
	TokenBucket *TokenBucketConfig `yaml:"tokenBucket,omitempty"`
}

// TokenBucketConfig настройки для token bucket
type TokenBucketConfig struct {
	// Количество запросов в секунду по умолчанию
	Rate float64 `yaml:"rate"`

	// Максимальный размер корзины
	Burst int `yaml:"burst"`
}

// RuleConfig правило rate limiting для конкретного пользователя/IP
type RuleConfig struct {
	// Количество запросов в секунду
	Rate float64 `yaml:"rate"`

	// Максимальный размер корзины
	Burst int `yaml:"burst"`
}

// LoggerConfig конфигурация логгера
type LoggerConfig struct {
	// Уровень логирования: debug, info, warn, error, fatal
	LogLevel string `yaml:"logLevel"`

	// IP узла
	NodeIP string `yaml:"nodeIP"`

	// IP пода (для Kubernetes)
	PodIP string `yaml:"podIP"`

	// Имя сервиса
	ServiceName string `yaml:"serviceName"`
}

// LoadFromFile загружает конфигурацию из YAML файла
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

// validate проверяет корректность конфигурации
func (c *Config) validate() error {
	// Проверяем метод балансировки
	switch c.LoadBalancer.Method {
	case "RoundRobin", "WeightedRoundRobin", "LeastConnections":
		// OK
	default:
		return fmt.Errorf("unsupported load balancing method: %s", c.LoadBalancer.Method)
	}

	// Проверяем наличие бэкендов
	if len(c.Backends) == 0 {
		return fmt.Errorf("no backends configured")
	}

	// Проверяем конфигурацию бэкендов
	for _, b := range c.Backends {
		if b.ID == "" {
			return fmt.Errorf("backend ID is required")
		}
		if b.URL == "" {
			return fmt.Errorf("backend URL is required")
		}
		if b.Weight != nil && *b.Weight <= 0 {
			return fmt.Errorf("backend weight must be positive")
		}
	}

	// Проверяем rate limiter
	if c.RateLimiter != nil && c.RateLimiter.Enabled {
		if c.RateLimiter.Type != "TokenBucket" {
			return fmt.Errorf("unsupported rate limiter type: %s", c.RateLimiter.Type)
		}
		if c.RateLimiter.TokenBucket == nil {
			return fmt.Errorf("token bucket configuration is required")
		}
		if c.RateLimiter.TokenBucket.Rate <= 0 {
			return fmt.Errorf("token bucket rate must be positive")
		}
		if c.RateLimiter.TokenBucket.Burst <= 0 {
			return fmt.Errorf("token bucket burst must be positive")
		}
	}

	// Проверяем конфигурацию логгера
	if c.Logger == nil {
		return fmt.Errorf("logger configuration is required")
	}

	switch c.Logger.LogLevel {
	case "debug", "info", "warn", "error", "fatal":
		// OK
	default:
		return fmt.Errorf("unsupported log level: %s", c.Logger.LogLevel)
	}

	if c.Logger.ServiceName == "" {
		return fmt.Errorf("logger service name is required")
	}

	return nil
}
